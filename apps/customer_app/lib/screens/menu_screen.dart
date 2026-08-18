import 'dart:async';

import 'package:flutter/material.dart';
// `ScrollCacheExtent` uchun — u `material.dart` orqali kelmaydi.
import 'package:flutter/rendering.dart';

import '../api.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import '../widgets/product_grid.dart';
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
      MaterialPageRoute(
        builder: (_) => MenuScreen(
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
  Set<String> _favorites = <String>{};
  bool _menuLoading = true;
  bool _menuOffline = false;

  List<_Section> _sections = const [];
  final _sectionKeys = <String, GlobalKey>{};
  final _chipKeys = <String, GlobalKey>{};
  String _activeCategory = '';

  /// Serverdan kelgan yakuniy summa. `null` — hali yo'q, vizual
  /// taxminga tushiladi (veb `lib/use-quote.ts` bilan bir xil naqsh).
  int? _quoteTiyin;
  int _quoteSeq = 0;

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
    _subscribe();
    _loadFavorites();
    _refreshQuote();
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    _menuSub?.cancel();
    _promoSub?.cancel();
    _scroll.dispose();
    _chipsScroll.dispose();
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

  /// Sevimlilar — anonim foydalanuvchida 401 keladi, bu XATO EMAS:
  /// shunchaki hech narsa belgilanmagan bo'ladi.
  Future<void> _loadFavorites() async {
    try {
      final ids = await api.favoriteIds();
      if (mounted) setState(() => _favorites = ids);
    } catch (_) {}
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
    if (_activeCategory.isEmpty && _sections.isNotEmpty) {
      _activeCategory = _sections.first.title;
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
  Future<void> _refreshQuote() async {
    if (_cart.isEmpty || _cart.restaurantId != _id) {
      if (mounted) setState(() => _quoteTiyin = null);
      return;
    }
    final seq = ++_quoteSeq;
    try {
      final res = await api.quote(_id, [
        for (final e in _cart.items.entries)
          {'product_id': e.key, 'qty': e.value}
      ]);
      if (!mounted || seq != _quoteSeq) return;
      final t = (res['total_tiyin'] as num?)?.toInt();
      // 0 yoki manfiy — "narx aniqlanmadi" deb qaraladi (veb bilan bir
      // xil qoida): noto'g'ri sozlangan aksiya bepul buyurtma
      // ko'rinishini bermasligi kerak.
      setState(() => _quoteTiyin = (t != null && t > 0) ? t : null);
    } catch (_) {
      // Anonim foydalanuvchi yoki tarmoq xatosi — vizual taxmin.
      if (mounted && seq == _quoteSeq) setState(() => _quoteTiyin = null);
    }
  }

  /// Server summasi bo'lmaganda ko'rsatiladigan taxmin.
  int get _visualTotal {
    var total = 0;
    final byId = {for (final p in _menu) (p['id'] as String? ?? ''): p};
    for (final e in _cart.items.entries) {
      final p = byId[e.key];
      if (p == null) continue;
      final d = computeProductDiscount(p, _promos);
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
    if (_sections.isEmpty || !_scroll.hasClients) return;

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
    if (next != _activeCategory) {
      setState(() => _activeCategory = next);
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
        _sections.indexWhere((s) => s.title == _activeCategory);
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

  void _add(Map<String, dynamic> p) => _cart.increment(
        restaurantId: _id,
        productId: (p['id'] as String?) ?? '',
        restaurantName: _name,
      );

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

  void _onFavoriteChanged(String productId, bool favorited) {
    setState(() {
      if (favorited) {
        _favorites.add(productId);
      } else {
        _favorites.remove(productId);
      }
    });
  }

  // ── Chizish ───────────────────────────────────────────────────────

  Widget _card(Map<String, dynamic> p) {
    final id = (p['id'] as String?) ?? '';
    final discount = computeProductDiscount(p, _promos);
    return ProductCard(
      product: p,
      qty: _cart.restaurantId == _id ? _cart.qtyOf(id) : 0,
      discount: discount,
      promoted: PromotionIndex(_promos).covers(p, discount),
      favorited: _favorites.contains(id),
      onAdd: () => _add(p),
      onRemove: () => _remove(p),
      onTap: () => _openDetail(p),
      onFavoriteChanged: (fav) => _onFavoriteChanged(id, fav),
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
        discount: computeProductDiscount(p, _promos),
        favorited: _favorites.contains(id),
        initialQty: _cart.restaurantId == _id ? _cart.qtyOf(id) : 0,
        onFavoriteChanged: (fav) => _onFavoriteChanged(id, fav),
        onConfirm: (qty) => _setQty(p, qty),
      ),
    );
  }

  Future<void> _openSearch() async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => _MenuSearchScreen(
          menu: _menu,
          buildCard: _card,
        ),
      ),
    );
    // Qidiruvda savat yoki yurak o'zgargan bo'lishi mumkin.
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final showChips = _sections.length > 1;
    final myCart = _cart.restaurantId == _id && !_cart.isEmpty;

    return Scaffold(
      backgroundColor: Colors.white,
      body: RefreshIndicator(
        color: kBrand,
        onRefresh: () async {
          _subscribe(force: true);
          await _loadFavorites();
        },
        child: CustomScrollView(
          controller: _scroll,
          physics: const AlwaysScrollableScrollPhysics(),
          // Kesh maydoni kengaytirildi: yonidagi bo'limlar tirik
          // qolsa, chip bosilganda pog'ona-pog'ona surish deyarli
          // kerak bo'lmaydi.
          scrollCacheExtent: const ScrollCacheExtent.viewport(1.5),
          slivers: [
            SliverAppBar(
              pinned: true,
              elevation: 0,
              scrolledUnderElevation: 0.5,
              backgroundColor: Colors.white,
              surfaceTintColor: Colors.transparent,
              foregroundColor: Colors.black,
              centerTitle: true,
              titleSpacing: 0,
              title: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (_logo.isNotEmpty) ...[
                    ClipRRect(
                      borderRadius: BorderRadius.circular(7),
                      child: SizedBox(
                        width: 28,
                        height: 28,
                        child: Image.network(
                          fullImageUrl(_logo),
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                  ],
                  Flexible(
                    child: Text(
                      _name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 18, fontWeight: FontWeight.bold),
                    ),
                  ),
                ],
              ),
              actions: [
                IconButton(
                  onPressed: _menu.isEmpty ? null : _openSearch,
                  icon: const Icon(Icons.search, size: 24),
                  tooltip: 'Qidirish',
                ),
              ],
              bottom: showChips
                  ? PreferredSize(
                      preferredSize: const Size.fromHeight(_kChipsHeight),
                      child: SizedBox(
                        height: _kChipsHeight,
                        child: ListView.separated(
                          key: _chipsListKey,
                          controller: _chipsScroll,
                          scrollDirection: Axis.horizontal,
                          padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
                          itemCount: _sections.length,
                          separatorBuilder: (_, __) => const SizedBox(width: 8),
                          itemBuilder: (_, i) {
                            final title = _sections[i].title;
                            return _CategoryChip(
                              key: _chipKeys[title],
                              label: title,
                              active: title == _activeCategory,
                              onTap: () => _scrollToCategory(title),
                            );
                          },
                        ),
                      ),
                    )
                  : null,
            ),

            if (_menuOffline) const SliverToBoxAdapter(child: _OfflineStrip()),

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

            // Suzuvchi tugma kontentni bosib qolmasin.
            const SliverToBoxAdapter(child: SizedBox(height: 96)),
          ],
        ),
      ),
      floatingActionButton: myCart
          ? _CartPill(
              totalTiyin: _quoteTiyin ?? _visualTotal,
              count: _cart.totalQty,
              onTap: () => Navigator.of(context).push(
                MaterialPageRoute(builder: (_) => const CartScreen()),
              ),
            )
          : null,
    );
  }
}

class _Section {
  final String title;
  final List<Map<String, dynamic>> items;
  const _Section(this.title, this.items);
}

// ═══════════════════════════════════════════════════════════════════
// TURKUM CHIPI
// ═══════════════════════════════════════════════════════════════════

class _CategoryChip extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;

  const _CategoryChip({
    super.key,
    required this.label,
    required this.active,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: active ? kBrand : Colors.transparent,
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(20),
        child: Container(
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(horizontal: 14),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(20),
            border: Border.all(
              color: active ? kBrand : const Color(0xFFE0E0E0),
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13.5,
              fontWeight: active ? FontWeight.bold : FontWeight.normal,
              color: active ? Colors.white : const Color(0xFF262626),
            ),
          ),
        ),
      ),
    );
  }
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
  final List<Map<String, dynamic>> menu;
  final Widget Function(Map<String, dynamic>) buildCard;

  const _MenuSearchScreen({required this.menu, required this.buildCard});

  @override
  State<_MenuSearchScreen> createState() => _MenuSearchScreenState();
}

class _MenuSearchScreenState extends State<_MenuSearchScreen> {
  final _controller = TextEditingController();
  final _cart = CartStore.instance;
  String _query = '';

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    _controller.dispose();
    super.dispose();
  }

  void _onCart() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final q = _query.trim().toLowerCase();
    final results = q.isEmpty
        ? const <Map<String, dynamic>>[]
        : widget.menu
            .where((p) =>
                ((p['name'] as String?) ?? '').toLowerCase().contains(q))
            .toList();

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        surfaceTintColor: Colors.transparent,
        foregroundColor: Colors.black,
        elevation: 0,
        titleSpacing: 0,
        title: Container(
          height: 42,
          padding: const EdgeInsets.symmetric(horizontal: 12),
          decoration: BoxDecoration(
            color: const Color(0xFFF7F7F7),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: const Color(0xFFE0E0E0)),
          ),
          child: Row(
            children: [
              const Icon(Icons.search, size: 20, color: Color(0xFF9E9E9E)),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  controller: _controller,
                  // Qidiruv oynasi ATAYLAB ochilgan — klaviatura darhol
                  // tayyor bo'lishi kerak.
                  autofocus: true,
                  textInputAction: TextInputAction.search,
                  decoration: const InputDecoration(
                    isDense: true,
                    border: InputBorder.none,
                    hintText: 'Taom qidirish...',
                  ),
                  onChanged: (v) => setState(() => _query = v),
                ),
              ),
            ],
          ),
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
    );
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
class _ProductSheet extends StatefulWidget {
  final Map<String, dynamic> product;
  final ProductDiscount? discount;
  final bool favorited;
  final int initialQty;
  final ValueChanged<bool> onFavoriteChanged;
  final ValueChanged<int> onConfirm;

  const _ProductSheet({
    required this.product,
    required this.discount,
    required this.favorited,
    required this.initialQty,
    required this.onFavoriteChanged,
    required this.onConfirm,
  });

  @override
  State<_ProductSheet> createState() => _ProductSheetState();
}

class _ProductSheetState extends State<_ProductSheet> {
  late int _qty = widget.initialQty > 0 ? widget.initialQty : 1;

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
                  child: image.isEmpty
                      ? Container(
                          color: const Color(0xFFF5F5F5),
                          child: const Icon(Icons.restaurant_menu,
                              size: 56, color: Color(0xFFBDBDBD)),
                        )
                      : Image.network(
                          fullImageUrl(image),
                          fit: BoxFit.contain,
                          errorBuilder: (_, __, ___) => Container(
                            color: const Color(0xFFF5F5F5),
                            child: const Icon(Icons.restaurant_menu,
                                size: 56, color: Color(0xFFBDBDBD)),
                          ),
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
                    onChanged: widget.onFavoriteChanged,
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
                                onTap: _qty > 1
                                    ? () => setState(() => _qty--)
                                    : null,
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
                                onTap: () => setState(() => _qty++),
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
                                widget.onConfirm(_qty);
                                Navigator.of(context).pop();
                              },
                              child: Text(
                                'Qo\'shish  ·  ${formatSum(unitPrice * _qty)}',
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

class _OfflineStrip extends StatelessWidget {
  const _OfflineStrip();

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 12, 16, 0),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF4E5),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: const Color(0xFFFFD9A8)),
      ),
      child: const Row(
        children: [
          Icon(Icons.cloud_off, size: 16, color: Color(0xFF9A5B00)),
          SizedBox(width: 8),
          Expanded(
            child: Text('Menyu yangilanmadi — saqlangan nusxa',
                style: TextStyle(fontSize: 12.5, color: Color(0xFF9A5B00))),
          ),
        ],
      ),
    );
  }
}
