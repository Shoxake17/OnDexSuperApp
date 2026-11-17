import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../payment_nav_policy.dart';
import '../screens/catalog_screen.dart' show kBrand;
import '../screens/payment_webview_screen.dart';
import '../widgets/pushed_sheet.dart';

/// RS-5 API.
extension TableBillApi on CustomerApi {
  Future<Map<String, dynamic>> orderBill(String orderId) async =>
      Map<String, dynamic>.from(await send('GET', '/orders/$orderId/bill') as Map);

  Future<Map<String, dynamic>> bill(String billId) async =>
      Map<String, dynamic>.from(await send('GET', '/bills/$billId') as Map);

  Future<Map<String, dynamic>> createBillShare(String billId, Map<String, dynamic> body) async =>
      Map<String, dynamic>.from(await send('POST', '/bills/$billId/shares', body) as Map);
}

/// Tanlangan qator kaliti.
String lineKey(String orderId, int line) => '$orderId#$line';

/// O'z taomlari rejimidagi summa (server bilan bir xil qoida: oxirgi qism
/// yaxlitlash qoldig'i bilan). Faqat ko'rsatish uchun — haqiqiy summani server
/// hisoblaydi.
int itemsAmount(Map<String, dynamic> bill, Map<String, int> picked) {
  var sum = 0;
  for (final o in (bill['orders'] as List? ?? const [])) {
    final om = o as Map;
    for (final l in (om['lines'] as List? ?? const [])) {
      final lm = l as Map;
      final q = picked[lineKey('${om['id']}', (lm['line'] as num).toInt())] ?? 0;
      if (q <= 0) continue;
      final qty = (lm['qty'] as num).toInt();
      final paid = (lm['paid_qty'] as num? ?? 0).toInt();
      final unit = (lm['unit_tiyin'] as num).toInt();
      final amount = (lm['amount_tiyin'] as num).toInt();
      sum += q == qty - paid ? amount - unit * paid : unit * q;
    }
  }
  return sum;
}

/// "Hisob" — stol hisobini ko'rish va o'z ulushini to'lash: teng bo'lish yoki
/// o'z taomlari + ixtiyoriy choy puli (ofitsiantga).
class BillScreen extends StatefulWidget {
  const BillScreen({super.key, required this.orderId});

  final String orderId;

  @override
  State<BillScreen> createState() => _BillScreenState();
}

class _BillScreenState extends State<BillScreen> {
  Map<String, dynamic>? _bill;
  String? _error;
  bool _paying = false;
  bool _equal = true;
  int _parts = 2;
  int _tip = 0;
  final Map<String, int> _picked = {};
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    customerLive.start();
    _live = LiveRefresher(bus: customerLive, onRefresh: _load, types: const {'order_status'},
        offlineInterval: const Duration(seconds: 15))
      ..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final b = await api.orderBill(widget.orderId);
      if (!mounted) return;
      setState(() {
        _bill = b;
        _error = null;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.statusCode == 503 ? 'Stol hisobi hali yoqilmagan.' : e.message);
    }
  }

  int get _remaining => ((_bill?['remaining_tiyin'] as num?) ?? 0).toInt();
  int get _due => ((_bill?['due_tiyin'] as num?) ?? 0).toInt();

  int get _amount {
    final b = _bill;
    if (b == null) return 0;
    final raw = _equal ? (_due + _parts - 1) ~/ _parts : itemsAmount(b, _picked);
    return raw > _remaining ? _remaining : raw;
  }

  Future<void> _pay() async {
    final b = _bill;
    if (b == null || _amount <= 0 || _paying) return;
    setState(() => _paying = true);
    try {
      final body = <String, dynamic>{'kind': _equal ? 'equal' : 'items', 'tip_percent': _tip};
      if (_equal) {
        body['parts'] = _parts;
      } else {
        body['items'] = [
          for (final e in _picked.entries)
            if (e.value > 0)
              {'order_id': e.key.split('#').first, 'line': int.parse(e.key.split('#').last), 'qty': e.value},
        ];
      }
      final res = await api.createBillShare((b['bill'] as Map)['id'] as String, body);
      final pay = res['pay'] as Map;
      final url = '${pay['pay_url'] ?? ''}';
      if (!isAcceptablePayUrl(url)) throw ApiException('To\'lov havolasi yaroqsiz');
      if (!mounted) return;
      await Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => PaymentWebViewScreen(payUrl: url, returnUrl: '${pay['return_url'] ?? ''}'),
      ));
      _picked.clear();
    } on ApiException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) setState(() => _paying = false);
    }
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final b = _bill;
    return PushedSheet(
      title: 'Hisob',
      child: SafeArea(
        top: false,
        child: b == null
            ? Center(child: _error == null ? const CircularProgressIndicator() : Text(_error!))
            : RefreshIndicator(onRefresh: _load, child: _body(b)),
      ),
    );
  }

  Widget _row(String l, int v, {bool bold = false, Color? color}) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 2),
        child: Row(children: [
          Expanded(child: Text(l, style: TextStyle(fontWeight: bold ? FontWeight.w700 : null, color: color))),
          Text(formatSum(v), style: TextStyle(fontWeight: bold ? FontWeight.w700 : null, color: color)),
        ]),
      );

  Widget _body(Map<String, dynamic> b) {
    final closed = (b['bill'] as Map)['status'] == 'paid' || _remaining == 0 && ((b['pending_tiyin'] as num?) ?? 0) == 0;
    final deposit = (((b['bill'] as Map)['deposit_tiyin'] as num?) ?? 0).toInt();
    final waiter = '${b['waiter_name'] ?? ''}'.trim();
    return ListView(padding: const EdgeInsets.fromLTRB(16, 8, 16, 24), children: [
      Text('${(b['bill'] as Map)['table_label'] ?? 'Stol'}', style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700)),
      const SizedBox(height: 10),
      _row('Buyurtmalar', ((b['total_tiyin'] as num?) ?? 0).toInt()),
      if (deposit > 0) _row('Bron zakladi', -deposit, color: Colors.green),
      _row('To\'langan', ((b['paid_tiyin'] as num?) ?? 0).toInt()),
      if (((b['pending_tiyin'] as num?) ?? 0) > 0) _row('To\'lanmoqda', ((b['pending_tiyin'] as num?) ?? 0).toInt(), color: Colors.orange),
      _row('Qoldi', _remaining, bold: true),
      const Divider(height: 24),
      if (closed)
        const Padding(
          padding: EdgeInsets.symmetric(vertical: 24),
          child: Text('Hisob to\'liq to\'langan. Rahmat!', textAlign: TextAlign.center,
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: Colors.green)),
        )
      else ...[
        SegmentedButton<bool>(
          segments: const [
            ButtonSegment(value: true, label: Text('Teng bo\'lish')),
            ButtonSegment(value: false, label: Text('O\'z taomlarim')),
          ],
          selected: {_equal},
          onSelectionChanged: (s) => setState(() => _equal = s.first),
        ),
        const SizedBox(height: 12),
        if (_equal)
          Row(children: [
            const Expanded(child: Text('Necha kishiga?')),
            IconButton.filledTonal(
                onPressed: _parts > 1 ? () => setState(() => _parts--) : null, icon: const Icon(Icons.remove)),
            SizedBox(width: 40, child: Text('$_parts', textAlign: TextAlign.center, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700))),
            IconButton.filledTonal(
                onPressed: _parts < 30 ? () => setState(() => _parts++) : null, icon: const Icon(Icons.add)),
          ])
        else
          for (final o in (b['orders'] as List? ?? const []))
            for (final l in ((o as Map)['lines'] as List? ?? const []))
              _lineTile(o, l as Map),
        const SizedBox(height: 12),
        const Text('Choy puli (ofitsiantga)', style: TextStyle(fontWeight: FontWeight.w600)),
        const SizedBox(height: 6),
        Wrap(spacing: 8, children: [
          for (final p in const [0, 5, 10])
            ChoiceChip(
              label: Text(p == 0 ? 'Yo\'q' : '$p%'),
              selected: _tip == p,
              selectedColor: kBrand.withValues(alpha: 0.15),
              onSelected: (_) => setState(() => _tip = p),
            ),
        ]),
        if (waiter.isNotEmpty)
          Padding(padding: const EdgeInsets.only(top: 6), child: Text('Ofitsiant: $waiter', style: const TextStyle(color: Colors.black54))),
        const SizedBox(height: 16),
        FilledButton(
          key: const ValueKey('pay-bill-share'),
          style: FilledButton.styleFrom(backgroundColor: kBrand, minimumSize: const Size.fromHeight(52)),
          onPressed: _amount <= 0 || _paying ? null : _pay,
          child: _paying
              ? const SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
              : Text('${formatSum(_amount + _amount * _tip ~/ 100)} to\'lash'),
        ),
        const SizedBox(height: 8),
        const Text('Qolganini ofitsiantga naqd yoki karta bilan to\'lashingiz mumkin.',
            textAlign: TextAlign.center, style: TextStyle(color: Colors.black54, fontSize: 12.5)),
      ],
    ]);
  }

  Widget _lineTile(Map o, Map l) {
    final qty = (l['qty'] as num).toInt();
    final paid = (l['paid_qty'] as num? ?? 0).toInt();
    final free = qty - paid;
    final key = lineKey('${o['id']}', (l['line'] as num).toInt());
    final picked = _picked[key] ?? 0;
    return ListTile(
      contentPadding: EdgeInsets.zero,
      title: Text('${l['name']}'),
      subtitle: Text(free <= 0 ? 'To\'langan' : '$free ta to\'lanmagan · ${formatSum((l['unit_tiyin'] as num).toInt())} dan'),
      trailing: free <= 0
          ? const Icon(Icons.check_circle, color: Colors.green)
          : Row(mainAxisSize: MainAxisSize.min, children: [
              IconButton(
                  onPressed: picked > 0 ? () => setState(() => _picked[key] = picked - 1) : null,
                  icon: const Icon(Icons.remove_circle_outline)),
              Text('$picked'),
              IconButton(
                  onPressed: picked < free ? () => setState(() => _picked[key] = picked + 1) : null,
                  icon: const Icon(Icons.add_circle_outline)),
            ]),
    );
  }
}
