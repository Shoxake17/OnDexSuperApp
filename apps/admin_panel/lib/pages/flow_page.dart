import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api.dart';

/// OnDexFlow API (Plane ONDEXECOSY-472). O'zgartiruvchi amallar `guarded`
/// orqali — server sabab va Authenticator kodini so'raydi (CC-0.4).
extension FlowApi on AdminApi {
  Future<List<Map<String, dynamic>>> flowApps() async =>
      _list((await send('GET', '/admin/flow/apps') as Map)['apps']);

  Future<Map<String, dynamic>> flowCreateApp(String name, String publicKey, {String id = ''}) async =>
      Map<String, dynamic>.from(await guarded('POST', '/admin/flow/apps',
          {'id': id, 'name': name, 'platform': 'android', 'public_key': publicKey}) as Map);

  Future<List<Map<String, dynamic>>> flowReleases(String appId) async =>
      _list((await send('GET', '/admin/flow/apps/$appId/releases') as Map)['releases']);

  Future<List<Map<String, dynamic>>> flowPatches(String releaseId) async =>
      _list((await send('GET', '/admin/flow/releases/$releaseId/patches') as Map)['patches']);

  Future<void> flowSetState(String patchId, String status, int rollout, int rev) =>
      guarded('POST', '/admin/flow/patches/$patchId/state',
          {'status': status, 'rollout_percent': rollout, 'rev': rev});

  static List<Map<String, dynamic>> _list(Object? v) =>
      [for (final x in (v as List? ?? const [])) Map<String, dynamic>.from(x as Map)];
}

const _statusTitle = <String, String>{
  'draft': 'Qoralama',
  'active': 'Faol',
  'paused': 'Pauza',
  'rolled_back': 'Qaytarilgan',
};

Color _statusColor(String s) => switch (s) {
      'active' => Colors.green.shade700,
      'paused' => Colors.orange.shade800,
      'rolled_back' => Colors.red.shade700,
      _ => Colors.grey.shade700,
    };

/// "OnDexFlow" — o'z OTA yangilanish tizimini boshqarish (FAQAT shu yerda).
///
/// Ilovalar -> relizlar -> yamoqlar. Yamoq vosita (`cmd/ondexflow`) bilan
/// QORALAMA bo'lib keladi; foydalanuvchilarga chiqarish, foizni oshirish,
/// pauza va orqaga qaytarish — shu sahifadan, har biri sabab + Authenticator
/// kodi bilan va auditga yoziladi.
class FlowPage extends StatefulWidget {
  const FlowPage({super.key});

  @override
  State<FlowPage> createState() => _FlowPageState();
}

class _FlowPageState extends State<FlowPage> {
  List<Map<String, dynamic>> _apps = [];
  String? _appId;
  List<Map<String, dynamic>> _releases = [];
  String? _releaseId;
  List<Map<String, dynamic>> _patches = [];
  String? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _loadApps();
  }

  Future<void> _run(Future<void> Function() f) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await f();
    } on ApiException catch (e) {
      _error = e.statusCode == 404 && _apps.isEmpty ? 'Server yangilanmagan (OnDexFlow yo\'q)' : e.message;
    } catch (e) {
      _error = '$e';
    }
    if (mounted) setState(() => _loading = false);
  }

  Future<void> _loadApps() => _run(() async {
        _apps = await api.flowApps();
        _appId ??= _apps.isNotEmpty ? _apps.first['id'] as String : null;
        await _loadReleasesInner();
      });

  Future<void> _loadReleasesInner() async {
    _releases = _appId == null ? [] : await api.flowReleases(_appId!);
    if (_releaseId == null || !_releases.any((r) => r['id'] == _releaseId)) {
      _releaseId = _releases.isNotEmpty ? _releases.first['id'] as String : null;
    }
    await _loadPatchesInner();
  }

  Future<void> _loadPatchesInner() async {
    _patches = _releaseId == null ? [] : await api.flowPatches(_releaseId!);
  }

  // ── Ilova qo'shish ──────────────────────────────────────────────────
  Future<void> _addApp() async {
    final name = TextEditingController();
    final key = TextEditingController();
    final appId = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (c) => AlertDialog(
        title: const Text('Ilovani OnDexFlow\'ga ulash'),
        content: SizedBox(
          width: 520,
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            const Text(
              'Ochiq kalitni kompyuteringizda oling:\n'
              'go run ./cmd/ondexflow keygen -name customer_app\n\n'
              'Shaxsiy kalit kompyuteringizda qoladi — bu yerga FAQAT ochiq kalit kiritiladi.',
              style: TextStyle(fontSize: 12.5, color: Colors.black54),
            ),
            const SizedBox(height: 12),
            TextField(
                key: const ValueKey('flow-app-name'),
                controller: name,
                decoration: const InputDecoration(labelText: 'Nomi (masalan OnDex mijoz)', border: OutlineInputBorder())),
            const SizedBox(height: 10),
            TextField(
                key: const ValueKey('flow-app-id'),
                controller: appId,
                decoration: const InputDecoration(
                    labelText: 'app_id (ixtiyoriy — shorebird.yaml dagi mavjud qiymat)', border: OutlineInputBorder())),
            const SizedBox(height: 10),
            TextField(
              key: const ValueKey('flow-app-key'),
              controller: key,
              minLines: 3,
              maxLines: 6,
              decoration: const InputDecoration(labelText: 'Ochiq kalit (base64)', border: OutlineInputBorder()),
            ),
          ]),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(c, false), child: const Text('Bekor')),
          FilledButton(onPressed: () => Navigator.pop(c, true), child: const Text('Ulash')),
        ],
      ),
    );
    if (ok != true) return;
    await _run(() async {
      final a = await api.flowCreateApp(name.text.trim(), key.text.trim(), id: appId.text.trim());
      _appId = a['id'] as String;
      _apps = await api.flowApps();
      await _loadReleasesInner();
      if (mounted) _showAppId(a);
    });
  }

  void _showAppId(Map<String, dynamic> a) {
    showDialog<void>(
      context: context,
      builder: (c) => AlertDialog(
        title: const Text('Ilova ulandi'),
        content: SelectableText(
          'apps/<ilova>/shorebird.yaml ga yozing:\n\n'
          'app_id: ${a['id']}\n'
          'base_url: https://api.ondex.uz/flow\n\n'
          'Kalit ID: ${a['key_id']} (vosita chiqargani bilan bir xil bo\'lishi shart)',
        ),
        actions: [
          TextButton(
            onPressed: () => Clipboard.setData(ClipboardData(text: '${a['id']}')),
            child: const Text('app_id nusxalash'),
          ),
          FilledButton(onPressed: () => Navigator.pop(c), child: const Text('Yopish')),
        ],
      ),
    );
  }

  // ── Yamoq amallari ──────────────────────────────────────────────────
  Future<void> _activate(Map<String, dynamic> p) async {
    final current = (p['rollout_percent'] as num?)?.toInt() ?? 0;
    var value = current > 0 ? current.toDouble() : 5.0;
    final percent = await showDialog<int>(
      context: context,
      builder: (c) => StatefulBuilder(
        builder: (c, set) => AlertDialog(
          title: Text('Yamoq #${p['number']} — necha foiz qurilmaga?'),
          content: SizedBox(
            width: 420,
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              Text('${value.round()} %', style: const TextStyle(fontSize: 28, fontWeight: FontWeight.w800)),
              Slider(
                key: const ValueKey('flow-rollout'),
                value: value,
                min: 1,
                max: 100,
                divisions: 99,
                onChanged: (v) => set(() => value = v),
              ),
              Wrap(spacing: 8, children: [
                for (final q in [1, 5, 25, 50, 100])
                  ActionChip(label: Text('$q%'), onPressed: () => set(() => value = q.toDouble())),
              ]),
              const SizedBox(height: 8),
              const Text(
                'Tavsiya: avval 5%, yiqilishlar yo\'qligini ko\'rib keyin 100%. '
                'Qurilma yamoqni ilova KEYINGI ochilganda qo\'llaydi.',
                style: TextStyle(fontSize: 12.5, color: Colors.black54),
              ),
            ]),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(c), child: const Text('Bekor')),
            FilledButton(onPressed: () => Navigator.pop(c, value.round()), child: const Text('Chiqarish')),
          ],
        ),
      ),
    );
    if (percent == null) return;
    await _run(() async {
      await api.flowSetState(p['id'] as String, 'active', percent, (p['rev'] as num).toInt());
      await _loadPatchesInner();
    });
  }

  Future<void> _simple(Map<String, dynamic> p, String status, String title, String body) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(title),
        content: Text(body),
        actions: [
          TextButton(onPressed: () => Navigator.pop(c, false), child: const Text('Bekor')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: status == 'rolled_back' ? Colors.red : null),
            onPressed: () => Navigator.pop(c, true),
            child: const Text('Tasdiqlash'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    await _run(() async {
      await api.flowSetState(p['id'] as String, status, 0, (p['rev'] as num).toInt());
      await _loadPatchesInner();
    });
  }

  // ── UI ──────────────────────────────────────────────────────────────
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          const Text('OnDexFlow', style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800)),
          const SizedBox(width: 12),
          const Chip(label: Text('Havodan yangilanish (OTA)')),
          const Spacer(),
          if (_loading) const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
          IconButton(tooltip: 'Yangilash', onPressed: _loadApps, icon: const Icon(Icons.refresh)),
          const SizedBox(width: 8),
          FilledButton.icon(
            key: const ValueKey('flow-add-app'),
            onPressed: _addApp,
            icon: const Icon(Icons.add),
            label: const Text('Ilova ulash'),
          ),
        ]),
        if (_error != null)
          Padding(padding: const EdgeInsets.only(top: 8), child: Text(_error!, style: const TextStyle(color: Colors.red))),
        const SizedBox(height: 16),
        Expanded(
          child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            SizedBox(width: 260, child: _column('Ilovalar', [for (final a in _apps) _appTile(a)],
                empty: 'Ilova ulanmagan — "Ilova ulash"')),
            const SizedBox(width: 16),
            SizedBox(width: 260, child: _column('Relizlar', [for (final r in _releases) _releaseTile(r)],
                empty: 'Reliz yo\'q — build_mobile.ps1 -Flow bilan yuboriladi')),
            const SizedBox(width: 16),
            Expanded(child: _column('Yamoqlar', [for (final p in _patches) _patchCard(p)],
                empty: _releaseId == null
                    ? 'Avval relizni tanlang'
                    : 'Bu relizga hali yamoq yo\'q.\n\n'
                        'Relizning o\'zi bu yerda faollashtirilmaydi — u APK sifatida bir marta '
                        'o\'rnatiladi. Keyingi Dart tuzatishlari shu relizga yamoq bo\'lib keladi '
                        '(build_mobile.ps1 -Flow -FlowPatch N) va shu yerda chiqariladi.')),
          ]),
        ),
      ]),
    );
  }

  Widget _column(String title, List<Widget> children, {String empty = 'Hozircha yo\'q'}) => Card(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Text(title, style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 16)),
            const SizedBox(height: 8),
            Expanded(
              child: children.isEmpty
                  ? Center(
                      child: Padding(
                        padding: const EdgeInsets.all(8),
                        child: Text(empty, textAlign: TextAlign.center, style: const TextStyle(color: Colors.black45)),
                      ),
                    )
                  : ListView(children: children),
            ),
          ]),
        ),
      );

  Widget _appTile(Map<String, dynamic> a) => ListTile(
        selected: a['id'] == _appId,
        title: Text('${a['name']}'),
        subtitle: Text('${a['platform']} · kalit ${a['key_id']}', style: const TextStyle(fontSize: 12)),
        onTap: () => _run(() async {
          _appId = a['id'] as String;
          _releaseId = null;
          await _loadReleasesInner();
        }),
        trailing: IconButton(
          tooltip: 'app_id',
          icon: const Icon(Icons.info_outline, size: 18),
          onPressed: () => _showAppId(a),
        ),
      );

  Widget _releaseTile(Map<String, dynamic> r) => ListTile(
        selected: r['id'] == _releaseId,
        title: Text('${r['version']}'),
        subtitle: Text(_date(r['created_at']), style: const TextStyle(fontSize: 12)),
        onTap: () => _run(() async {
          _releaseId = r['id'] as String;
          await _loadPatchesInner();
        }),
      );

  Widget _patchCard(Map<String, dynamic> p) {
    final status = '${p['status']}';
    final rollout = (p['rollout_percent'] as num?)?.toInt() ?? 0;
    final stats = Map<String, dynamic>.from((p['stats'] as Map?) ?? {});
    final installs = (stats['installs'] as num?)?.toInt() ?? 0;
    final failures = (stats['failures'] as num?)?.toInt() ?? 0;
    final failRate = installs + failures == 0 ? 0.0 : failures / (installs + failures);
    return Card(
      key: ValueKey('flow-patch-${p['number']}'),
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Text('Yamoq #${p['number']}', style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w800)),
            const SizedBox(width: 10),
            Chip(
              label: Text(_statusTitle[status] ?? status, style: TextStyle(color: _statusColor(status), fontWeight: FontWeight.w700)),
              backgroundColor: _statusColor(status).withValues(alpha: 0.08),
            ),
            if (status == 'active') ...[const SizedBox(width: 6), Chip(label: Text('$rollout% qurilma'))],
            const Spacer(),
            Text('kanal: ${p['channel']}', style: const TextStyle(color: Colors.black54)),
          ]),
          if ('${p['note']}'.isNotEmpty) Text('${p['note']}'),
          const SizedBox(height: 6),
          Text(
            'Yuklandi: ${stats['downloads'] ?? 0} · O\'rnatildi: $installs · Yiqildi: $failures'
            '   ·   yaratilgan ${_date(p['created_at'])}',
            style: const TextStyle(fontSize: 12.5, color: Colors.black54),
          ),
          if (failures >= 3 && failRate >= 0.2)
            Container(
              margin: const EdgeInsets.only(top: 8),
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(color: Colors.red.shade50, borderRadius: BorderRadius.circular(8)),
              child: Text(
                'Diqqat: yiqilishlar ${(failRate * 100).round()}%. Pauza yoki orqaga qaytarishni ko\'rib chiqing. '
                '(Hisobotlar qurilmalardan keladi va soxtalashtirilishi mumkin — shuning uchun avtomatik to\'xtatilmaydi.)',
                style: TextStyle(color: Colors.red.shade800, fontSize: 12.5),
              ),
            ),
          const SizedBox(height: 10),
          Wrap(spacing: 8, runSpacing: 8, children: [
            if (status == 'draft' || status == 'paused')
              FilledButton.icon(
                key: ValueKey('flow-activate-${p['number']}'),
                onPressed: () => _activate(p),
                icon: const Icon(Icons.rocket_launch_outlined, size: 18),
                label: Text(status == 'draft' ? 'Chiqarish' : 'Davom ettirish'),
              ),
            if (status == 'active') ...[
              OutlinedButton.icon(
                onPressed: () => _activate(p),
                icon: const Icon(Icons.tune, size: 18),
                label: const Text('Foizni o\'zgartirish'),
              ),
              OutlinedButton.icon(
                onPressed: () => _simple(p, 'paused', 'Pauza',
                    'Yangi qurilmalarga berilmaydi. O\'rnatib bo\'lganlarda qoladi.'),
                icon: const Icon(Icons.pause, size: 18),
                label: const Text('Pauza'),
              ),
            ],
            if (status == 'active' || status == 'paused')
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(foregroundColor: Colors.red),
                onPressed: () => _simple(p, 'rolled_back', 'Orqaga qaytarish',
                    'Barcha qurilmalar bu yamoqni O\'CHIRIB, oldingi holatga qaytadi. Qaytarib bo\'lmaydi — '
                    'tuzatish uchun yangi yamoq chiqariladi.'),
                icon: const Icon(Icons.undo, size: 18),
                label: const Text('Orqaga qaytarish'),
              ),
          ]),
        ]),
      ),
    );
  }

  String _date(Object? v) {
    final d = DateTime.tryParse('$v')?.toLocal();
    if (d == null) return '';
    String two(int x) => x.toString().padLeft(2, '0');
    return '${two(d.day)}.${two(d.month)}.${d.year} ${two(d.hour)}:${two(d.minute)}';
  }
}
