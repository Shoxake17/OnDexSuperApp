import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/product_grid.dart';
import 'catalog_screen.dart' show kBrand;
import 'menu_screen.dart';

/// Bitta turkum bo'yicha BARCHA restoranlardagi mos taomlar.
///
/// ┌─ NEGA BU EKRAN QAYTA YOZILDI ─────────────────────────────────────┐
/// `apps/web/app/(food)/search/page.tsx` aynan shu faylga havola
/// qiladi ("category_products_screen.dart bilan bir xil"), lekin fayl
/// YO'Q edi — native ko'chirishda yo'qolgan.
///
/// Natijada bosh sahifadagi turkum doiralari NATIVDA umuman
/// bosilmasdi: TMA'da turkum bosilsa taomlar ochilardi, ilovada esa
/// hech narsa bo'lmasdi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Ma'lumot `GET /products/search?q=<turkum>` dan keladi — ochiq
/// endpoint, restoranga bog'liq emas.
///
/// ┌─ SAVATGA TO'G'RIDAN-TO'G'RI QO'SHILMAYDI ────────────────────────┐
/// Kartochkalarda miqdor boshqaruvi YO'Q (`qty: null`). Sabab: bitta
/// buyurtma bitta restorandan bo'lishi shart, bu ro'yxatda esa taomlar
/// TURLI restoranlardan. Taom bosilsa o'sha restoran menyusi ochiladi
/// — "Sevimlilar" ekranidagi bilan bir xil qoida.
/// └───────────────────────────────────────────────────────────────────┘
class CategoryProductsScreen extends StatefulWidget {
  final String category;

  const CategoryProductsScreen({super.key, required this.category});

  @override
  State<CategoryProductsScreen> createState() => _CategoryProductsScreenState();
}

class _CategoryProductsScreenState extends State<CategoryProductsScreen> {
  List<Map<String, dynamic>>? _items;
  String? _error;
  Set<String> _favorites = <String>{};

  @override
  void initState() {
    super.initState();
    _load();
    _loadFavorites();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final list = await api.searchProducts(widget.category);
      if (!mounted) return;
      setState(() => _items = list.cast<Map<String, dynamic>>());
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e is ApiException ? e.message : 'Yuklab bo\'lmadi';
        _items ??= const [];
      });
    }
  }

  /// Yurak belgilarining boshlang'ich holati. Anonim foydalanuvchida
  /// 401 keladi — bu xato emas, shunchaki hech narsa belgilanmagan.
  Future<void> _loadFavorites() async {
    try {
      final ids = await api.favoriteIds();
      if (mounted) setState(() => _favorites = ids);
    } catch (_) {}
  }

  void _open(Map<String, dynamic> p) {
    final rid = (p['restaurant_id'] as String?) ?? '';
    if (rid.isEmpty) return;
    MenuScreen.open(
      context,
      rid,
      fallbackName: (p['restaurant_name'] as String?) ?? '',
    );
  }

  @override
  Widget build(BuildContext context) {
    final items = _items;

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        surfaceTintColor: Colors.transparent,
        foregroundColor: const Color(0xFF171717),
        elevation: 0,
        title: Text(
          widget.category,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
        ),
      ),
      body: RefreshIndicator(
        color: kBrand,
        onRefresh: _load,
        child: items == null
            ? const Center(child: CircularProgressIndicator())
            : items.isEmpty
                ? ListView(
                    children: [
                      const SizedBox(height: 100),
                      Center(
                        child: Text(
                          _error ?? 'Bu turkumda taom topilmadi',
                          textAlign: TextAlign.center,
                          style: const TextStyle(color: Color(0xFF757575)),
                        ),
                      ),
                      if (_error != null)
                        Center(
                          child: TextButton(
                            onPressed: _load,
                            child: const Text('Qaytadan urinish',
                                style: TextStyle(color: kBrand)),
                          ),
                        ),
                    ],
                  )
                : GridView.builder(
                    padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
                    // Kartochkada restoran nomi qatori bor — balandlik
                    // unga ham joy ajratishi kerak.
                    gridDelegate: productGridOf(context,
                        extraHeight: kCardFooterHeight),
                    itemCount: items.length,
                    itemBuilder: (_, i) => _CategoryCard(
                      product: items[i],
                      favorited:
                          _favorites.contains(items[i]['id'] as String? ?? ''),
                      onTap: () => _open(items[i]),
                      onFavoriteChanged: (fav) {
                        final id = (items[i]['id'] as String?) ?? '';
                        setState(() {
                          if (fav) {
                            _favorites.add(id);
                          } else {
                            _favorites.remove(id);
                          }
                        });
                      },
                    ),
                  ),
      ),
    );
  }
}

/// Kartochka menyudagi bilan AYNAN bir xil ([ProductCard]) — bu yerda
/// faqat pastda restoran nomi qo'shiladi va restoran yopiq bo'lsa
/// kartochka so'niq va bosilmaydi.
class _CategoryCard extends StatelessWidget {
  final Map<String, dynamic> product;
  final bool favorited;
  final VoidCallback onTap;
  final ValueChanged<bool> onFavoriteChanged;

  const _CategoryCard({
    required this.product,
    required this.favorited,
    required this.onTap,
    required this.onFavoriteChanged,
  });

  @override
  Widget build(BuildContext context) {
    final name = (product['restaurant_name'] as String?) ?? '';
    final open = product['restaurant_open'] == true;
    final logo = (product['restaurant_logo_url'] as String?) ?? '';

    return Opacity(
      opacity: open ? 1 : 0.4,
      child: ProductCard(
        product: product,
        favorited: favorited,
        onTap: open ? onTap : null,
        onFavoriteChanged: onFavoriteChanged,
        footer: Row(
          children: [
            if (logo.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(right: 4),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(4),
                  child: SizedBox(
                    width: 16,
                    height: 16,
                    child: Image.network(
                      fullImageUrl(logo),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                    ),
                  ),
                ),
              ),
            Expanded(
              child: Text(
                open ? name : '$name (yopiq)',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 11, color: Colors.grey),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
