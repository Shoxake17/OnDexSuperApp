import 'package:flutter/material.dart';

import '../api.dart';

/// "Qurt logistikasi": reyslar (yetkazish kunlari), kuryerga biriktirish va kuryerlardagi naqd pul.
///
/// Qurt buyurtmalari bir necha kun to'planib, reys kuni yetkaziladi. Qurt kuryerlari OnDex GO
/// ilovasiga telefon bilan kiradi, lekin restoran/platforma buyurtmalarini UMUMAN olmaydi
/// (alohida havza). Naqd pul mijozdan olinganda kuryerda yoziladi; shu yerda admin kuryerdan
/// pulni qabul qilib hisobni yopadi.
class QurtLogisticsPage extends StatefulWidget {
  const QurtLogisticsPage({super.key});

  @override
  State<QurtLogisticsPage> createState() => _QurtLogisticsPageState();
}

const _activeStatuses = 'confirmed,packed,collected,out_for_delivery,failed';

const _statusTitle = <String, String>{
  'new': 'Yangi',
  'confirmed': 'Qabul qilingan',
  'packed': 'Qadoqlangan',
  'collected': 'Kuryer oldi',
  'out_for_delivery': 'Yo\'lda',
  'failed': 'Yetkazilmadi',
  'delivered': 'Yetkazildi',
};

const _statusColor = <String, Color>{
  'confirmed': Color(0xFF3B82F6),
  'packed': Color(0xFF6366F1),
  'collected': Color(0xFF06B6D4),
  'out_for_delivery': Color(0xFFF97316),
  'failed': Color(0xFFEF4444),
};

class _QurtLogisticsPageState extends State<QurtLogisticsPage> {
  List<Map<String, dynamic>> _orders = [];
  List<Map<String, dynamic>> _couriers = [];
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _snack(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  List<Map<String, dynamic>> _maps(List<dynamic> l) => [
        for (final x in l)
          if (x is Map) Map<String, dynamic>.from(x)
      ];

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final res = await Future.wait([
        api.qurtOrders(statuses: _activeStatuses),
        api.qurtCouriers(),
      ]);
      if (!mounted) return;
      setState(() {
        _orders = _maps(res[0]);
        _couriers = _maps(res[1]);
      });
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  /// Amalni bajaradi: band bo'lsa ikkinchisi boshlanmaydi, xato SnackBar'da, oxirida yangilanadi.
  Future<void> _run(Future<String> Function() action) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      _snack(await action());
    } on ApiException catch (e) {
      _snack(e.message);
    } catch (e) {
      _snack('Xato: $e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    await _load();
  }

  Future<bool> _confirm(String title, String body, String action) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Text(body),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Bekor qilish')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true), child: Text(action)),
        ],
      ),
    );
    return ok == true;
  }

  String _courierName(String? id) {
    if (id == null || id.isEmpty) return '';
    for (final c in _couriers) {
      if (c['id'] == id) return '${c['name']}';
    }
    return 'Kuryer';
  }

  Future<String?> _pickCourier(String title) async {
    if (_couriers.isEmpty) {
      _snack('Avval "Kuryerlar" bo\'limida Qurt kuryerini qo\'shing');
      return null;
    }
    String? selected = _couriers.first['id'] as String;
    return showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, set) => AlertDialog(
          title: Text(title),
          content: SizedBox(
            width: 380,
            child: DropdownButtonFormField<String>(
              initialValue: selected,
              decoration: const InputDecoration(labelText: 'Kuryer'),
              items: [
                for (final c in _couriers)
                  DropdownMenuItem(
                      value: c['id'] as String,
                      child: Text('${c['name']} · ${c['phone'] ?? ''}')),
              ],
              onChanged: (v) => set(() => selected = v),
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx),
                child: const Text('Bekor qilish')),
            FilledButton(
                onPressed: () => Navigator.pop(ctx, selected),
                child: const Text('Biriktirish')),
          ],
        ),
      ),
    );
  }

  Future<void> _assignDate(String date) async {
    final id = await _pickCourier('$date reysini kuryerga biriktirish');
    if (id == null) return;
    await _run(() async {
      final r = await api.qurtAssign(id, date: date);
      return '${r['assigned']} ta buyurtma biriktirildi';
    });
  }

  Future<void> _assignOne(Map<String, dynamic> o) async {
    final id = await _pickCourier('№ ${o['number']} ni biriktirish');
    if (id == null) return;
    await _run(() async {
      final r = await api.qurtAssign(id, orderIds: [o['id'] as String]);
      return r['assigned'] == 1
          ? 'Biriktirildi'
          : 'Biriktirib bo\'lmadi (boshqa kuryerga biriktirilgan bo\'lishi mumkin)';
    });
  }

  Future<void> _step(
      String date, String from, String to, String label, int count) async {
    if (!await _confirm(label,
        '$date reysidagi $count ta buyurtma o\'tkaziladi.', 'Ha, o\'tkazish')) {
      return;
    }
    await _run(() async {
      final r = await api.qurtBatchTransition(date, from, to);
      return '${r['done']} ta o\'tdi'
          '${(r['skipped'] ?? 0) > 0 ? ', ${r['skipped']} ta o\'tmadi' : ''}';
    });
  }

  Future<void> _addCourier() async {
    final name = TextEditingController();
    final phone = TextEditingController(text: '+998');
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Qurt kuryeri qo\'shish'),
        content: SizedBox(
          width: 420,
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            TextField(
                controller: name,
                decoration: const InputDecoration(labelText: 'Ism')),
            const SizedBox(height: 8),
            TextField(
              controller: phone,
              keyboardType: TextInputType.phone,
              decoration: const InputDecoration(
                labelText: 'Telefon raqami',
                helperText: 'Kuryer OnDex GO ilovasiga shu raqam bilan kiradi '
                    '(Telegram/SMS kod). Boshqa rolli akkauntga ulanmaydi.',
              ),
            ),
          ]),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Bekor qilish')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Qo\'shish')),
        ],
      ),
    );
    if (ok != true) return;
    await _run(() async {
      await api.addQurtCourier(
          phone: phone.text.trim(), name: name.text.trim());
      return 'Kuryer qo\'shildi';
    });
  }

  Future<void> _settle(Map<String, dynamic> c) async {
    final sum = (c['summary'] as Map?) ?? const {};
    final held = (sum['held_tiyin'] as num?)?.toInt() ?? 0;
    final n = (sum['held_orders'] as num?)?.toInt() ?? 0;
    if (!await _confirm(
        'Pulni qabul qilish',
        '${c['name']} sizga ${formatSum(held)} ($n ta buyurtma) topshirdimi? '
            'Tasdiqlagach hisob yopiladi.',
        'Ha, qabul qildim')) {
      return;
    }
    await _run(() async {
      final r = await api.qurtSettle(c['id'] as String);
      return '${formatSum((r['total_tiyin'] as num?)?.toInt() ?? 0)} qabul qilindi';
    });
  }

  Future<void> _remove(Map<String, dynamic> c) async {
    if (!await _confirm(
        'Kuryerni olib tashlash',
        '${c['name']} Qurt kuryerlaridan olib tashlanadi va ilovadan chiqariladi.',
        'Olib tashlash')) {
      return;
    }
    await _run(() async {
      await api.removeQurtCourier(c['id'] as String);
      return 'Kuryer olib tashlandi';
    });
  }

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 2,
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(24, 20, 24, 0),
            child: Row(children: [
              Text('Qurt logistikasi',
                  style: Theme.of(context).textTheme.headlineSmall),
              const Spacer(),
              IconButton(
                tooltip: 'Yangilash',
                onPressed: _loading || _busy ? null : _load,
                icon: const Icon(Icons.refresh),
              ),
            ]),
          ),
          const TabBar(
              isScrollable: true,
              tabAlignment: TabAlignment.start,
              tabs: [
                Tab(text: 'Reyslar'),
                Tab(text: 'Kuryerlar va naqd pul'),
              ]),
          if (_busy) const LinearProgressIndicator(minHeight: 2),
          Expanded(
            child: _loading && _orders.isEmpty && _couriers.isEmpty
                ? const Center(child: CircularProgressIndicator())
                : _error != null && _orders.isEmpty && _couriers.isEmpty
                    ? Center(
                        child:
                            Column(mainAxisSize: MainAxisSize.min, children: [
                          Text(_error!),
                          const SizedBox(height: 12),
                          FilledButton(
                              onPressed: _load,
                              child: const Text('Qayta urinish')),
                        ]),
                      )
                    : TabBarView(children: [_batchesTab(), _couriersTab()]),
          ),
        ],
      ),
    );
  }

  // ── Reyslar ──────────────────────────────────────────────────────────

  Widget _batchesTab() {
    final byDate = <String, List<Map<String, dynamic>>>{};
    for (final o in _orders) {
      byDate.putIfAbsent('${o['delivery_date']}', () => []).add(o);
    }
    final dates = byDate.keys.toList()..sort();
    if (dates.isEmpty) {
      return const Center(
          child: Text(
              'Faol reys yo\'q — do\'konlar qabul qilgan buyurtmalar shu yerda ko\'rinadi'));
    }
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [for (final d in dates) _batchCard(d, byDate[d]!)],
    );
  }

  Widget _batchCard(String date, List<Map<String, dynamic>> list) {
    final counts = <String, int>{};
    var sum = 0;
    var unassigned = 0;
    for (final o in list) {
      final s = '${o['status']}';
      counts[s] = (counts[s] ?? 0) + 1;
      sum += (o['total_tiyin'] as num?)?.toInt() ?? 0;
      if ((o['courier_id'] ?? '') == '') unassigned++;
    }
    Widget stepBtn(String from, String to, String label) => counts[from] == null
        ? const SizedBox.shrink()
        : OutlinedButton(
            onPressed: _busy
                ? null
                : () => _step(date, from, to, label, counts[from]!),
            child: Text('$label (${counts[from]})'),
          );

    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      child: ExpansionTile(
        initiallyExpanded: dates0(date),
        shape: const Border(),
        title: Text('Reys: $date',
            style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 17)),
        subtitle: Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Wrap(spacing: 8, runSpacing: 6, children: [
            for (final e in counts.entries) _chip(e.key, e.value),
            Chip(
                visualDensity: VisualDensity.compact,
                label: Text('${list.length} ta · ${formatSum(sum)}')),
            if (unassigned > 0)
              Chip(
                  visualDensity: VisualDensity.compact,
                  backgroundColor: const Color(0x33F59E0B),
                  label: Text('Kuryersiz: $unassigned')),
          ]),
        ),
        childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
        children: [
          Align(
            alignment: Alignment.centerLeft,
            child: Wrap(spacing: 8, runSpacing: 8, children: [
              FilledButton.icon(
                onPressed: _busy ? null : () => _assignDate(date),
                icon: const Icon(Icons.person_add_alt_1, size: 18),
                label: const Text('Reysni kuryerga biriktirish'),
              ),
              stepBtn('packed', 'collected', 'Hammasi do\'kondan olindi'),
              stepBtn('collected', 'out_for_delivery', 'Reys yo\'lga chiqdi'),
              stepBtn('out_for_delivery', 'delivered', 'Hammasi yetkazildi'),
            ]),
          ),
          const SizedBox(height: 8),
          for (final o in list) _orderRow(o),
        ],
      ),
    );
  }

  // Birinchi (eng yaqin) reys ochiq turadi.
  bool dates0(String date) {
    final all = _orders.map((o) => '${o['delivery_date']}').toSet().toList()
      ..sort();
    return all.isNotEmpty && all.first == date;
  }

  Widget _chip(String status, int n) {
    final c = _statusColor[status] ?? Colors.grey;
    return Chip(
      visualDensity: VisualDensity.compact,
      side: BorderSide(color: c.withValues(alpha: .5)),
      backgroundColor: c.withValues(alpha: .15),
      label: Text('${_statusTitle[status] ?? status}: $n',
          style: TextStyle(color: c, fontWeight: FontWeight.w600)),
    );
  }

  Widget _orderRow(Map<String, dynamic> o) {
    final status = '${o['status']}';
    final courier = _courierName(o['courier_id'] as String?);
    final addr =
        (o['address'] is Map) ? '${(o['address'] as Map)['text'] ?? ''}' : '';
    final canUnassign = courier.isNotEmpty &&
        (status == 'confirmed' || status == 'packed' || status == 'collected');
    return ListTile(
      dense: true,
      contentPadding: EdgeInsets.zero,
      title: Text(
          '№ ${o['number']} · ${o['store_name'] ?? ''} · ${formatSum((o['total_tiyin'] as num?)?.toInt() ?? 0)}'),
      subtitle: Text([
        _statusTitle[status] ?? status,
        courier.isEmpty ? 'kuryersiz' : 'Kuryer: $courier',
        if ('${o['customer_name'] ?? ''}'.isNotEmpty) '${o['customer_name']}',
        if (addr.isNotEmpty) addr,
      ].join(' · ')),
      trailing: PopupMenuButton<String>(
        enabled: !_busy,
        onSelected: (v) {
          if (v == 'assign') _assignOne(o);
          if (v == 'unassign') {
            _run(() async {
              await api.qurtUnassign(o['id'] as String);
              return 'Kuryerdan olindi';
            });
          }
        },
        itemBuilder: (_) => [
          if (status != 'out_for_delivery')
            PopupMenuItem(
                value: 'assign',
                child: Text(courier.isEmpty
                    ? 'Kuryerga biriktirish'
                    : 'Boshqa kuryerga')),
          if (canUnassign)
            const PopupMenuItem(
                value: 'unassign', child: Text('Kuryerdan olish')),
        ],
      ),
    );
  }

  // ── Kuryerlar va naqd pul ────────────────────────────────────────────

  Widget _couriersTab() {
    var totalHeld = 0;
    for (final c in _couriers) {
      totalHeld +=
          (((c['summary'] as Map?)?['held_tiyin']) as num?)?.toInt() ?? 0;
    }
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Row(children: [
          Expanded(
            child: Text(
              'Kuryerlarda topshirilmagan naqd pul: ${formatSum(totalHeld)}',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          FilledButton.icon(
            onPressed: _busy ? null : _addCourier,
            icon: const Icon(Icons.add),
            label: const Text('Kuryer qo\'shish'),
          ),
        ]),
        const SizedBox(height: 6),
        Text(
          'Qurt kuryerlari OnDex GO ilovasiga telefon bilan kiradi va faqat Qurt reysini ko\'radi — '
          'restoran/platforma buyurtmalari ularga kelmaydi.',
          style: Theme.of(context).textTheme.bodySmall,
        ),
        const SizedBox(height: 16),
        if (_couriers.isEmpty)
          const Padding(
            padding: EdgeInsets.all(32),
            child: Center(child: Text('Hali Qurt kuryeri yo\'q')),
          ),
        for (final c in _couriers) _courierCard(c),
      ],
    );
  }

  Widget _courierCard(Map<String, dynamic> c) {
    final sum = (c['summary'] as Map?) ?? const {};
    final held = (sum['held_tiyin'] as num?)?.toInt() ?? 0;
    final heldN = (sum['held_orders'] as num?)?.toInt() ?? 0;
    final active = (sum['active_orders'] as num?)?.toInt() ?? 0;
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(children: [
          const CircleAvatar(child: Icon(Icons.delivery_dining)),
          const SizedBox(width: 14),
          Expanded(
            child:
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text('${c['name']}',
                  style: const TextStyle(
                      fontWeight: FontWeight.w700, fontSize: 16)),
              Text('${c['phone'] ?? ''}'),
              const SizedBox(height: 4),
              Text('Faol buyurtma: $active'),
            ]),
          ),
          Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
            Text(formatSum(held),
                style: TextStyle(
                    fontWeight: FontWeight.w800,
                    fontSize: 18,
                    color: held > 0 ? const Color(0xFFF59E0B) : null)),
            Text(held > 0 ? 'kuryerda ($heldN ta)' : 'pul topshirilgan'),
          ]),
          const SizedBox(width: 16),
          FilledButton(
            onPressed: _busy || held == 0 ? null : () => _settle(c),
            child: const Text('Pulni qabul qilish'),
          ),
          IconButton(
            tooltip: 'Olib tashlash',
            onPressed: _busy ? null : () => _remove(c),
            icon: const Icon(Icons.person_remove_outlined),
          ),
        ]),
      ),
    );
  }
}
