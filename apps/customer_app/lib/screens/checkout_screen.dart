import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import '../widgets/agent_overlay.dart' show AgentCheckoutConfirmAnchor;
import '../widgets/app_text_field.dart';
import '../widgets/page_sheet.dart';
import '../data/quote_service.dart';
import '../widgets/product_grid.dart' show discountLineLabel;
import '../widgets/qty_stepper.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'catalog_screen.dart' show kBrand;
import 'payment_webview_screen.dart';
import 'tracking_screen.dart';

// ═══════════════════════════════════════════════════════════════════
// MOCK MA'LUMOT — BITTA JOYDA
// ═══════════════════════════════════════════════════════════════════
//
// ┌─ NIMA UCHUN SHU YERDA VA SHUNDAY NOMLANGAN ───────────────────────┐
// Quyidagilar backendda HALI YO'Q. Ular ataylab BITTA blokda va
// `mock` prefiksi bilan turibdi — haqiqiy ma'lumot ulanganda nimani
// almashtirish kerakligi bir qarashda ko'rinsin, kod bo'ylab tarqalib
// ketmasin.
//
// ENG MUHIM QOIDA: mock qiymatlar PULGA TEGMAYDI. Yetkazish va xizmat
// haqi 0 — chunki server ularni hisoblamaydi va mijozdan olmaydi.
// Agar bu yerga 15 000 yozilsa, ekranda ko'ringan "Jami" serverda
// yoziladigan summadan FARQ QILARDI: mijoz bir raqamga rozi bo'lib,
// boshqasini to'lardi. Shuning uchun ular 0 va "Bepul" deb chiziladi.
// └───────────────────────────────────────────────────────────────────┘

/// Yetkazish haqi. MOCK: backend `QuoteResult` da bunday maydon yo'q.
/// 0 — hozir yetkazish uchun haq OLINMAYDI, ya'ni bu qiymat ayni
/// paytda HAQIQATGA MOS.
const int _mockDeliveryFeeTiyin = 0;

/// Xizmat yig'imi. MOCK: backendda yo'q, olinmaydi.
const int _mockServiceFeeTiyin = 0;

/// Restoran yetkazish vaqtini ko'rsatmagan bo'lsa ishlatiladigan
/// oraliq. MOCK: taxminiy qiymat, hech qayerdan hisoblanmaydi.
const String _mockEtaFallback = '25–40 daqiqa';

/// Buyurtmani rasmiylashtirish — NATIVE (maket: `image/rasmiy.png`).
///
/// ┌─ MAKETDAN NIMA OLINDI, NIMA OLINMADI ─────────────────────────────┐
/// OLINDI: sarlavha, manzil + yetkazish vaqti kartasi, buyurtma
/// tafsilotlari (rasm + miqdor boshqaruvi + narx), yakuniy hisob
/// kartasi va katta tugma.
///
/// OLINMADI va NEGA:
///   * "Yetkazish" va "Xizmat yig'imi" qatorlari — backend bunday
///     summalarni UMUMAN hisoblamaydi. `QuoteResult` faqat
///     subtotal/discount/total qaytaradi (`internal/orders/service.go`).
///     Ularni chizish mijozga hech qachon to'lamaydigan raqamni
///     ko'rsatish bo'lardi.
///   * "Kupon yoki promokod" — aksiyalar AVTOMATIK qo'llanadi
///     (`promotions.ApplyBest`), kod bo'yicha chegirma tizimi yo'q.
///   * "Izoh qoldirish" — `POST /orders` bunday maydonni qabul
///     qilmaydi. Kuryerga izoh esa MANZIL ichida, `AddressScreen` da
///     kiritiladi.
///   * 1-2-3 bosqich chizig'i — so'ralmadi.
///
/// Bular backend qo'llab-quvvatlagan kunda qo'shiladi; joyi tayyor.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ IKKI TUR ────────────────────────────────────────────────────────┐
/// STOL (dine-in): manzil KERAK EMAS. Buyurtma `table_token` bilan
/// yuboriladi va server uni `dine_in` deb belgilaydi.
///
/// YETKAZIB BERISH: manzil MAJBURIY. Mijozning saqlangan manzili
/// ishlatiladi, kerak bo'lsa xaritadan o'zgartiriladi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ SUMMA HAR DOIM SERVERDAN ────────────────────────────────────────┐
/// Boshlang'ich qiymat savat ekranidan keladi (ekran DARHOL to'liq
/// chiziladi), lekin bu yerda miqdor o'zgartirilsa summa QAYTA
/// so'raladi. Mijoz ko'rgan raqam va serverda yoziladigan raqam bir
/// xil bo'lishi shart.
/// └───────────────────────────────────────────────────────────────────┘
class CheckoutScreen extends StatefulWidget {
  final int quoteTiyin;
  final int? subtotalTiyin;
  final int? discountTiyin;
  final String? promotionName;

  /// Chegirmaning aksiya bergan qismi (tiyin) — qolgani mahsulotlarning
  /// o'z chegirma narxlari.
  final int promotionDiscountTiyin;

  /// Serverning QATOR bo'yicha hisobi (`product_id` -> qator summasi,
  /// tiyin) — savat ekranidagi so'rovdan ko'chiriladi, shuning uchun bu
  /// ekran ochilishi bilan taomlar yonida HAQIQIY narx turadi.
  /// Miqdor o'zgarsa `_refreshQuote()` uni yangilaydi.
  final Map<String, int> quoteLineTotals;

  const CheckoutScreen({
    super.key,
    required this.quoteTiyin,
    this.subtotalTiyin,
    this.discountTiyin,
    this.promotionName,
    this.promotionDiscountTiyin = 0,
    this.quoteLineTotals = const {},
  });

  @override
  State<CheckoutScreen> createState() => _CheckoutScreenState();
}

class _CheckoutScreenState extends State<CheckoutScreen> {
  final _cart = CartStore.instance;

  // ┌─ IDEMPOTENTLIK KALITI BIR MARTA ─────────────────────────────┐
  // Kalit ekran ochilganda BIR MARTA generatsiya qilinadi va barcha
  // qayta urinishlarda o'zgarmaydi. Tarmoq uzilib javob kelmasa,
  // mijoz qayta bosganda server SHU kalitni ko'rib dublikat buyurtma
  // YARATMAYDI (`api.dart` dagi `newIdempotencyKey` izohiga qarang).
  //
  // Uni `_submit` ichida generatsiya qilish ENG KLASSIK xato bo'lardi:
  // har bosishda yangi kalit chiqib, himoya butunlay ishlamasdi.
  // └───────────────────────────────────────────────────────────────┘
  final String _idempotencyKey = newIdempotencyKey();

  Map<String, dynamic>? _address;
  bool _loadingAddress = true;
  bool _submitting = false;
  String? _error;
  int _partySize = 2;

  /// Menyu — nom, rasm va tavsif shundan olinadi (savatda faqat ID va
  /// miqdor saqlanadi).
  List<Map<String, dynamic>> _menu = const [];
  StreamSubscription<Cached<List<dynamic>>>? _menuSub;

  /// Restoran — yetkazish vaqti (`eta_min/max_minutes`) uchun.
  Map<String, dynamic>? _restaurant;

  late int _total = widget.quoteTiyin;
  late int? _subtotal = widget.subtotalTiyin;
  late int? _discount = widget.discountTiyin;
  late String? _promotionName = widget.promotionName;
  /// Chegirmaning AYNAN aksiya bergan qismi — hisob qatori nomi shunga
  /// qarab tanlanadi (`discountLineLabel`).
  late int _promotionDiscount = widget.promotionDiscountTiyin;
  late Map<String, int> _lineTotals = widget.quoteLineTotals;
  bool _quoting = false;

  /// Eskirgan javoblardan himoya shu obyekt ichida.
  final _quoteFetcher = QuoteFetcher();

  /// Tanlangan to'lov usuli — serverga HAQIQATAN yuboriladi.
  ///
  /// `card` tanlansa buyurtma TO'LOV KUTIB yaratiladi (restoranga
  /// ko'rinmaydi), so'ng `POST /orders/{id}/pay` orqali Octo to'lov
  /// sahifasi ochiladi. Pul BLOKLANADI, restoran buyurtmani qabul
  /// qilgandagina yechiladi.
  ///
  /// Standart — naqd: kuryerga/affitsiantga to'lanadi.
  _PayMethod _payMethod = _PayMethod.cash;

  /// Kiritilgan kupon kodi. MOCK: kod bo'yicha chegirma tizimi
  /// backendda YO'Q (aksiyalar avtomatik qo'llanadi). Kod saqlanadi,
  /// lekin SUMMAGA TA'SIR QILMAYDI va serverga yuborilmaydi.
  String? _coupon;

  bool get _isDineIn => _cart.isDineIn;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    _subscribeMenu();
    _loadRestaurant();
    if (_isDineIn) {
      _loadingAddress = false;
    } else {
      _loadAddress();
    }
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    _menuSub?.cancel();
    super.dispose();
  }

  void _subscribeMenu() {
    final rid = _cart.restaurantId;
    if (rid == null) return;
    _menuSub = Repos.menu(rid).observe().listen((c) {
      if (!mounted || c.value == null) return;
      setState(() => _menu = c.value!.cast<Map<String, dynamic>>());
    });
  }

  Future<void> _loadRestaurant() async {
    final rid = _cart.restaurantId;
    if (rid == null) return;
    try {
      final list = await Repos.restaurants().peek();
      final found = list?.cast<Map<String, dynamic>>().firstWhere(
            (r) => r['id'] == rid,
            orElse: () => <String, dynamic>{},
          );
      if (mounted && found != null && found.isNotEmpty) {
        setState(() => _restaurant = found);
      }
    } catch (_) {
      // Yetkazish vaqti ko'rsatilmaydi — qolgani ishlayveradi.
    }
  }

  void _onCart() {
    if (!mounted) return;
    // Savat BO'SHASA bu ekranda qiladigan ish qolmaydi.
    if (_cart.isEmpty) {
      Navigator.of(context).pop();
      return;
    }
    setState(() {});
    _refreshQuote();
  }

  /// Miqdor o'zgargach yakuniy summani QAYTA so'raydi.
  ///
  /// So'rov `data/quote_service.dart` da — savat va menyu ekranlari
  /// bilan bitta kod.
  Future<void> _refreshQuote() async {
    final rid = _cart.restaurantId;
    if (rid == null || _cart.isEmpty) return;

    setState(() => _quoting = true);
    try {
      final q = await _quoteFetcher.fetch(rid);
      if (!mounted || q == null) return; // eskirgan javob
      setState(() {
        _quoting = false;
        // Ishonchsiz summa (0 yoki manfiy) ekrandagi hisobni
        // O'ZGARTIRMAYDI — savatdan kelgan qiymat qoladi.
        if (!q.isUsable) return;
        _total = q.totalTiyin!;
        _subtotal = q.subtotalTiyin;
        _discount = q.discountTiyin;
        _promotionName = q.promotionName;
        _promotionDiscount = q.promotionDiscountTiyin;
        _lineTotals = q.lineTotals;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _quoting = false;
        _error = errorText(e, 'Summani hisoblab bo\'lmadi');
      });
    }
  }

  Future<void> _loadAddress() async {
    try {
      final a = await api.getMyAddress();
      if (!mounted) return;
      setState(() {
        // lat/lng == 0 — manzil hali tanlanmagan.
        _address = ((a['lat'] as num?)?.toDouble() ?? 0) != 0 ? a : null;
        _loadingAddress = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loadingAddress = false);
    }
  }

  Future<void> _pickAddress() async {
    await Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => const AddressScreen()),
    );
    if (!mounted) return;
    setState(() => _loadingAddress = true);
    await _loadAddress();
  }

  /// Karta to'lovini boshlaydi va Octo sahifasini ILOVA ICHIDA ochadi.
  ///
  /// ┌─ NEGA WEBVIEW ────────────────────────────────────────────────┐
  /// Mijoz ilovadan chiqmasligi kerak. Karta ma'lumoti esa bizning
  /// kodimizdan o'tmasligi kerak (aks holda PCI DSS majburiy bo'ladi),
  /// shuning uchun sahifani o'zimiz chizmaymiz — Octo'nikini
  /// ko'rsatamiz. Octo'da mobil SDK yo'q.
  ///
  /// Ba'zi banklarning 3-D Secure oynasi WebView'da ochilmasligi
  /// mumkin — shuning uchun o'sha ekranda "Brauzerda ochish" tugmasi
  /// qoldirilgan (`PaymentWebViewScreen`).
  /// └───────────────────────────────────────────────────────────────┘
  Future<bool> _startCardPayment(String orderId) async {
    try {
      final p = await api.startPayment(orderId);
      final url = (p['pay_url'] as String?) ?? '';
      if (url.isEmpty) throw Exception('havola bo\'sh');

      if (!mounted) return false;
      await Navigator.of(context).push<bool>(
        MaterialPageRoute(
          builder: (_) => PaymentWebViewScreen(
            payUrl: url,
            returnUrl: (p['return_url'] as String?) ?? '',
          ),
        ),
      );
      // Oyna yopilgani to'lov o'tganini BILDIRMAYDI — haqiqiy holatni
      // faqat provayderning callback'i belgilaydi. Kuzatuv ekrani
      // serverdan holatni so'rab turadi.
      return true;
    } catch (e) {
      if (!mounted) return false;
      setState(() {
        _submitting = false;
        _error = errorText(
            e,
            'To\'lov sahifasini ochib bo\'lmadi. Buyurtma to\'lanmagan holda '
            'kutmoqda — "Buyurtmalarim" bo\'limidan qayta urinib ko\'ring.');
      });
      return false;
    }
  }

  Future<void> _submit() async {
    if (_submitting) return;

    final rid = _cart.restaurantId;
    if (rid == null || _cart.isEmpty) return;

    // Yetkazib berishda manzilsiz yuborilmaydi — server ham rad
    // etardi, lekin mijozga sababni SHU YERDA aytish to'g'ri.
    if (!_isDineIn && _address == null) {
      setState(() => _error = 'Yetkazib berish manzilini tanlang');
      return;
    }

    setState(() {
      _submitting = true;
      _error = null;
    });

    try {
      final order = await api.createOrder(
        items: [
          for (final e in _cart.items.entries)
            {'product_id': e.key, 'qty': e.value}
        ],
        idempotencyKey: _idempotencyKey,
        lat: _isDineIn ? null : (_address!['lat'] as num).toDouble(),
        lng: _isDineIn ? null : (_address!['lng'] as num).toDouble(),
        tableToken: _isDineIn ? _cart.tableToken : null,
        partySize: _isDineIn ? _partySize : null,
        paymentMethod: _payMethod == _PayMethod.card ? 'card' : 'cash',
      );

      if (!mounted) return;

      // ┌─ KARTA: TO'LOV SAHIFASI ──────────────────────────────────┐
      // Buyurtma yaratildi, lekin u TO'LOV KUTIB turibdi va
      // oshxonaga tushmagan. To'lov havolasi olinmasa — buyurtmani
      // shu yerda bekor qilamiz: mijoz "buyurtma berildi" deb
      // o'ylab qolmasligi kerak.
      // └───────────────────────────────────────────────────────────┘
      if (_payMethod == _PayMethod.card) {
        final orderId = (order['id'] as String?) ?? '';
        final ok = await _startCardPayment(orderId);
        if (!ok) return; // xato ekranda ko'rsatildi, savat saqlanib qoldi
        if (!mounted) return;
      }

      // Savat FAQAT muvaffaqiyatdan keyin tozalanadi. Oldin tozalansa
      // va so'rov yiqilsa, mijoz savatini yo'qotgan bo'lardi.
      //
      // Tinglovchi OLDIN olib tashlanadi: `clear()` `_onCart` ni
      // chaqiradi va u bo'sh savatni ko'rib ekranni yopib yuborardi —
      // kuzatuv ekraniga o'tishga ulgurmasdan.
      _cart.removeListener(_onCart);
      _cart.clear();

      final id = (order['id'] as String?) ?? '';
      // `pushReplacement`: orqaga bosilganda checkout'ga emas,
      // katalogga qaytadi — buyurtma allaqachon berilgan, unga
      // qaytishning ma'nosi yo'q.
      Navigator.of(context).pushReplacement(
        sheetRoute(TrackingScreen(orderId: id)),
      );
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = errorText(e, 'Buyurtma yuborilmadi');
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
        child: Scaffold(
      backgroundColor: const Color(0xFFFAFAFA),
      appBar: PageAppBar(
        backgroundColor: const Color(0xFFFAFAFA),
        centerTitle: true,
        titleSpacing: 0,
        leadingWidth: 64,
        leading: Padding(
          padding: const EdgeInsets.only(left: 16),
          child: Material(
            color: Colors.white,
            borderRadius: BorderRadius.circular(14),
            child: InkWell(
              borderRadius: BorderRadius.circular(14),
              onTap: () => Navigator.of(context).pop(),
              child: const SizedBox(
                width: 44,
                height: 44,
                child: Icon(Icons.arrow_back, size: 22),
              ),
            ),
          ),
        ),
        titleWidget: const Text('Rasmiylashtirish',
            style: TextStyle(fontSize: 21, fontWeight: FontWeight.bold)),
      ),
      body: _loadingAddress
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
                if (_isDineIn) _dineInCard() else _addressCard(),
                const SizedBox(height: 12),
                _orderDetailsCard(),
                const SizedBox(height: 12),
                _payMethodCard(),
                const SizedBox(height: 12),
                _couponCard(),
                const SizedBox(height: 12),
                _totalsCard(),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  _ErrorBox(text: _error!),
                ],
              ],
            ),
      bottomNavigationBar: _bottomBar(),
    ));
  }

  // ── Manzil va yetkazish vaqti ─────────────────────────────────────

  Widget _addressCard() {
    final a = _address;
    final text = ((a?['text'] as String?) ?? '').trim();
    final extras = a == null ? '' : _addressExtras(a);

    return _Card(
      child: Column(
        children: [
          _CardRow(
            icon: Icons.place_outlined,
            title: 'Yetkazib berish manzili',
            subtitle: a == null
                ? 'Yetkazib berish uchun manzil kerak'
                : (text.isEmpty ? 'Xaritada tanlangan manzil' : text),
            note: a == null ? null : (extras.isEmpty ? null : extras),
            onTap: _pickAddress,
          ),
          const Divider(height: 1, indent: 44),
          _CardRow(
            icon: Icons.schedule,
            title: 'Yetkazish vaqti',
            trailing: Text(_eta.text,
                style: const TextStyle(
                    fontSize: 14.5, color: Color(0xFF757575))),
          ),
        ],
      ),
    );
  }

  /// Yetkazish oralig'i.
  ///
  /// HAQIQIY manba — restoran yozib qo'ygan `eta_min/max_minutes`.
  /// Restoran uni kiritmagan bo'lsa MOCK oraliq ko'rsatiladi
  /// ([_mockEtaFallback]): bu maydon vaqtga emas, faqat kutishga
  /// ta'sir qiladi, ya'ni noto'g'ri bo'lsa ham pulga tegmaydi.
  ({String text, bool isMock}) get _eta {
    final min = (_restaurant?['eta_min_minutes'] as num?)?.toInt() ?? 0;
    final max = (_restaurant?['eta_max_minutes'] as num?)?.toInt() ?? 0;
    if (min > 0 && max > 0) {
      return (text: '$min–$max daqiqa', isMock: false);
    }
    return (text: _mockEtaFallback, isMock: true);
  }

  Widget _dineInCard() {
    final label = _cart.tableLabel;
    return _Card(
      child: Column(
        children: [
          _CardRow(
            icon: Icons.qr_code_2,
            title: tableText(label ?? ''),
            subtitle: 'Buyurtma to\'g\'ridan-to\'g\'ri oshxonaga tushadi',
          ),
          const Divider(height: 1, indent: 44),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('Necha kishisiz?',
                    style: TextStyle(fontWeight: FontWeight.w600)),
                const SizedBox(height: 10),
                Wrap(
                  spacing: 8,
                  children: [
                    for (final n in [1, 2, 3, 4, 5, 6])
                      ChoiceChip(
                        label: Text('$n'),
                        selected: _partySize == n,
                        selectedColor: kBrand.withValues(alpha: 0.16),
                        onSelected: (_) => setState(() => _partySize = n),
                      ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// Kvartira/qavat/domofon — bo'sh bo'lganlari ko'rsatilmaydi.
  String _addressExtras(Map<String, dynamic> a) {
    final parts = <String>[
      for (final k in ['entrance', 'floor', 'apartment'])
        if (((a[k] as String?) ?? '').trim().isNotEmpty)
          _label(k, a[k] as String),
    ];
    return parts.join(' · ');
  }

  static String _label(String key, String value) => switch (key) {
        'entrance' => '$value-podez',
        'floor' => '$value-qavat',
        'apartment' => '$value-xonadon',
        _ => value,
      };

  // ── Buyurtma tafsilotlari ─────────────────────────────────────────

  Widget _orderDetailsCard() {
    final rid = _cart.restaurantId ?? '';
    final byId = {for (final p in _menu) (p['id'] as String? ?? ''): p};
    final entries = _cart.items.entries.toList();

    return _Card(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 4),
            child: Row(
              children: [
                Icon(Icons.shopping_bag_outlined, size: 22, color: kBrand),
                SizedBox(width: 12),
                Text('Buyurtma tafsilotlari',
                    style:
                        TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              ],
            ),
          ),
          for (var i = 0; i < entries.length; i++) ...[
            if (i > 0) const Divider(height: 1, indent: 16, endIndent: 16),
            _OrderItemRow(
              product: byId[entries[i].key],
              productId: entries[i].key,
              qty: entries[i].value,
              restaurantId: rid,
              serverLineTotalTiyin: _lineTotals[entries[i].key],
            ),
          ],
        ],
      ),
    );
  }

  // ── To'lov usuli (MOCK) ───────────────────────────────────────────

  Widget _payMethodCard() {
    return _Card(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 8),
            child: Row(
              children: [
                Icon(Icons.credit_card, size: 22, color: kBrand),
                SizedBox(width: 12),
                Text('To\'lov usuli',
                    style:
                        TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              ],
            ),
          ),
          for (var i = 0; i < _PayMethod.values.length; i++) ...[
            if (i > 0) const Divider(height: 1, indent: 60, endIndent: 16),
            _PayMethodRow(
              method: _PayMethod.values[i],
              selected: _payMethod == _PayMethod.values[i],
              onTap: () =>
                  setState(() => _payMethod = _PayMethod.values[i]),
            ),
          ],
        ],
      ),
    );
  }

  // ── Kupon (MOCK) ──────────────────────────────────────────────────

  Widget _couponCard() {
    final code = _coupon;
    return _Card(
      child: _CardRow(
        icon: Icons.confirmation_number_outlined,
        title: 'Kupon yoki promokod',
        subtitle: code == null || code.isEmpty
            ? 'Chegirma uchun kod kiriting'
            : code,
        onTap: _enterCoupon,
      ),
    );
  }

  Future<void> _enterCoupon() async {
    final result = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _CouponSheet(initialCode: _coupon ?? ''),
    );
    if (result != null && mounted) {
      setState(() => _coupon = result.isEmpty ? null : result);
    }
  }

  // ── Yakuniy hisob ─────────────────────────────────────────────────

  Widget _totalsCard() {
    final sub = _subtotal;
    final disc = _discount ?? 0;

    return _Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            if (sub != null)
              _TotalLine(label: 'Savatdagi tovarlar', value: formatSum(sub)),
            if (disc > 0)
              _TotalLine(
                label: discountLineLabel(
                  discountTiyin: disc,
                  promotionDiscountTiyin: _promotionDiscount,
                  promotionName: _promotionName,
                ),
                value: '− ${formatSum(disc)}',
                color: const Color(0xFF16A34A),
              ),
            // Yetkazish: qiymat 0 bo'lsa "Bepul" — bu HAQIQAT, chunki
            // server yetkazish haqini umuman hisoblamaydi.
            _TotalLine(
              label: 'Yetkazish',
              value: _mockDeliveryFeeTiyin == 0
                  ? 'Bepul'
                  : formatSum(_mockDeliveryFeeTiyin),
              color: _mockDeliveryFeeTiyin == 0
                  ? const Color(0xFF16A34A)
                  : null,
            ),
            if (_mockServiceFeeTiyin > 0)
              _TotalLine(
                label: 'Xizmat yig\'imi',
                value: formatSum(_mockServiceFeeTiyin),
              ),
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: _DashedLine(),
            ),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text('Jami to\'lov:',
                    style:
                        TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                Text(
                  // Jami — SERVER qiymati. Yuqoridagi mock qatorlar
                  // unga QO'SHILMAYDI: ular 0 va serverda ham yo'q.
                  formatSum(_total),
                  style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: kBrand),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  // ── Pastki tugma ──────────────────────────────────────────────────

  Widget _bottomBar() {
    final ready = !_submitting && !_quoting && _total > 0;

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
        // ┌─ SHADDIY OG'ZAKI TASDIQNI SHU YERGA OLIB KELADI ─────────┐
        // Foydalanuvchi "ha" desa, ilova AYNAN shu tugmani bosadi va
        // AYNAN shu `_submit` chaqiriladi — naqd to'lov bilan.
        // Yangi buyurtma yo'li yozilmagan: server tekshiruvlari,
        // narx qayta hisoblanishi, idempotentlik kaliti — hammasi
        // o'z joyida qoladi.
        //
        // Tugma tayyor bo'lmasa (`ready == false`) ro'yxatga
        // `null` beriladi va Shaddiy hech narsa bosa olmaydi.
        // └──────────────────────────────────────────────────────────┘
        child: AgentCheckoutConfirmAnchor(
          onCashConfirm: ready ? _submitCash : null,
          child: SizedBox(
          height: 56,
          child: FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: kBrand,
              disabledBackgroundColor: kBrand.withValues(alpha: 0.5),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(16)),
            ),
            onPressed: ready ? _submit : null,
            child: _submitting
                ? const SizedBox(
                    width: 22,
                    height: 22,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Colors.white),
                  )
                : Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      const Icon(Icons.lock_outline, size: 18),
                      const SizedBox(width: 8),
                      // ┌─ MATN TANLANGAN USULGA QARAB O'ZGARADI ────┐
                      // Maketda har doim "... so'm to'lash". Lekin
                      // naqd tanlanganda ilova hech qanday pul
                      // yechmaydi — hisob kuryerga beriladi. O'shanda
                      // "to'lash" deyish mijozni shu yerda pul
                      // yechiladi deb o'ylatardi.
                      //
                      // Karta/Payme/Click uchun ham to'lov integratsiyasi
                      // hali YO'Q, lekin ular tanlanganda mijoz to'lov
                      // qadamini kutadi — matn maketdagidek qoladi.
                      // └────────────────────────────────────────────┘
                      Text(
                        _payMethod == _PayMethod.cash
                            ? 'Buyurtma berish · ${formatSum(_total)}'
                            : '${formatSum(_total)} to\'lash',
                        style: const TextStyle(
                            fontSize: 16.5, fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(width: 8),
                      const Icon(Icons.chevron_right, size: 22),
                    ],
                  ),
          ),
        ),
        ),
      ),
    );
  }

  /// Shaddiy og'zaki tasdiqdan keyin chaqiradigan yo'l.
  ///
  /// ┌─ NAQD — YAGONA AVTOMATIK YO'L ─────────────────────────────────┐
  /// Karta bilan to'lash uchun bank sahifasi ochiladi va u yerda
  /// SMS kod kiritiladi — buni ovoz bilan bajarib bo'lmaydi va
  /// bajarilmasligi ham kerak. Shuning uchun og'zaki tasdiq FAQAT
  /// naqd uchun: pul kuryerga beriladi, ilova hech qanday pul
  /// yechmaydi.
  ///
  /// To'lov usuli MAJBURAN naqdga o'tkaziladi — foydalanuvchi
  /// ekranda kartani tanlab qo'ygan bo'lsa ham, og'zaki "ha" karta
  /// to'lovini boshlab yubormasligi kerak.
  /// └────────────────────────────────────────────────────────────────┘
  Future<void> _submitCash() async {
    if (_submitting) return;
    if (_payMethod != _PayMethod.cash) {
      setState(() => _payMethod = _PayMethod.cash);
    }
    await _submit();
  }
}

// ═══════════════════════════════════════════════════════════════════
// KUPON PANELI
// ═══════════════════════════════════════════════════════════════════

/// Kupon kodini kiritish paneli.
///
/// ┌─ NEGA ALOHIDA StatefulWidget ─────────────────────────────────────┐
/// Avval `TextEditingController` chaqiruvchi metodda yaratilib,
/// `await showModalBottomSheet(...)` dan KEYIN darhol `dispose()`
/// qilinardi. Bu qurilmada QIZIL XATO ekranini berardi:
///
///   'package:flutter/src/widgets/framework.dart': Failed assertion:
///   '_dependents.isEmpty': is not true
///
/// Sabab: `showModalBottomSheet` marshrut yopilishi bilan DARHOL
/// qaytadi, lekin panel vidjetlari yopilish ANIMATSIYASI davomida
/// daraxtda qoladi va `TextField` hali o'sha kontrollerga obuna
/// bo'lib turadi. Obunachisi bor `Listenable` ni yo'q qilish esa
/// aynan shu assertni buzadi.
///
/// Endi kontroller panelning O'ZIGA tegishli va uning `dispose()`
/// ida tozalanadi — ya'ni vidjet daraxtdan chindan chiqqanda.
/// └───────────────────────────────────────────────────────────────────┘
class _CouponSheet extends StatefulWidget {
  final String initialCode;
  const _CouponSheet({required this.initialCode});

  @override
  State<_CouponSheet> createState() => _CouponSheetState();
}

class _CouponSheetState extends State<_CouponSheet> {
  late final _controller = TextEditingController(text: widget.initialCode);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      // Klaviatura panelni bosib qolmasin.
      padding: EdgeInsets.only(
          bottom: MediaQuery.of(context).viewInsets.bottom),
      child: Container(
        decoration: const BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
        ),
        padding: const EdgeInsets.fromLTRB(20, 16, 20, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 38,
                height: 4,
                decoration: BoxDecoration(
                  color: const Color(0xFFE0E0E0),
                  borderRadius: BorderRadius.circular(4),
                ),
              ),
            ),
            const SizedBox(height: 18),
            const Text('Kupon yoki promokod',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 12),
            AppTextField(
              controller: _controller,
              hint: 'Masalan: ONDEX10',
              autofocus: true,
              textCapitalization: TextCapitalization.characters,
              textInputAction: TextInputAction.done,
              onSubmitted: (v) => Navigator.of(context).pop(v.trim()),
            ),
            const SizedBox(height: 10),
            // Bu matn ATAYLAB bor: kupon tizimi hali ulanmagan va kod
            // kiritgan mijoz chegirmani KUTIB qolmasligi kerak.
            const Row(
              children: [
                Icon(Icons.info_outline, size: 15, color: Color(0xFF9E9E9E)),
                SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Kupon tizimi hali ulanmagan — kod summani '
                    'o\'zgartirmaydi. Aksiyalar avtomatik qo\'llanadi.',
                    style: TextStyle(
                        fontSize: 12, color: Color(0xFF9E9E9E), height: 1.4),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            SizedBox(
              height: 50,
              width: double.infinity,
              child: FilledButton(
                style: FilledButton.styleFrom(
                  backgroundColor: kBrand,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(14)),
                ),
                onPressed: () =>
                    Navigator.of(context).pop(_controller.text.trim()),
                child: const Text('Saqlash',
                    style: TextStyle(
                        fontSize: 15.5, fontWeight: FontWeight.bold)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// TO'LOV USULLARI — HAMMASI MOCK
// ═══════════════════════════════════════════════════════════════════
//
// ┌─ NIMA ISHLAYDI, NIMA YO'Q ────────────────────────────────────────┐
// Backendda to'lov tizimi UMUMAN yo'q: na jadval, na endpoint, na
// Payme/Click integratsiyasi. `POST /orders` to'lov usulini qabul
// qilmaydi.
//
// Ya'ni bu ro'yxat — KO'RINISH. Qaysi usul tanlansa ham buyurtma bir
// xil yaratiladi va hisob amalda kuryerga to'lanadi.
//
// Haqiqiy to'lov ulanganda: bu enum saqlanadi, `createOrder` ga
// `payment_method` qo'shiladi va karta/Payme/Click tanlanganda
// buyurtmadan KEYIN to'lov ekrani ochiladi.
// └───────────────────────────────────────────────────────────────────┘
/// To'lov tizimlari logotiplari — `assets/payments/`.
const _kVisa = 'assets/payments/visa.png';
const _kMastercard = 'assets/payments/mastercard.png';
const _kUzcard = 'assets/payments/uzcard.png';
const _kHumo = 'assets/payments/humo.png';
const _kPayme = 'assets/payments/payme.png';
const _kClick = 'assets/payments/click.png';

enum _PayMethod {
  /// Chap tomonda umumiy karta ikoni, O'NG tomonda esa qabul
  /// qilinadigan tizimlar logotiplari — maketdagidek.
  card(
    title: 'Bank kartasi',
    subtitle: 'Visa, Mastercard, UzCard, Humo',
    icon: Icons.credit_card,
    tile: Color(0xFFFDE8E2),
    iconColor: kBrand,
    trailingLogos: [_kVisa, _kMastercard, _kUzcard, _kHumo],
  ),
  cash(
    title: 'Naqd pul',
    subtitle: 'Yetkazib berishda',
    icon: Icons.payments_outlined,
    tile: Color(0xFFE8F5E9),
    iconColor: Color(0xFF2E7D32),
  ),

  /// Payme va Click uchun ikon O'RNIGA brend logotipi turadi — ular
  /// tanilgan belgilar va umumiy ikonka ularni tanib olishni
  /// qiyinlashtirardi.
  payme(
    title: 'Payme',
    subtitle: 'Ilova orqali to\'lov',
    icon: Icons.account_balance_wallet_outlined,
    tile: Colors.white,
    iconColor: Color(0xFF00838F),
    logo: _kPayme,
  ),
  click(
    title: 'Click',
    subtitle: 'Ilova orqali to\'lov',
    icon: Icons.bolt,
    tile: Colors.white,
    iconColor: Color(0xFFC62828),
    logo: _kClick,
  );

  final String title;
  final String subtitle;

  /// Zaxira ikon — logotip fayli topilmasa ishlatiladi.
  final IconData icon;
  final Color tile;
  final Color iconColor;

  /// Chapdagi katakcha ichida ikon O'RNIGA chiziladigan logotip.
  final String? logo;

  /// Qator oxirida chiziladigan logotiplar.
  final List<String> trailingLogos;

  const _PayMethod({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.tile,
    required this.iconColor,
    this.logo,
    this.trailingLogos = const [],
  });
}

class _PayMethodRow extends StatelessWidget {
  final _PayMethod method;
  final bool selected;
  final VoidCallback onTap;

  const _PayMethodRow({
    required this.method,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
        child: Row(
          children: [
            // Radio o'rniga qo'lda chizilgan doira — Material radiosi
            // o'z chekinishini olib kelib, qatorlarni notekis qilardi.
            Container(
              width: 20,
              height: 20,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  color: selected ? kBrand : const Color(0xFFBDBDBD),
                  width: 2,
                ),
              ),
              child: selected
                  ? Center(
                      child: Container(
                        width: 10,
                        height: 10,
                        decoration: const BoxDecoration(
                          shape: BoxShape.circle,
                          color: kBrand,
                        ),
                      ),
                    )
                  : null,
            ),
            const SizedBox(width: 14),
            Container(
              width: 44,
              height: 32,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: method.tile,
                borderRadius: BorderRadius.circular(8),
                // Oq katakcha (Payme/Click logotiplari) fonda
                // yo'qolib ketmasin.
                border: method.tile == Colors.white
                    ? Border.all(color: const Color(0xFFEEEEEE))
                    : null,
              ),
              clipBehavior: Clip.antiAlias,
              child: method.logo == null
                  ? Icon(method.icon, size: 19, color: method.iconColor)
                  : Padding(
                      padding: const EdgeInsets.all(3),
                      child: _Logo(
                        asset: method.logo!,
                        // Logotip KESILMASIN — `contain`.
                        fallback: Icon(method.icon,
                            size: 19, color: method.iconColor),
                      ),
                    ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(method.title,
                      style: const TextStyle(
                          fontSize: 15, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 1),
                  Text(method.subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 12.5, color: Color(0xFF9E9E9E))),
                ],
              ),
            ),
            if (method.trailingLogos.isNotEmpty) ...[
              const SizedBox(width: 8),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  for (final logo in method.trailingLogos)
                    Padding(
                      padding: const EdgeInsets.only(left: 5),
                      child: SizedBox(
                        width: 30,
                        height: 20,
                        child: _Logo(asset: logo),
                      ),
                    ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Logotip rasmi.
///
/// `contain` — turli nisbatdagi logotiplar (Visa keng, Humo kvadratga
/// yaqin) KESILMASDAN, o'z nisbatini saqlab chiziladi.
///
/// Fayl topilmasa ilova YIQILMAYDI: zaxira vidjet yoki bo'sh joy
/// qoladi. Bu muhim, chunki assetlar `image/` papkasidan nusxa
/// olinadi va u repoga kirmaydi.
class _Logo extends StatelessWidget {
  final String asset;
  final Widget? fallback;

  const _Logo({required this.asset, this.fallback});

  @override
  Widget build(BuildContext context) {
    return Image.asset(
      asset,
      fit: BoxFit.contain,
      errorBuilder: (_, __, ___) => fallback ?? const SizedBox.shrink(),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// QAYTA ISHLATILADIGAN BO'LAKLAR
// ═══════════════════════════════════════════════════════════════════

/// Oq karta — maketdagi barcha bloklar shu ko'rinishda.
class _Card extends StatelessWidget {
  final Widget child;
  const _Card({required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: const Color(0xFFEEEEEE)),
      ),
      clipBehavior: Clip.antiAlias,
      child: child,
    );
  }
}

/// Ikon + sarlavha + izoh qatori.
class _CardRow extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final String? note;
  final Widget? trailing;
  final VoidCallback? onTap;

  const _CardRow({
    required this.icon,
    required this.title,
    this.subtitle,
    this.note,
    this.trailing,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final row = Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 22, color: kBrand),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title,
                    style: const TextStyle(
                        fontSize: 15.5, fontWeight: FontWeight.bold)),
                if (subtitle != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 3),
                    child: Text(subtitle!,
                        style: const TextStyle(
                            fontSize: 14.5, color: Color(0xFF424242))),
                  ),
                if (note != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(note!,
                        style: const TextStyle(
                            fontSize: 13, color: Color(0xFF9E9E9E))),
                  ),
              ],
            ),
          ),
          if (trailing != null) ...[const SizedBox(width: 8), trailing!],
          if (onTap != null)
            const Padding(
              padding: EdgeInsets.only(left: 6),
              child: Icon(Icons.chevron_right,
                  size: 20, color: Color(0xFF9E9E9E)),
            ),
        ],
      ),
    );

    if (onTap == null) return row;
    return InkWell(onTap: onTap, child: row);
  }
}

/// Buyurtmadagi bitta taom — rasm, nom, tavsif, miqdor va narx.
class _OrderItemRow extends StatelessWidget {
  final Map<String, dynamic>? product;
  final String productId;
  final int qty;
  final String restaurantId;

  /// Serverning shu qator uchun yakuniy summasi (tiyin). `null` —
  /// javob yo'q; shunda mahsulotning O'Z chegirma narxiga tushamiz
  /// (aksiyalar bu yerda taxmin qilinmaydi: server ularni butun savatga
  /// qo'llaydi, ya'ni bitta qatorga "to'g'ri" ulush faqat serverdan
  /// kelishi mumkin).
  final int? serverLineTotalTiyin;

  const _OrderItemRow({
    required this.product,
    required this.productId,
    required this.qty,
    required this.restaurantId,
    required this.serverLineTotalTiyin,
  });

  @override
  Widget build(BuildContext context) {
    final cart = CartStore.instance;
    final p = product;
    final name = (p?['name'] as String?) ?? 'Yuklanmoqda…';
    final desc = ((p?['description'] as String?) ?? '').trim();
    final image = (p?['image_url'] as String?) ?? '';
    final price = (p?['price_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (p?['discount_price_tiyin'] as num?)?.toInt() ?? 0;
    final unit = (discount > 0 && discount < price) ? discount : price;
    final lineSubtotal = price * qty;
    final lineTotal = serverLineTotalTiyin ?? unit * qty;

    return Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: SizedBox(
              width: 76,
              height: 76,
              child: image.isEmpty
                  ? Container(
                      color: const Color(0xFFF5F5F5),
                      child: const Icon(Icons.restaurant_menu,
                          color: Color(0xFFBDBDBD)),
                    )
                  : RemoteImage(
                      url: image,
                      placeholder: Container(
                        color: const Color(0xFFF5F5F5),
                        child: const Icon(Icons.restaurant_menu,
                            color: Color(0xFFBDBDBD)),
                      ),
                    ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 15.5, fontWeight: FontWeight.w600)),
                if (desc.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(desc,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 13,
                            height: 1.35,
                            color: Color(0xFF9E9E9E))),
                  ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              // Savat ekranidagi bilan AYNAN bir xil vidjet — alohida
              // nusxa emas (`widgets/qty_stepper.dart`).
              QtyStepper(
                qty: qty,
                onAdd: () => cart.increment(
                    restaurantId: restaurantId, productId: productId),
                onRemove: () => cart.decrement(
                    restaurantId: restaurantId, productId: productId),
              ),
              const SizedBox(height: 10),
              Text(
                formatSum(lineTotal),
                style: const TextStyle(
                    fontSize: 15.5,
                    fontWeight: FontWeight.bold,
                    color: kBrand),
              ),
              // Chegirma bo'lsa tan narx ham ko'rinadi — mijoz nechi
              // so'm yutganini ko'rsin (savat ekranidagi bilan bir xil).
              if (lineTotal < lineSubtotal)
                Text(
                  formatSum(lineSubtotal),
                  style: const TextStyle(
                    fontSize: 12,
                    color: Color(0xFF9E9E9E),
                    decoration: TextDecoration.lineThrough,
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _TotalLine extends StatelessWidget {
  final String label;
  final String value;
  final Color? color;
  const _TotalLine({required this.label, required this.value, this.color});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Expanded(
            child: Text(label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                    fontSize: 14.5, color: Color(0xFF757575))),
          ),
          const SizedBox(width: 12),
          Text(value,
              style: TextStyle(
                  fontSize: 14.5,
                  color: color ?? const Color(0xFF424242))),
        ],
      ),
    );
  }
}

/// Maketdagi punktir chiziq.
class _DashedLine extends StatelessWidget {
  const _DashedLine();

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 1,
      child: CustomPaint(painter: _DashedPainter(), child: Container()),
    );
  }
}

class _DashedPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = const Color(0xFFE0E0E0)
      ..strokeWidth = 1;
    const dash = 4.0;
    const gap = 4.0;
    var x = 0.0;
    while (x < size.width) {
      canvas.drawLine(Offset(x, 0), Offset(x + dash, 0), paint);
      x += dash + gap;
    }
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

class _ErrorBox extends StatelessWidget {
  final String text;
  const _ErrorBox({required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFBEAE9),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline, size: 18, color: Color(0xFFB3261E)),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: const TextStyle(
                    color: Color(0xFFB3261E), fontSize: 13.5)),
          ),
        ],
      ),
    );
  }
}
