import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import 'ar_table_screen.dart';
import '../live.dart';
import '../widgets/common.dart';
import '../widgets/order_status.dart';
import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import '../widgets/product_grid.dart' show discountLineLabel;
import 'catalog_screen.dart' show kBrand;
import 'payment_webview_screen.dart';

/// Buyurtma kuzatuvi — NATIVE (mobil oqimdagi OXIRGI WebView shu edi).
///
/// ┌─ JONLI YANGILANISH ───────────────────────────────────────────────┐
/// Holat WebSocket orqali keladi (`lib/live.dart` — butun ilova uchun
/// bitta kanal). Zaxira so'rov sikli `LiveRefresher` ichida: soket
/// ulangan bo'lsa siyrak, uzilgan bo'lsa tez-tez.
///
/// Qayta ulanish mantig'i BU YERDA YOZILMAGAN — u `ondex_core` da,
/// bitta joyda (eksponensial backoff + jitter bilan).
/// └───────────────────────────────────────────────────────────────────┘
class TrackingScreen extends StatefulWidget {
  final String orderId;

  const TrackingScreen({super.key, required this.orderId});

  @override
  State<TrackingScreen> createState() => _TrackingScreenState();
}

/// To'lov kutayotgan buyurtma uchun ogohlantirish.
///
/// ┌─ NEGA ALOHIDA KARTOCHKA ──────────────────────────────────────────┐
/// Karta tanlangan buyurtma pul bloklanmaguncha OSHXONAGA TUSHMAYDI.
/// Mijoz to'lov oynasini yopib yuborsa, buyurtma jimgina kutib turardi
/// va u "buyurtma qabul qilindi" deb o'ylab qolardi. Shuning uchun
/// holat ochiq aytiladi va qayta to'lash tugmasi beriladi.
/// └───────────────────────────────────────────────────────────────────┘
class _AwaitingPaymentCard extends StatelessWidget {
  final VoidCallback onPay;
  final bool busy;
  final String? error;

  const _AwaitingPaymentCard({
    required this.onPay,
    required this.busy,
    required this.error,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF4E5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: const Color(0xFFFFD9A8)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Row(
            children: [
              Icon(Icons.credit_card, size: 20, color: Color(0xFFB26A00)),
              SizedBox(width: 8),
              Expanded(
                child: Text('To\'lov kutilmoqda',
                    style: TextStyle(
                        fontWeight: FontWeight.bold, color: Color(0xFFB26A00))),
              ),
            ],
          ),
          const SizedBox(height: 6),
          const Text(
            'Buyurtma restoranga YUBORILMAGAN. To\'lovni yakunlaganingizdan '
            'keyin u avtomatik oshxonaga tushadi.\n\n'
            'Pul darhol yechilmaydi — u vaqtincha bloklanadi va restoran '
            'buyurtmani qabul qilgandagina yechiladi.',
            style: TextStyle(fontSize: 13, height: 1.4, color: Color(0xFF6B4A00)),
          ),
          if (error != null) ...[
            const SizedBox(height: 8),
            Text(error!,
                style: const TextStyle(fontSize: 12.5, color: Color(0xFFB3261E))),
          ],
          const SizedBox(height: 12),
          SizedBox(
            height: 46,
            width: double.infinity,
            child: FilledButton.icon(
              style: FilledButton.styleFrom(
                backgroundColor: kBrand,
                shape:
                    RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              onPressed: busy ? null : onPay,
              icon: busy
                  ? const SizedBox(
                      height: 16,
                      width: 16,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white))
                  : const Icon(Icons.lock_outline, size: 18),
              label: Text(busy ? 'Ochilmoqda…' : 'To\'lovni yakunlash'),
            ),
          ),
        ],
      ),
    );
  }
}

class _TrackingScreenState extends State<TrackingScreen> {
  Map<String, dynamic>? _order;
  bool _loading = true;
  String? _error;
  late final LiveRefresher _live;

  /// Karta to'lovi holati (to'lov sahifasi yopilib qolgan holat uchun).
  bool _paying = false;
  String? _payError;

  /// "Stolni kameraga tuting" taklifi BIR MARTA ko'rsatiladi.
  ///
  /// Ro'yxat har 15-20 soniyada qayta yuklanadi (`LiveRefresher`);
  /// bayroqsiz taklif har yangilanishda qayta ochilib, mijozni
  /// bezovta qilardi.
  bool _arPromptShown = false;

  /// Shu buyurtmadagi 3D modeli bor taomlar. FAQAT stol
  /// buyurtmasida — yetkazib berishda mijoz stol oldida emas.
  List<ArDish> get _arDishes {
    final o = _order;
    if (o == null) return const [];
    if ((o['type'] as String?) != 'dine_in') return const [];
    return ArDish.fromOrder(o);
  }

  @override
  void initState() {
    super.initState();
    _load();
    customerLive.start();
    _live = LiveRefresher(
      bus: customerLive,
      onRefresh: _load,
      // Faqat SHU buyurtmaga tegishli hodisalar.
      types: const {'order_status', 'courier_assigned', 'dispatch_failed'},
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  /// Buyurtma berilgandan keyin "Stolni kameraga tuting" taklifi.
  ///
  /// Taklif SHU YERDA, buyurtma yuklangandan keyin chiqadi (checkout
  /// ekranida emas): u paytda buyurtma tarkibi hali serverdan
  /// olinmagan va qaysi taomda 3D borligi noma'lum bo'ladi.
  void _maybePromptAr() {
    if (_arPromptShown) return;
    final dishes = _arDishes;
    if (dishes.isEmpty) return;
    _arPromptShown = true;

    // Ekran chizilib bo'lgach ochiladi: `_load` `initState` dan ham
    // chaqiriladi va o'sha paytda dialog ochib bo'lmaydi.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      showDialog<void>(
        context: context,
        builder: (ctx) => _ArPromptDialog(
          count: dishes.length,
          onOpen: () {
            Navigator.pop(ctx);
            _openAr();
          },
        ),
      );
    });
  }

  void _openAr() {
    final dishes = _arDishes;
    if (dishes.isEmpty) return;
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => ArTableScreen(dishes: dishes),
    ));
  }

  Future<void> _load() async {
    try {
      final o = await api.getOrder(widget.orderId);
      if (!mounted) return;
      setState(() {
        _order = o;
        _loading = false;
        _error = null;
      });
      _maybePromptAr();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        // Buyurtma ALLAQACHON ko'rsatilgan bo'lsa uni o'chirmaymiz —
        // tarmoq uzilgani uchun mijoz holatini yo'qotmasligi kerak.
        if (_order == null) {
          _error = errorText(e, 'Buyurtmani ochib bo\'lmadi');
        }
      });
    }
  }

  /// To'lov kutayotgan KARTA buyurtmasimi.
  ///
  /// Server `payment_state` ni faqat karta buyurtmasida to'ldiradi,
  /// shuning uchun naqd buyurtmada bu hech qachon rost bo'lmaydi.
  static bool _awaitingPayment(Map<String, dynamic> o) {
    final method = (o['payment_method'] as String?) ?? '';
    final state = (o['payment_state'] as String?) ?? '';
    return method == 'card' && state != 'held' && state != 'paid';
  }

  /// To'lov sahifasini QAYTA ochadi.
  ///
  /// Mijoz to'lov oynasini yopib yuborgan yoki to'lov bank tomonda
  /// uzilib qolgan bo'lishi mumkin (masalan "takroriy SMS xabarlarining
  /// maksimal soni"). Shuning uchun bu yerda `retry: true` yuboriladi —
  /// server eski urinishni yopib, YANGI tranzaksiya ochadi. Aks holda
  /// mijoz o'sha o'lik havolaga qaytaverib tuzoqqa tushib qolardi.
  Future<void> _payAgain() async {
    if (_paying) return;
    setState(() {
      _paying = true;
      _payError = null;
    });
    try {
      final p = await api.startPayment(widget.orderId, retry: true);
      final url = (p['pay_url'] as String?) ?? '';
      if (url.isEmpty) throw Exception('havola bo\'sh');
      if (!mounted) return;
      await Navigator.of(context).push<bool>(
        MaterialPageRoute(
          builder: (_) => PaymentWebViewScreen(
            payUrl: url,
            returnUrl: (p['return_url'] as String?) ?? '',
          ),
        ),
      );
      if (!mounted) return;
      setState(() => _paying = false);
      // Oyna yopilishi to'lov o'tganini bildirmaydi — serverdan
      // haqiqiy holatni so'raymiz.
      await _load();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _paying = false;
        _payError = errorText(e, 'To\'lov sahifasini ochib bo\'lmadi');
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final o = _order;

    return SheetPage(
        child: Scaffold(
      backgroundColor: Colors.white,
      appBar: PageAppBar(
        title: o == null
            ? 'Buyurtma'
            : '№ ${(o['order_number'] as String?) ?? ''}',
      ),
      body: _loading && o == null
          ? const Center(child: CircularProgressIndicator())
          : _error != null && o == null
              ? ErrorView(message: _error!, onRetry: _load)
              : RefreshIndicator(
                  color: kBrand,
                  onRefresh: _load,
                  child: ListView(
                    padding: const EdgeInsets.fromLTRB(16, 12, 16, 32),
                    children: [
                      _PlacedAt(order: o!),
                      const SizedBox(height: 12),
                      // To'lov kutayotgan buyurtma — eng tepada, chunki
                      // bu yerda mijozdan HARAKAT talab qilinadi.
                      if (_awaitingPayment(o)) ...[
                        _AwaitingPaymentCard(
                          onPay: _payAgain,
                          busy: _paying,
                          error: _payError,
                        ),
                        const SizedBox(height: 14),
                      ],
                      _StatusHeader(order: o),
                      // ── Stolda AR ko'rish ──
                      //
                      // Faqat STOL buyurtmasida va faqat 3D modeli bor
                      // taom bo'lsa. Holat tepasida turadi: mijoz
                      // taomni kutayotgan paytda aynan shu qiziq.
                      if (_arDishes.isNotEmpty) ...[
                        const SizedBox(height: 16),
                        _ArCard(
                          count: _arDishes.length,
                          onOpen: _openAr,
                        ),
                      ],
                      const SizedBox(height: 22),
                      _Timeline(order: o),
                      const SizedBox(height: 22),
                      _ItemsCard(order: o),
                      const SizedBox(height: 14),
                      _WhereCard(order: o),
                      if (_isFinished(o)) ...[
                        const SizedBox(height: 26),
                        SizedBox(
                          height: 52,
                          child: FilledButton(
                            style: FilledButton.styleFrom(
                              backgroundColor: kBrand,
                              shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(14)),
                            ),
                            // Buyurtma tugagan — bu ekranda qiladigan
                            // ish qolmadi. Vebda ham shu tugma bor.
                            onPressed: () => Navigator.of(context)
                                .popUntil((r) => r.isFirst),
                            child: const Text('Bosh sahifaga qaytish',
                                style: TextStyle(
                                    fontSize: 16,
                                    fontWeight: FontWeight.bold)),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
    ));
  }
}

// ═══════════════════════════════════════════════════════════════════

/// "Stolni kameraga tuting" taklifi — buyurtma berilgandan keyin
/// bir marta chiqadi.
class _ArPromptDialog extends StatelessWidget {
  const _ArPromptDialog({required this.count, required this.onOpen});

  final int count;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      contentPadding: const EdgeInsets.fromLTRB(24, 26, 24, 8),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(
              color: kBrand.withValues(alpha: 0.12),
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.photo_camera_rounded,
                color: kBrand, size: 30),
          ),
          const SizedBox(height: 16),
          const Text(
            'Stolni kameraga tuting',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 19, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            count == 1
                ? 'Buyurtma qilgan taomingizni stol ustida, haqiqiy '
                    'o\'lchamda ko\'rishingiz mumkin.'
                : 'Buyurtma qilgan $count ta taomingizni stol ustida, '
                    'haqiqiy o\'lchamda ko\'rishingiz mumkin.',
            textAlign: TextAlign.center,
            style: const TextStyle(
                fontSize: 13.5, color: Color(0xFF616161), height: 1.45),
          ),
        ],
      ),
      actionsPadding: const EdgeInsets.fromLTRB(16, 8, 16, 14),
      actions: [
        Column(
          children: [
            SizedBox(
              width: double.infinity,
              height: 46,
              child: FilledButton.icon(
                onPressed: onOpen,
                style: FilledButton.styleFrom(
                  backgroundColor: kBrand,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                ),
                icon: const Icon(Icons.view_in_ar_rounded, size: 19),
                label: const Text('Ko\'rish',
                    style: TextStyle(
                        fontSize: 15, fontWeight: FontWeight.bold)),
              ),
            ),
            TextButton(
              onPressed: () => Navigator.pop(context),
              style: TextButton.styleFrom(
                  foregroundColor: const Color(0xFF9E9E9E)),
              child: const Text('Keyinroq'),
            ),
          ],
        ),
      ],
    );
  }
}

/// Kuzatuv ekranidagi doimiy AR kartochkasi — taklif oynasi yopilgan
/// bo'lsa ham mijoz uni istalgan paytda ochishi kerak.
class _ArCard extends StatelessWidget {
  const _ArCard({required this.count, required this.onOpen});

  final int count;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFFFFF3E9),
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        onTap: onOpen,
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            children: [
              const Icon(Icons.view_in_ar_rounded, color: kBrand, size: 24),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('Stolda ko\'rish',
                        style: TextStyle(
                            fontSize: 14.5, fontWeight: FontWeight.w700)),
                    const SizedBox(height: 2),
                    Text(
                      count == 1
                          ? 'Taomni kamera orqali stol ustida ko\'ring'
                          : '$count ta taomni kamera orqali ko\'ring',
                      style: const TextStyle(
                          fontSize: 12.5, color: Color(0xFF757575)),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right, color: kBrand),
            ],
          ),
        ),
      ),
    );
  }
}

/// Buyurtma tugagan (yoki bekor bo'lgan) — bu ekranda qiladigan ish
/// qolmadi.
bool _isFinished(Map<String, dynamic> o) {
  final s = (o['status'] as String?) ?? '';
  return s == 'delivered' || s == 'served' || s == 'cancelled' ||
      s == 'rejected';
}

/// "№ ... · 14:32 da joylandi" — vebdagi sarlavha ostidagi qator.
class _PlacedAt extends StatelessWidget {
  final Map<String, dynamic> order;
  const _PlacedAt({required this.order});

  @override
  Widget build(BuildContext context) {
    final number = (order['order_number'] as String?) ?? '';
    final at = DateTime.tryParse((order['created_at'] as String?) ?? '');
    if (number.isEmpty && at == null) return const SizedBox.shrink();

    final parts = <String>[
      if (number.isNotEmpty) number,
      if (at != null)
        '${at.toLocal().hour.toString().padLeft(2, '0')}:'
            '${at.toLocal().minute.toString().padLeft(2, '0')} da joylandi',
    ];
    return Text(
      parts.join('  ·  '),
      style: const TextStyle(fontSize: 13, color: Color(0xFF757575)),
    );
  }
}

/// Holat kartasi — MARKAZLASHGAN va holat rangida bo'yalgan.
///
/// Avval bu chapga tekislangan oddiy qator edi. Vebda esa u sahifaning
/// eng ko'zga tashlanadigan elementi (`rounded-2xl p-5 text-center`) —
/// mijoz ekranni ochganda birinchi navbatda "buyurtmam qay holatda?"
/// degan savolga javob izlaydi.
class _StatusHeader extends StatelessWidget {
  final Map<String, dynamic> order;
  const _StatusHeader({required this.order});

  @override
  Widget build(BuildContext context) {
    final status = (order['status'] as String?) ?? '';
    final isDineIn = (order['type'] as String?) == 'dine_in';
    final (label, icon, color) = statusStyleOf(status, dineIn: isDineIn);
    final courierId = (order['courier_id'] as String?) ?? '';
    final table = (order['table_label'] as String?) ?? '';
    final partySize = (order['party_size'] as num?)?.toInt() ?? 0;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 20),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Column(
        children: [
          Icon(icon, color: color, size: 40),
          const SizedBox(height: 8),
          Text(label,
              textAlign: TextAlign.center,
              style: TextStyle(
                  fontSize: 18, fontWeight: FontWeight.bold, color: color)),
          const SizedBox(height: 4),
          Text(
            _hint(status, isDineIn),
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 13, color: Color(0xFF757575)),
          ),
          // Kuryer ID — stol buyurtmasida ma'nosiz (kuryer yo'q).
          if (!isDineIn && courierId.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text('Kuryer: $courierId',
                  style: const TextStyle(
                      fontSize: 13, color: Color(0xFF757575))),
            ),
          if (isDineIn && table.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                // `tableText` — `ondex_core` da. Qo'lda "$table-stol"
                // deb yozilsa, restoran stolni "Stol-1" deb nomlaganda
                // "Stol-1-stol" chiqardi.
                partySize > 0
                    ? '${tableText(table)} · $partySize kishi'
                    : tableText(table),
                style:
                    const TextStyle(fontSize: 13, color: Color(0xFF757575)),
              ),
            ),
        ],
      ),
    );
  }

  /// Har holat uchun mijoz NIMA KUTISHI kerakligi.
  ///
  /// Holat nomining o'zi yetarli emas: "Tayyor" — mijoz uchun nima
  /// degani? Shuning uchun har biriga kutish izohi qo'shiladi.
  static String _hint(String status, bool dineIn) {
    // Stolda `ready` — taom tayyor va AFFITSIANT olib kelmoqda. Kuryer
    // bu buyurtmaga umuman chaqirilmaydi (backend dispatch'ni ishga
    // tushirmaydi), shuning uchun "kuryer kutilmoqda" yozuvi mijozni
    // hech qachon sodir bo'lmaydigan narsani kutishga majbur qilardi.
    if (dineIn && status == 'ready') {
      return 'Affitsiant taomni stolingizga olib kelmoqda';
    }
    return switch (status) {
      'created' => 'Restoran buyurtmani ko\'rishini kutmoqdamiz',
      'accepted' => 'Restoran qabul qildi, tayyorlash boshlanadi',
      'preparing' => 'Taomingiz tayyorlanmoqda',
      'ready' => 'Tayyor — kuryer olib ketishini kutmoqda',
      'picked_up' => 'Kuryer yo\'lda',
      'delivered' => 'Yoqimli ishtaha!',
      'served' => 'Yoqimli ishtaha!',
      'rejected' => 'Restoran buyurtmani qabul qila olmadi',
      'cancelled' => 'Buyurtma bekor qilindi',
      _ => '',
    };
  }
}

/// Bosqichlar chizig'i — buyurtma tarixidan quriladi.
class _Timeline extends StatelessWidget {
  final Map<String, dynamic> order;
  const _Timeline({required this.order});

  @override
  Widget build(BuildContext context) {
    final status = (order['status'] as String?) ?? '';
    final isDineIn = (order['type'] as String?) == 'dine_in';

    // Bekor qilingan/rad etilgan buyurtmada bosqichlar ma'nosiz.
    if (status == 'rejected' || status == 'cancelled') {
      return const SizedBox.shrink();
    }

    // Stolda "yo'lda" bosqichi YO'Q — affitsiant stolga olib keladi.
    // Ro'yxat `widgets/order_status.dart` da (bitta manba): ilgari u shu
    // yerda qo'lda yozilgan edi va "Buyurtmalarim" kartochkasidagi
    // chiziq bilan mos kelmasdi.
    //
    // Bu TARIX chizig'i, shuning uchun `dineInTimelineLabels` — oxirgi
    // qadam "Stolga berildi" (vaqti bilan), qisqa chiziqdagi "Tayyor"
    // emas.
    final stages = isDineIn ? dineInTimelineLabels : stageLabels;
    final reached = _reachedIndex(status, isDineIn);

    return Column(
      children: [
        for (var i = 0; i < stages.length; i++)
          _Step(
            label: stages[i],
            done: i <= reached,
            isLast: i == stages.length - 1,
            at: _timeOf(i, isDineIn),
          ),
      ],
    );
  }

  /// Joriy holat qaysi bosqichga to'g'ri kelishi.
  static int _reachedIndex(String status, bool dineIn) {
    if (dineIn) {
      return switch (status) {
        'accepted' => 0,
        'preparing' => 1,
        'ready' => 1,
        'served' => 2,
        _ => -1,
      };
    }
    return switch (status) {
      'accepted' => 0,
      'preparing' => 1,
      'ready' => 1,
      'picked_up' => 2,
      'delivered' => 3,
      _ => -1,
    };
  }

  /// Bosqich vaqti — buyurtma tarixidan.
  String? _timeOf(int stage, bool dineIn) {
    final history = (order['history'] as List?) ?? const [];
    final target = dineIn
        ? ['accepted', 'preparing', 'served'][stage.clamp(0, 2)]
        : ['accepted', 'preparing', 'picked_up', 'delivered'][stage.clamp(0, 3)];

    for (final h in history) {
      if (h is Map && h['to'] == target) {
        final at = DateTime.tryParse((h['at'] as String?) ?? '');
        if (at == null) return null;
        final l = at.toLocal();
        return '${l.hour.toString().padLeft(2, '0')}:'
            '${l.minute.toString().padLeft(2, '0')}';
      }
    }
    return null;
  }
}

class _Step extends StatelessWidget {
  final String label;
  final bool done;
  final bool isLast;
  final String? at;

  const _Step({
    required this.label,
    required this.done,
    required this.isLast,
    required this.at,
  });

  @override
  Widget build(BuildContext context) {
    final color = done ? kBrand : const Color(0xFFD5D5D5);
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Column(
            children: [
              Container(
                width: 18,
                height: 18,
                decoration: BoxDecoration(
                  color: done ? kBrand : Colors.white,
                  shape: BoxShape.circle,
                  border: Border.all(color: color, width: 2),
                ),
                child: done
                    ? const Icon(Icons.check, size: 11, color: Colors.white)
                    : null,
              ),
              if (!isLast)
                Expanded(
                  child: Container(width: 2, color: color),
                ),
            ],
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : 18),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      label,
                      style: TextStyle(
                        fontWeight: done ? FontWeight.w600 : FontWeight.normal,
                        color: done ? null : const Color(0xFF9E9E9E),
                      ),
                    ),
                  ),
                  if (at != null)
                    Text(at!,
                        style: const TextStyle(
                            fontSize: 12.5, color: Color(0xFF9E9E9E))),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ItemsCard extends StatelessWidget {
  final Map<String, dynamic> order;
  const _ItemsCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final items = (order['items'] as List?) ?? const [];
    final total = (order['total_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (order['discount_tiyin'] as num?)?.toInt() ?? 0;
    final subtotal = (order['subtotal_tiyin'] as num?)?.toInt() ?? 0;
    final promotionName = ((order['promotion_name'] as String?) ?? '').trim();
    final promotionDiscount =
        (order['promotion_discount_tiyin'] as num?)?.toInt() ?? 0;
    final totalItems = items.fold<int>(
        0, (a, it) => a + (it is Map ? ((it['qty'] as num?)?.toInt() ?? 0) : 0));
    // `dineIn` uzatiladi: hozir ikkala turda ham rang bir xil, lekin
    // stol uslubi keyin o'zgarsa bu chaqiruv jimgina eskirib qolmasin.
    final statusColor = statusStyleOf(
      (order['status'] as String?) ?? '',
      dineIn: (order['type'] as String?) == 'dine_in',
    ).$3;

    return _Panel(
      title: 'Buyurtma tarkibi',
      // Taom soni — vebdagi holat rangidagi belgi.
      trailing: totalItems > 0
          ? Container(
              padding:
                  const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: statusColor.withValues(alpha: 0.16),
                borderRadius: BorderRadius.circular(20),
              ),
              child: Text('$totalItems ta taom',
                  style: TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w600,
                      color: statusColor)),
            )
          : null,
      child: Column(
        children: [
          for (final it in items)
            if (it is Map)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Row(
                  children: [
                    // Rasm o'lchami QAT'IY — turli o'lchamdagi rasmlar
                    // ro'yxatni notekis ko'rsatardi.
                    ClipRRect(
                      borderRadius: BorderRadius.circular(10),
                      child: SizedBox(
                        width: 56,
                        height: 56,
                        child: _ItemImage(
                            url: (it['image_url'] as String?) ?? ''),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text((it['name'] as String?) ?? '',
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                  fontSize: 14,
                                  fontWeight: FontWeight.w500)),
                          const SizedBox(height: 2),
                          Text(
                            '${(it['qty'] as num?)?.toInt() ?? 1} × '
                            '${formatSum((it['price_tiyin'] as num?)?.toInt() ?? 0)}',
                            style: const TextStyle(
                                fontSize: 12, color: Color(0xFF757575)),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      formatSum(((it['price_tiyin'] as num?)?.toInt() ?? 0) *
                          ((it['qty'] as num?)?.toInt() ?? 1)),
                      style: const TextStyle(
                          fontSize: 14, fontWeight: FontWeight.bold),
                    ),
                  ],
                ),
              ),
          const Divider(height: 20),
          if (discount > 0) ...[
            _Line(label: 'Taomlar', value: formatSum(subtotal)),
            _Line(
              // Aksiya nomi FAQAT chegirmaning hammasi o'sha aksiyadan
              // bo'lganda ko'rsatiladi: savatga bir vaqtda bir nechta
              // aksiya va mahsulot chegirmasi tushishi mumkin (qoida
              // savat/checkout ekranlari bilan bitta manbadan —
              // `discountLineLabel`).
              label: discountLineLabel(
                discountTiyin: discount,
                promotionDiscountTiyin: promotionDiscount,
                promotionName: promotionName,
              ),
              value: '− ${formatSum(discount)}',
              color: const Color(0xFF16A34A),
            ),
            const SizedBox(height: 6),
          ],
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text('Jami',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              Text(formatSum(total),
                  style: const TextStyle(
                      fontWeight: FontWeight.bold, fontSize: 17)),
            ],
          ),
        ],
      ),
    );
  }
}

class _Line extends StatelessWidget {
  final String label;
  final String value;
  final Color? color;
  const _Line({required this.label, required this.value, this.color});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(color: Color(0xFF757575))),
          Text(value, style: TextStyle(color: color)),
        ],
      ),
    );
  }
}

/// Qayerga — stol yoki yetkazib berish manzili.
class _WhereCard extends StatelessWidget {
  final Map<String, dynamic> order;
  const _WhereCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final isDineIn = (order['type'] as String?) == 'dine_in';
    final table = (order['table_label'] as String?) ?? '';
    final addr = order['delivery_address'];

    if (isDineIn) {
      return _Panel(
        title: 'Qayerda',
        child: Row(
          children: [
            const Icon(Icons.qr_code_2, size: 18, color: kBrand),
            const SizedBox(width: 8),
            Text(table.isEmpty ? 'Stolda' : tableText(table)),
          ],
        ),
      );
    }

    final text = (addr is Map ? (addr['text'] as String?) : null) ?? '';
    return _Panel(
      title: 'Yetkazib berish',
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.place_outlined, size: 18, color: kBrand),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text.isEmpty ? 'Xaritada tanlangan manzil' : text),
          ),
        ],
      ),
    );
  }
}

/// Buyurtma tarkibidagi taom rasmi.
class _ItemImage extends StatelessWidget {
  final String url;
  const _ItemImage({required this.url});

  static const _placeholder = ColoredBox(
    color: Color(0xFFF5F5F5),
    child: Center(
      child: Icon(Icons.restaurant_menu, size: 22, color: Color(0xFFBDBDBD)),
    ),
  );

  @override
  Widget build(BuildContext context) =>
      RemoteImage(url: url, placeholder: _placeholder);
}

class _Panel extends StatelessWidget {
  final String title;
  final Widget child;

  /// Sarlavha o'ng tomonidagi ixtiyoriy belgi (masalan taom soni).
  final Widget? trailing;

  const _Panel({required this.title, required this.child, this.trailing});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFE5E5E5)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(title,
                    style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: Color(0xFF757575))),
              ),
              if (trailing != null) trailing!,
            ],
          ),
          const SizedBox(height: 10),
          child,
        ],
      ),
    );
  }
}

