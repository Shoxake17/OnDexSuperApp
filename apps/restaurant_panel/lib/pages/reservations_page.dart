import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../theme.dart';
import '../widgets/page_header.dart';
import 'reservations/reservations_api.dart';
import 'reservations/reservation_settings_dialog.dart';

/// "Bronlar" (RS-4) — kun bo'yicha stol bronlari: kim, qachon, nechta
/// kishi, qaysi stol, zaklad holati; amallar — keldi, kelmadi, yakunlash,
/// bekor qilish (sabab bilan, zaklad mijozga to'liq qaytadi).
///
/// Jonli: yangi bron/bekor qilish restoran kanali orqali darhol ko'rinadi.
class ReservationsPage extends StatefulWidget {
  const ReservationsPage({super.key});

  @override
  State<ReservationsPage> createState() => _ReservationsPageState();
}

/// Toshkent (UTC+5) — server vaqtlari shu zonada ko'rsatiladi.
DateTime tashkent(DateTime utc) => utc.toUtc().add(const Duration(hours: 5));

String _hm(DateTime d) => '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';

const _months = ['yanvar', 'fevral', 'mart', 'aprel', 'may', 'iyun', 'iyul', 'avgust',
  'sentabr', 'oktabr', 'noyabr', 'dekabr'];
const _weekdays = ['Dushanba', 'Seshanba', 'Chorshanba', 'Payshanba', 'Juma', 'Shanba', 'Yakshanba'];

/// Holat matni va ranglari.
(String, Color, Color) reservationStatusStyle(String s) => switch (s) {
      'held' => ('Zaklad kutilmoqda', OnDexColors.amber, OnDexColors.amberBg),
      'confirmed' => ('Tasdiqlangan', OnDexColors.info, OnDexColors.infoBg),
      'seated' => ('Keldi', OnDexColors.success, OnDexColors.successBg),
      'completed' => ('Yakunlandi', OnDexColors.inkDim, OnDexColors.pageBg),
      'cancelled' => ('Bekor qilingan', OnDexColors.danger, OnDexColors.dangerBg),
      'no_show' => ('Kelmadi', OnDexColors.warning, OnDexColors.warningBg),
      'expired' => ('To\'lanmadi', OnDexColors.inkDim, OnDexColors.pageBg),
      _ => (s, OnDexColors.inkDim, OnDexColors.pageBg),
    };

/// Zaklad holati matni.
String depositText(Map<String, dynamic> r) {
  final amount = (r['deposit_tiyin'] as num?)?.toInt() ?? 0;
  if (amount == 0) return 'Zakladsiz';
  final sum = formatSum(amount);
  final via = r['deposit_method'] == 'wallet' ? 'Wallet' : 'karta';
  return switch (r['deposit_state']) {
    'pending' => 'Zaklad $sum — to\'lanmagan',
    'paid' => 'Zaklad $sum to\'langan ($via)',
    'applied' => 'Zaklad $sum — hisobdan ayiring',
    'refunded' => 'Zaklad $sum mijozga qaytarildi',
    'kept' => 'Zaklad $sum restoranda qoldi',
    _ => 'Zaklad $sum',
  };
}

class _ReservationsPageState extends State<ReservationsPage> {
  DateTime _day = tashkent(DateTime.now());
  List<Map<String, dynamic>> _items = [];
  bool _loading = true;
  String? _error;
  final Set<String> _busy = {};
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    _live = LiveRefresher(
      bus: restaurantLive,
      onRefresh: _load,
      types: const {
        'reservation_new', 'reservation_confirmed', 'reservation_cancelled', 'reservation_seated',
        'reservation_no_show', 'reservation_expired', 'reservation_reminder',
      },
      offlineInterval: const Duration(seconds: 30),
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  /// Tanlangan Toshkent kunining UTC chegaralari.
  (DateTime, DateTime) _range() {
    final startUtc = DateTime.utc(_day.year, _day.month, _day.day).subtract(const Duration(hours: 5));
    return (startUtc, startUtc.add(const Duration(days: 1)));
  }

  Future<void> _load() async {
    final (from, to) = _range();
    try {
      final list = await api.reservations(from, to);
      list.sort((a, b) => '${a['starts_at']}'.compareTo('${b['starts_at']}'));
      if (!mounted) return;
      setState(() {
        _items = list;
        _loading = false;
        _error = null;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.statusCode == 503
            ? 'Stol broni serverda hali yoqilmagan.'
            : 'Bronlarni yuklab bo\'lmadi: ${e.message}';
      });
    }
  }

  void _shiftDay(int days) {
    setState(() {
      _day = _day.add(Duration(days: days));
      _loading = true;
    });
    _load();
  }

  void _toast(String m) => ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(m)));

  Future<void> _run(String id, Future<void> Function() action, String done) async {
    if (_busy.contains(id)) return;
    setState(() => _busy.add(id));
    try {
      await action();
      _toast(done);
    } on ApiException catch (e) {
      _toast(e.message);
    } finally {
      if (mounted) setState(() => _busy.remove(id));
    }
    await _load();
  }

  Future<void> _cancel(Map<String, dynamic> r) async {
    final ctrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Bron bekor qilinsinmi?'),
        content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(((r['deposit_tiyin'] as num?) ?? 0) > 0
              ? 'Mijozga xabar boradi va zaklad TO\'LIQ qaytariladi.'
              : 'Mijozga xabar boradi.'),
          const SizedBox(height: 12),
          TextField(
            controller: ctrl,
            autofocus: true,
            maxLength: 300,
            decoration: const InputDecoration(labelText: 'Sabab (mijoz ko\'radi)', border: OutlineInputBorder()),
          ),
        ]),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Yo\'q')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: OnDexColors.danger),
            onPressed: () {
              final v = ctrl.text.trim();
              if (v.isNotEmpty) Navigator.pop(ctx, v);
            },
            child: const Text('Bekor qilish'),
          ),
        ],
      ),
    );
    if (reason == null) return;
    await _run(r['id'] as String, () => api.cancelReservation(r['id'] as String, reason), 'Bron bekor qilindi');
  }

  Future<void> _openSettings() async {
    final saved = await showReservationSettingsDialog(context);
    if (saved == true) _toast('Bron sozlamalari saqlandi');
  }

  @override
  Widget build(BuildContext context) {
    final active = _items.where((r) => ['held', 'confirmed', 'seated'].contains(r['status'])).toList();
    final guests = active.fold<int>(0, (s, r) => s + ((r['party_size'] as num?)?.toInt() ?? 0));
    final title = '${_day.day} ${_months[_day.month - 1]}, ${_weekdays[_day.weekday - 1]}';
    return PageScaffold(
      title: 'Bronlar',
      subtitle: 'Stol bronlari va zakladlar',
      error: _error,
      action: PageActionButton(icon: Icons.tune_rounded, label: 'Bron sozlamalari', onPressed: _openSettings),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          IconButton(tooltip: 'Oldingi kun', onPressed: () => _shiftDay(-1), icon: const Icon(Icons.chevron_left_rounded)),
          Text(title, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
          IconButton(tooltip: 'Keyingi kun', onPressed: () => _shiftDay(1), icon: const Icon(Icons.chevron_right_rounded)),
          TextButton(
            onPressed: () {
              setState(() => _day = tashkent(DateTime.now()));
              _load();
            },
            child: const Text('Bugun'),
          ),
          const Spacer(),
          Text('${active.length} ta faol bron · $guests kishi',
              style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim, fontWeight: FontWeight.w600)),
        ]),
        const SizedBox(height: 12),
        Expanded(
          child: _loading
              ? const Center(child: CircularProgressIndicator())
              : _items.isEmpty
                  ? const Center(
                      child: Text('Bu kunga bron yo\'q', style: TextStyle(color: OnDexColors.inkDim, fontSize: 14)))
                  : ListView.separated(
                      itemCount: _items.length,
                      separatorBuilder: (_, __) => const SizedBox(height: 10),
                      itemBuilder: (_, i) => _ReservationCard(
                        r: _items[i],
                        busy: _busy.contains(_items[i]['id']),
                        onSeat: () => _run(_items[i]['id'] as String,
                            () => api.reservationAction(_items[i]['id'] as String, 'seat'), 'Mehmon keldi deb belgilandi'),
                        onComplete: () => _run(_items[i]['id'] as String,
                            () => api.reservationAction(_items[i]['id'] as String, 'complete'), 'Stol bo\'shadi'),
                        onNoShow: () => _run(_items[i]['id'] as String,
                            () => api.reservationAction(_items[i]['id'] as String, 'no-show'), 'Kelmadi deb belgilandi'),
                        onCancel: () => _cancel(_items[i]),
                      ),
                    ),
        ),
      ]),
    );
  }
}

class _ReservationCard extends StatelessWidget {
  const _ReservationCard({
    required this.r,
    required this.busy,
    required this.onSeat,
    required this.onComplete,
    required this.onNoShow,
    required this.onCancel,
  });

  final Map<String, dynamic> r;
  final bool busy;
  final VoidCallback onSeat, onComplete, onNoShow, onCancel;

  @override
  Widget build(BuildContext context) {
    final start = tashkent(DateTime.parse(r['starts_at'] as String));
    final end = tashkent(DateTime.parse(r['ends_at'] as String));
    final status = '${r['status']}';
    final (label, fg, bg) = reservationStatusStyle(status);
    final name = '${r['customer_name'] ?? ''}'.trim();
    final phone = '${r['customer_phone'] ?? ''}';
    final comment = '${r['comment'] ?? ''}'.trim();
    final reason = '${r['cancel_reason'] ?? ''}'.trim();
    final noShowReady = DateTime.now().toUtc().isAfter(DateTime.parse(r['starts_at'] as String).add(const Duration(minutes: 15)));

    final actions = <Widget>[
      if (status == 'confirmed') FilledButton.icon(onPressed: busy ? null : onSeat, icon: const Icon(Icons.how_to_reg_rounded, size: 18), label: const Text('Keldi')),
      if (status == 'confirmed' && noShowReady)
        OutlinedButton(onPressed: busy ? null : onNoShow, child: const Text('Kelmadi')),
      if (status == 'seated') FilledButton(onPressed: busy ? null : onComplete, child: const Text('Stol bo\'shadi')),
      if (status == 'held' || status == 'confirmed')
        TextButton(
          onPressed: busy ? null : onCancel,
          style: TextButton.styleFrom(foregroundColor: OnDexColors.danger),
          child: const Text('Bekor qilish'),
        ),
    ];

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        SizedBox(
          width: 86,
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(_hm(start), style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
            Text('— ${_hm(end)}', style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
          ]),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Wrap(spacing: 8, runSpacing: 6, crossAxisAlignment: WrapCrossAlignment.center, children: [
              Text('${r['table_label']} · ${r['party_size']} kishi',
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: OnDexColors.primary)),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(6)),
                child: Text(label, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: fg)),
              ),
            ]),
            const SizedBox(height: 6),
            Text([if (name.isNotEmpty) name, if (phone.isNotEmpty) phone].join(' · '),
                style: const TextStyle(fontSize: 13.5, color: OnDexColors.ink)),
            Text(depositText(r), style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
            if (comment.isNotEmpty)
              Text('Izoh: $comment', style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
            if (reason.isNotEmpty && status == 'cancelled')
              Text('Sabab: $reason', style: const TextStyle(fontSize: 12.5, color: OnDexColors.danger)),
            if (actions.isNotEmpty) ...[
              const SizedBox(height: 10),
              Wrap(spacing: 8, runSpacing: 8, children: actions),
            ],
          ]),
        ),
      ]),
    );
  }
}
