import 'package:flutter/material.dart';

import '../api.dart';
import '../category_icons.dart';
import '../data/catalog_repository.dart';
import '../widgets/app_text_field.dart';
import '../widgets/common.dart';
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
  final _restaurants = Repos.restaurants();
  final _categories = Repos.categories();

  /// "Pastga tortib yangilash" oqimni qaytadan ishga tushiradi.
  /// Kalit o'zgarishi `StreamBuilder` ni yangi obunaga majburlaydi.
  int _reloadTick = 0;

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
    // ┌─ BOSH SAHIFADA DUMALOQ BURCHAK YO'Q ────────────────────────┐
    // Restoranlar ro'yxati — ilovaning "tagi". Uning ostida boshqa
    // sahifa turmaydi, shuning uchun karta bo'lib ko'rinishi kerak
    // emas: tarkib tizim panelidan darhol boshlanadi. Tizim panelining
    // rangini `home_shell.dart` belgilaydi (u butun ekranni qamraydi).
    // └─────────────────────────────────────────────────────────────┘
    return Container(
        color: Colors.white,
        child: RefreshIndicator(
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
                            child: SizedBox(height: _kHeaderHeight)),
                        SliverToBoxAdapter(
                          child:
                              _CategoryRow(repo: _categories, tick: _reloadTick),
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

                        if (list.isEmpty)
                          const SliverFillRemaining(
                            hasScrollBody: false,
                            child: Center(
                              child: Padding(
                                padding: EdgeInsets.all(32),
                                child: Text('Hozircha restoran yo\'q'),
                              ),
                            ),
                          )
                        else
                          SliverPadding(
                            padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
                            sliver: SliverList.separated(
                              itemCount: list.length,
                              separatorBuilder: (_, __) =>
                                  const SizedBox(height: 16),
                              itemBuilder: (_, i) => _RestaurantCard(r: list[i]),
                            ),
                          ),
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
        borderRadius: scrolled
            ? const BorderRadius.vertical(bottom: Radius.circular(20))
            : BorderRadius.zero,
        // Soya skroll paytida sezilarli bo'lishi kerak — sarlavha
        // tarkibdan ajralib turadi. Ikki qatlam: keng va yumshoq
        // (chuqurlik) + tor va aniq (chekka chizig'i).
        boxShadow: scrolled
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

  @override
  void initState() {
    super.initState();
    _loadUnread();
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
    // belgilangan — belgi yangilanishi kerak.
    if (mounted) _loadUnread();
  }

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
                // Shahar qat'iy: platforma Chust uchun. Maketdagi
                // "Toshkent, Chilonzor" — shunchaki namuna edi.
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
                        Text(
                          'Chust',
                          style: Theme.of(context)
                              .textTheme
                              .bodyMedium
                              ?.copyWith(fontWeight: FontWeight.w600),
                        ),
                        const SizedBox(width: 2),
                        const Icon(Icons.keyboard_arrow_down,
                            size: 18, color: Color(0xFF757575)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
          // Uchta amal — veb tomonidagi `header-actions.tsx` bilan
          // bir xil tartibda: qidiruv, hamyon, bildirishnoma.
          _IconButton(
            icon: Icons.search,
            label: 'Qidirish',
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) =>
                    _RestaurantSearchScreen(restaurants: widget.restaurants),
              ),
            ),
          ),
          const SizedBox(width: 8),
          _IconButton(
            icon: Icons.account_balance_wallet_outlined,
            label: 'Hamyon',
            onTap: () => _push(const WalletScreen()),
          ),
          const SizedBox(width: 8),
          _IconButton(
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

/// Dumaloq ikon tugmasi.
class _IconButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  /// O'qilmaganlar soni. 0 — belgi chizilmaydi.
  final int badge;

  const _IconButton({
    required this.icon,
    required this.label,
    required this.onTap,
    this.badge = 0,
  });

  /// Belgida ko'rsatiladigan eng katta son — undan yuqorisi "9+".
  static const _maxBadge = 9;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: InkResponse(
        radius: 24,
        onTap: onTap,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: const Color(0xFFE5E5E5)),
                color: Colors.white,
              ),
              child: Icon(icon, size: 20, color: const Color(0xFF666666)),
            ),
            if (badge > 0)
              Positioned(
                right: -2,
                top: -2,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  constraints: const BoxConstraints(minWidth: 18),
                  height: 18,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: const Color(0xFFEF4444),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  // Maketdagi qizil nuqta — lekin raqam bilan:
                  // "nechta?" degan savol nuqtadan javob olmaydi.
                  child: Text(
                    badge > _maxBadge ? '$_maxBadge+' : '$badge',
                    style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.bold,
                      color: Colors.white,
                      height: 1,
                    ),
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

  @override
  Widget build(BuildContext context) {
    final q = _query.trim().toLowerCase();
    final results = q.isEmpty
        ? const <Map<String, dynamic>>[]
        : widget.restaurants
            .where((r) =>
                ((r['name'] as String?) ?? '').toLowerCase().contains(q))
            .toList();

    return PageSheet(
        child: Scaffold(
      backgroundColor: Colors.white,
      appBar: PageAppBar(
        titleSpacing: 0,
        titleWidget: AppTextField(
          controller: _controller,
          hint: 'Restoran qidirish...',
          icon: Icons.search,
          autofocus: true,
          textInputAction: TextInputAction.search,
          onChanged: (v) => setState(() => _query = v),
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
                  q.isEmpty ? 'Restoran nomini yozing' : 'Mos restoran topilmadi',
                  style: const TextStyle(color: Color(0xFF757575)),
                ),
              ),
            )
          : ListView.separated(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
              itemCount: results.length,
              separatorBuilder: (_, __) => const SizedBox(height: 16),
              itemBuilder: (_, i) => _RestaurantCard(r: results[i]),
            ),
    ));
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
          // └─────────────────────────────────────────────────────────┘
          height: 120,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
            itemCount: list.length,
            separatorBuilder: (_, __) => const SizedBox(width: 14),
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
      width: 76,
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        // Turkum bosilsa — o'sha turkumdagi taomlar (vebdagi
        // `/search?category=...`). Avval plitka UMUMAN bosilmasdi.
        onTap: () => Navigator.of(context).push(
          sheetRoute(CategoryProductsScreen(category: name)),
        ),
        child: Column(
          children: [
            Container(
              width: 62,
              height: 62,
              // Ichki chekinish: surat ramkaga tegib turmasin (vebdagi
              // `p-1` bilan bir xil).
              padding: const EdgeInsets.all(5),
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                color: Color(0xFFF5F5F5),
              ),
              clipBehavior: Clip.antiAlias,
              child: asset == null
                  ? const Icon(Icons.restaurant, color: Color(0xFF9E9E9E))
                  : Image.asset(
                      asset,
                      // `contain` — surat TO'LIQ ko'rinsin. Avval
                      // `cover` edi va u kvadrat rasmni dumaloq
                      // ramkaga sig'dirish uchun chetlarini KESIB
                      // tashlardi (vebda esa `object-contain`).
                      fit: BoxFit.contain,
                      // Asset ro'yxatdan tushib qolsa ilova YIQILMAYDI.
                      errorBuilder: (_, __, ___) => const Icon(
                          Icons.restaurant, color: Color(0xFF9E9E9E)),
                    ),
            ),
            const SizedBox(height: 6),
            Text(
              name,
              maxLines: 2,
              textAlign: TextAlign.center,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 12, height: 1.15),
            ),
          ],
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// RESTORAN KARTASI
// ═══════════════════════════════════════════════════════════════════

/// Cover rasmi ustidagi oq matn uchun soya.
///
/// Gradient o'rniga ATAYLAB shu ishlatiladi (veb `restaurant-card.tsx`
/// bilan bir xil qaror): soya faqat harflar atrofida bo'ladi, brend
/// suratini bosmaydi.
const _textShadow = [
  Shadow(color: Color(0xBF000000), blurRadius: 4, offset: Offset(0, 1)),
];

class _RestaurantCard extends StatelessWidget {
  final Map<String, dynamic> r;
  const _RestaurantCard({required this.r});

  @override
  Widget build(BuildContext context) {
    final open = r['open'] == true;
    final cover = (r['cover_url'] as String?) ?? '';
    final name = (r['name'] as String?) ?? '';
    final tags = ((r['tags'] as String?) ?? '').trim();
    final address = ((r['address'] as String?) ?? '').trim();

    // 0 = kiritilmagan. Soxta "0.0 ★" yoki "0 daqiqa" KO'RSATILMAYDI —
    // veb tomondagi `restaurant-card.tsx` bilan bir xil qoida.
    final rating = (r['rating'] as num?)?.toDouble() ?? 0;
    final ratingCount = (r['rating_count'] as num?)?.toInt() ?? 0;
    final etaMin = (r['eta_min_minutes'] as num?)?.toInt() ?? 0;
    final etaMax = (r['eta_max_minutes'] as num?)?.toInt() ?? 0;
    final hasRating = rating > 0;
    final hasEta = etaMin > 0 && etaMax > 0;

    final card = Container(
      height: 200,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(24),
        // Cover yo'q bo'lsa fon TO'Q bo'lishi shart: oq matn och
        // kulrang ustida o'qilmasdi.
        color: const Color(0xFF3A3A3A),
      ),
      clipBehavior: Clip.antiAlias,
      child: Stack(
        fit: StackFit.expand,
        children: [
          if (cover.isNotEmpty)
            // Rasm kelmasa karta baribir o'qiladi — matn ostidagi
            // to'q fon qoladi.
            RemoteImage(url: cover)
          else
            const Center(
              child: Icon(Icons.storefront_outlined,
                  size: 64, color: Color(0x40FFFFFF)),
            ),

          // ┌─ COVER RASMI O'ZGARTIRILMAYDI ──────────────────────────┐
          // Avval bu yerda pastdan yuqoriga qorayadigan gradient
          // turardi va u brend suratini bosib, kartani xira
          // ko'rsatardi. Vebda u ATAYLAB olib tashlangan
          // (`restaurant-card.tsx`), lekin native tomonda qolib
          // ketgan — ikkalasi shu sababdan farq qilardi.
          //
          // Matn o'qilishi endi gradient bilan emas, HARF SOYASI
          // bilan ta'minlanadi: soya faqat harflar atrofida bo'ladi,
          // rasmning o'ziga tegmaydi.
          // └─────────────────────────────────────────────────────────┘

          Positioned(
            right: 12,
            top: 12,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: Colors.white,
                borderRadius: BorderRadius.circular(20),
              ),
              child: Text(
                open ? 'Ochiq' : 'Yopiq',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.bold,
                  color: open ? const Color(0xFF16A34A) : const Color(0xFFDC2626),
                ),
              ),
            ),
          ),

          Positioned(
            left: 16,
            // O'ngda "Ochiq/Yopiq" belgisi turadi — matn uning ostiga
            // kirib ketmasligi uchun keng chekinish (vebdagi `pr-24`).
            right: 96,
            bottom: 14,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                    shadows: _textShadow,
                  ),
                ),
                if (tags.isNotEmpty)
                  Text(
                    tags,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 13,
                        shadows: _textShadow),
                  ),
                if (address.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Row(
                      children: [
                        const Icon(Icons.place_outlined,
                            size: 12, color: Colors.white, shadows: _textShadow),
                        const SizedBox(width: 3),
                        Expanded(
                          child: Text(
                            address,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                color: Colors.white,
                                fontSize: 12,
                                shadows: _textShadow),
                          ),
                        ),
                      ],
                    ),
                  ),
                if (hasEta || hasRating)
                  Padding(
                    padding: const EdgeInsets.only(top: 10),
                    child: Wrap(
                      spacing: 8,
                      children: [
                        if (hasEta)
                          _Chip(
                            icon: Icons.schedule,
                            iconColor: const Color(0xFF444444),
                            text: '$etaMin–$etaMax daqiqa',
                          ),
                        if (hasRating)
                          _Chip(
                            icon: Icons.star,
                            iconColor: kBrand,
                            text: ratingCount > 0
                                ? '${rating.toStringAsFixed(1)} ($ratingCount+)'
                                : rating.toStringAsFixed(1),
                          ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );

    // Yopiq restoran bosilmaydi — menyusi ochilsa mijoz savat
    // to'ldirib, checkout'da rad javob olardi.
    if (!open) return Opacity(opacity: 0.6, child: card);

    return InkWell(
      borderRadius: BorderRadius.circular(20),
      // Menyu endi NATIVE. Restoran `Map`i uzatiladi — menyu ekrani
      // sarlavhani qayta so'ramaydi va internet bo'lmasa ham to'g'ri
      // chizadi (`menu_screen.dart` izohiga qarang).
      onTap: () => Navigator.of(context).push(
        // Menyu pastdan suzib chiqadi (`widgets/sheet_page.dart`).
        sheetRoute(MenuScreen(restaurant: r)),
      ),
      child: card,
    );
  }
}

class _Chip extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String text;

  const _Chip({required this.icon, required this.iconColor, required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: iconColor),
          const SizedBox(width: 5),
          Text(
            text,
            style: const TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: Color(0xFF262626),
            ),
          ),
        ],
      ),
    );
  }
}

/// Yangilash yiqilganda — mazmun ustidagi kichik ogohlantirish.
