import 'dart:async';

// `ValueListenable` uchun — u `material.dart` orqali kelmaydi.
import 'package:flutter/foundation.dart';

import 'package:flutter/material.dart';
// `ScrollCacheExtent` uchun — u `material.dart` orqali kelmaydi.
import 'package:flutter/rendering.dart';

import '../api.dart';
import '../data/agent_driver.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import '../data/favorites_store.dart';
import '../data/quote_service.dart';
import '../services/book_cafe_game.dart';
import '../widgets/app_text_field.dart';
import '../widgets/common.dart';
import '../widgets/model_3d_view.dart';
import '../widgets/page_sheet.dart';
import '../widgets/product_grid.dart';
import '../widgets/sheet_page.dart';
import 'cart_screen.dart';
import 'catalog_screen.dart' show kBrand;

/// Restoran menyusi — NATIVE.
///
/// ┌─ VEB BILAN PARITY ────────────────────────────────────────────────┐
/// Bu ekran `apps/web/app/(food)/restaurants/[id]/menu-content.tsx`
/// ning aynan ko'rinishini beradi: yopishqoq sarlavha (orqaga + logo va
/// nom o'rtada + qidiruv), turkum chiplari, ikki ustunli kartochka
/// to'ri, o'ng pastda suzuvchi savat tugmasi.
///
/// Avval bu yerda 200px'lik muqova rasmi va bitta ustunli ro'yxat bor
/// edi — TMA va native ilova bir-biriga umuman o'xshamasdi. Endi
/// ikkalasi bir xil.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ RESTORAN MA'LUMOTI PARAMETR SIFATIDA ────────────────────────────┐
/// Katalog kartasi bosilganda restoran `Map`i shu yerga UZATILADI,
/// qaytadan so'ralmaydi. Ikki sabab:
///   * qo'shimcha tarmoq so'rovi yo'q — ekran bir zumda ochiladi;
///   * internet bo'lmasa ham sarlavha to'g'ri chiziladi (katalog
///     keshdan kelgan bo'lsa, menyu ham keshdan keladi).
/// └───────────────────────────────────────────────────────────────────┘
class MenuScreen extends StatefulWidget {
  final Map<String, dynamic> restaurant;

  const MenuScreen({super.key, required this.restaurant});

  /// Restoran menyusini FAQAT ID bo'yicha ochadi.
  ///
  /// ┌─ NEGA YORDAMCHI KERAK ──────────────────────────────────────────┐
  /// Menyuga uch joydan kelinadi: katalog kartasi, stol QR kodi va
  /// sevimlilar ro'yxati. Katalogda restoran `Map`i qo'lda bor,
  /// qolgan ikkitasida esa faqat ID.
  ///
  /// Har birida "keshdan topib, topilmasa bo'sh Map yasash" mantig'i
  /// qayta yozilsa — bu STACK ICHIDA dublikat bo'lardi. Shuning uchun
  /// u BIR joyda.
  /// └─────────────────────────────────────────────────────────────────┘
  ///
  /// Kesh ishlatiladi, tarmoq EMAS: menyu ekrani baribir o'z
  /// so'rovini yuboradi, sarlavha uchun qo'shimcha kutish shart emas.
  static Future<void> open(
    BuildContext context,
    String restaurantId, {
    String? fallbackName,
  }) async {
    Map<String, dynamic>? found;
    try {
      final cached = await Repos.restaurants().peek();
      found = cached?.cast<Map<String, dynamic>>().firstWhere(
            (r) => r['id'] == restaurantId,
            orElse: () => <String, dynamic>{},
          );
      if (found != null && found.isEmpty) found = null;
    } catch (_) {
      // Kesh o'qilmasa ham menyu ochilaveradi — sarlavhada faqat nom
      // bo'ladi, logo bo'lmaydi.
    }
    if (!context.mounted) return;

    await Navigator.of(context).push(
      sheetRoute(
        MenuScreen(
          restaurant: found ??
              {'id': restaurantId, 'name': fallbackName ?? '', 'open': true},
        ),
      ),
    );
  }

  @override
  State<MenuScreen> createState() => _MenuScreenState();
}

/// Chiplar qatorining balandligi — skroll-kuzatuv chizig'ini hisoblashda
/// ham ishlatiladi, shuning uchun bitta doimiy.
const double _kChipsHeight = 46;

/// "Kafeni 3D da aylanib ko'rish" chizig'i.
///
/// Menyu ustida turadi, lekin uni bosib qolmaydi: taomlar asosiy
/// mazmun, 3D esa qo'shimcha imkoniyat.
class _GameBanner extends StatelessWidget {
  const _GameBanner({
    required this.ready,
    required this.progress,
    required this.subtitle,
    required this.onTap,
  });

  /// Sahna qurilmada bormi.
  final bool ready;

  /// `null` — yuklanmayapti. `-1` — hajm noma'lum. 0..1 — jarayon.
  final double? progress;

  final String subtitle;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final busy = progress != null;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
      child: Material(
        color: kBrand.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          onTap: busy ? null : onTap,
          borderRadius: BorderRadius.circular(14),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            child: Column(
              children: [
                Row(
                  children: [
                    Icon(
                      // Yuklab olish kerak bo'lsa boshqa ikonka:
                      // foydalanuvchi bosishdan OLDIN nima bo'lishini
                      // bilib tursin.
                      ready
                          ? Icons.view_in_ar_rounded
                          : Icons.cloud_download_outlined,
                      color: kBrand,
                      size: 24,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text(
                            'Kafeni aylanib ko\'ring',
                            style: TextStyle(
                              fontSize: 15,
                              fontWeight: FontWeight.w600,
                              color: Color(0xFF171717),
                            ),
                          ),
                          Text(
                            busy ? 'Yuklanmoqda...' : subtitle,
                            style: const TextStyle(
                              fontSize: 12,
                              color: Color(0xFF6B6B6B),
                            ),
                          ),
                        ],
                      ),
                    ),
                    if (!busy) const Icon(Icons.chevron_right, color: kBrand),
                  ],
                ),
                if (busy) ...[
                  const SizedBox(height: 10),
                  ClipRRect(
                    borderRadius: BorderRadius.circular(4),
                    child: LinearProgressIndicator(
                      // Manfiy qiymat "hajm noma'lum" degani - shunda
                      // aniq foiz o'rniga cheksiz ko'rsatkich chiziladi.
                      value: progress! < 0 ? null : progress,
                      minHeight: 5,
                      color: kBrand,
                      backgroundColor: kBrand.withValues(alpha: 0.2),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _MenuScreenState extends State<MenuScreen> {
  final _scroll = ScrollController();
  final _chipsScroll = ScrollController();
  final _chipsListKey = GlobalKey();
  final _cart = CartStore.instance;

  late final _menuRepo = Repos.menu(_id);
  late final _promoRepo = Repos.promotions(_id);
  StreamSubscription<Cached<List<dynamic>>>? _menuSub;
  StreamSubscription<Cached<List<dynamic>>>? _promoSub;

  List<Map<String, dynamic>> _menu = const [];
  List<Map<String, dynamic>> _promos = const [];
  bool _menuLoading = true;
  bool _menuOffline = false;

  List<_Section> _sections = const [];
  final _sectionKeys = <String, GlobalKey>{};
  final _chipKeys = <String, GlobalKey>{};
  /// ┌─ NEGA `setState` EMAS ────────────────────────────────────────┐
  /// Skroll paytida turkum o'zgarganda `setState` chaqirilsa BUTUN
  /// menyu daraxti (barcha bo'limlar va to'rlar) qaytadan quriladi —
  /// aynan shu turkum chegarasida sezilgan "qotish" shundan edi.
  ///
  /// `ValueNotifier` bilan faqat chiplar qatori va sarlavha soyasi
  /// qayta chiziladi, ro'yxatga umuman tegilmaydi.
  /// └───────────────────────────────────────────────────────────────┘
  final _activeCategory = ValueNotifier<String>('');

  /// Serverdan kelgan yakuniy summa. `null` — hali yo'q, vizual
  /// taxminga tushiladi (veb `lib/use-quote.ts` bilan bir xil naqsh).
  int? _quoteTiyin;

  /// Eskirgan javoblardan himoya shu obyekt ichida.
  final _quoteFetcher = QuoteFetcher();

  /// Ro'yxat joyidan siljiganmi — sarlavha soyasi shunga bog'liq.
  final _headerScrolled = ValueNotifier<bool>(false);

  String get _id => (widget.restaurant['id'] as String?) ?? '';
  String get _name => (widget.restaurant['name'] as String?) ?? '';
  String get _logo => (widget.restaurant['logo_url'] as String?) ?? '';

  /// Yopishqoq sarlavha ostidagi chiziq — bo'lim shu chiziqdan
  /// yuqoriga chiqsa "joriy" hisoblanadi.
  double get _headerLine =>
      MediaQuery.of(context).padding.top +
      kToolbarHeight +
      (_sections.length > 1 ? _kChipsHeight : 0);

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    _scroll.addListener(_onScroll);
    // Shaddiy buyurtmani KO'RSATIB berishi uchun ekran o'zini
    // ro'yxatdan o'tkazadi: skroll, taomlarning joylari va "+"
    // amali. Boshqaruvchi ekranning ichiga kirmaydi — u faqat shu
    // yerda e'lon qilingan narsalardan foydalanadi
    // (`lib/data/agent_driver.dart`).
    AgentStage.instance.registerMenu(
      restaurantId: _id,
      scroll: _scroll,
      add: _addById,
    );
    _subscribe();
    FavoritesStore.instance.load(force: true);
    _refreshQuote();
    _checkGame();
  }

  /// ┌─ 3D MAKET ───────────────────────────────────────────────────┐
  /// Maket RESTORANGA bog'langan: manzili va SHA-256 restoran
  /// yozuvida (`scene_3d_url`). Maketi yo'q restoranda banner
  /// umuman chizilmaydi — begona maket bog'lanib qolmasin.
  ///
  /// Maketning o'zi ilova bilan kelmaydi (~104 MB), shuning uchun
  /// banner ikki holatda bo'ladi:
  ///   * fayl yo'q  → "yuklab olish" taklifi va hajmi
  ///   * fayl bor   → to'g'ridan-to'g'ri ochiladi
  ///
  /// Tekshiruv fonda ketadi va menyuni kutib turmaydi.
  /// └──────────────────────────────────────────────────────────────┘
  late final SceneInfo? _sceneInfo = BookCafeGame.infoOf(widget.restaurant);
  SceneStatus? _scene;

  /// `null` — yuklab olinmayapti. 0..1 — jarayon. -1 — hajm noma'lum.
  double? _downloading;

  Future<void> _checkGame() async {
    final info = _sceneInfo;
    if (info == null) return;
    final st = await BookCafeGame.status(
      restaurantId: _id,
      sha256: info.sha256,
    );
    if (mounted) setState(() => _scene = st);
  }

  Future<void> _onGameTap() async {
    if (_downloading != null) return;

    if (_scene?.ready == true) {
      await _openGame();
      return;
    }
    await _downloadScene();
  }

  Future<void> _downloadScene() async {
    final info = _sceneInfo;
    if (info == null) return;

    setState(() => _downloading = -1);
    final err = await BookCafeGame.download(
      restaurantId: _id,
      info: info,
      onProgress: (p) {
        if (mounted) setState(() => _downloading = p ?? -1);
      },
    );
    if (!mounted) return;
    setState(() => _downloading = null);

    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Yuklab olinmadi: $err')),
      );
      return;
    }
    await _checkGame();
    if (mounted) await _openGame();
  }

  Future<void> _openGame() async {
    final info = _sceneInfo;
    if (info == null) return;
    // â”Œâ”€ HOLAT IKKI TOMONGA â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
    // 3D tomonga hozirgi savat va sevimlilar uzatiladi, u yopilgach
    // o'zgargani qaytariladi. O'yin serverga o'zi yozmaydi: unda
    // foydalanuvchi tokeni yo'q va bo'lishi ham kerak emas.
    // â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
    // Sevimlilar `FavoritesStore` orqali: u API chaqiruvini ham,
    // ekranlarni xabardor qilishni ham o'zi bajaradi. To'g'ridan-
    // to'g'ri `api` ga murojaat qilinsa, ro'yxat ikki joyda ajralib
    // qolardi.
    await FavoritesStore.instance.load();
    final favBefore = Set<String>.from(FavoritesStore.instance.ids);
    final cartBefore = _cart.restaurantId == _id
        ? Map<String, int>.from(_cart.items)
        : <String, int>{};

    final res = await BookCafeGame.open(
      restaurantId: _id,
      sha256: info.sha256,
      // ┌─ HAQIQIY STOL RAQAMI ──────────────────────────────────────┐
      // Avval bu yerda bo'sh satr turardi va o'yin o'z standartiga
      // ("5-stol") qaytardi - shuning uchun HAMMA stol 5-stol bo'lib
      // ko'rinardi.
      //
      // Endi stol QR kodni skanerlaganda boshlangan seansdan olinadi.
      // Seans bo'lmasa bo'sh qoladi va o'yin stol raqamini umuman
      // ko'rsatmaydi - yolg'on raqamdan ko'ra yo'qligi yaxshiroq.
      // └────────────────────────────────────────────────────────────┘
      tableLabel: _cart.restaurantId == _id ? (_cart.tableLabel ?? '') : '',
      apiBase: apiBaseUrl,
      // Yuklanish ekranida kafening O'Z logotipi va nomi chiqadi.
      // Ma'lumot shu yerda allaqachon bor - 3D tomon uni API'dan
      // qayta so'ramaydi.
      name: _name,
      logoUrl: _logo.isEmpty ? '' : fullImageUrl(_logo),
      favorites: favBefore,
      cart: cartBefore,
    );

    if (!mounted) return;
    if (res.error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('3D maket ochilmadi: ${res.error}')),
      );
      // Fayl buzilgan bo'lishi mumkin - holatni yangilaymiz.
      _checkGame();
      return;
    }
    await _applyGameState(res, favBefore, cartBefore);

    // ┌─ BUYURTMANI OnDex RASMIYLASHTIRADI ────────────────────────┐
    // O'yin buyurtma bermaydi va bera olmaydi: unda foydalanuvchi
    // tokeni yo'q. U faqat niyatni qaytaradi, biz esa odatiy savat
    // oqimini ochamiz - u yerda manzil, to'lov va stol seansi
    // allaqachon to'g'ri ishlaydi.
    // └────────────────────────────────────────────────────────────┘
    if (res.orderRequested && mounted && !_cart.isEmpty) {
      // Savat ekrani boshqa joylarda ham shu tarzda ochiladi.
      await Navigator.of(context).push(sheetRoute(const CartScreen()));
    }
  }

  /// 3D maketdan qaytgan holatni OnDex tomonga qo'llaydi.
  ///
  /// â”Œâ”€ FAQAT FARQ YOZILADI â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
  /// Sevimlilar ro'yxatini butunlay qayta yozish mumkin edi, lekin u
  /// BOSHQA restoranlarning mahsulotlarini ham o'z ichiga oladi -
  /// o'yin esa faqat shu kafening menyusini ko'radi. Hammasini
  /// yozish qolganlarini o'chirib yuborardi.
  ///
  /// Shuning uchun faqat FARQ qo'llanadi va u ham menyudagi
  /// mahsulotlar bilan cheklanadi.
  /// â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
  Future<void> _applyGameState(
    GameResult res,
    Set<String> favBefore,
    Map<String, int> cartBefore,
  ) async {
    if (!res.hasState) return;
    // Holat boshqa restorannikimi - tegmaymiz.
    if (res.restaurantId.isNotEmpty && res.restaurantId != _id) return;

    // Menyuda bor mahsulotlargina qabul qilinadi: o'yin qaytargan
    // begona ID savatga tushmasligi kerak.
    final known = _menu.map((p) => '${p['id'] ?? ''}').toSet();

    // â”€â”€ Savat â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
    for (final id in {...cartBefore.keys, ...res.cart.keys}) {
      if (!known.contains(id)) continue;
      final want = res.cart[id] ?? 0;
      if (_cart.qtyOf(id) == want) continue;
      _cart.setQty(restaurantId: _id, productId: id, qty: want);
    }

    // â”€â”€ Sevimlilar â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
    for (final id in known) {
      final was = favBefore.contains(id);
      final now = res.favorites.contains(id);
      if (was == now) continue;
      // `toggle` holatni teskarisiga o'giradi - biz esa aynan
      // O'ZGARGANLARINI chaqiramiz, ya'ni natija to'g'ri chiqadi.
      try {
        await FavoritesStore.instance.toggle(id);
      } on ApiException {
        // Bitta sevimli saqlanmasa qolganlari baribir yoziladi.
      }
    }
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    // Ekran yopilgach registr TOZALANADI: aks holda boshqaruvchi
    // mavjud bo'lmagan ekrandagi tugmani "bosishga" urinardi.
    AgentStage.instance.unregisterMenu(_id);
    _cart.removeListener(_onCart);
    _menuSub?.cancel();
    _promoSub?.cancel();
    _scroll.dispose();
    _chipsScroll.dispose();
    _activeCategory.dispose();
    _headerScrolled.dispose();
    super.dispose();
  }

  // ── Ma'lumot ──────────────────────────────────────────────────────

  void _subscribe({bool force = false}) {
    _menuSub?.cancel();
    _promoSub?.cancel();

    _menuSub = _menuRepo.observe(force: force).listen((c) {
      if (!mounted) return;
      setState(() {
        if (c.value != null) {
          _menu = c.value!.cast<Map<String, dynamic>>();
          _rebuildSections();
        }
        _menuLoading = c.showSpinner;
        _menuOffline = c.error != null;
      });
    });

    // Aksiyalar YIQILSA ham menyu ko'rinaveradi — ular faqat lenta va
    // chegirma narxiga ta'sir qiladi, taomlar ro'yxatiga emas.
    _promoSub = _promoRepo.observe(force: force).listen((c) {
      if (!mounted || c.value == null) return;
      setState(() => _promos = c.value!.cast<Map<String, dynamic>>());
    });
  }

  void _rebuildSections() {
    final order = <String>[];
    final map = <String, List<Map<String, dynamic>>>{};
    for (final p in _menu) {
      final key = categoryOf(p);
      if (!map.containsKey(key)) {
        map[key] = [];
        order.add(key);
      }
      map[key]!.add(p);
    }
    _sections = [for (final k in order) _Section(k, map[k]!)];
    for (final s in _sections) {
      _sectionKeys.putIfAbsent(s.title, () => GlobalKey());
      _chipKeys.putIfAbsent(s.title, () => GlobalKey());
    }
    if (_activeCategory.value.isEmpty && _sections.isNotEmpty) {
      _activeCategory.value = _sections.first.title;
    }
  }

  void _onCart() {
    if (!mounted) return;
    setState(() {});
    _refreshQuote();
  }

  /// Yakuniy summani serverdan so'raydi.
  ///
  /// Eskirgan javoblar `_quoteSeq` bilan filtrlanadi: mijoz tez "+"
  /// bosganda javoblar tartibsiz kelishi mumkin va eskisi yangisini
  /// bosib ketardi.
  /// So'rov `data/quote_service.dart` da — savat va rasmiylashtirish
  /// ekranlari bilan bitta kod.
  Future<void> _refreshQuote() async {
    try {
      final q = await _quoteFetcher.fetch(_id);
      if (!mounted || q == null) return; // eskirgan javob
      setState(() => _quoteTiyin = q.totalTiyin);
    } catch (_) {
      // Anonim foydalanuvchi yoki tarmoq xatosi — vizual taxmin.
      if (mounted) setState(() => _quoteTiyin = null);
    }
  }

  /// Savatning CHEGIRMASIZ summasi — aksiyaning "minimal buyurtma
  /// summasi" shartini tekshirish uchun (`computeProductDiscount`).
  /// Savat boshqa restorandan bo'lsa 0.
  int get _rawSubtotal => rawSubtotal(_menu, restaurantId: _id);

  /// Server summasi bo'lmaganda ko'rsatiladigan taxmin.
  int get _visualTotal {
    var total = 0;
    final byId = productsById(_menu);
    final subtotal = _rawSubtotal;
    for (final e in _cart.items.entries) {
      final p = byId[e.key];
      if (p == null) continue;
      final d = computeProductDiscount(p, _promos, cartSubtotalTiyin: subtotal);
      final unit = d?.discountedPriceTiyin ??
          (p['price_tiyin'] as num?)?.toInt() ??
          0;
      total += unit * e.value;
    }
    return total;
  }

  // ── Skroll-kuzatuv ────────────────────────────────────────────────

  /// Qaysi turkum bo'limida turganini aniqlab, mos chipni belgilaydi.
  ///
  /// `IntersectionObserver` ga o'xshash "kesishuv" mantig'i ATAYLAB
  /// ishlatilmadi (veb versiyada ham): bir vaqtda bir necha bo'lim
  /// ko'rinib turishi mumkin va qaysi biri "joriy" ekani noaniq
  /// qolardi. Bu yerda aniq qoida: sarlavha chizig'idan YUQORIDA
  /// boshlangan ENG OXIRGI bo'lim — joriy.
  ///
  /// Ro'yxatdagi INDEKS bo'yicha ishlaydi, `currentContext` bo'yicha
  /// emas: ekrandan uzoqda qolgan bo'limlarni Flutter yo'q qiladi va
  /// ularning konteksti `null` bo'ladi — indeks esa har doim bor.
  void _onScroll() {
    if (!_scroll.hasClients) return;

    // Sarlavha soyasi — ro'yxat joyidan siljigani belgisi.
    // `ValueNotifier` faqat qiymat O'ZGARGANDA xabar beradi.
    _headerScrolled.value = _scroll.offset > 2;

    if (_sections.isEmpty) return;

    final line = _headerLine + 8;
    var activeIndex = 0;
    for (var i = 0; i < _sections.length; i++) {
      final ctx = _sectionKeys[_sections[i].title]?.currentContext;
      if (ctx == null) continue;
      final box = ctx.findRenderObject() as RenderBox?;
      if (box == null || !box.hasSize) continue;
      if (box.localToGlobal(Offset.zero).dy > line) {
        activeIndex = i - 1;
        break;
      }
      activeIndex = i;
    }
    if (activeIndex < 0) activeIndex = 0;

    // Eng pastga yetganda MAJBURAN oxirgi turkum: oxirgi bo'lim
    // ekrandan qisqa bo'lsa uning tepasi chiziqdan hech qachon
    // yuqoriga chiqmaydi va chip oldingi turkumda qotib qolardi.
    if (_scroll.offset >= _scroll.position.maxScrollExtent - 4) {
      activeIndex = _sections.length - 1;
    }

    final next = _sections[activeIndex].title;
    if (next != _activeCategory.value) {
      _activeCategory.value = next;
      _centerChip(next);
    }
  }

  /// Faol chipni gorizontal ro'yxat markaziga suradi.
  ///
  /// `Scrollable.ensureVisible` ATAYLAB ishlatilmaydi — u BARCHA
  /// ota-skrollerlarni, jumladan VERTIKAL ro'yxatni ham suradi va
  /// foydalanuvchining barmoq bilan skroll qilishiga xalaqit berardi.
  /// Bu yerda faqat chiplar konteynerining `offset`i o'zgaradi.
  void _centerChip(String category) {
    final ctx = _chipKeys[category]?.currentContext;
    final listCtx = _chipsListKey.currentContext;
    if (ctx == null || listCtx == null || !_chipsScroll.hasClients) return;
    final box = ctx.findRenderObject() as RenderBox?;
    final listBox = listCtx.findRenderObject() as RenderBox?;
    if (box == null || listBox == null || !box.hasSize) return;

    final dx = box.localToGlobal(Offset.zero, ancestor: listBox).dx;
    final target = _chipsScroll.offset +
        dx -
        (listBox.size.width - box.size.width) / 2;
    _chipsScroll.animateTo(
      target.clamp(0.0, _chipsScroll.position.maxScrollExtent),
      duration: const Duration(milliseconds: 250),
      curve: Curves.easeOut,
    );
  }

  /// Chip bosilganda — o'sha bo'limga suradi.
  ///
  /// Bo'lim hali qurilmagan bo'lishi mumkin (ekrandan uzoq): bunday
  /// holatda bir ekran surib, qayta urinamiz. Qadam yo'nalishi
  /// indekslar farqidan aniqlanadi.
  Future<void> _scrollToCategory(String category) async {
    final targetIndex = _sections.indexWhere((s) => s.title == category);
    if (targetIndex < 0) return;
    final currentIndex =
        _sections.indexWhere((s) => s.title == _activeCategory.value);
    final down = targetIndex >= (currentIndex < 0 ? 0 : currentIndex);

    for (var attempt = 0; attempt < 20; attempt++) {
      if (!mounted || !_scroll.hasClients) return;
      final ctx = _sectionKeys[category]?.currentContext;
      final box = ctx?.findRenderObject() as RenderBox?;
      if (box != null && box.hasSize) {
        final dy = box.localToGlobal(Offset.zero).dy;
        final target = (_scroll.offset + dy - _headerLine)
            .clamp(0.0, _scroll.position.maxScrollExtent);
        await _scroll.animateTo(
          target,
          duration: const Duration(milliseconds: 280),
          curve: Curves.easeOut,
        );
        return;
      }
      final step = _scroll.position.viewportDimension * 0.9;
      final next = (down ? _scroll.offset + step : _scroll.offset - step)
          .clamp(0.0, _scroll.position.maxScrollExtent);
      if (next == _scroll.offset) return;
      _scroll.jumpTo(next);
      // Keyingi kadrni kutamiz — shundagina yangi bo'limlar quriladi.
      await WidgetsBinding.instance.endOfFrame;
    }
  }

  // ── Savat ─────────────────────────────────────────────────────────

  void _add(Map<String, dynamic> p) => _addById((p['id'] as String?) ?? '');

  /// Savatga qo'shish — "+" tugmasi ham, Shaddiy ham AYNAN shuni
  /// chaqiradi. Ikki xil yo'l bo'lsa, ulardan biri (masalan restoran
  /// almashtirish tekshiruvi) e'tibordan chetda qolardi.
  void _addById(String productId) {
    if (productId.isEmpty) return;
    _cart.increment(
      restaurantId: _id,
      productId: productId,
      restaurantName: _name,
    );
  }

  void _remove(Map<String, dynamic> p) => _cart.decrement(
        restaurantId: _id,
        productId: (p['id'] as String?) ?? '',
      );

  void _setQty(Map<String, dynamic> p, int qty) => _cart.setQty(
        restaurantId: _id,
        productId: (p['id'] as String?) ?? '',
        qty: qty,
        restaurantName: _name,
      );

  // ── Chizish ───────────────────────────────────────────────────────

  Widget _card(Map<String, dynamic> p) {
    final id = (p['id'] as String?) ?? '';
    final discount =
        computeProductDiscount(p, _promos, cartSubtotalTiyin: _rawSubtotal);
    // Kalit — Shaddiy taomni topib, unga skroll qilib, "barmoq"ni
    // aynan shu kartochka ustiga qo'yishi uchun.
    return KeyedSubtree(
      key: AgentStage.instance.productKey(id),
      child: _cardBody(p, id, discount),
    );
  }

  Widget _cardBody(
      Map<String, dynamic> p, String id, ProductDiscount? discount) {
    return ProductCard(
      product: p,
      qty: _cart.restaurantId == _id ? _cart.qtyOf(id) : 0,
      discount: discount,
      promoted: PromotionIndex(_promos).covers(p, discount),
      // Yurakcha holati `FavoritesStore` dan keladi — bu yerda
      // saqlanmaydi. Kartochkaning o'zi umumiy paketda
      // (`packages/ondex_menu`), yurak esa mijozga xos — uyaga qo'yiladi.
      topLeft: FavoriteButton(
        productId: id,
        initialFavorited: FavoritesStore.instance.contains(id),
      ),
      onAdd: () => _add(p),
      // Shaddiy "barmog'i" AYNAN shu tugmani topishi uchun.
      addKey: AgentStage.instance.addKey(id),
      onRemove: () => _remove(p),
      onTap: () => _openDetail(p),
    );
  }

  Future<void> _openDetail(Map<String, dynamic> p) async {
    final id = (p['id'] as String?) ?? '';
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _ProductSheet(
        product: p,
        discount:
            computeProductDiscount(p, _promos, cartSubtotalTiyin: _rawSubtotal),
        favorited: FavoritesStore.instance.contains(id),
        initialQty: _cart.restaurantId == _id ? _cart.qtyOf(id) : 0,
        onConfirm: (qty) => _setQty(p, qty),
      ),
    );
  }

  Future<void> _openSearch() async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => _MenuSearchScreen(
          restaurantId: _id,
          menu: _menu,
          buildCard: _searchCard,
        ),
      ),
    );
    // Qidiruvda savat yoki yurak o'zgargan bo'lishi mumkin.
    if (mounted) setState(() {});
  }

  /// Qidiruv natijalari uchun kartochka — `_card` bilan bir xil ko'rinish,
  /// lekin AgentStage'ning GLOBAL kalitlarisiz.
  ///
  /// ┌─ NEGA ALOHIDA ("RenderRepaintBoundary was mutated" xatosi) ───────┐
  /// `_card` har bir taomga BITTA, butun menyu ekrani davomida
  /// saqlanadigan `GlobalKey` beradi (`AgentStage.productKey`/`addKey` —
  /// Shaddiy yordamchisining "barmog'i" aynan shu taom/tugma ustida
  /// turishi uchun).
  ///
  /// Qidiruv ekrani asosiy menyu USTIGA push qilinadi — ya'ni asosiy
  /// ro'yxat ekranda ko'rinmasa ham TIRIK qoladi. Qidiruv natijasida
  /// asosiy ro'yxatda ALLAQACHON chizilgan taom chiqsa, bitta GlobalKey
  /// ikkita joyda BIR VAQTDA ishlatilib qolardi. Flutter buni
  /// RenderObject'ni noto'g'ri paytda (asosiy ro'yxat layout jarayonida
  /// emasligida) "ko'chirish" deb hisoblab, aynan shu xatoni berardi —
  /// va orqaga qaytishda ham (asosiy ro'yxat qayta tiklanganda) xuddi
  /// shu sabab bilan takrorlanardi.
  ///
  /// Qidiruv natijalarida Shaddiy "barmog'i" umuman ishlatilmaydi (u
  /// faqat asosiy menyuga ro'yxatdan o'tgan — `AgentStage.registerMenu`),
  /// shuning uchun bu yerda GlobalKey'ga ehtiyoj yo'q — oddiy, faqat
  /// shu ro'yxatga tegishli `ValueKey` yetarli.
  /// └──────────────────────────────────────────────────────────────────┘
  Widget _searchCard(Map<String, dynamic> p) {
    final id = (p['id'] as String?) ?? '';
    final discount =
        computeProductDiscount(p, _promos, cartSubtotalTiyin: _rawSubtotal);
    return ProductCard(
      key: ValueKey('search_$id'),
      product: p,
      qty: _cart.restaurantId == _id ? _cart.qtyOf(id) : 0,
      discount: discount,
      promoted: PromotionIndex(_promos).covers(p, discount),
      topLeft: FavoriteButton(
        productId: id,
        initialFavorited: FavoritesStore.instance.contains(id),
      ),
      onAdd: () => _add(p),
      onRemove: () => _remove(p),
      onTap: () => _openDetail(p),
    );
  }

  @override
  Widget build(BuildContext context) {
    final showChips = _sections.length > 1;
    final myCart = _cart.restaurantId == _id && !_cart.isEmpty;

    return SheetPage(
        child: Scaffold(
      backgroundColor: Colors.white,
      body: Column(
        children: [
          // ┌─ SARLAVHA SKROLLDAN TASHQARIDA ───────────────────────┐
          // Ilgari u `SliverAppBar` edi, ya'ni ro'yxatning BIR
          // QISMI. Shu sababli uning ustidagi tortish imo-ishorasi
          // skrollga tegishli bo'lib qolar va sahifani yopish
          // ishlamasdi (jonli sinovda tasdiqlandi).
          //
          // Endi sarlavha ro'yxatning YONIDA turadi: tortish faqat
          // shu yerga tegishli, ro'yxat esa o'z skrolli va "tortib
          // yangilash" ini saqlab qoladi.
          // └───────────────────────────────────────────────────────┘
          _MenuHeader(
              name: _name,
              logo: _logo,
              scrolled: _headerScrolled,
              onSearch: _menu.isEmpty ? null : _openSearch,
              chips: showChips
                  ? SizedBox(
                      height: _kChipsHeight,
                      // Faqat chiplar qatori qayta chiziladi — ro'yxat
                      // tegilmaydi (skroll silliq qoladi).
                      child: ValueListenableBuilder<String>(
                        valueListenable: _activeCategory,
                        builder: (_, active, __) => ListView.separated(
                          key: _chipsListKey,
                          controller: _chipsScroll,
                          scrollDirection: Axis.horizontal,
                          padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
                          itemCount: _sections.length,
                          separatorBuilder: (_, __) => const SizedBox(width: 8),
                          itemBuilder: (_, i) {
                            final title = _sections[i].title;
                            // Chip umumiy paketdan — affitsiant ilovasining
                            // buyurtma ekrani ham AYNI chipni ishlatadi.
                            return MenuCategoryChip(
                              key: _chipKeys[title],
                              label: title,
                              active: title == active,
                              onTap: () => _scrollToCategory(title),
                            );
                          },
                        ),
                      ),
                    )
                  : null,
          ),
          Expanded(
            child: RefreshIndicator(
        color: kBrand,
        onRefresh: () async {
          _subscribe(force: true);
          await FavoritesStore.instance.load(force: true);
        },
        child: CustomScrollView(
          controller: _scroll,
          physics: const AlwaysScrollableScrollPhysics(),
          // Kesh maydoni kengaytirildi: yonidagi bo'limlar tirik
          // qolsa, chip bosilganda pog'ona-pog'ona surish deyarli
          // kerak bo'lmaydi.
          scrollCacheExtent: const ScrollCacheExtent.viewport(1.5),
          slivers: [
            if (_menuOffline) const SliverToBoxAdapter(
                          child: OfflineNotice(
                            message: 'Menyu yangilanmadi — saqlangan nusxa',
                            margin: EdgeInsets.fromLTRB(16, 12, 16, 0),
                          ),
                        ),

            // Banner FAQAT maketi bor restoranda. `_sceneInfo` restoran
            // yozuvidan olinadi, ya'ni ulanmagan kafeda u `null` va
            // taklif umuman chizilmaydi.
            if (_sceneInfo != null && _scene?.available == true)
              SliverToBoxAdapter(
                child: _GameBanner(
                  ready: _scene!.ready,
                  progress: _downloading,
                  subtitle: _scene!.ready
                      ? 'Ichkariga kirib, stolga o\'tirib buyurtma bering'
                      : 'Ichkarini ko\'rish uchun ${_sceneInfo.sizeText} '
                          'yuklab olinadi',
                  onTap: _onGameTap,
                ),
              ),

            if (_menuLoading)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: Center(child: CircularProgressIndicator()),
              )
            else if (_menu.isEmpty)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: Center(
                  child: Padding(
                    padding: EdgeInsets.all(32),
                    child: Text('Menyu hozircha bo\'sh'),
                  ),
                ),
              )
            else
              for (final s in _sections) ...[
                SliverToBoxAdapter(
                  child: Padding(
                    key: _sectionKeys[s.title],
                    padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
                    child: Text(
                      s.title,
                      style: const TextStyle(
                          fontSize: 18, fontWeight: FontWeight.bold),
                    ),
                  ),
                ),
                SliverPadding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  sliver: SliverGrid.builder(
                    gridDelegate: productGridOf(context),
                    itemCount: s.items.length,
                    itemBuilder: (_, i) => _card(s.items[i]),
                  ),
                ),
              ],

            // ┌─ BO'SH JOY FAQAT KERAK BO'LGANDA ───────────────────┐
            // Bu joy suzuvchi "Savat" tugmasi ostidagi taomlarni
            // bosib qolmasligi uchun. Lekin u SHARTSIZ qo'yilgan edi:
            // savat bo'sh bo'lsa tugma ham yo'q, joy esa qolaverar va
            // ro'yxat oxirida sababsiz bo'shliq ko'rinardi.
            // └─────────────────────────────────────────────────────┘
            SliverToBoxAdapter(child: SizedBox(height: myCart ? 96 : 16)),
          ],
        ),
      ),
          ),
        ],
      ),
      floatingActionButton: myCart
          ? _CartPill(
              totalTiyin: _quoteTiyin ?? _visualTotal,
              count: _cart.totalQty,
              onTap: () => Navigator.of(context).push(
                // Savat ham pastdan suzib chiqadi.
                sheetRoute(const CartScreen()),
              ),
            )
          : null,
    ));
  }
}

/// Menyuning qotirilgan sarlavhasi: orqaga, logo + nom, qidiruv va
/// (ixtiyoriy) turkum chiplari.
///
/// `AppBar` ATAYLAB ishlatilmadi — u faqat `Scaffold.appBar` yoki
/// `SliverAppBar` sifatida to'g'ri turadi, bizga esa u ustunning oddiy
/// bo'lagi bo'lishi kerak (izohga qarang).
class _MenuHeader extends StatelessWidget {
  final String name;
  final String logo;
  final ValueListenable<bool> scrolled;
  final VoidCallback? onSearch;
  final Widget? chips;

  const _MenuHeader({
    required this.name,
    required this.logo,
    required this.scrolled,
    required this.onSearch,
    required this.chips,
  });

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<bool>(
      valueListenable: scrolled,
      // Sarlavha tarkibi soya o'zgarganda QAYTA QURILMAYDI — u
      // `child` sifatida bir marta quriladi va shundayligicha
      // uzatiladi.
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            height: kToolbarHeight,
            child: Row(
              children: [
                IconButton(
                  onPressed: () => Navigator.of(context).maybePop(),
                  icon: const Icon(Icons.arrow_back),
                  tooltip: 'Orqaga',
                ),
                Expanded(
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      if (logo.isNotEmpty) ...[
                        ClipRRect(
                          borderRadius: BorderRadius.circular(7),
                          child: SizedBox(
                            width: 28,
                            height: 28,
                            child: RemoteImage(url: logo),
                          ),
                        ),
                        const SizedBox(width: 8),
                      ],
                      Flexible(
                        child: Text(
                          name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              fontSize: 18, fontWeight: FontWeight.bold),
                        ),
                      ),
                    ],
                  ),
                ),
                IconButton(
                  onPressed: onSearch,
                  icon: const Icon(Icons.search, size: 24),
                  tooltip: 'Qidirish',
                ),
              ],
            ),
          ),
          if (chips != null) chips!,
        ],
      ),
      builder: (_, isScrolled, child) => AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOut,
        decoration: BoxDecoration(
          color: Colors.white,
          // Ro'yxat sarlavha ostidan suriladi — skroll boshlangach
          // soya ularni ajratib turadi (bosh sahifadagi bilan bir xil).
          boxShadow: isScrolled
              ? [
                  BoxShadow(
                    color: Colors.black.withValues(alpha: 0.10),
                    blurRadius: 14,
                    offset: const Offset(0, 4),
                  ),
                ]
              : const [],
        ),
        child: child,
      ),
    );
  }
}

class _Section {
  final String title;
  final List<Map<String, dynamic>> items;
  const _Section(this.title, this.items);
}

// ═══════════════════════════════════════════════════════════════════
// SUZUVCHI SAVAT TUGMASI
// ═══════════════════════════════════════════════════════════════════

class _CartPill extends StatelessWidget {
  final int totalTiyin;
  final int count;
  final VoidCallback onTap;

  const _CartPill({
    required this.totalTiyin,
    required this.count,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return FloatingActionButton.extended(
      onPressed: onTap,
      backgroundColor: kBrand,
      foregroundColor: Colors.white,
      elevation: 4,
      extendedPadding: const EdgeInsets.symmetric(horizontal: 22),
      label: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            formatSum(totalTiyin),
            style: const TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
          ),
          const SizedBox(width: 12),
          Stack(
            clipBehavior: Clip.none,
            children: [
              const Icon(Icons.shopping_basket_outlined, size: 22),
              Positioned(
                right: -7,
                top: -7,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  constraints: const BoxConstraints(minWidth: 16),
                  decoration: BoxDecoration(
                    color: Colors.black,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Text(
                    '$count',
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.bold,
                        color: Colors.white),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// QIDIRUV
// ═══════════════════════════════════════════════════════════════════

/// Menyu ichidan qidiruv — asosiy sahifa HAR DOIM to'liq, filtrlanmagan
/// menyuni ko'rsatadi (veb bilan bir xil qaror).
///
/// Kartochkani O'ZI chizmaydi: menyu ekranidan `buildCard` funksiyasi
/// uzatiladi — savat, sevimli va aksiya mantig'i bitta joyda qoladi.
class _MenuSearchScreen extends StatefulWidget {
  final String restaurantId;
  final List<Map<String, dynamic>> menu;
  final Widget Function(Map<String, dynamic>) buildCard;

  const _MenuSearchScreen({
    required this.restaurantId,
    required this.menu,
    required this.buildCard,
  });

  @override
  State<_MenuSearchScreen> createState() => _MenuSearchScreenState();
}

class _MenuSearchScreenState extends State<_MenuSearchScreen> {
  final _controller = TextEditingController();
  final _cart = CartStore.instance;
  String _query = '';

  Timer? _debounce;

  /// Backend (MeiliSearch) natijasi — `null` bo'lsa hali kelmagan
  /// (yoki so'rov muvaffaqiyatsiz bo'lgan) va quyidagi lokal filtr
  /// ko'rsatiladi.
  ///
  /// ┌─ NEGA IKKALA YO'L HAM BOR ──────────────────────────────────────┐
  /// `GET /products/search` — global, BARCHA restoranlar bo'yicha
  /// qidiradi va endi MeiliSearch orqali xato-kechiruvchan (masalan
  /// "mohito" yozilsa "Moxito" ham topiladi) — buni bu restoranga
  /// tegishlilarigacha TORAYTIRAMIZ (`restaurant_id` bo'yicha).
  ///
  /// Lekin bu — tarmoq so'rovi, `widget.menu` esa allaqachon qo'lda
  /// (keshdan) mavjud. Shuning uchun natija HAR SAFAR mahalliy filtr
  /// bilan boshlanadi (bir zumda ko'rinadi), so'ngra tarmoq javobi
  /// kelganda MeiliSearch natijasiga ALMASHADI. Tarmoq nosoz bo'lsa
  /// (yoki sekin bo'lsa) — foydalanuvchi baribir natijasiz qolmaydi,
  /// faqat xato-kechiruvchan qidiruvdan mahrum bo'ladi.
  /// └────────────────────────────────────────────────────────────────┘
  List<Map<String, dynamic>>? _remoteResults;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _cart.removeListener(_onCart);
    _controller.dispose();
    super.dispose();
  }

  void _onCart() {
    if (mounted) setState(() {});
  }

  void _onQueryChanged(String v) {
    setState(() {
      _query = v;
      _remoteResults = null;
    });
    _debounce?.cancel();
    final q = v.trim();
    if (q.isEmpty) return;
    // 300ms — har bosishda emas, yozish to'xtaganda so'raladi.
    _debounce = Timer(const Duration(milliseconds: 300), () => _searchRemote(q));
  }

  Future<void> _searchRemote(String q) async {
    try {
      final list = (await api.searchProducts(q)).cast<Map<String, dynamic>>();
      // So'rov davomida foydalanuvchi matnni o'zgartirgan bo'lishi
      // mumkin — eskirgan javob yangi so'rovni bosib qolmasin.
      if (!mounted || q != _query.trim()) return;
      setState(() {
        _remoteResults =
            list.where((p) => p['restaurant_id'] == widget.restaurantId).toList();
      });
    } catch (_) {
      // Tarmoq xatosi — jim qolamiz, lokal filtr ko'rinishda qoladi
      // (pastdagi `results` hisoblanishiga qarang).
    }
  }

  @override
  Widget build(BuildContext context) {
    final q = _query.trim().toLowerCase();
    final localResults = q.isEmpty
        ? const <Map<String, dynamic>>[]
        : widget.menu
            .where((p) =>
                ((p['name'] as String?) ?? '').toLowerCase().contains(q))
            .toList();
    final results = _remoteResults ?? localResults;

    return PageSheet(
        child: Scaffold(
      backgroundColor: Colors.white,
      appBar: PageAppBar(
        titleSpacing: 0,
        titleWidget: AppTextField(
          controller: _controller,
          hint: 'Taom qidirish...',
          icon: Icons.search,
          // Qidiruv oynasi ATAYLAB ochilgan — klaviatura darhol
          // tayyor bo'lishi kerak.
          autofocus: true,
          textInputAction: TextInputAction.search,
          onChanged: _onQueryChanged,
        ),
        actions: [
          IconButton(
            onPressed: () => Navigator.of(context).pop(),
            icon: const Icon(Icons.close),
            tooltip: 'Yopish',
          ),
        ],
      ),
      body: results.isEmpty
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Text(
                  q.isEmpty ? 'Taom nomini yozing' : 'Mos taom topilmadi',
                  style: const TextStyle(color: Color(0xFF757575)),
                ),
              ),
            )
          : GridView.builder(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
              gridDelegate: productGridOf(context),
              itemCount: results.length,
              itemBuilder: (_, i) => widget.buildCard(results[i]),
            ),
    ));
  }
}

// ═══════════════════════════════════════════════════════════════════
// TAOM TAFSILOTI
// ═══════════════════════════════════════════════════════════════════

/// Pastdan chiquvchi panel — veb `product-detail-sheet.tsx` bilan
/// parity: kvadrat rasm, tortish chizig'i, o'ng pastda yurak, pastda
/// miqdor boshqaruvi va "Qo'shish · summa" tugmasi.
///
/// Reyting/ingredient kabi elementlar ATAYLAB yo'q — backend'da bunday
/// ma'lumot yo'q (loyihaning "soxta raqam yo'q" qoidasi).
/// "3D" almashtirgichi — rasm ustida suzadi.
///
/// Yoqilganda brend rangida bo'ladi, ya'ni mijoz qaysi rejimda
/// turganini bir qarashda ko'radi.
class _View3DToggle extends StatelessWidget {
  const _View3DToggle({required this.active, required this.onTap});

  final bool active;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: active ? kBrand : const Color(0xCCFFFFFF),
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(20),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                active ? Icons.photo_outlined : Icons.view_in_ar_rounded,
                size: 16,
                color: active ? Colors.white : const Color(0xFF424242),
              ),
              const SizedBox(width: 5),
              Text(
                active ? 'Rasm' : '3D',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: active ? Colors.white : const Color(0xFF424242),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ProductSheet extends StatefulWidget {
  final Map<String, dynamic> product;
  final ProductDiscount? discount;
  final bool favorited;
  final int initialQty;
  final ValueChanged<int> onConfirm;

  const _ProductSheet({
    required this.product,
    required this.discount,
    required this.favorited,
    required this.initialQty,
    required this.onConfirm,
  });

  @override
  State<_ProductSheet> createState() => _ProductSheetState();
}

class _ProductSheetState extends State<_ProductSheet> {
  late int _qty = widget.initialQty > 0 ? widget.initialQty : 1;

  /// 3D ko'rinish yoqilganmi.
  ///
  /// Standart — O'CHIQ: 3D chizish xotira va batareya talab qiladi,
  /// mijozlarning ko'pchiligi esa oddiy rasmga qaraydi. WebView faqat
  /// foydalanuvchi almashtirgichni bosganda quriladi.
  bool _show3D = false;

  /// ┌─ MIQDOR DARHOL SAVATGA YOZILADI ────────────────────────────────┐
  /// Ilgari paneldagi "+/−" faqat MAHALLIY sonni o'zgartirardi va
  /// savatga yozish uchun "Qo'shish" tugmasini bosish shart edi.
  /// Natijada mijoz panelda miqdorni o'zgartirib, panelni yopib
  /// yuborsa — menyudagi son eski holicha qolardi va ikkalasi
  /// bir-biriga mos kelmasdi.
  ///
  /// Endi taom ALLAQACHON savatda bo'lsa, har bosish darhol savatga
  /// yoziladi: menyu, savat va bu panel bir vaqtda yangilanadi.
  /// Savatda bo'lmasa esa faqat son tanlanadi — mijoz "Qo'shish"
  /// bosmaguncha savatga hech narsa tushmaydi.
  /// └─────────────────────────────────────────────────────────────────┘
  bool get _inCart => widget.initialQty > 0;

  void _changeQty(int delta) {
    final next = _qty + delta;
    if (next < 1) return;
    setState(() => _qty = next);
    if (_inCart) widget.onConfirm(next);
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.product;
    final name = (p['name'] as String?) ?? '';
    final image = (p['image_url'] as String?) ?? '';
    final price = (p['price_tiyin'] as num?)?.toInt() ?? 0;
    final available = p['available'] != false;
    final weight = (p['weight'] as num?)?.toDouble() ?? 0;
    final unit = formatWeightUnit((p['weight_unit'] as String?) ?? '');
    final desc = ((p['description'] as String?) ?? '').trim();
    final unitPrice = widget.discount?.discountedPriceTiyin ?? price;
    // 3D model — restoran paneli orqali yaratilgan bo'lsa keladi.
    // Bo'lmasa almashtirgich umuman ko'rsatilmaydi.
    final model3D = ((p['model_3d_url'] as String?) ?? '').trim();

    return Container(
      // Veb bilan bir xil: ekran tepasidan sal pastroq boshlanadi.
      height: MediaQuery.of(context).size.height - 28,
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        children: [
          // ── Rasm ────────────────────────────────────────────────
          //
          // `BoxFit.contain`: mahsulot rasmlari serverda 1:1 saqlanadi,
          // shuning uchun kvadrat konteynerni AYNAN to'ldiradi — na
          // kesiladi, na yonlarda boshqa rangli chiziq qoladi.
          AspectRatio(
            aspectRatio: 1,
            child: Stack(
              children: [
                Positioned.fill(
                  child: _show3D && model3D.isNotEmpty
                      ? Model3DView(modelUrl: model3D)
                      : image.isEmpty
                          ? Container(
                              color: const Color(0xFFF5F5F5),
                              child: const Icon(Icons.restaurant_menu,
                                  size: 56, color: Color(0xFFBDBDBD)),
                            )
                          : RemoteImage(
                              url: image,
                              fit: BoxFit.contain,
                              placeholder: Container(
                                color: const Color(0xFFF5F5F5),
                                child: const Icon(Icons.restaurant_menu,
                                    size: 56, color: Color(0xFFBDBDBD)),
                              ),
                            ),
                ),
                // 3D / rasm almashtirgichi — faqat model bor taomda.
                if (model3D.isNotEmpty)
                  Positioned(
                    left: 12,
                    top: 12,
                    child: _View3DToggle(
                      active: _show3D,
                      onTap: () => setState(() => _show3D = !_show3D),
                    ),
                  ),
                // Tortish chizig'i RASM USTIDA suzadi — alohida qator
                // bo'lsa tepada ortiqcha yo'lak hosil qilardi.
                Positioned(
                  top: 10,
                  left: 0,
                  right: 0,
                  child: Center(
                    child: Container(
                      width: 38,
                      height: 4,
                      decoration: BoxDecoration(
                        color: const Color(0x66000000),
                        borderRadius: BorderRadius.circular(4),
                      ),
                    ),
                  ),
                ),
                if (widget.discount != null)
                  Positioned(
                    right: 12,
                    top: 12,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: kBrand,
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Text(
                        widget.discount!.label,
                        style: const TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.bold,
                            color: Colors.white),
                      ),
                    ),
                  ),
                Positioned(
                  right: 12,
                  bottom: 12,
                  child: FavoriteButton(
                    productId: (p['id'] as String?) ?? '',
                    initialFavorited: widget.favorited,
                  ),
                ),
              ],
            ),
          ),

          // ── Matn ────────────────────────────────────────────────
          Expanded(
            child: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(20, 18, 20, 18),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text.rich(
                    TextSpan(
                      text: name,
                      style: const TextStyle(
                          fontSize: 20, fontWeight: FontWeight.bold),
                      children: [
                        if (weight > 0)
                          TextSpan(
                            text:
                                '  ${weight % 1 == 0 ? weight.toInt() : weight.toStringAsFixed(1)} $unit'
                                    .trimRight(),
                            style: const TextStyle(
                                fontSize: 15,
                                fontWeight: FontWeight.normal,
                                color: Color(0xFF757575)),
                          ),
                      ],
                    ),
                  ),
                  if (widget.discount != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        children: [
                          Text(
                            formatSum(unitPrice),
                            style: const TextStyle(
                                fontSize: 18,
                                fontWeight: FontWeight.bold,
                                color: Color(0xFFE53935)),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            formatSum(price),
                            style: const TextStyle(
                              fontSize: 13.5,
                              color: Color(0xFF757575),
                              decoration: TextDecoration.lineThrough,
                            ),
                          ),
                        ],
                      ),
                    ),
                  Padding(
                    padding: const EdgeInsets.only(top: 14),
                    child: Text(
                      desc.isEmpty ? 'Tavsif kiritilmagan' : desc,
                      style: TextStyle(
                        fontSize: 15,
                        height: 1.5,
                        color: const Color(0xFF616161),
                        fontStyle:
                            desc.isEmpty ? FontStyle.italic : FontStyle.normal,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),

          // ── Pastki panel ────────────────────────────────────────
          SafeArea(
            top: false,
            child: Container(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 12),
              decoration: const BoxDecoration(
                border: Border(
                    top: BorderSide(color: Color(0xFFEEEEEE), width: 1)),
              ),
              child: available
                  ? Row(
                      children: [
                        Container(
                          height: 52,
                          padding: const EdgeInsets.symmetric(horizontal: 4),
                          decoration: BoxDecoration(
                            color: const Color(0xFFF5F5F5),
                            borderRadius: BorderRadius.circular(16),
                          ),
                          child: Row(
                            children: [
                              _StepButton(
                                icon: Icons.remove,
                                onTap: _qty > 1 ? () => _changeQty(-1) : null,
                              ),
                              SizedBox(
                                width: 26,
                                child: Text(
                                  '$_qty',
                                  textAlign: TextAlign.center,
                                  style: const TextStyle(
                                      fontWeight: FontWeight.bold,
                                      fontSize: 15),
                                ),
                              ),
                              _StepButton(
                                icon: Icons.add,
                                onTap: () => _changeQty(1),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: SizedBox(
                            height: 52,
                            child: FilledButton(
                              style: FilledButton.styleFrom(
                                backgroundColor: kBrand,
                                shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(16)),
                              ),
                              onPressed: () {
                                // Savatda bo'lsa miqdor allaqachon
                                // yozilgan — tugma faqat yopadi.
                                if (!_inCart) widget.onConfirm(_qty);
                                Navigator.of(context).pop();
                              },
                              child: Text(
                                _inCart
                                    ? 'Tayyor  ·  ${formatSum(unitPrice * _qty)}'
                                    : 'Qo\'shish  ·  ${formatSum(unitPrice * _qty)}',
                                style: const TextStyle(
                                    fontSize: 15.5,
                                    fontWeight: FontWeight.bold),
                              ),
                            ),
                          ),
                        ),
                      ],
                    )
                  : const Padding(
                      padding: EdgeInsets.symmetric(vertical: 14),
                      child: Text('Hozircha mavjud emas',
                          style: TextStyle(color: Color(0xFF757575))),
                    ),
            ),
          ),
        ],
      ),
    );
  }
}

class _StepButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback? onTap;
  const _StepButton({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkResponse(
      onTap: onTap,
      radius: 22,
      child: SizedBox(
        width: 40,
        height: 44,
        child: Icon(
          icon,
          size: 19,
          color: onTap == null ? const Color(0xFFBDBDBD) : Colors.black,
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// OFFLAYN BELGISI
// ═══════════════════════════════════════════════════════════════════

