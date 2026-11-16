import 'dart:async';

import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';

/// RS-5 API (panel).
extension BillsApi on RestaurantApi {
  Future<List<Map<String, dynamic>>> openBills() async {
    final l = await send('GET', '/restaurants/$rid/bills') as List<dynamic>? ?? [];
    return [for (final x in l) Map<String, dynamic>.from(x as Map)];
  }

  Future<List<Map<String, dynamic>>> tips(DateTime from, DateTime to) async {
    final q = 'from=${Uri.encodeQueryComponent(from.toUtc().toIso8601String())}'
        '&to=${Uri.encodeQueryComponent(to.toUtc().toIso8601String())}';
    final l = await send('GET', '/restaurants/$rid/tips?$q') as List<dynamic>? ?? [];
    return [for (final x in l) Map<String, dynamic>.from(x as Map)];
  }

  Future<void> closeBill(String billId, String method) => send('POST', '/bills/$billId/close', {'method': method});
}

/// "Hisoblar": stollarning ochiq hisoblari (mehmonlar ilovada qancha to'lagani,
/// qolgani, naqd/terminal yopish) va oxirgi 7 kun choy pullari ofitsiant bo'yicha.
Future<void> showBillsDialog(BuildContext context) => showDialog<void>(
      context: context,
      builder: (_) => const Dialog(insetPadding: EdgeInsets.all(24), child: _BillsBody()),
    );

class _BillsBody extends StatefulWidget {
  const _BillsBody();

  @override
  State<_BillsBody> createState() => _BillsBodyState();
}

class _BillsBodyState extends State<_BillsBody> {
  List<Map<String, dynamic>> _bills = [];
  List<Map<String, dynamic>> _tips = [];
  String? _error;
  final Set<String> _busy = {};
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 10), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final now = DateTime.now();
      final b = await api.openBills();
      final t = await api.tips(now.subtract(const Duration(days: 7)), now.add(const Duration(minutes: 1)));
      if (!mounted) return;
      setState(() {
        _bills = b;
        _tips = t;
        _error = null;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.statusCode == 503 ? 'Stol hisobi serverda hali yoqilmagan.' : e.message);
    }
  }

  Future<void> _close(Map<String, dynamic> b, String method) async {
    final id = (b['bill'] as Map)['id'] as String;
    if (_busy.contains(id)) return;
    setState(() => _busy.add(id));
    final messenger = ScaffoldMessenger.maybeOf(context);
    try {
      await api.closeBill(id, method);
    } on ApiException catch (e) {
      messenger?.showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) setState(() => _busy.remove(id));
    }
    await _load();
  }

  int _n(Map m, String k) => ((m[k] as num?) ?? 0).toInt();

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 860,
      height: 600,
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Row(children: [
            const Text('Stol hisoblari', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
            const Spacer(),
            IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close_rounded)),
          ]),
          if (_error != null) Text(_error!, style: const TextStyle(color: OnDexColors.danger)),
          const SizedBox(height: 8),
          Expanded(
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Expanded(
                flex: 3,
                child: _bills.isEmpty
                    ? const Center(child: Text('Ochiq hisob yo\'q', style: TextStyle(color: OnDexColors.inkDim)))
                    : ListView.separated(
                        itemCount: _bills.length,
                        separatorBuilder: (_, __) => const Divider(height: 20),
                        itemBuilder: (_, i) {
                          final b = _bills[i];
                          final id = (b['bill'] as Map)['id'] as String;
                          final remaining = _n(b, 'remaining_tiyin');
                          return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                            Text('${(b['bill'] as Map)['table_label']}',
                                style: const TextStyle(fontWeight: FontWeight.w700, color: OnDexColors.primary, fontSize: 15)),
                            Text('Jami ${formatSum(_n(b, 'total_tiyin'))} · ilovada to\'landi ${formatSum(_n(b, 'paid_tiyin'))}'
                                '${_n(b, 'pending_tiyin') > 0 ? ' · to\'lanmoqda ${formatSum(_n(b, 'pending_tiyin'))}' : ''}',
                                style: const TextStyle(color: OnDexColors.inkDim, fontSize: 13)),
                            Text('Qoldi: ${formatSum(remaining)}',
                                style: const TextStyle(fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                            if (remaining > 0)
                              Padding(
                                padding: const EdgeInsets.only(top: 6),
                                child: Wrap(spacing: 8, children: [
                                  FilledButton(
                                      onPressed: _busy.contains(id) ? null : () => _close(b, 'cash'),
                                      child: const Text('Naqd qabul qilindi')),
                                  OutlinedButton(
                                      onPressed: _busy.contains(id) ? null : () => _close(b, 'terminal'),
                                      child: const Text('Terminal')),
                                ]),
                              ),
                          ]);
                        },
                      ),
              ),
              const VerticalDivider(width: 32),
              Expanded(
                flex: 2,
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  const Text('Choy pullari (7 kun)', style: TextStyle(fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                  const SizedBox(height: 8),
                  if (_tips.isEmpty) const Text('Hali yo\'q', style: TextStyle(color: OnDexColors.inkDim)),
                  for (final t in _tips)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 4),
                      child: Row(children: [
                        Expanded(child: Text('${'${t['waiter_name'] ?? ''}'.trim().isEmpty ? 'Biriktirilmagan' : t['waiter_name']} (${t['count']})')),
                        Text(formatSum(_n(t, 'tips_tiyin')), style: const TextStyle(fontWeight: FontWeight.w700)),
                      ]),
                    ),
                ]),
              ),
            ]),
          ),
        ]),
      ),
    );
  }
}
