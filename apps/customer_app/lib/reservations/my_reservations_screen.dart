import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../payment_nav_policy.dart';
import '../screens/catalog_screen.dart' show kBrand;
import '../screens/payment_webview_screen.dart';
import '../widgets/sheet_scaffold.dart';
import 'reservation_api.dart';
import 'reservation_screen.dart' show hhmm, toTashkent;

/// Holat nomi va rangi (mijoz uchun).
(String, Color) customerReservationStatus(String s) => switch (s) {
      'held' => ('Zaklad to\'lanishi kutilmoqda', Colors.orange),
      'confirmed' => ('Tasdiqlangan', Colors.green),
      'seated' => ('Restoranda', Colors.blue),
      'completed' => ('Yakunlangan', Colors.grey),
      'cancelled' => ('Bekor qilingan', Colors.red),
      'no_show' => ('Kelmadingiz', Colors.red),
      'expired' => ('To\'lanmagani uchun bekor', Colors.grey),
      _ => (s, Colors.grey),
    };

/// Zaklad bo'yicha mijozga tushunarli matn.
String customerDepositText(Map<String, dynamic> r) {
  final amount = (r['deposit_tiyin'] as num?)?.toInt() ?? 0;
  if (amount == 0) return '';
  final sum = formatSum(amount);
  return switch (r['deposit_state']) {
    'pending' => 'Zaklad $sum — to\'lanmagan',
    'paid' => 'Zaklad $sum to\'langan',
    'applied' => 'Zaklad $sum hisobingizdan ayriladi',
    'refunded' => 'Zaklad $sum qaytarildi',
    'kept' => 'Zaklad $sum qaytarilmaydi',
    _ => 'Zaklad $sum',
  };
}

/// "Bronlarim" — mijozning stol bronlari (jonli).
class MyReservationsScreen extends StatefulWidget {
  const MyReservationsScreen({super.key});

  @override
  State<MyReservationsScreen> createState() => _MyReservationsScreenState();
}

class _MyReservationsScreenState extends State<MyReservationsScreen> {
  List<Map<String, dynamic>> _items = [];
  bool _loading = true;
  String? _error;
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    customerLive.start();
    _live = LiveRefresher(
      bus: customerLive,
      onRefresh: _load,
      types: const {'reservation_new', 'reservation_cancelled', 'reservation_expired', 'reservation_reminder'},
      offlineInterval: const Duration(seconds: 20),
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final l = await api.myReservations();
      if (!mounted) return;
      setState(() {
        _items = l;
        _loading = false;
        _error = null;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        if (_items.isEmpty) _error = e.message;
      });
    }
  }

  void _toast(String m) => ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(m)));

  Future<void> _cancel(Map<String, dynamic> r) async {
    final until = DateTime.tryParse('${r['free_cancel_until']}');
    final free = until != null && DateTime.now().toUtc().isBefore(until);
    final paid = r['deposit_state'] == 'paid';
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Bron bekor qilinsinmi?'),
        content: Text(!paid
            ? 'Stol bo\'shatiladi.'
            : free
                ? 'Zaklad to\'liq qaytariladi.'
                : 'Bron vaqtigacha 2 soatdan kam qoldi — zaklad QAYTARILMAYDI.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Yo\'q')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Bekor qilish'),
          ),
        ],
      ),
    );
    if (yes != true) return;
    try {
      final x = await api.cancelMyReservation(r['id'] as String);
      _toast(x['deposit_state'] == 'refunded' ? 'Bron bekor qilindi, zaklad qaytarildi' : 'Bron bekor qilindi');
    } on ApiException catch (e) {
      _toast(e.message);
    }
    _load();
  }

  Future<void> _pay(Map<String, dynamic> r) async {
    try {
      final p = await api.reservationPayLink(r['id'] as String);
      final url = '${p['pay_url'] ?? ''}';
      if (!isAcceptablePayUrl(url)) throw ApiException('To\'lov havolasi yaroqsiz');
      if (!mounted) return;
      await Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => PaymentWebViewScreen(payUrl: url, returnUrl: '${p['return_url'] ?? ''}'),
      ));
    } on ApiException catch (e) {
      _toast(e.message);
    }
    _load();
  }

  @override
  Widget build(BuildContext context) {
    return SheetScaffold(
      title: 'Bronlarim',
      child: SafeArea(
        top: false,
        child: RefreshIndicator(
          onRefresh: _load,
          child: _loading
              ? const Center(child: CircularProgressIndicator())
              : _items.isEmpty
                  ? ListView(children: [
                      const SizedBox(height: 120),
                      Center(
                        child: Text(_error ?? 'Hozircha bron yo\'q', style: const TextStyle(color: Colors.grey)),
                      ),
                    ])
                  : ListView.separated(
                      padding: const EdgeInsets.all(16),
                      itemCount: _items.length,
                      separatorBuilder: (_, _) => const SizedBox(height: 12),
                      itemBuilder: (_, i) => _card(_items[i]),
                    ),
        ),
      ),
    );
  }

  Widget _card(Map<String, dynamic> r) {
    final start = toTashkent(DateTime.parse(r['starts_at'] as String));
    final (label, color) = customerReservationStatus('${r['status']}');
    final dep = customerDepositText(r);
    final status = '${r['status']}';
    final reason = '${r['cancel_reason'] ?? ''}'.trim();
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: Colors.black12),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Expanded(
            child: Text('${r['restaurant_name'] ?? 'Restoran'}',
                style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
          ),
          Text(label, style: TextStyle(color: color, fontWeight: FontWeight.w700, fontSize: 12.5)),
        ]),
        const SizedBox(height: 4),
        Text('${start.day.toString().padLeft(2, '0')}.${start.month.toString().padLeft(2, '0')} · ${hhmm(start)} · '
            '${r['party_size']} kishi · ${r['table_label']}'),
        if (dep.isNotEmpty) Text(dep, style: const TextStyle(color: Colors.black54, fontSize: 13)),
        if (status == 'cancelled' && reason.isNotEmpty)
          Text('Sabab: $reason', style: const TextStyle(color: Colors.red, fontSize: 13)),
        if (status == 'held' || status == 'confirmed')
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Wrap(spacing: 8, children: [
              if (status == 'held' && r['deposit_method'] == 'card')
                FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: kBrand),
                  onPressed: () => _pay(r),
                  child: const Text('Zakladni to\'lash'),
                ),
              OutlinedButton(onPressed: () => _cancel(r), child: const Text('Bekor qilish')),
            ]),
          ),
      ]),
    );
  }
}
