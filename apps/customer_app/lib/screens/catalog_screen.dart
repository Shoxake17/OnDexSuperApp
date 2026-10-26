import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../category_icons.dart';
import '../data/address_scope.dart';
import '../data/catalog_repository.dart';
import '../widgets/app_text_field.dart';
import '../widgets/common.dart';
import '../widgets/header_action.dart';
import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'category_products_screen.dart';
import 'menu_screen.dart';
import 'notifications_screen.dart';
import 'wallet_screen.dart';

/// Bosh sahifa — restoranlar katalogi. NATIVE (maket: image/restarant.png).
///
/// ┌─ 2-BOSQICH: WEBVIEW O'RNIGA ──────────────────────────────────────┐
/// Bu ekran ilgari `MiniAppWebView` (Next.js) edi. Endi native, ya'ni:
///   * birinchi kadr TARMOQDAN emas, keshdan chiziladi;
///   * skroll GPU'da, WebView'ning parse/layout siklisiz;
///   * ilova internet umuman bo'lmaganda ham ochiladi.
///
/// Menyu, savat, checkout va buyurtma kuzatuvi ham native — mobil
/// oqimda WebView UMUMAN qolmadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ YUKLANISH MANTIG'I BU YERDA YO'Q ────────────────────────────────┐
/// Ekran `_loading`/`_error`/`_items` holatlarini SAQLAMAYDI. Hammasi
/// `Cached<T>` ichida keladi va spinner qoidasi bitta joyda:
/// `Cached.showSpinner` (`packages/ondex_core`). Aynan shu tarqoqlikni
/// yo'q qilish "ba'zan loading chiqadi, ba'zan chiqmaydi" holatini
/// bartaraf etadi.
/// └───────────────────────────────────────────────────────────────────┘
class CatalogScreen extends StatefulWidget {
  const CatalogScreen({super.key});

  @override
  State<CatalogScreen> createState() => _CatalogScreenState();
}

class _CatalogScreenState extends State<CatalogScreen> {
  /// Repozitoriylar bir marta quriladi — `build` ichida yaratilsa
  /// har qayta chizishda yangi oqim ochilib, so'rov takrorlanardi.
  ///
  /// ⚠️ `_restaurants` — ISTISNO: uning kesh kaliti manzilga bog'liq
  /// (`Repos.restaurants()` izohiga qarang), shuning uchun manzil
  /// o'zgarganda u QAYTA quriladi. Busiz ekran eski shaharning
  /// ro'yxatini ushlab qolardi.
  Repository<List<dynamic>> _restaurants = Repos.restaurants();
  final _categories = Repos.categories();
  final _banners = Repos.banners();

  /// "Pastga tortib yangilash" oqimni qaytadan ishga tushiradi.
  /// Kalit o'zgarishi `StreamBuilder` ni yangi obunaga majburlaydi.
  int _reloadTick = 0;

  @override
  void initState() {
    super.initState();
    AddressScope.instance.addListener(_onAddressChanged);
  }

  @override
  void dispose() {
    AddressScope.instance.removeListener(_onAddressChanged);
    super.dispose();
  }

  /// Manzil o'zgardi — yangi kalit bilan yangi repozitoriy va yangi
  /// obuna. Server ro'yxatni manzil shahri bilan cheklaydi, ya'ni
  /// javob ham boshqacha bo'ladi.
  void _onAddressChanged() {
    if (!mounted) return;
    setState(() {
      _restaurants = Repos.restaurants();
      _reloadTick++;
    });
  }

  /// Ro'yxat joyidan siljiganmi — sarlavhaning dumaloq burchagi va
  /// soyasi shunga qarab paydo bo'ladi.
  bool _scrolled = false;

  Future<void> _refresh() async {
    setState(() => _reloadTick++);
  }

  /// `setState` FAQAT chegaradan o'tganda chaqiriladi — har piksel
  /// uchun emas, aks holda skroll paytida ekran uzluksiz qayta
  /// chizilardi.
  bool _onScroll(ScrollNotification n) {
    if (n.metrics.axis != Axis.vertical) return false;
    final scrolled = n.metrics.pixels > 2;
    if (scrolled != _scrolled) setState(() => _scrolled = scrolled);
    return false;
  }

  @override
  Widget build(BuildContext context) {
    // ┌─ BU EKRAN — TAB, QOBIQ EMAS ────────────────────────────────┐
    // Katalog `RestaurantShell` ning birinchi tab'i. Dumaloq burchak,
    // tizim paneli chizig'i va `Scaffold` (ya'ni MATERIAL) o'sha
    // qobiqdan keladi — bu yerda takrorlanmaydi.
    //
    // MUHIM: shu sababdan `CatalogScreen` ni to'g'ridan-to'g'ri
    // `Navigator.push` QILMANG. Qobiqsiz push qilinganda sarlavhadagi
    // `HeaderActionButton` lar (`InkResponse`) Material ancestor topolmay,
    // butun ekran "No Material widget found" bilan qizarib ketadi —
    // aynan shu xato jonli sinovda topilgan. Restoran bo'limi HAR
    // DOIM `RestaurantShell` orqali ochiladi.
    // └─────────────────────────────────────────────────────────────┘
    return RefreshIndicator(
      color: kBrand,
      onRefresh: _refresh,
      // Aylanma ko'rsatkich QOTIRILGAN sarlavha ostidan chiqsin —
      // aks holda u sarlavha orqasida ko'rinmay qolardi.
      edgeOffset: _kHeaderHeight,
      child: StreamBuilder<Cached<List<dynamic>>>(
        key: ValueKey('restaurants-$_reloadTick'),
        stream: _restaurants.observe(force: _reloadTick > 0),
        builder: (context, snap) {
          final c = snap.data ?? const Cached<List<dynamic>>(refreshing: true);

          // YAGONA spinner holati: hech qachon ko'rilmagan ma'lumot.
          if (c.showSpinner) {
            return const Center(child: CircularProgressIndicator());
          }

          final list = (c.value ?? const []).cast<Map<String, dynamic>>();

          // ┌─ SARLAVHA QOTIRILGAN ─────────────────────────────────┐
          // Logo, shahar va uchta ikon skroll bilan yuqoriga chiqib
          // ketmaydi — ular `Stack` ning ustki qatlamida turadi.
          // Ro'yxat esa ularning OSTIDAN suriladi, shuning uchun
          // skroll boshlanishi bilan sarlavhaning pastki burchaklari
          // dumaloqlanadi va soya paydo bo'ladi: tarkib "ostiga
          // kirib ketayotgani" ko'rinib turadi.
          // └───────────────────────────────────────────────────────┘
          return Stack(
            children: [
              Positioned.fill(
                child: NotificationListener<ScrollNotification>(
                  onNotification: _onScroll,
                  child: CustomScrollView(
                    // `always` — ro'yxat qisqa bo'lsa ham pastga tortib
                    // yangilash ishlashi kerak.
                    physics: const AlwaysScrollableScrollPhysics(),
                    slivers: [
                      // Qotirilgan sarlavha egallagan joy.
                      const SliverToBoxAdapter(
                        child: SizedBox(height: _kHeaderHeight),
                      ),
                      SliverToBoxAdapter(
                        child: _PromoBannerSection(
                          repo: _banners,
                          tick: _reloadTick,
                          restaurants: list,
                        ),
                      ),
                      SliverToBoxAdapter(
                        child: _CategoryRow(
                          repo: _categories,
                          tick: _reloadTick,
                        ),
                      ),

                      // Yangilash tarmoqda yiqilgan bo'lsa — eski ro'yxat
                      // QOLADI, ustiga kichik ogohlantirish chiqadi. Bu
                      // ataylab: xato uchun mazmunni o'chirish eng yomon
                      // xatti-harakat.
                      if (c.error != null)
                        const SliverToBoxAdapter(
                          child: OfflineNotice(
                            message:
                                'Yangilab bo\'lmadi — saqlangan ro\'yxat ko\'rsatilmoqda',
                          ),
                        ),

                      // ⚠️ Ro'yxat SERVERDA manzil shahri bo'yicha
                      // filtrlanadi (`visibleRestaurants`) — bo'sh
                      // ro'yxat "umuman yo'q" emas, "shu shaharda
                      // yo'q" degani bo'lishi mumkin.
                      if (list.isEmpty)
                        const SliverFillRemaining(
                          hasScrollBody: false,
                          child: Center(
                            child: Padding(
                              padding: EdgeInsets.all(32),
                              child: Text(
                                'Manzilingiz hududida hali restoran yo\'q',
                                textAlign: TextAlign.center,
                              ),
                            ),
                          ),
                        )
                      else ...[
                        // Bo'lim sarlavhasi ("Yaqin atrofdagi
                        // restoranlar") ATAYLAB yo'q — foydalanuvchi
                        // talabiga ko'ra olib tashlandi: ro'yxat o'zi
                        // bo'lim ekanligi banner/kategoriyalardan keyin
                        // allaqachon ko'rinib turadi.
                        SliverPadding(
                          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
                          sliver: SliverList.separated(
                            itemCount: list.length,
                            separatorBuilder:
                                (_, _) => const SizedBox(height: 20),
                            itemBuilder: (_, i) => _RestaurantCard(r: list[i]),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
              Positioned(
                top: 0,
                left: 0,
                right: 0,
                child: _StickyHeader(
                  scrolled: _scrolled,
                  child: _Header(restaurants: list),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

/// Qotirilgan sarlavhaning balandligi.
///
/// Aniq son ATAYLAB: ro'yxat aynan shuncha bo'sh joydan boshlanishi
/// kerak, aks holda birinchi turkumlar sarlavha ostida qolib ketardi.
/// [_Header] ning ichki balandligi shu songa moslangan.
const double _kHeaderHeight = 80;

/// Sarlavha qobig'i — skroll boshlangach pastki burchaklari
/// dumaloqlanadi va soya chiqadi.
class _StickyHeader extends StatelessWidget {
  final bool scrolled;
  final Widget child;

  const _StickyHeader({required this.scrolled, required this.child});

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 180),
      curve: Curves.easeOut,
      height: _kHeaderHeight,
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius:
            scrolled
                ? const BorderRadius.vertical(bottom: Radius.circular(20))
                : BorderRadius.zero,
        // Soya skroll paytida sezilarli bo'lishi kerak — sarlavha
        // tarkibdan ajralib turadi. Ikki qatlam: keng va yumshoq
        // (chuqurlik) + tor va aniq (chekka chizig'i).
        boxShadow:
            scrolled
                ? [
                  BoxShadow(
                    color: Colors.black.withValues(alpha: 0.13),
                    blurRadius: 18,
                    offset: const Offset(0, 6),
                  ),
                  BoxShadow(
                    color: Colors.black.withValues(alpha: 0.06),
                    blurRadius: 4,
                    offset: const Offset(0, 2),
                  ),
                ]
                : const [],
      ),
      child: child,
    );
  }
}

/// Brend rangi — `main.dart` dagi `kBrand` bilan bir xil.
const kBrand = Color(0xFFF4511E);

// ═══════════════════════════════════════════════════════════════════
// SARLAVHA
// ═══════════════════════════════════════════════════════════════════

class _Header extends StatefulWidget {
  /// Qidiruv oynasiga uzatiladi — u alohida so'rov yubormaydi,
  /// allaqachon keshdan kelgan ro'yxatni filtrlaydi (veb bilan bir xil).
  final List<Map<String, dynamic>> restaurants;

  const _Header({required this.restaurants});

  @override
  State<_Header> createState() => _HeaderState();
}

class _HeaderState extends State<_Header> {
  int _unread = 0;

  /// Saqlangan manzil matni (`super_home_screen.dart` bilan bir xil manba).
  ///
  /// ⚠️ Boshlang'ich qiymat `AddressScope` dan OLINADI. Avval bu yerda
  /// `kNoAddressLabel` doimiysi turardi — natijada bosh sahifada manzil
  /// to'g'ri, restoran sahifasida esa "Manzilni tanlang" ko'rinardi:
  /// `initState` dagi `refresh()` serverdan o'zgarmagan manzilni olardi
  /// va `AddressScope` hech kimga xabar bermasdi.
  String _addressLabel = AddressScope.instance.label;

  @override
  void initState() {
    super.initState();
    AddressScope.instance.addListener(_onAddressChanged);
    _loadUnread();
    _loadAddress();
  }

  @override
  void dispose() {
    AddressScope.instance.removeListener(_onAddressChanged);
    super.dispose();
  }

  /// Kirmagan foydalanuvchida endpoint 401 qaytaradi — zaxira yorliq qoladi.
  Future<void> _loadAddress() => AddressScope.instance.refresh();

  void _onAddressChanged() {
    if (!mounted) return;
    setState(() => _addressLabel = AddressScope.instance.label);
  }

  /// O'qilmagan bildirishnomalar soni.
  ///
  /// Kirmagan foydalanuvchida endpoint 401 qaytaradi — bu NORMAL
  /// holat, xato emas. Bosh sahifa anonim ham ochiladi va unda
  /// shunchaki belgi chizilmaydi.
  ///
  /// Interval ATAYLAB yo'q: bosh sahifa uzoq ochiq turadi va har necha
  /// soniyada so'rov yuborish batareyani bekorga yeydi. Son ekranga
  /// qaytilganda yangilanadi.
  Future<void> _loadUnread() async {
    try {
      final n = await api.unreadNotificationCount();
      if (mounted) setState(() => _unread = n);
    } catch (_) {}
  }

  Future<void> _push(Widget screen) async {
    // Hamma sahifa bir xil ochiladi: pastdan suzib chiqadi va pastga
    // tortib yopiladi (`widgets/sheet_page.dart`).
    await Navigator.of(context).push(sheetRoute(screen));
    // Bildirishnomalar ochilgan bo'lsa ular o'qilgan deb
    // belgilangan — belgi yangilanishi kerak. Manzil ekranidan
    // qaytilgan bo'lsa yorliq ham yangi manzilni ko'rsatsin.
    if (mounted) {
      _loadUnread();
      _loadAddress();
    }
  }

  void _openSearch(BuildContext context) => Navigator.of(context).push(
    MaterialPageRoute(
      builder: (_) => _RestaurantSearchScreen(restaurants: widget.restaurants),
    ),
  );

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Logotip matn bilan — maketdagi "OnDex" wordmark.
                RichText(
                  text: const TextSpan(
                    style: TextStyle(
                      fontSize: 32,
                      fontWeight: FontWeight.w800,
                      height: 1,
                      letterSpacing: -0.8,
                      color: Color(0xFF171717),
                    ),
                    children: [
                      TextSpan(text: 'On'),
                      TextSpan(text: 'Dex', style: TextStyle(color: kBrand)),
                    ],
                  ),
                ),
                const SizedBox(height: 6),
                // Saqlangan manzil; yo'q bo'lsa `kNoAddressLabel`. Avval
                // bu yerda "Chust" qattiq yozilgan edi.
                //
                // Qator BOSILADI va manzil ekranini ochadi (vebda ham
                // `<Link href="/address">`) — shu sabab yonida pastga
                // ko'rsatkich turadi.
                InkWell(
                  onTap: () => _push(const AddressScreen()),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: 2),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.place_outlined, size: 16),
                        const SizedBox(width: 5),
                        Flexible(
                          child: Text(
                            _addressLabel,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: Theme.of(context).textTheme.bodyMedium
                                ?.copyWith(fontWeight: FontWeight.w600),
                          ),
                        ),
                        const SizedBox(width: 2),
                        const Icon(
                          Icons.keyboard_arrow_down,
                          size: 18,
                          color: Color(0xFF757575),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
          // Amallar: qidiruv, hamyon, bildirishnoma.
          //
          // ┌─ YORDAMCHI TUGMASI BU YERDA YO'Q ─────────────────────┐
          // Ilgari bu qatorda birinchi bo'lib `Icons.auto_awesome`
          // (Yordamchi) turardi. Olib tashlandi: Shaddiy'ga kirish
          // nuqtasi BITTA bo'lishi kerak — pastki menyuning
          // markazidagi yuz tugmasi (`ShaddiyFab`, `home_shell`).
          //
          // Ikki kirish nuqtasi bir-biriga mos kelmasdi: bu tugma
          // holatdan qat'i nazar chizilardi, FAB esa `AiStatus` ga
          // qarab kulrang "!" ko'rsatardi — foydalanuvchi bir joyda
          // ishlaydigan, boshqa joyda ishlamaydigan yordamchini
          // ko'rardi.
          // └───────────────────────────────────────────────────────┘
          HeaderActionButton(
            icon: Icons.search,
            label: 'Qidirish',
            onTap: () => _openSearch(context),
          ),
          const SizedBox(width: 8),
          HeaderActionButton(
            icon: Icons.account_balance_wallet_outlined,
            label: 'Hamyon',
            onTap: () => _push(const WalletScreen()),
          ),
          const SizedBox(width: 8),
          HeaderActionButton(
            icon: Icons.notifications_none,
            label: 'Bildirishnomalar',
            badge: _unread,
            onTap: () => _push(const NotificationsScreen()),
          ),
        ],
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// RESTORAN QIDIRUVI
// ═══════════════════════════════════════════════════════════════════

/// Bosh sahifadagi qidiruv — keshdagi ro'yxatni filtrlaydi, tarmoqqa
/// so'rov yubormaydi (veb `home-content.tsx` bilan bir xil qaror:
/// asosiy sahifa har doim to'liq ro'yxatni ko'rsatadi, filtr faqat shu
/// oynada ishlaydi).
class _RestaurantSearchScreen extends StatefulWidget {
  final List<Map<String, dynamic>> restaurants;
  const _RestaurantSearchScreen({required this.restaurants});

  @override
  State<_RestaurantSearchScreen> createState() =>
      _RestaurantSearchScreenState();
}

class _RestaurantSearchScreenState extends State<_RestaurantSearchScreen> {
  final _controller = TextEditingController();
  String _query = '';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// Taomlar bo'yicha qidiruvga o'tadi — `CategoryProductsScreen` ni
  /// AYNAN turkum doiralari ishlatadigan yo'l bilan (`GET
  /// /products/search?q=...`, endi MeiliSearch orqali xato-kechiruvchan),
  /// faqat "turkum" o'rniga foydalanuvchi erkin yozgan matn beriladi.
  ///
  /// ┌─ NEGA ALOHIDA EKRANGA O'TILADI, SHU YERDA CHIZILMAYDI ────────────┐
  /// Bu ekrandagi restoran natijalari — LOKAL, tarmoqqa chiqmaydi
  /// (allaqachon keshdagi ro'yxatni filtrlaydi). Taom qidiruvi esa har
  /// bosishda tarmoqqa so'rov yuborishni talab qiladi — buni shu
  /// ekranda debounce bilan qayta qurish o'rniga, aynan shu ishni
  /// allaqachon to'g'ri bajaradigan ekranga (`CategoryProductsScreen` —
  /// yuklanish/xato holatlari, "Sevimlilar" bilan bir xil kartochka)
  /// qayta ishlatiladi.
  /// └────────────────────────────────────────────────────────────────────┘
  void _searchProducts(String value) {
    final q = value.trim();
    if (q.isEmpty) return;
    Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => CategoryProductsScreen(category: q)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final rawQuery = _query.trim();
    final q = rawQuery.toLowerCase();
    final results =
        q.isEmpty
            ? const <Map<String, dynamic>>[]
            : widget.restaurants
                .where(
                  (r) =>
                      ((r['name'] as String?) ?? '').toLowerCase().contains(q),
                )
                .toList();

    return PageSheet(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: PageAppBar(
          titleSpacing: 0,
          titleWidget: AppTextField(
            controller: _controller,
            hint: 'Restoran yoki taom qidirish...',
            icon: Icons.search,
            autofocus: true,
            textInputAction: TextInputAction.search,
            onChanged: (v) => setState(() => _query = v),
            // Klaviaturadagi "Qidirish" — taomlar bo'yicha (restoranlar
            // yuqorida allaqachon jonli ko'rinadi).
            onSubmitted: _searchProducts,
          ),
          actions: [
            IconButton(
              onPressed: () => Navigator.of(context).pop(),
              icon: const Icon(Icons.close),
              tooltip: 'Yopish',
            ),
          ],
        ),
        body: ListView(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
          children: [
            if (rawQuery.isEmpty)
              const Padding(
                padding: EdgeInsets.all(32),
                child: Center(
                  child: Text(
                    'Restoran yoki taom nomini yozing',
                    style: TextStyle(color: Color(0xFF757575)),
                  ),
                ),
              )
            else ...[
              // ┌─ TAOMLAR — HAR DOIM TAKLIF QILINADI ────────────────────┐
              // Restoran nomi mos kelmasa ham, xuddi shu matn taom nomi
              // bo'lishi mumkin ("mohito" — restoran emas, ichimlik).
              // Foydalanuvchi klaviaturada "Qidirish" bosishi shart
              // emas — qator har doim ko'rinadi va bosilishi mumkin.
              // └────────────────────────────────────────────────────────┘
              _ProductSearchPrompt(
                query: rawQuery,
                onTap: () => _searchProducts(rawQuery),
              ),
              const SizedBox(height: 16),
              if (results.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 24),
                  child: Center(
                    child: Text(
                      'Mos restoran topilmadi',
                      style: TextStyle(color: Color(0xFF757575)),
                    ),
                  ),
                )
              else
                ...results.map(
                  (r) => Padding(
                    padding: const EdgeInsets.only(bottom: 16),
                    child: _RestaurantCard(r: r),
                  ),
                ),
            ],
          ],
        ),
      ),
    );
  }
}

/// `'<so'rov>' bo'yicha taomlarni qidirish` qatori — `_RestaurantCard`
/// bilan bir xil uslubda, lekin taomlarga olib boradi.
class _ProductSearchPrompt extends StatelessWidget {
  final String query;
  final VoidCallback onTap;

  const _ProductSearchPrompt({required this.query, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFFF7F7F7),
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
          child: Row(
            children: [
              const Icon(Icons.restaurant_menu, size: 20, color: kBrand),
              const SizedBox(width: 10),
              Expanded(
                child: RichText(
                  text: TextSpan(
                    style: const TextStyle(
                      fontSize: 14,
                      color: Color(0xFF1A1A1A),
                    ),
                    children: [
                      const TextSpan(text: '"'),
                      TextSpan(
                        text: query,
                        style: const TextStyle(fontWeight: FontWeight.w700),
                      ),
                      const TextSpan(text: '" bo\'yicha taomlarni qidirish'),
                    ],
                  ),
                ),
              ),
              const Icon(
                Icons.chevron_right,
                size: 20,
                color: Color(0xFF9E9E9E),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// TURKUMLAR
// ═══════════════════════════════════════════════════════════════════

class _CategoryRow extends StatelessWidget {
  final Repository<List<dynamic>> repo;
  final int tick;

  const _CategoryRow({required this.repo, required this.tick});

  @override
  Widget build(BuildContext context) {
    return StreamBuilder<Cached<List<dynamic>>>(
      key: ValueKey('categories-$tick'),
      stream: repo.observe(force: tick > 0),
      builder: (context, snap) {
        final list = snap.data?.value ?? const [];
        // Turkumlar hali kelmagan bo'lsa qator UMUMAN chizilmaydi —
        // bo'sh joy "nimadir buzildi" degan taassurot bermasin.
        if (list.isEmpty) return const SizedBox.shrink();

        return SizedBox(
          // ┌─ BALANDLIK O'LCHANDI, TAXMIN QILINMADI ─────────────────┐
          // Avval 104 edi va u YETMASDI: chekinish (12+8) + doira (62)
          // + oraliq (6) + ikki qatorli nom (12px × 1.15 × 2 ≈ 28) =
          // ~116. Debug build'da qurilmada "BOTTOM OVERFLOWED" qizil
          // bannerí chiqqan; release build'da esa banner ko'rsatilmaydi
          // va nom jimgina kesilardi — shuning uchun bu uzoq vaqt
          // sezilmagan.
          //
          // Doira 62→50 ga kichraygandan keyin ham baland balandlik
          // saqlanadi: nom hamon ikki qatorgacha chiqishi mumkin.
          // └─────────────────────────────────────────────────────────┘
          height: 112,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
            itemCount: list.length,
            // ┌─ 5 TASI BITTA EKRANGA SIG'ISHI UCHUN ───────────────────┐
            // Oraliq 14→8: ekranda 5-chi turkum ham skrollsiz to'liq
            // ko'rinsin (`_CategoryTile.width` bilan birga o'lchandi).
            // └───────────────────────────────────────────────────────────┘
            separatorBuilder: (_, _) => const SizedBox(width: 8),
            itemBuilder: (_, i) => _CategoryTile(name: _nameOf(list[i])),
          ),
        );
      },
    );
  }

  /// Turkum JSON'i satr ham, obyekt ham bo'lishi mumkin — backend
  /// ikkala shaklni ham qaytargan. Ikkalasini ham qabul qilamiz.
  static String _nameOf(dynamic raw) {
    if (raw is String) return raw;
    if (raw is Map) return (raw['name'] ?? raw['title'] ?? '').toString();
    return '';
  }
}

class _CategoryTile extends StatelessWidget {
  final String name;
  const _CategoryTile({required this.name});

  @override
  Widget build(BuildContext context) {
    // Jadval va normalizatsiya `lib/category_icons.dart` da — vebdagi
    // `lib/categoryIcons.ts` bilan bir xil. Avval bu yerda qat'iy
    // solishtiruvli qisqa jadval turardi va ko'plik shakllar
    // ("Burgerlar", "Steyklar") umuman topilmasdi.
    final asset = categoryIconFor(name);

    return SizedBox(
      // ┌─ 5 TASI BITTA EKRANGA SIG'ADI ──────────────────────────────┐
      // Avval 76 edi — ekranga atigi ~4 ta sig'ar, 5-chisi yarmi
      // kesilgan holda ko'rinardi. 58 — 5 ta turkum + 4 ta 8px oraliq
      // 360dp kenglikdagi ekranda (16+16 chekinishdan keyin ~328dp
      // qolgan joyga) to'liq sig'adigan qilib o'lchangan.
      // └────────────────────────────────────────────────────────────┘
      width: 58,
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        // Turkum bosilsa — o'sha turkumdagi taomlar (vebdagi
        // `/search?category=...`). Avval plitka UMUMAN bosilmasdi.
        onTap:
            () => Navigator.of(
              context,
            ).push(sheetRoute(CategoryProductsScreen(category: name))),
        child: Column(
          children: [
            Container(
              width: 50,
              height: 50,
              // Ichki chekinish: surat ramkaga tegib turmasin (vebdagi
              // `p-1` bilan bir xil).
              padding: const EdgeInsets.all(4),
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                color: Color(0xFFF5F5F5),
              ),
              clipBehavior: Clip.antiAlias,
              child:
                  asset == null
                      ? const Icon(
                        Icons.restaurant,
                        size: 20,
                        color: Color(0xFF9E9E9E),
                      )
                      : Image.asset(
                        asset,
                        // `contain` — surat TO'LIQ ko'rinsin. Avval
                        // `cover` edi va u kvadrat rasmni dumaloq
                        // ramkaga sig'dirish uchun chetlarini KESIB
                        // tashlardi (vebda esa `object-contain`).
                        fit: BoxFit.contain,
                        // Asset ro'yxatdan tushib qolsa ilova YIQILMAYDI.
                        errorBuilder:
                            (_, _, _) => const Icon(
                              Icons.restaurant,
                              size: 20,
                              color: Color(0xFF9E9E9E),
                            ),
                      ),
            ),
            const SizedBox(height: 6),
            Text(
              name,
              maxLines: 2,
              textAlign: TextAlign.center,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 11, height: 1.15),
            ),
          ],
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// REKLAMA BANNERI — ADMIN PANELIDAN BOSHQARILADI
// ═══════════════════════════════════════════════════════════════════

/// Turkumlar va restoranlar ro'yxati orasidagi reklama bloki.
///
/// ┌─ DINAMIK — YANDEX EATS/GO USLUBI ──────────────────────────────────┐
/// Avval bu blokning matni ham, rasmi ham kodga QATTIQ YOZILGAN edi:
/// yangi aksiya chiqarish uchun ilovani qayta qurish kerak bo'lardi.
/// Endi butunlay serverdan (`GET /promo-banners`) keladi — rasm
/// `internal/banners` orqali ADMIN PANELIDAN yuklanadi, ixtiyoriy
/// ravishda bitta restoranga bog'lanadi (bosilsa o'sha restoran
/// menyusi ochiladi).
///
/// Birorta ham FAOL banner bo'lmasa, bo'lim BUTUNLAY YASHIRILADI —
/// bo'sh joy yoki "yuklanmoqda" aylanmasi "nimadir buzildi" degan
/// taassurot bermasin (`_restaurantsSection`dagi "Barchasi" havolasi
/// olib tashlangani bilan bir xil falsafa).
/// └────────────────────────────────────────────────────────────────────┘
class _PromoBannerSection extends StatefulWidget {
  final Repository<List<dynamic>> repo;
  final int tick;

  /// Banner `restaurant_id`sini HAQIQIY restoran yozuviga aylantirish
  /// uchun — bosilganda `MenuScreen` AYNAN `_RestaurantCard` bilan bir
  /// xil to'liq obyektni oladi (internetsiz ham to'g'ri chiziladi).
  final List<Map<String, dynamic>> restaurants;

  const _PromoBannerSection({
    required this.repo,
    required this.tick,
    required this.restaurants,
  });

  @override
  State<_PromoBannerSection> createState() => _PromoBannerSectionState();
}

class _PromoBannerSectionState extends State<_PromoBannerSection> {
  final _pageController = PageController();
  Timer? _timer;

  /// Oxirgi marta oqimdan kelgan banner soni. `setState` SHART EMAS —
  /// faqat davriy taymer uchun zaxira, ekran bu o'zgarishdan allaqachon
  /// `StreamBuilder` orqali xabardor.
  int _count = 0;

  @override
  void initState() {
    super.initState();
    // ┌─ AVTOMATIK ALMASHISH — har 5 soniyada ───────────────────────┐
    // Oxirgisidan keyin birinchisiga qaytadi (aylana). Bitta banner
    // bo'lsa yoki ro'yxat hali kelmagan bo'lsa (`_count <= 1`) hech
    // narsa qilmaydi — `PageController`ga ulanmagan holatda
    // `animateToPage` chaqirish xato tashlardi.
    // └──────────────────────────────────────────────────────────────┘
    _timer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (!_pageController.hasClients || _count <= 1) return;
      final current = _pageController.page?.round() ?? 0;
      _pageController.animateToPage(
        (current + 1) % _count,
        duration: const Duration(milliseconds: 400),
        curve: Curves.easeInOut,
      );
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    _pageController.dispose();
    super.dispose();
  }

  Map<String, dynamic>? _restaurantFor(String id) {
    if (id.isEmpty) return null;
    for (final r in widget.restaurants) {
      if ((r['id'] ?? '').toString() == id) return r;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return StreamBuilder<Cached<List<dynamic>>>(
      key: ValueKey('banners-${widget.tick}'),
      stream: widget.repo.observe(force: widget.tick > 0),
      builder: (context, snap) {
        final list =
            (snap.data?.value ?? const []).cast<Map<String, dynamic>>();
        _count = list.length;
        if (list.isEmpty) return const SizedBox.shrink();

        return Padding(
          padding: const EdgeInsets.fromLTRB(16, 4, 16, 4),
          // Server "cover" ni 2.4:1 da kesadi (`internal/images.
          // coverAspectRatio`). Bu yerda 2.5 — biroz pastroq, restoran
          // kartasidan farqlanishi uchun qasddan (`BoxFit.cover` ortiqcha
          // chekkani kesib beradi). 2.5 dan kattaroq qilinsa banner
          // matni (masalan "Book Cafe" logotipi) pastdan kesilib qoladi.
          child: AspectRatio(
            aspectRatio: 2.5,
            child: PageView.builder(
              controller: _pageController,
              itemCount: list.length,
              itemBuilder:
                  (_, i) => Padding(
                    // Bir nechta banner bo'lsa, keyingisi chekkadan
                    // bir oz ko'rinib tursin — "yana bor" ishorasi.
                    padding: EdgeInsets.only(right: list.length > 1 ? 10 : 0),
                    child: _PromoBannerCard(
                      banner: list[i],
                      restaurant: _restaurantFor(
                        (list[i]['restaurant_id'] ?? '').toString(),
                      ),
                    ),
                  ),
            ),
          ),
        );
      },
    );
  }
}

class _PromoBannerCard extends StatelessWidget {
  final Map<String, dynamic> banner;

  /// `null` — banner havolasiz (faqat ko'rsatish uchun) yoki bog'langan
  /// restoran shu manzilda/shaharda ko'rinmaydi. Ikkala holatda ham
  /// banner bosilmaydi — "ishlaydigandek" tugma qo'yish jimgina hech
  /// narsa qilmaydigan tugmadan yomonroq.
  final Map<String, dynamic>? restaurant;

  const _PromoBannerCard({required this.banner, this.restaurant});

  @override
  Widget build(BuildContext context) {
    final image = (banner['image_url'] ?? '').toString();
    final card = ClipRRect(
      borderRadius: BorderRadius.circular(20),
      child: RemoteImage(
        url: image,
        fit: BoxFit.cover,
        placeholder: Container(color: const Color(0xFFFDF1E7)),
      ),
    );
    final r = restaurant;
    if (r == null) return card;
    return InkWell(
      borderRadius: BorderRadius.circular(20),
      onTap:
          () =>
              Navigator.of(context).push(sheetRoute(MenuScreen(restaurant: r))),
      child: card,
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// RESTORAN KARTASI
// ═══════════════════════════════════════════════════════════════════

/// Ro'yxatdagi bitta restoran — katta surat kartasi (`image/restarannts.png`).
///
/// Tepada to'liq kenglikdagi muqova surati, pastida nom+reyting va
/// yetib borish vaqti/turkum qatori.
///
/// ┌─ MAKETDAGI AYRIM BELGILAR QASDAN YO'Q ─────────────────────────────┐
/// Rasmda chegirma yorlig'i ("-10%"), sovg'a belgisi va yurakcha
/// (sevimli) ham bor. Ular QO'SHILMADI — bular uchun HAQIQIY ma'lumot
/// hali yo'q: chegirmalar faqat bitta restoran bo'yicha so'raladi
/// (`GET /restaurants/{id}/active-promotions`), ro'yxat uchun ularni
/// yig'adigan endpoint yo'q; restoranni "sevimli" qilish esa backendda
/// umuman mavjud emas (faqat MAHSULOT sevimli bo'lishi mumkin,
/// `internal/favorites`). Soxta belgi chizib qo'yish — bosilganda
/// ishlamaydigan tugma — bu loyihada qoida bilan taqiqlangan.
/// └──────────────────────────────────────────────────────────────────┘
class _RestaurantCard extends StatelessWidget {
  final Map<String, dynamic> r;
  const _RestaurantCard({required this.r});

  @override
  Widget build(BuildContext context) {
    // "Ochiq/Yopiq" — qo'lda tugma emas, ish vaqti bilan (server hisobi,
    // `ondex_core` dagi `RestaurantOpenStatus`).
    final now = DateTime.now();
    final status = RestaurantOpenStatus.fromJson(r).at(now);
    final open = status.open;
    final cover = (r['cover_url'] as String?) ?? '';
    final logo = ((r['logo_url'] as String?) ?? '').trim();
    // Katta surat uchun MUQOVA birinchi o'rinda — u taom/muassasa
    // manzarasi, logotip esa (ko'pincha kvadrat belgi) faqat ehtiyot
    // nusxa sifatida, muqova bo'lmaganda.
    final image = cover.isNotEmpty ? cover : logo;
    final name = (r['name'] as String?) ?? '';
    final address = ((r['address'] as String?) ?? '').trim();

    // 0 = kiritilmagan. Soxta "0.0 ★" yoki "0 daqiqa" KO'RSATILMAYDI —
    // veb tomondagi `restaurant-card.tsx` bilan bir xil qoida.
    final rating = (r['rating'] as num?)?.toDouble() ?? 0;
    final etaMin = (r['eta_min_minutes'] as num?)?.toInt() ?? 0;
    final etaMax = (r['eta_max_minutes'] as num?)?.toInt() ?? 0;
    final hasRating = rating > 0;
    final hasEta = etaMin > 0 && etaMax > 0;

    // `null` — hisoblab bo'lmadi (`internal/httpapi/scene_access.go`,
    // `withDeliveryFee`): bunda HECH narsa ko'rsatilmaydi, "bepul" deb
    // FARAZ QILINMAYDI. 0 — haqiqatan ham bepul (oylik maoshli kuryer).
    final deliveryFeeTiyin = (r['delivery_fee_tiyin'] as num?)?.toInt();

    final card = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          // ┌─ BANNER BILAN BIR XIL BURCHAK/CHEGARA, LEKIN BO'YI KO'PROQ ──┐
          // Burchak radiusi (20) tepadagi reklama banneri bilan bir xil
          // (`_PromoBannerSection`) — ikkala blok bir xil uslubda
          // ko'rinadi. Nisbat esa ATAYLAB farqli: restoran surati 2.0
          // (bannerdan balandroq — taom surati kichik bo'lib qolmasin),
          // banner 2.7 (reklama matni uchun keng, past panel yetarli).
          // Chegara oq fonli muqova/logotip (masalan Book Cafe) sahifa
          // foniga qo'shilib ketmasligi uchun kerak.
          // └────────────────────────────────────────────────────────────┘
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(20),
            border: Border.all(color: const Color(0xFFE5E7EB)),
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(19),
            child: AspectRatio(
              aspectRatio: 2.0,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  image.isEmpty
                      ? Container(color: const Color(0xFFF3F4F6))
                      : RemoteImage(
                        url: image,
                        fit: BoxFit.cover,
                        placeholder: Container(
                          color: const Color(0xFFF3F4F6),
                        ),
                      ),
                // Yopiq bo'lsa surat qoraytiriladi — mijoz oldindan
                // bilib, bosmaydi. Holat yorlig'ining o'zi endi doim
                // o'ng yuqori burchakda (pastda emas).
                if (!open) Container(color: const Color(0x66000000)),
                Positioned(
                  top: 8,
                  right: 8,
                  child: _StatusChip(open: open, label: status.label),
                ),
                ],
              ),
            ),
          ),
        ),
        const SizedBox(height: 8),
        // ┌─ IKKI USTUN ────────────────────────────────────────────────────┐
        // Chap ustun (3 qator): nom, yetkazish narxi, manzil. O'ng ustun
        // (2 qator, nomdan boshlab tepaga tekislangan): reyting TEPADA,
        // yetkazish vaqti uning OSTIDA — yonma-yon EMAS. Turkum teglari
        // (`tags`) qasddan YO'Q: yuqoridagi "Turkumlar" qatori shu
        // ma'lumotni beradi.
        // └───────────────────────────────────────────────────────────────┘
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                      color: Color(0xFF111827),
                    ),
                  ),
                  if (deliveryFeeTiyin != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 5),
                      child: _DeliveryFeeChip(tiyin: deliveryFeeTiyin),
                    ),
                  if (address.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Row(
                        children: [
                          const Icon(
                            Icons.place_outlined,
                            size: 14,
                            color: Color(0xFF9CA3AF),
                          ),
                          const SizedBox(width: 3),
                          Expanded(
                            child: Text(
                              address,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                fontSize: 12,
                                color: Color(0xFF6B7280),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
            ),
            if (hasRating || hasEta) ...[
              const SizedBox(width: 10),
              Padding(
                // o'ngdan ozgina bo'shliq — chekkaga yopishib qolmasin;
                // tepadan ozgina tushirilgan — nom bilan ikkalasi
                // bitta chiziqqa to'g'ri kelib, "yopishib" qolmasin.
                padding: const EdgeInsets.only(right: 2, top: 3),
                child: Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  if (hasRating)
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.star, size: 15, color: kBrand),
                        const SizedBox(width: 2),
                        Text(
                          rating.toStringAsFixed(1),
                          style: const TextStyle(
                            fontSize: 13.5,
                            fontWeight: FontWeight.w700,
                            color: Color(0xFF111827),
                          ),
                        ),
                      ],
                    ),
                  if (hasRating && hasEta) const SizedBox(height: 4),
                  if (hasEta)
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(
                          Icons.directions_walk,
                          size: 14,
                          color: Color(0xFF9CA3AF),
                        ),
                        const SizedBox(width: 3),
                        Text(
                          '$etaMin–$etaMax daq',
                          style: const TextStyle(
                            fontSize: 12,
                            color: Color(0xFF6B7280),
                          ),
                        ),
                      ],
                    ),
                ],
                ),
              ),
            ],
          ],
        ),
      ],
    );

    // Yopiq restoran bosilmaydi — menyusi ochilsa mijoz savat
    // to'ldirib, checkout'da rad javob olardi.
    if (!open) return card;

    return InkWell(
      borderRadius: BorderRadius.circular(18),
      // Menyu endi NATIVE. Restoran `Map`i uzatiladi — menyu ekrani
      // sarlavhani qayta so'ramaydi va internet bo'lmasa ham to'g'ri
      // chizadi (`menu_screen.dart` izohiga qarang).
      onTap:
          () => Navigator.of(context).push(
            // Menyu pastdan suzib chiqadi (`widgets/sheet_page.dart`).
            sheetRoute(MenuScreen(restaurant: r)),
          ),
      child: card,
    );
  }
}

/// Ochiq/yopiqlik blokcha — har doim ko'rinadi (faqat yopiqda emas).
/// Server hisobidan (`RestaurantOpenStatus`), qo'lda tugma emas.
class _StatusChip extends StatelessWidget {
  final bool open;
  final String label;
  const _StatusChip({required this.open, required this.label});

  @override
  Widget build(BuildContext context) {
    final color = open ? const Color(0xFF16A34A) : const Color(0xFF6B7280);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
      decoration: BoxDecoration(
        color: open ? const Color(0xFFECFDF5) : const Color(0xFFF3F4F6),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 6,
            height: 6,
            decoration: BoxDecoration(shape: BoxShape.circle, color: color),
          ),
          const SizedBox(width: 5),
          Text(
            label,
            style: TextStyle(
              fontSize: 11.5,
              fontWeight: FontWeight.w600,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}

/// Yetkazish narxi blokchasi — 0 bo'lsa "Bepul yetkazish" (yashil),
/// aks holda aniq summa. Qiymat `null` bo'lsa bu vidjet umuman
/// chaqirilmaydi (`_RestaurantCard` dagi `if (deliveryFeeTiyin != null)`).
class _DeliveryFeeChip extends StatelessWidget {
  final int tiyin;
  const _DeliveryFeeChip({required this.tiyin});

  @override
  Widget build(BuildContext context) {
    final free = tiyin <= 0;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
      decoration: BoxDecoration(
        color: free ? const Color(0xFFECFDF5) : const Color(0xFFF3F4F6),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.moped_outlined,
            size: 13,
            color: free ? const Color(0xFF16A34A) : const Color(0xFF6B7280),
          ),
          const SizedBox(width: 4),
          Text(
            free ? 'Bepul yetkazish' : 'Yetkazish ${formatSum(tiyin)}',
            style: TextStyle(
              fontSize: 11.5,
              fontWeight: FontWeight.w600,
              color: free ? const Color(0xFF16A34A) : const Color(0xFF6B7280),
            ),
          ),
        ],
      ),
    );
  }
}
