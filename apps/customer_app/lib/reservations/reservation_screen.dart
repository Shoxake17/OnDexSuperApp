import 'package:flutter/material.dart';

import '../api.dart';
import '../payment_nav_policy.dart';
import '../screens/catalog_screen.dart' show kBrand;
import '../screens/payment_webview_screen.dart';
import '../widgets/sheet_page.dart';
import '../widgets/sheet_scaffold.dart';
import 'my_reservations_screen.dart';
import 'reservation_api.dart';

/// Toshkent (UTC+5) vaqti — bron vaqtlari shu zonada ko'rsatiladi.
DateTime toTashkent(DateTime utc) => utc.toUtc().add(const Duration(hours: 5));

String hhmm(DateTime d) => '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';

const _wd = ['Du', 'Se', 'Ch', 'Pa', 'Ju', 'Sh', 'Ya'];

/// "Stol bron qilish" (RS-3): kishi soni, kun, bo'sh vaqt, zaklad usuli.
///
/// Zaklad qoidasi (egasi qarori) mijozga OLDINDAN aniq ko'rsatiladi:
/// 2+ soat oldin bekor = to'liq qaytadi; kech yoki kelmasa — qaytmaydi;
/// kelganda yakuniy hisobdan ayriladi.
class ReservationScreen extends StatefulWidget {
  const ReservationScreen({super.key, required this.restaurantId, required this.restaurantName});

  final String restaurantId;
  final String restaurantName;

  @override
  State<ReservationScreen> createState() => _ReservationScreenState();
}

class _ReservationScreenState extends State<ReservationScreen> {
  int _party = 2;
  late DateTime _day = _today();
  DateTime? _slot;
  Map<String, dynamic>? _avail;
  bool _loading = true;
  bool _submitting = false;
  String? _error;
  String _method = 'card';
  final _comment = TextEditingController();

  static DateTime _today() {
    final t = toTashkent(DateTime.now());
    return DateTime(t.year, t.month, t.day);
  }

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _comment.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
      _slot = null;
    });
    try {
      final a = await api.reservationAvailability(widget.restaurantId, _day, _party);
      if (!mounted) return;
      setState(() {
        _avail = a;
        _loading = false;
        if (a['card'] != true && a['wallet'] == true) _method = 'wallet';
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.statusCode == 422 ? 'Bu restoranda hozircha stol broni yo\'q.' : e.message;
      });
    }
  }

  List<Map<String, dynamic>> get _slots => [
        for (final s in (_avail?['slots'] as List? ?? const [])) Map<String, dynamic>.from(s as Map),
      ];

  int get _deposit {
    for (final s in _slots) {
      if (DateTime.parse(s['starts_at'] as String).isAtSameMomentAs(_slot ?? DateTime(0))) {
        return (s['deposit_tiyin'] as num?)?.toInt() ?? 0;
      }
    }
    return 0;
  }

  Future<void> _submit() async {
    final slot = _slot;
    if (slot == null || _submitting) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      final res = await api.createReservation(widget.restaurantId,
          startsAt: slot, partySize: _party, method: _deposit > 0 ? _method : '', comment: _comment.text.trim());
      final pay = res['pay'] as Map?;
      if (pay != null) {
        final url = '${pay['pay_url'] ?? ''}';
        if (!isAcceptablePayUrl(url)) throw ApiException('To\'lov havolasi yaroqsiz');
        if (!mounted) return;
        await Navigator.of(context).push(MaterialPageRoute(
          builder: (_) => PaymentWebViewScreen(payUrl: url, returnUrl: '${pay['return_url'] ?? ''}'),
        ));
      }
      if (!mounted) return;
      // Holat (to'landimi) — serverdan; "Bronlarim" ro'yxati jonli yangilanadi.
      Navigator.of(context).pushReplacement(sheetRoute(const MyReservationsScreen()));
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = e.statusCode == 402
            ? 'OnDex Wallet balansi yetarli emas — karta bilan to\'lang.'
            : e.message;
      });
      if (e.statusCode == 409) _load();
    }
  }

  @override
  Widget build(BuildContext context) {
    final a = _avail;
    final perPerson = (a?['deposit_per_person_tiyin'] as num?)?.toInt() ?? 0;
    final hours = (a?['free_cancel_hours'] as num?)?.toInt() ?? 2;
    final slotMin = (a?['slot_minutes'] as num?)?.toInt() ?? 120;
    final deposit = _deposit;
    return SheetScaffold(
      title: 'Stol bron qilish',
      child: SafeArea(
        top: false,
        child: ListView(padding: const EdgeInsets.fromLTRB(16, 8, 16, 24), children: [
          Text(widget.restaurantName, style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700)),
          const SizedBox(height: 16),
          Row(children: [
            const Expanded(child: Text('Necha kishi?', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600))),
            IconButton.filledTonal(
              key: const ValueKey('party-minus'),
              onPressed: _party > 1
                  ? () {
                      _party--;
                      _load();
                    }
                  : null,
              icon: const Icon(Icons.remove),
            ),
            SizedBox(width: 44, child: Text('$_party', textAlign: TextAlign.center, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700))),
            IconButton.filledTonal(
              key: const ValueKey('party-plus'),
              onPressed: _party < 20
                  ? () {
                      _party++;
                      _load();
                    }
                  : null,
              icon: const Icon(Icons.add),
            ),
          ]),
          const SizedBox(height: 16),
          SizedBox(
            height: 64,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: 14,
              separatorBuilder: (_, _) => const SizedBox(width: 8),
              itemBuilder: (_, i) {
                final d = _today().add(Duration(days: i));
                final sel = d == _day;
                return ChoiceChip(
                  selected: sel,
                  selectedColor: kBrand.withValues(alpha: 0.15),
                  label: Column(mainAxisSize: MainAxisSize.min, children: [
                    Text(i == 0 ? 'Bugun' : (i == 1 ? 'Ertaga' : _wd[d.weekday - 1]), style: const TextStyle(fontSize: 12)),
                    Text('${d.day}.${d.month.toString().padLeft(2, '0')}', style: const TextStyle(fontWeight: FontWeight.w700)),
                  ]),
                  onSelected: (_) {
                    _day = d;
                    _load();
                  },
                );
              },
            ),
          ),
          const SizedBox(height: 16),
          if (_loading)
            const Padding(padding: EdgeInsets.all(24), child: Center(child: CircularProgressIndicator()))
          else if (a != null && _slots.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 24),
              child: Text(
                  ((a['bookable_tables'] as num?) ?? 1) == 0
                      ? 'Restoran hali bron uchun stollarni sozlamagan. Keyinroq urinib ko\'ring yoki restoranga qo\'ng\'iroq qiling.'
                      : 'Bu kunga bo\'sh joy yo\'q — boshqa kunni tanlang.',
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: Colors.grey)),
            )
          else if (a != null) ...[
            Text('Vaqt (stol $slotMin daqiqaga band qilinadi)', style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
            const SizedBox(height: 8),
            Wrap(spacing: 8, runSpacing: 8, children: [
              for (final s in _slots)
                ChoiceChip(
                  label: Text(hhmm(toTashkent(DateTime.parse(s['starts_at'] as String)))),
                  selected: _slot != null && DateTime.parse(s['starts_at'] as String).isAtSameMomentAs(_slot!),
                  selectedColor: kBrand.withValues(alpha: 0.15),
                  onSelected: (_) => setState(() => _slot = DateTime.parse(s['starts_at'] as String)),
                ),
            ]),
          ],
          if (a != null && _slot != null) ...[
            const SizedBox(height: 20),
            if (deposit > 0) ...[
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(color: kBrand.withValues(alpha: 0.07), borderRadius: BorderRadius.circular(12)),
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text('Zaklad: ${formatSum(deposit)}', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                  if (perPerson > 0)
                    Text('Kishi boshiga ${formatSum(perPerson)}', style: const TextStyle(fontSize: 12.5, color: Colors.black54)),
                  const SizedBox(height: 8),
                  Text(
                    '• Kelganingizda zaklad yakuniy hisobdan ayriladi.\n'
                    '• Bron vaqtidan $hours soat oldin bekor qilsangiz — to\'liq qaytariladi.\n'
                    '• Kechroq bekor qilsangiz yoki kelmasangiz — qaytarilmaydi.',
                    style: const TextStyle(fontSize: 13, height: 1.45),
                  ),
                ]),
              ),
              const SizedBox(height: 12),
              const Text('Zaklad to\'lovi', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
              RadioGroup<String>(
                groupValue: _method,
                onChanged: (v) => setState(() => _method = v ?? _method),
                child: Column(children: [
                  if (a['card'] == true)
                    const RadioListTile<String>(
                        value: 'card',
                        title: Text('Karta (Payme, Click, Uzum, Humo, Uzcard)'),
                        contentPadding: EdgeInsets.zero),
                  if (a['wallet'] == true)
                    const RadioListTile<String>(
                        value: 'wallet', title: Text('OnDex Wallet'), contentPadding: EdgeInsets.zero),
                ]),
              ),
            ] else
              const Text('Bu restoranda bron uchun zaklad olinmaydi.', style: TextStyle(color: Colors.black54)),
            const SizedBox(height: 8),
            TextField(
              controller: _comment,
              maxLength: 300,
              decoration: const InputDecoration(labelText: 'Izoh (ixtiyoriy)', hintText: 'Masalan: tug\'ilgan kun, deraza yonida', border: OutlineInputBorder()),
            ),
          ],
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: const TextStyle(color: Colors.red)),
          ],
          const SizedBox(height: 16),
          FilledButton(
            key: const ValueKey('reserve-submit'),
            style: FilledButton.styleFrom(backgroundColor: kBrand, minimumSize: const Size.fromHeight(52)),
            onPressed: _slot == null || _submitting ? null : _submit,
            child: _submitting
                ? const SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                : Text(deposit > 0 ? 'Bron qilish va ${formatSum(deposit)} to\'lash' : 'Bron qilish'),
          ),
        ]),
      ),
    );
  }
}
