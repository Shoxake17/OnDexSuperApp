import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';

/// RS-5 API (ofitsiant).
extension WaiterBillApi on WaiterApi {
  Future<List<Map<String, dynamic>>> openBills() async {
    final l = await send('GET', '/waiter/bills') as List<dynamic>? ?? [];
    return [for (final x in l) Map<String, dynamic>.from(x as Map)];
  }

  Future<void> closeBill(String billId, String method) =>
      send('POST', '/bills/$billId/close', {'method': method});
}

/// Stol hisobi: jami, mehmonlar ilovada to'lagani, choy puli, qolgani.
/// "Naqd qabul qildim" / "Terminal" — qolganini yopadi (mehmon hozir ilovada
/// to'layotgan bo'lsa server kuting deydi). Berilgan (served) buyurtmalar
/// faol ro'yxatdan chiqsa ham hisob shu yerda ko'rinadi.
class TableBillCard extends StatefulWidget {
  const TableBillCard({super.key, required this.tableId});

  final String tableId;

  @override
  State<TableBillCard> createState() => _TableBillCardState();
}

class _TableBillCardState extends State<TableBillCard> {
  Map<String, dynamic>? _bill;
  bool _busy = false;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 15), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final list = await api.openBills();
      Map<String, dynamic>? found;
      for (final b in list) {
        if ((b['bill'] as Map)['table_id'] == widget.tableId) found = b;
      }
      if (mounted) setState(() => _bill = found);
    } catch (_) {
      // Server hali RS-5'siz yoki tarmoq — karta ko'rinmaydi.
    }
  }

  Future<void> _close(String method) async {
    final b = _bill;
    if (b == null || _busy) return;
    final remaining = ((b['remaining_tiyin'] as num?) ?? 0).toInt();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(method == 'cash' ? 'Naqd qabul qildingizmi?' : 'Terminal orqali qabul qildingizmi?'),
        content: Text('${formatSum(remaining)} — hisob yopiladi.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Yo\'q')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Ha, qabul qildim')),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await api.closeBill((b['bill'] as Map)['id'] as String, method);
      messenger.showSnackBar(const SnackBar(content: Text('Hisob yopildi')));
    } on ApiException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final b = _bill;
    if (b == null) return const SizedBox.shrink();
    int n(String k) => ((b[k] as num?) ?? 0).toInt();
    final deposit = (((b['bill'] as Map)['deposit_tiyin'] as num?) ?? 0).toInt();
    Widget row(String l, int v, {bool bold = false, Color? c}) => Row(children: [
          Expanded(child: Text(l, style: TextStyle(color: c ?? kInkFaint, fontWeight: bold ? FontWeight.w700 : null))),
          Text(formatSum(v), style: TextStyle(color: c ?? kInk, fontWeight: bold ? FontWeight.w700 : null)),
        ]);
    return Container(
      key: const ValueKey('table-bill-card'),
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(color: kBorder),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const Text('Stol hisobi', style: TextStyle(fontWeight: FontWeight.w700, color: kInk, fontSize: 15)),
        const SizedBox(height: 8),
        row('Buyurtmalar', n('total_tiyin')),
        if (deposit > 0) row('Bron zakladi', -deposit),
        row('Mehmonlar ilovada to\'ladi', n('paid_tiyin')),
        if (n('pending_tiyin') > 0) row('Hozir to\'lanmoqda', n('pending_tiyin'), c: kReadyColor),
        if (n('tips_tiyin') > 0) row('Choy puli', n('tips_tiyin'), c: kReadyColor),
        const Divider(height: 16),
        row('Qoldi', n('remaining_tiyin'), bold: true, c: kInk),
        if (n('remaining_tiyin') > 0) ...[
          const SizedBox(height: 10),
          Row(children: [
            Expanded(
              child: FilledButton(
                key: const ValueKey('bill-close-cash'),
                onPressed: _busy ? null : () => _close('cash'),
                style: FilledButton.styleFrom(backgroundColor: kReadyColor),
                child: const Text('Naqd'),
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: OutlinedButton(
                onPressed: _busy ? null : () => _close('terminal'),
                child: const Text('Terminal'),
              ),
            ),
          ]),
        ],
      ]),
    );
  }
}
