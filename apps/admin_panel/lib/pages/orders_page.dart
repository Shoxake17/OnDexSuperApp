import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';

class OrdersPage extends StatefulWidget {
  const OrdersPage({super.key});

  @override
  State<OrdersPage> createState() => _OrdersPageState();
}

class _OrdersPageState extends State<OrdersPage> {
  List<dynamic> _list = [];
  bool _loading = true;
  late final LiveRefresher _live;

  /// Backend `restaurant_name` / `courier_name` bo'sh yoki umuman
  /// jo'natmasligi uchun fallback kesh. Buyurtmalar ro'yxati bilan
  /// BIRGA yuklanadi. Avvalgi backend versiyalarda (yoki restoran
  /// o'chirilganda) jadvalda `#bdb543` ko'rindi — bu bug.
  Map<String, String> _restaurantNames = {};
  Map<String, String> _courierNames = {};

  static const _statusColors = {
    'created': Colors.blue,
    'accepted': Colors.indigo,
    'preparing': Colors.orange,
    'ready': Colors.amber,
    'picked_up': Colors.teal,
    'delivered': Colors.green,
    // `served` — STOL buyurtmasining yakuniy holati. Bu yerda yo'q
    // edi va zal buyurtmalari rangsiz chizilardi (bug.md 78-band).
    // Rangi `delivered` bilan bir xil: ikkalasi ham "mijoz taomni
    // oldi" degani, faqat yo'li boshqa.
    'served': Colors.green,
    'cancelled': Colors.grey,
    'rejected': Colors.red,
  };

  @override
  void initState() {
    super.initState();
    _load();
    // Avval har 5 soniyada so'rov ketardi — endi buyurtma hodisasi
    // kelishi bilan DARHOL yangilanadi, so'rov esa zaxira
    // (`live.dart` dagi izoh).
    _live = LiveRefresher(
      bus: adminLive,
      onRefresh: _load,
      types: const {'new_order', 'order_status', 'courier_assigned'},
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      // Buyurtmalar, restoranlar va kuryerlarni PARALLEL yuklaymiz.
      final lFuture = api.orders();
      final rFuture = _loadRestaurantNameMap();
      final cFuture = _loadCourierNameMap();
      final l = await lFuture;
      final rn = await rFuture;
      final cn = await cFuture;
      if (!mounted) return;
      setState(() {
        _list = l;
        _restaurantNames = rn;
        _courierNames = cn;
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loading = false);
    }
  }

  Future<Map<String, String>> _loadRestaurantNameMap() async {
    final m = <String, String>{};
    try {
      final list = await api.restaurants();
      for (final r in list.cast<Map<String, dynamic>>()) {
        final id = (r['id']?.toString() ?? '').trim();
        final name = (r['name']?.toString() ?? '').trim();
        if (id.isNotEmpty && name.isNotEmpty) m[id] = name;
      }
    } catch (_) {}
    return m;
  }

  Future<Map<String, String>> _loadCourierNameMap() async {
    final m = <String, String>{};
    try {
      final list = await api.couriers();
      for (final c in list.cast<Map<String, dynamic>>()) {
        final id = (c['id']?.toString() ?? '').trim();
        final name = (c['name']?.toString() ?? '').trim();
        if (id.isNotEmpty && name.isNotEmpty) m[id] = name;
      }
    } catch (_) {}
    return m;
  }

  String _time(String? iso) {
    if (iso == null) return '';
    final t = DateTime.tryParse(iso)?.toLocal();
    if (t == null) return '';
    two(int n) => n.toString().padLeft(2, '0');
    return '${two(t.day)}.${two(t.month)} ${two(t.hour)}:${two(t.minute)}';
  }

  /// ┌─ NOM YOKI ID (FALLBACK BILAN) ──────────────────────────────────┐
  /// 1) Avval backend `{kind}_name` ni jo'natadimi deb tekshiramiz.
  /// 2) Bo'sh bo'lsa — frontend yuklagan keshga (_restaurantNames /
  ///    _courierNames) qaraymiz (bu backend eski versiyalari uchun).
  /// 3) Hamasi bo'sh — `#id6` formatida (tarix uchun).
  ///
  /// Bu avvalgi versiyalarda sodir bo'lgan "#bdb543" xatosini butunlay
  /// bartaraf etadi.
  /// └──────────────────────────────────────────────────────────────────┘
  String _name(Map<String, dynamic> o, String kind) {
    var name = (o['${kind}_name']?.toString() ?? '').trim();
    if (name.isNotEmpty) return name;
    final id = (o['${kind}_id']?.toString() ?? '').trim();
    if (id.isEmpty) return '—';
    if (kind == 'restaurant') {
      name = _restaurantNames[id] ?? '';
    } else if (kind == 'courier') {
      name = _courierNames[id] ?? '';
    }
    if (name.isNotEmpty) return name;
    return '#${id.substring(0, id.length < 6 ? id.length : 6)}';
  }

  /// Kuryer hali tayinlanmagan bo'lishi NORMAL holat (yangi buyurtma,
  /// yoki stol buyurtmasi — unga kuryer umuman chaqirilmaydi).
  String _courier(Map<String, dynamic> o) {
    final id = (o['courier_id'] as String?) ?? '';
    if (id.isEmpty) return '—';
    return _name(o, 'courier');
  }

  /// ┌─ MIJOZ: ISM YO'Q BO'LSA TELEFON ─────────────────────────────────┐
  /// Ro'yxatdan o'tishda faqat telefon raqami so'raladi — ism ixtiyoriy
  /// va ko'p mijozda bo'sh. Faqat ismga tayansak ustun yana bo'sh
  /// ko'rinardi. Telefon esa HAR DOIM bor: u login identifikatori.
  /// └──────────────────────────────────────────────────────────────────┘
  Widget _customerCell(Map<String, dynamic> o) {
    final name = (o['customer_name'] as String?)?.trim() ?? '';
    final phone = (o['customer_phone'] as String?)?.trim() ?? '';
    if (name.isEmpty && phone.isEmpty) return const Text('—');
    if (name.isEmpty) return Text(phone);
    if (phone.isEmpty) return Text(name);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Text(name),
        Text(phone, style: const TextStyle(fontSize: 11, color: Colors.grey)),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('Buyurtmalar',
                  style: Theme.of(context).textTheme.headlineMedium),
              const Spacer(),
              Text('Har 5 soniyada yangilanadi',
                  style: Theme.of(context).textTheme.bodySmall),
            ],
          ),
          const SizedBox(height: 16),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _list.isEmpty
                    ? const Center(child: Text('Hozircha buyurtma yo\'q'))
                    : SingleChildScrollView(
                        child: SizedBox(
                          width: double.infinity,
                          child: DataTable(
                            columns: const [
                              DataColumn(label: Text('Vaqt')),
                              DataColumn(label: Text('№')),
                              DataColumn(label: Text('Restoran')),
                              DataColumn(label: Text('Mijoz')),
                              DataColumn(label: Text('Kuryer')),
                              DataColumn(label: Text('Summa')),
                              DataColumn(label: Text('Holat')),
                            ],
                            rows: [
                              for (final o in _list.cast<Map<String, dynamic>>())
                                DataRow(cells: [
                                  DataCell(Text(_time(o['created_at']))),
                                  DataCell(Text(
                                      o['order_number']?.toString() ?? '—')),
                                  DataCell(Text(_name(o, 'restaurant'))),
                                  DataCell(_customerCell(o)),
                                  DataCell(Text(_courier(o))),
                                  DataCell(
                                      Text(formatSum((o['total_tiyin'] ?? 0) as int))),
                                  DataCell(Chip(
                                    label: Text(
                                      orderStatusLabel(
                                          (o['status'] as String?) ?? ''),
                                      style: const TextStyle(
                                          color: Colors.white, fontSize: 12),
                                    ),
                                    backgroundColor:
                                        _statusColors[o['status']] ??
                                            Colors.grey,
                                    padding: EdgeInsets.zero,
                                  )),
                                ]),
                            ],
                          ),
                        ),
                      ),
          ),
        ],
      ),
    );
  }
}
