import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/sheet_page.dart';
import 'catalog_screen.dart' show kBrand;

const _ink = Color(0xFF111827);
const _muted = Color(0xFF6B7280);
const _line = Color(0xFFEDEDED);

// ── Umumiy yordamchilar (savat ekrani ham ishlatadi) ─────────────────────

const _weekdays = [
  'dushanba',
  'seshanba',
  'chorshanba',
  'payshanba',
  'juma',
  'shanba',
  'yakshanba',
];
const _months = [
  'yanvar',
  'fevral',
  'mart',
  'aprel',
  'may',
  'iyun',
  'iyul',
  'avgust',
  'sentabr',
  'oktabr',
  'noyabr',
  'dekabr',
];

/// "2026-11-03" -> "3-noyabr, seshanba". Noto'g'ri qiymat o'zi qaytariladi.
String formatQurtDate(String ymd) {
  final d = DateTime.tryParse(ymd);
  if (d == null) return ymd;
  return '${d.day}-${_months[d.month - 1]}, ${_weekdays[d.weekday - 1]}';
}

/// Holat nomi mijoz tilida.
String qurtStatusLabel(String status) => switch (status) {
  'new' => 'Do\'kon ko\'rib chiqmoqda',
  'confirmed' => 'Do\'kon tasdiqladi',
  'rejected' => 'Do\'kon rad etdi',
  'packed' => 'Qadoqlandi',
  'collected' => 'Logistika do\'kondan oldi',
  'out_for_delivery' => 'Yo\'lda',
  'delivered' => 'Yetkazildi',
  'failed' => 'Yetkazib bo\'lmadi — qayta uriniladi',
  'cancelled' => 'Bekor qilindi',
  _ => status,
};

Color qurtStatusColor(String status) => switch (status) {
  'new' => const Color(0xFFF59E0B),
  'confirmed' || 'packed' => const Color(0xFF3B82F6),
  'collected' || 'out_for_delivery' => kBrand,
  'delivered' => const Color(0xFF16A34A),
  'failed' => const Color(0xFFDC2626),
  _ => const Color(0xFF6B7280), // rejected, cancelled
};

Widget qurtStatusChip(String status) {
  final c = qurtStatusColor(status);
  return Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
    decoration: BoxDecoration(
      color: c.withValues(alpha: 0.12),
      borderRadius: BorderRadius.circular(20),
    ),
    child: Text(
      qurtStatusLabel(status),
      style: TextStyle(color: c, fontSize: 11.5, fontWeight: FontWeight.w700),
    ),
  );
}

int _tiyin(dynamic v) => (v as num?)?.toInt() ?? 0;

List<Map<String, dynamic>> _items(Map<String, dynamic> o) =>
    ((o['items'] as List?) ?? [])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList();

String _itemsSummary(Map<String, dynamic> o) {
  final items = _items(o);
  if (items.isEmpty) return '';
  final first = items
      .take(2)
      .map((i) => '${i['name']} × ${i['qty']}')
      .join(', ');
  return items.length > 2 ? '$first va yana ${items.length - 2} ta' : first;
}

String _time(dynamic iso) {
  final t = DateTime.tryParse((iso ?? '').toString())?.toLocal();
  if (t == null) return '';
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(t.day)}.${two(t.month)} ${two(t.hour)}:${two(t.minute)}';
}

// ═════════════════════════════════════════════════════════════════════════
// BUYURTMALARIM
// ═════════════════════════════════════════════════════════════════════════

/// Mijozning qurt buyurtmalari ro'yxati.
class QurtOrdersScreen extends StatefulWidget {
  const QurtOrdersScreen({super.key});

  @override
  State<QurtOrdersScreen> createState() => _QurtOrdersScreenState();
}

class _QurtOrdersScreenState extends State<QurtOrdersScreen> {
  List<Map<String, dynamic>> _orders = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = _orders.isEmpty;
      _error = null;
    });
    try {
      final list = await api.qurtOrders();
      if (!mounted) return;
      setState(() => _orders = list);
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = errorText(e, 'Buyurtmalarni yuklab bo\'lmadi'));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _open(Map<String, dynamic> o) async {
    await Navigator.of(context).push(
      sheetRoute(
        QurtOrderDetailScreen(orderId: (o['id'] ?? '').toString(), initial: o),
      ),
    );
    if (mounted) _load(); // bekor qilingan/yangilangan bo'lishi mumkin
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: AppBar(
          backgroundColor: Colors.white,
          surfaceTintColor: Colors.white,
          elevation: 0,
          title: const Text(
            'Qurt buyurtmalarim',
            style: TextStyle(
              color: _ink,
              fontWeight: FontWeight.w800,
              fontSize: 18,
            ),
          ),
          iconTheme: const IconThemeData(color: _ink),
        ),
        body: RefreshIndicator(color: kBrand, onRefresh: _load, child: _body()),
      ),
    );
  }

  Widget _body() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(color: kBrand));
    }
    if (_error != null && _orders.isEmpty) {
      return ListView(
        children: [
          const SizedBox(height: 120),
          Center(child: Text(_error!, style: const TextStyle(color: _muted))),
          const SizedBox(height: 12),
          Center(
            child: OutlinedButton(
              onPressed: _load,
              child: const Text('Qayta urinish'),
            ),
          ),
        ],
      );
    }
    if (_orders.isEmpty) {
      return ListView(
        children: const [
          SizedBox(height: 120),
          Icon(Icons.receipt_long_outlined, size: 56, color: Colors.black26),
          SizedBox(height: 12),
          Center(
            child: Text(
              'Hali qurt buyurtmangiz yo\'q',
              style: TextStyle(color: _muted, fontSize: 15),
            ),
          ),
        ],
      );
    }
    return ListView.separated(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      itemCount: _orders.length,
      separatorBuilder: (_, _) => const SizedBox(height: 10),
      itemBuilder:
          (_, i) =>
              _OrderCard(order: _orders[i], onTap: () => _open(_orders[i])),
    );
  }
}

class _OrderCard extends StatelessWidget {
  const _OrderCard({required this.order, required this.onTap});
  final Map<String, dynamic> order;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final status = (order['status'] ?? '').toString();
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: _line),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    '№ ${order['number'] ?? ''}',
                    style: const TextStyle(
                      fontWeight: FontWeight.w800,
                      fontSize: 15,
                      color: _ink,
                    ),
                  ),
                ),
                qurtStatusChip(status),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              (order['store_name'] ?? '').toString(),
              style: const TextStyle(color: _muted, fontSize: 12.5),
            ),
            const SizedBox(height: 4),
            Text(
              _itemsSummary(order),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: _ink, fontSize: 13.5),
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                const Icon(
                  Icons.local_shipping_outlined,
                  size: 16,
                  color: _muted,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    formatQurtDate((order['delivery_date'] ?? '').toString()),
                    style: const TextStyle(color: _muted, fontSize: 12.5),
                  ),
                ),
                Text(
                  formatSum(_tiyin(order['total_tiyin'])),
                  style: const TextStyle(
                    color: kBrand,
                    fontWeight: FontWeight.w800,
                    fontSize: 15,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ═════════════════════════════════════════════════════════════════════════
// BUYURTMA TAFSILOTI
// ═════════════════════════════════════════════════════════════════════════

/// Bitta buyurtma: holat tarixi, tarkibi, manzil, reys sanasi; kerak bo'lsa bekor qilish.
class QurtOrderDetailScreen extends StatefulWidget {
  const QurtOrderDetailScreen({super.key, required this.orderId, this.initial});
  final String orderId;
  final Map<String, dynamic>? initial;

  @override
  State<QurtOrderDetailScreen> createState() => _QurtOrderDetailScreenState();
}

class _QurtOrderDetailScreenState extends State<QurtOrderDetailScreen> {
  Map<String, dynamic>? _order;
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _order = widget.initial;
    _load();
  }

  Future<void> _load() async {
    try {
      final o = await api.qurtOrder(widget.orderId);
      if (mounted) setState(() => _order = o);
    } catch (e) {
      if (mounted && _order == null) {
        setState(() => _error = errorText(e, 'Buyurtmani yuklab bo\'lmadi'));
      }
    }
  }

  Future<void> _cancel() async {
    final yes = await showDialog<bool>(
      context: context,
      builder:
          (c) => AlertDialog(
            title: const Text('Buyurtmani bekor qilasizmi?'),
            content: const Text(
              'Do\'kon hali ko\'rmagan buyurtmani bekor qilish mumkin.',
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(c, false),
                child: const Text('Yo\'q'),
              ),
              TextButton(
                onPressed: () => Navigator.pop(c, true),
                child: const Text(
                  'Ha, bekor qilish',
                  style: TextStyle(color: Color(0xFFDC2626)),
                ),
              ),
            ],
          ),
    );
    if (yes != true || _busy) return;
    setState(() => _busy = true);
    try {
      final o = await api.cancelQurtOrder(widget.orderId);
      if (mounted) setState(() => _order = o);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(errorText(e, 'Bekor qilib bo\'lmadi'))),
      );
      _load(); // holat o'zgargan bo'lishi mumkin (do'kon tasdiqlagan)
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final o = _order;
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: AppBar(
          backgroundColor: Colors.white,
          surfaceTintColor: Colors.white,
          elevation: 0,
          iconTheme: const IconThemeData(color: _ink),
          title: Text(
            o == null ? 'Buyurtma' : '№ ${o['number']}',
            style: const TextStyle(
              color: _ink,
              fontWeight: FontWeight.w800,
              fontSize: 18,
            ),
          ),
        ),
        body:
            o == null
                ? Center(
                  child:
                      _error == null
                          ? const CircularProgressIndicator(color: kBrand)
                          : Text(
                            _error!,
                            style: const TextStyle(color: _muted),
                          ),
                )
                : RefreshIndicator(
                  color: kBrand,
                  onRefresh: _load,
                  child: _content(o),
                ),
      ),
    );
  }

  Widget _content(Map<String, dynamic> o) {
    final status = (o['status'] ?? '').toString();
    final history = ((o['history'] as List?) ?? []).whereType<Map>().toList();
    final addr =
        (o['address'] is Map)
            ? Map<String, dynamic>.from(o['address'] as Map)
            : {};
    final reason = (o['reject_reason'] ?? '').toString();
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 28),
      children: [
        Row(children: [qurtStatusChip(status)]),
        if (reason.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            'Sabab: $reason',
            style: const TextStyle(color: _muted, fontSize: 13),
          ),
        ],
        const SizedBox(height: 14),
        _infoRow(Icons.storefront_outlined, (o['store_name'] ?? '').toString()),
        _infoRow(
          Icons.local_shipping_outlined,
          'Yetkazish: ${formatQurtDate((o['delivery_date'] ?? '').toString())}',
        ),
        _infoRow(Icons.place_outlined, (addr['text'] ?? '').toString()),
        _infoRow(Icons.payments_outlined, 'Naqd — yetkazilganda to\'lanadi'),
        const SizedBox(height: 14),
        const Text(
          'Tarkibi',
          style: TextStyle(fontWeight: FontWeight.w800, fontSize: 15),
        ),
        const SizedBox(height: 8),
        for (final it in _items(o))
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    '${it['name']} × ${it['qty']}',
                    style: const TextStyle(fontSize: 14),
                  ),
                ),
                Text(
                  formatSum(_tiyin(it['price_tiyin']) * _tiyin(it['qty'])),
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
              ],
            ),
          ),
        const Divider(height: 24),
        Row(
          children: [
            const Expanded(
              child: Text(
                'Jami',
                style: TextStyle(fontWeight: FontWeight.w800, fontSize: 16),
              ),
            ),
            Text(
              formatSum(_tiyin(o['total_tiyin'])),
              style: const TextStyle(
                color: kBrand,
                fontWeight: FontWeight.w800,
                fontSize: 18,
              ),
            ),
          ],
        ),
        const SizedBox(height: 20),
        const Text(
          'Holat tarixi',
          style: TextStyle(fontWeight: FontWeight.w800, fontSize: 15),
        ),
        const SizedBox(height: 8),
        for (final h in history.reversed)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 5),
            child: Row(
              children: [
                Icon(
                  Icons.circle,
                  size: 9,
                  color: qurtStatusColor((h['to'] ?? '').toString()),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    qurtStatusLabel((h['to'] ?? '').toString()),
                    style: const TextStyle(fontSize: 13.5),
                  ),
                ),
                Text(
                  _time(h['at']),
                  style: const TextStyle(color: _muted, fontSize: 12),
                ),
              ],
            ),
          ),
        if (status == 'new') ...[
          const SizedBox(height: 20),
          OutlinedButton.icon(
            key: const ValueKey('qurt-order-cancel'),
            onPressed: _busy ? null : _cancel,
            icon: const Icon(Icons.close, size: 18),
            label: Text(
              _busy ? 'Bekor qilinmoqda...' : 'Buyurtmani bekor qilish',
            ),
            style: OutlinedButton.styleFrom(
              foregroundColor: const Color(0xFFDC2626),
              side: const BorderSide(color: Color(0xFFDC2626)),
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(14),
              ),
            ),
          ),
        ],
      ],
    );
  }

  Widget _infoRow(IconData icon, String text) {
    if (text.trim().isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: _muted),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              text,
              style: const TextStyle(fontSize: 14, color: _ink),
            ),
          ),
        ],
      ),
    );
  }
}

// ═════════════════════════════════════════════════════════════════════════
// BUYURTMA QABUL QILINDI
// ═════════════════════════════════════════════════════════════════════════

/// Buyurtma muvaffaqiyatli yaratilgach ko'rsatiladi.
class QurtOrderPlacedScreen extends StatelessWidget {
  const QurtOrderPlacedScreen({super.key, required this.orders});
  final List<Map<String, dynamic>> orders;

  @override
  Widget build(BuildContext context) {
    final date =
        orders.isEmpty ? '' : (orders.first['delivery_date'] ?? '').toString();
    final total = orders.fold<int>(0, (a, o) => a + _tiyin(o['total_tiyin']));
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        body: SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              children: [
                const Spacer(),
                const CircleAvatar(
                  radius: 38,
                  backgroundColor: Color(0xFFDCFCE7),
                  child: Icon(
                    Icons.check_rounded,
                    size: 46,
                    color: Color(0xFF16A34A),
                  ),
                ),
                const SizedBox(height: 20),
                const Text(
                  'Buyurtmangiz qabul qilindi!',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 22,
                    fontWeight: FontWeight.w800,
                    color: _ink,
                  ),
                ),
                const SizedBox(height: 10),
                Text(
                  orders.length > 1
                      ? '${orders.length} ta do\'kondan buyurtma: ${orders.map((o) => o['number']).join(', ')}'
                      : 'Buyurtma raqami: ${orders.isEmpty ? '' : orders.first['number']}',
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: _muted, fontSize: 14),
                ),
                const SizedBox(height: 18),
                Container(
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: const Color(0xFFFFF4EE),
                    borderRadius: BorderRadius.circular(16),
                  ),
                  child: Column(
                    children: [
                      Row(
                        children: [
                          const Icon(
                            Icons.local_shipping_outlined,
                            color: kBrand,
                          ),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Text(
                              'Yetkazish: ${formatQurtDate(date)}',
                              style: const TextStyle(
                                fontWeight: FontWeight.w700,
                                fontSize: 15,
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Buyurtmalar bir necha kun to\'planib, reys bilan yetkaziladi. '
                        'To\'lov — yetkazilganda naqd: ${formatSum(total)}.',
                        style: const TextStyle(
                          color: _muted,
                          fontSize: 13,
                          height: 1.35,
                        ),
                      ),
                    ],
                  ),
                ),
                const Spacer(),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton(
                    key: const ValueKey('qurt-placed-orders'),
                    onPressed:
                        () => Navigator.of(
                          context,
                        ).pushReplacement(sheetRoute(const QurtOrdersScreen())),
                    style: FilledButton.styleFrom(
                      backgroundColor: kBrand,
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(16),
                      ),
                    ),
                    child: const Text(
                      'Buyurtmalarimga o\'tish',
                      style: TextStyle(fontWeight: FontWeight.w700),
                    ),
                  ),
                ),
                const SizedBox(height: 8),
                TextButton(
                  onPressed: () => Navigator.of(context).maybePop(),
                  child: const Text('Xaridni davom ettirish'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
