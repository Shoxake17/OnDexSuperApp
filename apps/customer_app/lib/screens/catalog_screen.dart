import 'package:flutter/material.dart';

import '../api.dart';
import '../data/catalog_repository.dart';
import 'menu_screen.dart';

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

  Future<void> _refresh() async {
    setState(() => _reloadTick++);
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      bottom: false,
      child: RefreshIndicator(
        color: kBrand,
        onRefresh: _refresh,
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

            return CustomScrollView(
              // `always` — ro'yxat qisqa bo'lsa ham pastga tortib
              // yangilash ishlashi kerak.
              physics: const AlwaysScrollableScrollPhysics(),
              slivers: [
                const SliverToBoxAdapter(child: _Header()),
                SliverToBoxAdapter(
                  child: _CategoryRow(repo: _categories, tick: _reloadTick),
                ),

                // Yangilash tarmoqda yiqilgan bo'lsa — eski ro'yxat
                // QOLADI, ustiga kichik ogohlantirish chiqadi. Bu
                // ataylab: xato uchun mazmunni o'chirish eng yomon
                // xatti-harakat.
                if (c.error != null)
                  const SliverToBoxAdapter(child: _OfflineNotice()),

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
                      separatorBuilder: (_, __) => const SizedBox(height: 16),
                      itemBuilder: (_, i) => _RestaurantCard(r: list[i]),
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

/// Brend rangi — `main.dart` dagi `kBrand` bilan bir xil.
const kBrand = Color(0xFFF4511E);

// ═══════════════════════════════════════════════════════════════════
// SARLAVHA
// ═══════════════════════════════════════════════════════════════════

class _Header extends StatelessWidget {
  const _Header();

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
                Row(
                  children: [
                    const Icon(Icons.place_outlined, size: 16),
                    const SizedBox(width: 4),
                    Text(
                      // VAQTINCHALIK — Shorebird OTA sinovi uchun.
                      // Yamoq qo'llanganini ko'z bilan tasdiqlash uchun
                      // matn ataylab o'zgartirildi. Sinov tugagach
                      // 'Chust' ga qaytariladi.
                      'Chust · OTA ✓',
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          // Uchta amal — veb tomonidagi `header-actions.tsx` bilan
          // bir xil tartibda: qidiruv, hamyon, bildirishnoma.
          const _IconButton(icon: Icons.search, label: 'Qidirish'),
          const SizedBox(width: 8),
          const _IconButton(
              icon: Icons.account_balance_wallet_outlined, label: 'Hamyon'),
          const SizedBox(width: 8),
          const _IconButton(
              icon: Icons.notifications_none, label: 'Bildirishnomalar'),
        ],
      ),
    );
  }
}

/// Dumaloq ikon tugmasi.
///
/// Hozircha amali YO'Q — qidiruv, hamyon va bildirishnoma ekranlari
/// keyingi bosqichda native qilinadi. Tugma ko'rinadi, lekin bosilganda
/// hech narsa qilmasligi noto'g'ri bo'lardi, shuning uchun qisqa
/// tushuntirish chiqadi.
class _IconButton extends StatelessWidget {
  final IconData icon;
  final String label;

  const _IconButton({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: InkResponse(
        radius: 24,
        onTap: () => ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$label — tez orada')),
        ),
        child: Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            border: Border.all(color: const Color(0xFFE5E5E5)),
            color: Colors.white,
          ),
          child: Icon(icon, size: 20, color: const Color(0xFF666666)),
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
          height: 104,
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

  /// Nomdan asset yo'lini topadi.
  ///
  /// Veb tomonda ayni jadval `lib/categoryIcons.ts` da. Bu DUBLIKAT,
  /// lekin STACKLAR ORASIDA (Dart va TypeScript) — bir stack ichida
  /// ikki nusxa yo'q. Rasm fayllarining o'zi esa bitta manba:
  /// Flutter `assets/categories/`, veb undan nusxa oladi.
  static const _icons = <String, String>{
    'burger': 'burger', 'kfc': 'kfc', 'pizza': 'pizza', 'lavash': 'lavash',
    'sushi': 'sushi', 'kabob': 'kabob', 'somsa': 'somsa', 'hotdog': 'hotdog',
    'steyk': 'steyk', 'sandvich': 'sandvich', 'salat': 'salat',
    'pishiriq': 'pishiriq', 'bolalar': 'bolalar', 'norin': 'norin',
    'osh': 'osh', 'vok': 'vok', 'halal': 'halal', 'gazak': 'gazak',
    'desert': 'dessert', 'shirinlik': 'shirinlik', 'shirinliklar': 'shirinlik',
    'ichimlik': 'ichimlik', 'ichimliklar': 'ichimlik',
    'fastfood': 'fastfood', 'fast food': 'fastfood',
    'milliy': 'milliy', 'milliy taomlar': 'milliy',
    'yevropa': 'yevropa', 'yevropa taomlar': 'yevropa',
    'italya': 'italya', 'yapon': 'yapon', 'turkcha': 'turkcha',
    'lagmon': 'lag\'mon', 'lag\'mon': 'lag\'mon',
    'suyuq ovqat': 'suyuq-ovqat', 'quyuq ovqatlar': 'quyuq-ovqatlar',
  };

  String? get _asset {
    final key = name.trim().toLowerCase();
    final file = _icons[key];
    return file == null ? null : 'assets/categories/$file.png';
  }

  @override
  Widget build(BuildContext context) {
    final asset = _asset;
    return SizedBox(
      width: 76,
      child: Column(
        children: [
          Container(
            width: 62,
            height: 62,
            decoration: const BoxDecoration(
              shape: BoxShape.circle,
              color: Color(0xFFF5F5F5),
            ),
            clipBehavior: Clip.antiAlias,
            child: asset == null
                ? const Icon(Icons.restaurant, color: Color(0xFF9E9E9E))
                : Image.asset(
                    asset,
                    fit: BoxFit.cover,
                    // Asset ro'yxatdan tushib qolsa ilova YIQILMAYDI.
                    errorBuilder: (_, __, ___) =>
                        const Icon(Icons.restaurant, color: Color(0xFF9E9E9E)),
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
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// RESTORAN KARTASI
// ═══════════════════════════════════════════════════════════════════

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
        borderRadius: BorderRadius.circular(20),
        color: const Color(0xFF3A3A3A),
      ),
      clipBehavior: Clip.antiAlias,
      child: Stack(
        fit: StackFit.expand,
        children: [
          if (cover.isNotEmpty)
            Image.network(
              fullImageUrl(cover),
              fit: BoxFit.cover,
              // Rasm kelmasa karta baribir o'qiladi — matn ostidagi
              // to'q fon qoladi.
              errorBuilder: (_, __, ___) => const SizedBox.shrink(),
            ),

          // Matn o'qilishi uchun pastdan yuqoriga qorayish. Cover
          // rasmining yuqori qismi ochiq qoladi.
          const DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.bottomCenter,
                end: Alignment.center,
                colors: [Color(0xCC000000), Color(0x00000000)],
              ),
            ),
          ),

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
            right: 16,
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
                  ),
                ),
                if (tags.isNotEmpty)
                  Text(
                    tags,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: Colors.white70, fontSize: 13),
                  ),
                if (address.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Row(
                      children: [
                        const Icon(Icons.place_outlined,
                            size: 12, color: Colors.white70),
                        const SizedBox(width: 3),
                        Expanded(
                          child: Text(
                            address,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                color: Colors.white70, fontSize: 12),
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
        MaterialPageRoute(builder: (_) => MenuScreen(restaurant: r)),
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
class _OfflineNotice extends StatelessWidget {
  const _OfflineNotice();

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 4, 16, 8),
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
            child: Text(
              'Yangilab bo\'lmadi — saqlangan ro\'yxat ko\'rsatilmoqda',
              style: TextStyle(fontSize: 12.5, color: Color(0xFF9A5B00)),
            ),
          ),
        ],
      ),
    );
  }
}
