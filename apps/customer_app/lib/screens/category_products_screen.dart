import 'package:flutter/material.dart';

import '../widgets/page_sheet.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../data/favorites_store.dart';
import '../widgets/product_grid.dart';
import '../widgets/sheet_page.dart';
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

  @override
  void initState() {
    super.initState();
    _load();
    FavoritesStore.instance.load(force: true);
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
        _error = errorText(e, 'Yuklab bo\'lmadi');
        _items ??= const [];
      });
    }
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

    return SheetPage(
        child: Scaffold(
      backgroundColor: Colors.white,
      appBar: PageAppBar(title: widget.category),
      body: RefreshIndicator(
        color: kBrand,
        onRefresh: _load,
        child: items == null
            ? const Center(child: CircularProgressIndicator())
            : items.isEmpty
                ? ErrorViewList(
                    message: _error ?? 'Bu turkumda taom topilmadi',
                    // Xato bo'lmasa (shunchaki bo'sh turkum) qayta
                    // urinishning ma'nosi yo'q.
                    onRetry: _error == null ? null : _load,
                    icon: _error == null
                        ? Icons.search_off
                        : Icons.error_outline,
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
                      favorited: FavoritesStore.instance
                          .contains(items[i]['id'] as String? ?? ''),
                      onTap: () => _open(items[i]),
                    ),
                  ),
      ),
    ));
  }
}

/// Kartochka menyudagi bilan AYNAN bir xil ([ProductCard]) — bu yerda
/// faqat pastda restoran nomi qo'shiladi va restoran yopiq bo'lsa
/// kartochka so'niq va bosilmaydi.
class _CategoryCard extends StatelessWidget {
  final Map<String, dynamic> product;
  final bool favorited;
  final VoidCallback onTap;

  const _CategoryCard({
    required this.product,
    required this.favorited,
    required this.onTap,
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
        onTap: open ? onTap : null,
        topLeft: FavoriteButton(
          productId: (product['id'] as String?) ?? '',
          initialFavorited: favorited,
        ),
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
                    child: RemoteImage(url: logo),
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
