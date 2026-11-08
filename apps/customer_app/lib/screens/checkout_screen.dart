import 'dart:async';

import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../widgets/order_blocks.dart';
import '../data/address_scope.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import '../data/payment_options.dart';
import '../widgets/agent_overlay.dart' show AgentCheckoutConfirmAnchor;
import '../widgets/app_text_field.dart';
import '../data/quote_service.dart';
import '../widgets/product_grid.dart' show discountLineLabel;
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import '../payment_nav_policy.dart' show isAcceptablePayUrl;
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
// ENG MUHIM QOIDA: mock qiymatlar PULGA TEGMAYDI. Xizmat yig'imi 0 —
// chunki server uni hisoblamaydi va mijozdan olmaydi. Agar bu yerga
// 15 000 yozilsa, ekranda ko'ringan "Jami" serverda yoziladigan
// summadan FARQ QILARDI: mijoz bir raqamga rozi bo'lib, boshqasini
// to'lardi.
//
// YETKAZISH HAQI endi mock EMAS: u serverdan keladi (`delivery_fee_tiyin`,
// yetkazib beruvchining maosh usulidan) va faqat SHU ekranda
// "Yetkazish" qatorida ko'rinadi.
// └───────────────────────────────────────────────────────────────────┘

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
///   * "Xizmat yig'imi" qatori — backend uni hisoblamaydi. Uni chizish
///     mijozga hech qachon to'lamaydigan raqamni ko'rsatish bo'lardi.
///     ("Yetkazish" qatori esa HAQIQIY: `QuoteResult.delivery_fee_tiyin`.)
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
  /// TO'LANADIGAN jami (taomlar + yetkazish haqi) — serverning `total_tiyin`.
  final int quoteTiyin;

  /// Yetkazish haqi (tiyin), `quoteTiyin` ga ALLAQACHON qo'shilgan.
  /// 0 — bepul yoki stol buyurtmasi.
  final int deliveryFeeTiyin;
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
    this.deliveryFeeTiyin = 0,
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
  late int _deliveryFee = widget.deliveryFeeTiyin;
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
    // ┌─ EKRAN OCHILGANDA NARX QAYTA SO'RALADI ───────────────────────┐
    // `widget.deliveryFeeTiyin` — savat ekranida ESKIROQ paytda
    // olingan hisob. Yetkazish haqi restoran sozlamasi EMAS, HOZIR
    // ishlayotgan kuryerlarning maosh usulidan jonli hisoblanadi
    // (`staff.DeliveryFee`): mijoz savatni ko'rib, checkout'ga o'tib,
    // manzilni tanlab o'tirgan vaqtida xodimlar ro'yxati o'zgarishi
    // mumkin. Busiz checkout "Bepul" ko'rsatardi-yu, `_submit()`
    // esa HOZIRGI (ko'pincha yuqoriroq) haq bilan yaratilgan
    // buyurtmani 409 bilan rad etardi — mijoz sababni tushunmasdan
    // qolardi. Endi ekran ochilishi bilan HAQIQIY raqam ko'rinadi.
    // └──────────────────────────────────────────────────────────────┘
    _refreshQuote();
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
        setState(() {
          _restaurant = found;
          _ensureSelectedMethodVisible();
        });
      }
    } catch (_) {
      // Yetkazish vaqti ko'rsatilmaydi — qolgani ishlayveradi.
    }
    // Keshdagi ro'yxat eskirgan bo'lishi mumkin: restoran to'lov usulini
    // yaqinda o'chirgan bo'lsa mijoz uni ko'rib qolmasligi uchun
    // to'lov usullari yangi javobdan olinadi.
    try {
      final fresh = await api.restaurant(rid);
      if (!mounted || fresh.isEmpty) return;
      setState(() {
        _restaurant = {...?_restaurant, ...fresh};
        _ensureSelectedMethodVisible();
      });
    } catch (_) {
      // Tarmoq xatosi: keshdagi qiymat qoladi, server baribir tekshiradi.
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
        _total = q.payableTiyin!;
        _deliveryFee = q.deliveryFeeTiyin;
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
      final lat = (a['lat'] as num?)?.toDouble() ?? 0;
      final lng = (a['lng'] as num?)?.toDouble() ?? 0;
      // ┌─ UMUMIY MANBA HAM YANGILANADI ──────────────────────────────┐
      // Bu ekran manzilni serverdan O'ZI o'qiydi (buyurtmaga podyezd/
      // qavat/kvartira ham kerak, ular `AddressScope` da saqlanmaydi).
      // Lekin natijani faqat o'ziga olib qo'ysa, manzil boshqa joyda
      // o'zgargan bo'lganda (masalan vebda) bosh sahifa va restoran
      // sarlavhasi eski nomda qolardi. Endi server javobi umumiy
      // manbaga ham quyiladi — `set` o'zgarish bo'lmasa hech kimni
      // bezovta qilmaydi.
      // └─────────────────────────────────────────────────────────────┘
      AddressScope.instance.set(
        lat: lat,
        lng: lng,
        text: (a['text'] ?? '').toString().trim(),
      );
      if (!mounted) return;
      setState(() {
        // lat/lng == 0 — manzil hali tanlanmagan.
        _address = lat != 0 ? a : null;
        _loadingAddress = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loadingAddress = false);
    }
  }

  Future<void> _pickAddress() async {
    await Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => const AddressScreen()));
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
      // Mijoz aynan bitta ilovani tanlagan bo'lsa (`channel`), server
      // o'sha ilovaga to'g'ridan-to'g'ri havola qaytaradi; bo'sh
      // bo'lsa — universal to'lov sahifasi.
      final p = await api.startPayment(
        orderId,
        paymentSystem: _payMethod.channel,
      );
      final url = (p['pay_url'] as String?) ?? '';
      // Faqat https (bug.md 85d) — web bilan bir xil shart.
      if (!isAcceptablePayUrl(url)) throw Exception('to\'lov havolasi yaroqsiz');

      // ┌─ ILOVA HAVOLASI WEBVIEW'DA OCHILMAYDI ────────────────────┐
      // Mijoz Payme/Click/Uzum ni tanlasa, shlyuz o'sha ilovaga
      // havola (Universal/App Link) qaytaradi. U `https://` bilan
      // boshlanadi, lekin WEBVIEW ICHIDA yuklansa Android App Link
      // mexanizmi ISHLAMAYDI — ilova ochilmaydi, oddiy veb sahifa
      // ko'rinadi va mijoz "nega Payme ochilmadi?" deb qoladi.
      //
      // Shuning uchun kanal tanlangan bo'lsa havola TIZIMGA
      // uzatiladi: o'rnatilgan bo'lsa ilova, bo'lmasa brauzer
      // ochiladi. Mijoz qaytgach holatni kuzatuv ekrani serverdan
      // so'rab oladi (WebView'dagi kabi).
      // └────────────────────────────────────────────────────────────┘
      if (_payMethod.channel.isNotEmpty) {
        final opened = await launchUrl(
          Uri.parse(url),
          mode: LaunchMode.externalApplication,
        );
        // Ochilmasa WebView zaxira yo'l bo'lib qoladi — mijoz
        // to'lovsiz qolib ketmasin.
        if (opened) return true;
      }

      if (!mounted) return false;
      await Navigator.of(context).push<bool>(
        MaterialPageRoute(
          builder:
              (_) => PaymentWebViewScreen(
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
          'kutmoqda — "Buyurtmalarim" bo\'limidan qayta urinib ko\'ring.',
        );
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
            {'product_id': e.key, 'qty': e.value},
        ],
        idempotencyKey: _idempotencyKey,
        lat: _isDineIn ? null : (_address!['lat'] as num).toDouble(),
        lng: _isDineIn ? null : (_address!['lng'] as num).toDouble(),
        tableToken: _isDineIn ? _cart.tableToken : null,
        partySize: _isDineIn ? _partySize : null,
        // `apiValue` — enum'ning O'ZIDA, to'liq `switch` bilan
        // (bug.md 81-band). Avval bu yerda `== card ? 'card' : 'cash'`
        // turardi va Payme/Click jimgina NAQD ga aylanardi.
        paymentMethod: _payMethod.apiValue,
        // ┌─ EKRANDA KO'RSATILGAN SUMMA QO'RIQLANADI ──────────────────┐
        // Bu ekranda ko'rsatilgan `_total` so'rov yuborilgunga qadar
        // ESKIRISHI mumkin: yetkazish haqi RESTORAN SOZLAMASI emas,
        // HOZIR ishlayotgan kuryerlarning maosh usulidan jonli
        // hisoblanadi (`staff.DeliveryFee`). Shu oraliqda xodimlar
        // ro'yxati o'zgarsa (masalan kuryerning kirishi yopilsa),
        // buyurtma JIMGINA boshqa (ko'pincha 0) haq bilan yozilib
        // qolardi — mijoz "savatda hisoblangan" bilan "Buyurtmalarim"da
        // ko'rgan raqam farq qilganini angladi. `assistant_screen.dart`
        // da bu himoya bor edi, shu ekranda esa YO'Q edi.
        // └───────────────────────────────────────────────────────────┘
        expectedTotalTiyin: _total,
      );

      if (!mounted) return;

      // ┌─ KARTA: TO'LOV SAHIFASI ──────────────────────────────────┐
      // Buyurtma yaratildi, lekin u TO'LOV KUTIB turibdi va
      // oshxonaga tushmagan. To'lov havolasi olinmasa — buyurtmani
      // shu yerda bekor qilamiz: mijoz "buyurtma berildi" deb
      // o'ylab qolmasligi kerak.
      // └───────────────────────────────────────────────────────────┘
      if (_payMethod.isOnline) {
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
      Navigator.of(
        context,
      ).pushReplacement(sheetRoute(TrackingScreen(orderId: id)));
    } catch (e) {
      if (!mounted) return;
      // 409 — narx so'rov yuborilgunga qadar o'zgargan (yetkazish haqi
      // jonli hisoblanadi). Buyurtma YARATILMAGAN — eski raqamni
      // jimgina o'tkazib yubormasdan, ekranni YANGI summaga yangilab
      // mijozga qayta ko'rsatamiz (`assistant_screen.dart` dagi bilan
      // bir xil naqsh).
      if (e is ApiException && e.statusCode == 409) {
        await _refreshQuote();
        if (!mounted) return;
        setState(() {
          _submitting = false;
          _error = 'Narx o\'zgardi — yangi summani tekshirib, qayta urining';
        });
        return;
      }
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
        appBar: const OrderPageAppBar(),
        body:
            _loadingAddress
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
                      OrderErrorBox(text: _error!),
                    ],
                  ],
                ),
        bottomNavigationBar: _bottomBar(),
      ),
    );
  }

  // ── Manzil va yetkazish vaqti ─────────────────────────────────────

  Widget _addressCard() {
    final a = _address;
    final text = ((a?['text'] as String?) ?? '').trim();
    final extras = a == null ? '' : _addressExtras(a);

    return OrderCard(
      child: Column(
        children: [
          OrderCardRow(
            icon: Icons.place_outlined,
            title: 'Yetkazib berish manzili',
            subtitle:
                a == null
                    ? 'Yetkazib berish uchun manzil kerak'
                    : (text.isEmpty ? 'Xaritada tanlangan manzil' : text),
            note: a == null ? null : (extras.isEmpty ? null : extras),
            onTap: _pickAddress,
          ),
          const Divider(height: 1, indent: 44),
          OrderCardRow(
            icon: Icons.schedule,
            title: 'Yetkazish vaqti',
            trailing: Text(
              _eta.text,
              style: const TextStyle(fontSize: 14.5, color: Color(0xFF757575)),
            ),
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
    return OrderCard(
      child: Column(
        children: [
          OrderCardRow(
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
                const Text(
                  'Necha kishisiz?',
                  style: TextStyle(fontWeight: FontWeight.w600),
                ),
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

    return OrderCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const OrderDetailsHeader(),
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

  /// Usul shu restoranda mijozga KO'RSATILADIMI.
  ///
  /// ┌─ FAQAT RESTORAN YOQQAN USULLAR KO'RINADI ──────────────────────┐
  /// Restoran paneldagi "To'lov usullari" da Naqd / Bank kartasi /
  /// Payme / Click / Uzum ni alohida yoqadi. Server shu tanlovdan
  /// `payment_options` ni hisoblaydi (sozlama VA to'lov hisobi ulangan
  /// bo'lishi): o'chirilgan usul bu yerda KULRANG emas, UMUMAN yo'q.
  /// Server `/pay` da ham shuni tekshiradi — bu yerdagi yashirish
  /// mijozga qulaylik, to'siq esa serverda.
  ///
  /// `payment_options` yo'q bo'lsa (restoran hali yuklanmagan yoki eski
  /// server) usul OCHIQ qoladi: oxirgi so'z baribir serverniki, yuklanish
  /// paytida hamma usulni yashirish mijozni chalg'itardi.
  /// └────────────────────────────────────────────────────────────────┘
  bool _methodAvailable(_PayMethod m) =>
      paymentOptionVisible(_restaurant?['payment_options'], m.optionKey);

  List<_PayMethod> get _visibleMethods => [
    for (final m in _PayMethod.values)
      if (_methodAvailable(m)) m,
  ];

  /// Tanlangan usul yashirilgan bo'lsa (restoran uni o'chirgan) birinchi
  /// ko'rinadigan usulga o'tadi — mijoz ko'rinmaydigan usul bilan
  /// buyurtma berib yubormasligi uchun.
  void _ensureSelectedMethodVisible() {
    if (_methodAvailable(_payMethod)) return;
    final visible = _visibleMethods;
    if (visible.isEmpty) return;
    _payMethod =
        visible.contains(_PayMethod.cash) ? _PayMethod.cash : visible.first;
  }

  Widget _payMethodCard() {
    return OrderCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 8),
            child: Row(
              children: [
                Icon(Icons.credit_card, size: 22, color: kBrand),
                SizedBox(width: 12),
                Text(
                  'To\'lov usuli',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
                ),
              ],
            ),
          ),
          if (_visibleMethods.isEmpty)
            const Padding(
              padding: EdgeInsets.fromLTRB(16, 4, 16, 16),
              child: Text(
                'Bu restoran hozircha to\'lov usulini qabul qilmayapti. '
                'Keyinroq urinib ko\'ring.',
                key: ValueKey('no-pay-methods'),
                style: TextStyle(fontSize: 13.5, color: Color(0xFF757575)),
              ),
            ),
          for (var i = 0; i < _visibleMethods.length; i++) ...[
            if (i > 0) const Divider(height: 1, indent: 60, endIndent: 16),
            _PayMethodRow(
              key: ValueKey('pay-method-${_visibleMethods[i].name}'),
              method: _visibleMethods[i],
              selected: _payMethod == _visibleMethods[i],
              onTap: () => setState(() => _payMethod = _visibleMethods[i]),
            ),
          ],
        ],
      ),
    );
  }

  // ── Kupon (MOCK) ──────────────────────────────────────────────────

  Widget _couponCard() {
    final code = _coupon;
    return OrderCard(
      child: OrderCardRow(
        icon: Icons.confirmation_number_outlined,
        title: 'Kupon yoki promokod',
        subtitle:
            code == null || code.isEmpty ? 'Chegirma uchun kod kiriting' : code,
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

    return OrderCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            if (sub != null)
              OrderTotalLine(label: 'Savatdagi tovarlar', value: formatSum(sub)),
            if (disc > 0)
              OrderTotalLine(
                label: discountLineLabel(
                  discountTiyin: disc,
                  promotionDiscountTiyin: _promotionDiscount,
                  promotionName: _promotionName,
                ),
                value: '− ${formatSum(disc)}',
                color: const Color(0xFF16A34A),
              ),
            // Yetkazish haqi FAQAT shu ekranda ko'rinadi (menyu va savatda
            // yo'q) va HAQIQIY: serverdan keladi, "Jami" ga allaqachon
            // qo'shilgan. Stol buyurtmasida kuryer yo'q — qator umuman
            // chizilmaydi.
            if (!_isDineIn)
              OrderTotalLine(
                key: const ValueKey('checkout-delivery-line'),
                label: 'Yetkazish',
                value: _deliveryFee <= 0 ? 'Bepul' : formatSum(_deliveryFee),
                color: _deliveryFee <= 0 ? const Color(0xFF16A34A) : null,
              ),
            if (_mockServiceFeeTiyin > 0)
              OrderTotalLine(
                label: 'Xizmat yig\'imi',
                value: formatSum(_mockServiceFeeTiyin),
              ),
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: OrderDashedLine(),
            ),
            // Jami — SERVER qiymati (yetkazish haqi bilan). Yuqoridagi mock qator
            // (xizmat yig'imi) 0 va unga QO'SHILMAYDI: u serverda ham yo'q.
            OrderGrandTotal(
              valueTiyin: _total,
              valueKey: const ValueKey('checkout-total'),
            ),
          ],
        ),
      ),
    );
  }

  // ── Pastki tugma ──────────────────────────────────────────────────

  Widget _bottomBar() {
    final ready =
        !_submitting && !_quoting && _total > 0 && _methodAvailable(_payMethod);

    // Tugma ko'rinishi umumiy blokda (widgets/order_blocks.dart). Shaddiy og'zaki
    // tasdig'i AYNAN shu tugmani bosadi va AYNAN shu _submit chaqiriladi — naqd
    // to'lov bilan: server tekshiruvlari, narx qayta hisoblanishi va idempotentlik
    // kaliti o'z joyida qoladi. Tugma tayyor bo'lmasa null beriladi.
    //
    // Matn tanlangan usulga qarab o'zgaradi: naqdda ilova pul yechmaydi (hisob
    // kuryerga beriladi), shuning uchun "to'lash" emas, "Buyurtma berish".
    return OrderSubmitBar(
      label:
          _payMethod == _PayMethod.cash
              ? 'Buyurtma berish · ${formatSum(_total)}'
              : '${formatSum(_total)} to\'lash',
      ready: ready,
      submitting: _submitting,
      onPressed: _submit,
      wrapButton:
          (button) => AgentCheckoutConfirmAnchor(
            onCashConfirm: ready ? _submitCash : null,
            child: button,
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
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
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
            const Text(
              'Kupon yoki promokod',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
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
                      fontSize: 12,
                      color: Color(0xFF9E9E9E),
                      height: 1.4,
                    ),
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
                    borderRadius: BorderRadius.circular(14),
                  ),
                ),
                onPressed:
                    () => Navigator.of(context).pop(_controller.text.trim()),
                child: const Text(
                  'Saqlash',
                  style: TextStyle(fontSize: 15.5, fontWeight: FontWeight.bold),
                ),
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
const _kUzum = 'assets/payments/uzum.png';

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

  /// Payme, Click va Uzum uchun ikon O'RNIGA brend logotipi turadi —
  /// ular tanilgan belgilar va umumiy ikonka ularni tanib olishni
  /// qiyinlashtirardi.
  ///
  /// ┌─ ENDI ISHLAYDI (ilgari "Tez orada" edi) ────────────────────────┐
  /// Avval bu usullar `available: false` edi, chunki Octo'ning
  /// so'rovi to'lov tizimini qabul qilmasdi — u faqat o'z sahifasiga
  /// havola qaytarardi.
  ///
  /// Multicard (Rahmat) esa qabul qiladi: `POST /payment` da
  /// `payment_system` berilsa, javobda AYNAN o'sha ilovaga havola
  /// (deeplink) keladi. Shuning uchun uchalasi ham haqiqiy variantga
  /// aylandi.
  ///
  /// ⚠️ Buyurtmaning to'lov USULI baribir `card` bo'lib qoladi
  /// (`apiValue`): Payme ham, Click ham — onlayn to'lov. Farq faqat
  /// mijoz qaysi ilovada to'lashida va u `channel` orqali ketadi.
  /// └──────────────────────────────────────────────────────────────────┘
  payme(
    title: 'Payme',
    subtitle: 'Payme ilovasida',
    icon: Icons.account_balance_wallet_outlined,
    tile: Colors.white,
    iconColor: Color(0xFF00838F),
    logo: _kPayme,
    channel: 'payme',
  ),
  click(
    title: 'Click',
    subtitle: 'Click ilovasida',
    icon: Icons.bolt,
    tile: Colors.white,
    iconColor: Color(0xFFC62828),
    logo: _kClick,
    channel: 'click',
  ),
  uzum(
    title: 'Uzum',
    subtitle: 'Uzum Bank ilovasida',
    icon: Icons.account_balance_outlined,
    tile: Colors.white,
    iconColor: Color(0xFF7000FF),
    logo: _kUzum,
    channel: 'uzum',
  );

  /// Serverga yuboriladigan qiymat (`POST /orders` dagi
  /// `payment_method`).
  ///
  /// ┌─ NEGA `switch`, TERNARY EMAS ───────────────────────────────────┐
  /// Avval bu joyda `== card ? 'card' : 'cash'` turardi va AYNAN shu
  /// naqsh xatoni tug'dirgan: yangi usul qo'shilganda u jimgina
  /// "naqd" ga tushardi.
  ///
  /// Dart `switch` ifodasi enum uchun TO'LIQLIKNI talab qiladi —
  /// yangi qiymat qo'shilsa kod KOMPILYATSIYA BO'LMAYDI va muallif
  /// qaror qabul qilishga majbur bo'ladi.
  /// └──────────────────────────────────────────────────────────────────┘
  ///
  /// ⚠️ Payme/Click/Uzum ham `card`: server `cash`/`card`/`wallet`
  /// dan boshqasini QABUL QILMAYDI va qilmasligi ham kerak — bular
  /// to'lov usuli emas, to'lov KANALI.
  String get apiValue => switch (this) {
    _PayMethod.card ||
    _PayMethod.payme ||
    _PayMethod.click ||
    _PayMethod.uzum => 'card',
    _PayMethod.cash => 'cash',
  };

  /// Oldindan to'lanadigan usulmi (to'lov sahifasi ochiladimi).
  bool get isOnline => this != _PayMethod.cash;

  /// Serverning `payment_options` dagi kaliti. `switch` — yangi usul
  /// qo'shilsa kompilyator kalit yozishni TALAB qiladi (jimgina
  /// "ko'rinadi" bo'lib qolmasin).
  String get optionKey => switch (this) {
    _PayMethod.card => 'card',
    _PayMethod.cash => 'cash',
    _PayMethod.payme => 'payme',
    _PayMethod.click => 'click',
    _PayMethod.uzum => 'uzum',
  };

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

  /// To'lov shlyuziga yuboriladigan XIZMAT kodi (`?ps=`). Bo'sh —
  /// universal to'lov sahifasi (karta va hammasi bir joyda).
  final String channel;

  const _PayMethod({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.tile,
    required this.iconColor,
    this.logo,
    this.trailingLogos = const [],
    this.channel = '',
  });
}

class _PayMethodRow extends StatelessWidget {
  final _PayMethod method;
  final bool selected;

  /// `null` — usul hozir ishlamaydi (bug.md 81-band): qator
  /// ko'rinadi, lekin bosilmaydi va xiralashgan holda chiziladi.
  final VoidCallback? onTap;

  const _PayMethodRow({
    super.key,
    required this.method,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final enabled = onTap != null;
    return Opacity(
      // Xiralashish — "bu yerda nimadir bor, lekin hozir emas"
      // degan yagona ko'rinish signali.
      opacity: enabled ? 1 : 0.45,
      child: _buildRow(context, enabled),
    );
  }

  Widget _buildRow(BuildContext context, bool enabled) {
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
              child:
                  selected
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
                border:
                    method.tile == Colors.white
                        ? Border.all(color: const Color(0xFFEEEEEE))
                        : null,
              ),
              clipBehavior: Clip.antiAlias,
              child:
                  method.logo == null
                      ? Icon(method.icon, size: 19, color: method.iconColor)
                      : Padding(
                        padding: const EdgeInsets.all(3),
                        child: _Logo(
                          asset: method.logo!,
                          // Logotip KESILMASIN — `contain`.
                          fallback: Icon(
                            method.icon,
                            size: 19,
                            color: method.iconColor,
                          ),
                        ),
                      ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    method.title,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 1),
                  Text(
                    method.subtitle,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 12.5,
                      color: Color(0xFF9E9E9E),
                    ),
                  ),
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
      errorBuilder: (_, _, _) => fallback ?? const SizedBox.shrink(),
    );
  }
}

/// Buyurtmadagi bitta taom: ko'rinishi umumiy blokda (widgets/order_blocks.dart),
/// bu yerda faqat restoran ma'lumoti (menyu keshi, server qator summasi) hisoblanadi.
class _OrderItemRow extends StatelessWidget {
  final Map<String, dynamic>? product;
  final String productId;
  final int qty;
  final String restaurantId;

  /// Serverning shu qator uchun yakuniy summasi (tiyin). null — javob yo'q;
  /// shunda mahsulotning O'Z chegirma narxiga tushamiz (aksiyalar bu yerda
  /// taxmin qilinmaydi: server ularni butun savatga qo'llaydi).
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
    final price = (p?['price_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (p?['discount_price_tiyin'] as num?)?.toInt() ?? 0;
    final unit = (discount > 0 && discount < price) ? discount : price;
    return OrderItemTile(
      name: (p?['name'] as String?) ?? 'Yuklanmoqda…',
      description: (p?['description'] as String?) ?? '',
      imageUrl: (p?['image_url'] as String?) ?? '',
      qty: qty,
      lineTotalTiyin: serverLineTotalTiyin ?? unit * qty,
      strikeTotalTiyin: price * qty,
      onAdd: () => cart.increment(restaurantId: restaurantId, productId: productId),
      onRemove: () => cart.decrement(restaurantId: restaurantId, productId: productId),
    );
  }
}