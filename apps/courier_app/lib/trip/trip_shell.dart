import 'dart:async';

import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../push.dart';
import '../screens/login_screen.dart';
import '../session.dart';
import '../theme.dart';
import '../widgets/slide_to_confirm.dart';

/// Reys kuryerining asosiy ekrani: do'kon unga biriktirgan buyurtmalar.
///
/// Oqim: 1) do'konga borib qadoqlangan buyurtmalarni oladi ("Oldim"); 2) mijozlarga yetkazadi
/// ("Yo'lga chiqdim" -> surib "Yetkazdim — naqd oldim"); yetkaza olmasa sabab bilan
/// "Yetkazib bo'lmadi" (keyin qayta urinish mumkin).
///
/// Mijozning ismi, telefoni va manzili FAQAT do'kondan olingandan keyin ko'rinadi (server beradi).
/// Naqd pul yetkazilganda tizimda kuryerda yoziladi; hisobni do'kon kuryerdan pulni qabul
/// qilganda yopadi — yuqoridagi "Sizda naqd pul" ko'rsatkichi shu.
///
/// Buyurtmalar kuryerga AVTOMATIK tushmaydi: do'kon (merchant panel, "Logistika") ularni
/// qo'lda biriktiradi; shundan keyin kuryerga push boradi va ro'yxat yangilanadi.
///
/// Bu ekran restoran/platforma buyurtmalarini KO'RSATMAYDI va ularning taklif/WebSocket oqimiga
/// ulanmaydi: reys kuryeri alohida havzada.
class TripShell extends StatefulWidget {
  const TripShell({super.key});

  @override
  State<TripShell> createState() => _TripShellState();
}

const _statusTitle = <String, String>{
  'confirmed': 'Do\'kon qadoqlamoqda',
  'packed': 'Olishga tayyor',
  'collected': 'Sizda',
  'out_for_delivery': 'Yo\'lda',
  'failed': 'Yetkazilmadi',
};

class _TripShellState extends State<TripShell> with WidgetsBindingObserver {
  List<Map<String, dynamic>> _orders = [];
  Map<String, dynamic> _summary = const {};
  String _name = '';
  bool _loading = true;
  bool _busy = false;
  String? _error;
  Timer? _poll;
  // Oldingi ro'yxat: do'kon yangi buyurtma biriktirganini aniqlash uchun.
  Set<String>? _knownIds;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _load();
    // Do'kon buyurtmani QO'LDA biriktirganda serverdan push keladi va ro'yxat darrov yangilanadi.
    // Push kechiksa yoki o'chiq bo'lsa ham, ochiq ekran har 30 soniyada o'zi yangilanadi.
    unawaited(
      registerCourierPush(onOfferHint: () => unawaited(_load(silent: true))),
    );
    _poll = Timer.periodic(const Duration(seconds: 30), (_) {
      if (!_busy) _load(silent: true);
    });
  }

  @override
  void dispose() {
    _poll?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  // Ilovaga qaytganda ro'yxat yangilanadi (do'kon yangi buyurtma biriktirgan bo'lishi mumkin).
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && !_busy) _load(silent: true);
  }

  void _snack(String msg, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), backgroundColor: error ? kDanger : null),
    );
  }

  Future<void> _load({bool silent = false}) async {
    if (!silent) {
      setState(() => _loading = true);
    }
    try {
      final me = Map<String, dynamic>.from(
        await api.send('GET', '/courier/qurt/me') as Map,
      );
      final res = await api.send('GET', '/courier/qurt/orders');
      final list = (res is Map ? res['orders'] as List? : null) ?? [];
      if (!mounted) return;
      final ids = {
        for (final o in list)
          if (o is Map) '${o['id']}',
      };
      final fresh = _knownIds == null ? 0 : ids.difference(_knownIds!).length;
      _knownIds = ids;
      if (fresh > 0) {
        _snack(
          fresh == 1
              ? 'Sizga yangi buyurtma biriktirildi'
              : '$fresh ta yangi buyurtma biriktirildi',
        );
      }
      setState(() {
        _name = '${me['name'] ?? ''}';
        _summary = Map<String, dynamic>.from(
          (me['summary'] as Map?) ?? const {},
        );
        _orders = [
          for (final o in list)
            if (o is Map) Map<String, dynamic>.from(o),
        ];
        _error = null;
      });
    } on ApiException catch (e) {
      if (mounted && !silent) {
        setState(() => _error = e.message);
      }
    } catch (_) {
      if (mounted && !silent) {
        setState(
          () => _error = 'Serverga ulanib bo\'lmadi — internetni tekshiring',
        );
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  /// Holatni o'zgartiradi. Muvaffaqiyatli bo'lsa `true`. Xato bo'lsa xabar chiqadi va ro'yxat
  /// yangilanadi (holat boshqa joyda o'zgargan bo'lishi mumkin).
  Future<bool> _transition(String id, String to, {String reason = ''}) async {
    if (_busy) return false;
    setState(() => _busy = true);
    var ok = false;
    try {
      await api.send('POST', '/courier/qurt/orders/$id/transition', {
        'to': to,
        if (reason.isNotEmpty) 'reason': reason,
      });
      ok = true;
    } on ApiException catch (e) {
      _snack(e.message, error: true);
    } catch (_) {
      _snack('Serverga ulanib bo\'lmadi', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    await _load(silent: true);
    return ok;
  }

  Future<void> _collectAll(
    String store,
    List<Map<String, dynamic>> packed,
  ) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(store),
        content: Text('${packed.length} ta buyurtmani do\'kondan oldingizmi?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Yo\'q'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Ha, oldim'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    var done = 0;
    for (final o in packed) {
      if (await _transition(o['id'] as String, 'collected')) done++;
    }
    if (done == packed.length) _snack('$done ta buyurtma olindi');
  }

  Future<void> _failed(Map<String, dynamic> o) async {
    final reason = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('№ ${o['number']} — yetkazib bo\'lmadi'),
        content: TextField(
          controller: reason,
          maxLength: 200,
          decoration: const InputDecoration(
            labelText: 'Sabab',
            hintText: 'Masalan: mijoz telefonni ko\'tarmadi',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Bekor qilish'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Yuborish'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    if (reason.text.trim().length < 3) {
      _snack('Sababni yozing (kamida 3 belgi)', error: true);
      return;
    }
    await _transition(o['id'] as String, 'failed', reason: reason.text.trim());
  }

  Future<void> _open(Uri uri) async {
    try {
      if (!await launchUrl(uri, mode: LaunchMode.externalApplication)) {
        _snack('Ochib bo\'lmadi', error: true);
      }
    } catch (_) {
      _snack('Ochib bo\'lmadi', error: true);
    }
  }

  Future<void> _logout() async {
    final active = (_summary['active_orders'] as num?)?.toInt() ?? 0;
    if (active > 0) {
      _snack(
        'Sizda $active ta yakunlanmagan buyurtma bor — avval ularni yetkazing',
        error: true,
      );
      return;
    }
    // Push tokeni chiqishdan OLDIN o'chiriladi: chiqqan kuryerga buyurtma xabari ketmasin.
    await unregisterCourierPush();
    try {
      await api.logout();
    } catch (_) {
      // Mahalliy chiqish baribir bajariladi.
    }
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(
      context,
    ).pushReplacement(MaterialPageRoute(builder: (_) => const LoginScreen()));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(_name.isEmpty ? 'Reys' : 'Reys · $_name'),
        actions: [
          IconButton(
            tooltip: 'Yangilash',
            onPressed: _loading || _busy ? null : _load,
            icon: const Icon(Icons.refresh),
          ),
          IconButton(
            tooltip: 'Chiqish',
            onPressed: _logout,
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: _loading && _orders.isEmpty
          ? const Center(child: CircularProgressIndicator())
          : _error != null && _orders.isEmpty
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(_error!, textAlign: TextAlign.center),
                    const SizedBox(height: 16),
                    FilledButton(
                      onPressed: _load,
                      child: const Text('Qayta urinish'),
                    ),
                  ],
                ),
              ),
            )
          : RefreshIndicator(
              onRefresh: () => _load(silent: true),
              child: _body(),
            ),
    );
  }

  Widget _body() {
    final held = (_summary['held_tiyin'] as num?)?.toInt() ?? 0;
    final heldN = (_summary['held_orders'] as num?)?.toInt() ?? 0;

    final byDate = <String, List<Map<String, dynamic>>>{};
    for (final o in _orders) {
      byDate.putIfAbsent('${o['delivery_date']}', () => []).add(o);
    }
    final dates = byDate.keys.toList()..sort();

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
      children: [
        _cashCard(held, heldN),
        const SizedBox(height: 12),
        if (dates.isEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 48),
            child: Column(
              children: [
                Icon(Icons.local_shipping_outlined, size: 56, color: kInkFaint),
                SizedBox(height: 12),
                Text(
                  'Hozircha sizga buyurtma biriktirilmagan',
                  style: TextStyle(color: kInkMuted),
                ),
              ],
            ),
          ),
        for (final d in dates) ..._dateSection(d, byDate[d]!),
      ],
    );
  }

  Widget _cashCard(int held, int heldN) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: held > 0 ? const Color(0xFFFFF4E5) : kCardBg,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        children: [
          Icon(
            Icons.payments_outlined,
            color: held > 0 ? kWarning : kInkMuted,
            size: 30,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Sizda naqd pul',
                  style: TextStyle(color: kInkMuted),
                ),
                Text(
                  formatSum(held),
                  style: const TextStyle(
                    fontSize: 22,
                    fontWeight: FontWeight.w800,
                    color: kInk,
                  ),
                ),
              ],
            ),
          ),
          Text(
            held > 0
                ? '$heldN ta buyurtma\nadminga topshiring'
                : 'hammasi topshirilgan',
            textAlign: TextAlign.right,
            style: const TextStyle(color: kInkMuted, fontSize: 12.5),
          ),
        ],
      ),
    );
  }

  List<Widget> _dateSection(String date, List<Map<String, dynamic>> list) {
    final pickup = list
        .where((o) => o['status'] == 'confirmed' || o['status'] == 'packed')
        .toList();
    final deliver = list
        .where(
          (o) =>
              o['status'] == 'collected' ||
              o['status'] == 'out_for_delivery' ||
              o['status'] == 'failed',
        )
        .toList();

    final byStore = <String, List<Map<String, dynamic>>>{};
    for (final o in pickup) {
      byStore.putIfAbsent('${o['store_name']}', () => []).add(o);
    }

    return [
      Padding(
        padding: const EdgeInsets.only(top: 12, bottom: 8),
        child: Text(
          'Reys: $date',
          style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800),
        ),
      ),
      if (byStore.isNotEmpty) ...[
        const _SectionLabel('1. Do\'konlardan olish'),
        for (final e in byStore.entries) _storeCard(e.key, e.value),
      ],
      if (deliver.isNotEmpty) ...[
        const _SectionLabel('2. Mijozlarga yetkazish'),
        for (final o in deliver) _deliveryCard(o),
      ],
    ];
  }

  Widget _storeCard(String store, List<Map<String, dynamic>> list) {
    final packed = list.where((o) => o['status'] == 'packed').toList();
    final waiting = list.length - packed.length;
    return Card(
      elevation: 0,
      color: kCardBg,
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.storefront, color: kBrandColor),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    store,
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 16,
                    ),
                  ),
                ),
                Text('${list.length} ta'),
              ],
            ),
            const SizedBox(height: 8),
            for (final o in list)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Text(
                  '№ ${o['number']} · ${_itemsLine(o)}${o['status'] == 'confirmed' ? '  (qadoqlanmoqda)' : ''}',
                  style: const TextStyle(color: kInkMuted, fontSize: 13),
                ),
              ),
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                onPressed: _busy || packed.isEmpty
                    ? null
                    : () => _collectAll(store, packed),
                icon: const Icon(Icons.inventory_2_outlined),
                label: Text(
                  packed.isEmpty
                      ? 'Do\'kon hali qadoqlamagan'
                      : waiting > 0
                      ? 'Tayyorini oldim (${packed.length})'
                      : 'Hammasini oldim (${packed.length})',
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _itemsLine(Map<String, dynamic> o) {
    final items = (o['items'] as List?) ?? const [];
    return items.map((i) => '${(i as Map)['name']} × ${i['qty']}').join(', ');
  }

  Widget _deliveryCard(Map<String, dynamic> o) {
    final status = '${o['status']}';
    final cust = (o['customer'] as Map?) ?? const {};
    final addr = (cust['address'] as Map?) ?? const {};
    final lat = (addr['lat'] as num?)?.toDouble();
    final lng = (addr['lng'] as num?)?.toDouble();
    final phone = '${cust['phone'] ?? ''}';
    final total = (o['total_tiyin'] as num?)?.toInt() ?? 0;
    final detail = [
      if ('${addr['entrance'] ?? ''}'.isNotEmpty) 'kirish ${addr['entrance']}',
      if ('${addr['floor'] ?? ''}'.isNotEmpty) '${addr['floor']}-qavat',
      if ('${addr['apartment'] ?? ''}'.isNotEmpty)
        'xonadon ${addr['apartment']}',
      if ('${addr['intercom'] ?? ''}'.isNotEmpty) 'domofon ${addr['intercom']}',
    ].join(', ');

    return Card(
      elevation: 0,
      color: kCardBg,
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  '№ ${o['number']}',
                  style: const TextStyle(
                    fontWeight: FontWeight.w800,
                    fontSize: 16,
                  ),
                ),
                const SizedBox(width: 8),
                Chip(
                  visualDensity: VisualDensity.compact,
                  label: Text(_statusTitle[status] ?? status),
                  backgroundColor: status == 'failed'
                      ? const Color(0x33DC2626)
                      : null,
                ),
                const Spacer(),
                Text(
                  formatSum(total),
                  style: const TextStyle(
                    fontWeight: FontWeight.w800,
                    fontSize: 16,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              '${cust['name'] ?? ''}',
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
            Text('${addr['text'] ?? ''}'),
            if (detail.isNotEmpty)
              Text(detail, style: const TextStyle(color: kInkMuted)),
            if ('${addr['comment'] ?? ''}'.isNotEmpty)
              Text(
                'Izoh: ${addr['comment']}',
                style: const TextStyle(color: kInkMuted),
              ),
            const SizedBox(height: 6),
            Text(
              _itemsLine(o),
              style: const TextStyle(color: kInkMuted, fontSize: 13),
            ),
            if (status == 'failed' && '${o['reject_reason'] ?? ''}'.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 6),
                child: Text(
                  'Sabab: ${o['reject_reason']}',
                  style: const TextStyle(color: kDanger),
                ),
              ),
            const SizedBox(height: 10),
            Row(
              children: [
                if (phone.isNotEmpty)
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _open(Uri(scheme: 'tel', path: phone)),
                      icon: const Icon(Icons.call, size: 18),
                      label: const Text('Qo\'ng\'iroq'),
                    ),
                  ),
                if (phone.isNotEmpty && lat != null && lng != null)
                  const SizedBox(width: 8),
                if (lat != null && lng != null)
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _open(
                        Uri.parse(
                          'https://www.google.com/maps/dir/?api=1&destination=$lat,$lng',
                        ),
                      ),
                      icon: const Icon(Icons.navigation_outlined, size: 18),
                      label: const Text('Yo\'l'),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 10),
            if (status == 'collected' || status == 'failed')
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: _busy
                      ? null
                      : () =>
                            _transition(o['id'] as String, 'out_for_delivery'),
                  icon: const Icon(Icons.local_shipping_outlined),
                  label: Text(
                    status == 'failed'
                        ? 'Qayta urinish — yo\'lga chiqdim'
                        : 'Yo\'lga chiqdim',
                  ),
                ),
              ),
            if (status == 'out_for_delivery') ...[
              SlideToConfirm(
                label: 'Yetkazdim — ${formatSum(total)} naqd oldim',
                busy: _busy,
                onConfirm: () => _transition(o['id'] as String, 'delivered'),
              ),
              const SizedBox(height: 8),
              SizedBox(
                width: double.infinity,
                child: TextButton(
                  onPressed: _busy ? null : () => _failed(o),
                  child: const Text(
                    'Yetkazib bo\'lmadi',
                    style: TextStyle(color: kDanger),
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  const _SectionLabel(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 8, top: 4),
    child: Text(
      text,
      style: const TextStyle(
        color: kInkMuted,
        fontWeight: FontWeight.w700,
        fontSize: 13,
      ),
    ),
  );
}
