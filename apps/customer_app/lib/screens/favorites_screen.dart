import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../widgets/empty_state.dart';
import '../widgets/product_grid.dart';
import '../widgets/sheet_scaffold.dart';
import 'menu_screen.dart';

/// "Istaklarim" — mijoz yurak belgisi bilan saqlagan BARCHA mahsulotlar,
/// restoranidan qat'i nazar (bosh sahifadagi turkum bo'yicha qidiruv bilan
/// bir xil "restoranga bog'liq emas" mantiq). Taom bosilsa — o'sha
/// taomning restorani menyusiga o'tiladi (bitta buyurtma bitta restorandan
/// bo'lishi shart, shuning uchun bu yerdan to'g'ridan-to'g'ri savatga
/// qo'shilmaydi).
class FavoritesScreen extends StatefulWidget {
  const FavoritesScreen({super.key});

  @override
  State<FavoritesScreen> createState() => FavoritesScreenState();
}

/// PUBLIC State — HomeShell IndexedStack ichida bu ekranni doim "tirik"
/// ushlab turadi (tab almashtirilganda initState() QAYTA chaqirilmaydi),
/// shuning uchun HomeShell tab Istaklarim'ga o'tganda GlobalKey orqali
/// to'g'ridan-to'g'ri [reload]ni chaqiradi — shu orqali boshqa ekranda
/// (masalan menyu sahifasida) bosilgan yurak belgisi ham RESTARTSIZ,
/// darhol shu yerda ko'rinadi.
class FavoritesScreenState extends State<FavoritesScreen> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _items = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// silent=true — allaqachon ma'lumot bor bo'lsa (tab qayta ochilganda)
  /// spinner ko'rsatmasdan, jimgina fon rejimida yangilaydi — foydalanuvchi
  /// hech qanday "yuklanmoqda" chaqnashini sezmaydi.
  Future<void> reload({bool silent = false}) => _load(silent: silent);

  Future<void> _load({bool silent = false}) async {
    if (!silent) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final list = await api.favorites();
      if (!mounted) return;
      setState(() {
        _items = list.cast<Map<String, dynamic>>();
        _loading = false;
      });
    } catch (e) {
      if (!mounted || silent) return;
      setState(() {
        _loading = false;
        _error = '$e';
      });
    }
  }

  void _removeFromView(String productId) {
    setState(() => _items.removeWhere((p) => p['id'] == productId));
  }

  /// Sevimli taom bosilganda — o'sha restoranning NATIVE menyusi.
  ///
  /// Avval bu yerda `miniAppRoute` (WebView) ochilardi. Endi menyu
  /// native, va restoran ma'lumoti keshdan topiladi — o'sha mantiq
  /// `MenuScreen.open` da, bitta joyda.
  void _openRestaurant(Map<String, dynamic> p) {
    final id = (p['restaurant_id'] as String?) ?? '';
    if (id.isEmpty) return;
    MenuScreen.open(
      context,
      id,
      fallbackName: (p['restaurant_name'] as String?) ?? '',
    );
  }

  @override
  Widget build(BuildContext context) {
    return SheetScaffold(
      // Sarlavha "Sevimlilar" — pastki menyudagi yorliq bilan ham,
      // vebdagi sahifa nomi bilan ham bir xil. Avval bu yerda
      // "Istaklarim" turardi, ya'ni bitta bo'lim ilovaning o'zida ikki
      // xil atalardi.
      title: 'Sevimlilar',
      child: RefreshIndicator(
        onRefresh: _load,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? ListView(
                    children: [
                      const SizedBox(height: 120),
                      Center(child: Text('Xato: $_error')),
                    ],
                  )
                : _items.isEmpty
                    ? const EmptyStateList(
                        image: 'assets/empty/favorites.png',
                        title: 'Sevimli mahsulotlarni saqlash uchun '
                            'yurakchani bosing',
                      )
                    : GridView.builder(
                        padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
                        // Bu ekrandagi kartochkada QO'SHIMCHA qator bor
                        // (restoran nomi) — balandlik unga ham joy
                        // ajratishi kerak.
                        gridDelegate: productGridOf(context,
                            extraHeight: kCardFooterHeight),
                        itemCount: _items.length,
                        itemBuilder: (context, i) => _FavoriteCard(
                          product: _items[i],
                          onTap: () => _openRestaurant(_items[i]),
                          onRemoved: () =>
                              _removeFromView(_items[i]['id'] as String),
                        ),
                      ),
      ),
    );
  }
}

/// Sevimli taom kartochkasi.
///
/// Kartochkaning O'ZI menyudagi bilan AYNAN bir xil
/// ([ProductCard]) — bu yerda faqat ikkita farq qo'shiladi:
///   * pastda restoran nomi (menyuda kerak emas — u allaqachon
///     sarlavhada turadi);
///   * restoran yopiq bo'lsa kartochka so'niq va bosilmaydi.
///
/// Miqdor boshqaruvi ATAYLAB yo'q (`qty: null`): bitta buyurtma bitta
/// restorandan bo'lishi shart, shuning uchun bu yerdan to'g'ridan-to'g'ri
/// savatga qo'shilmaydi — avval restoran menyusiga o'tiladi.
class _FavoriteCard extends StatelessWidget {
  final Map<String, dynamic> product;
  final VoidCallback onTap;
  final VoidCallback onRemoved;
  const _FavoriteCard(
      {required this.product, required this.onTap, required this.onRemoved});

  @override
  Widget build(BuildContext context) {
    final restaurantName = product['restaurant_name'] as String? ?? '';
    final restaurantOpen = product['restaurant_open'] == true;
    final restaurantLogo = product['restaurant_logo_url'] as String? ?? '';

    return Opacity(
      opacity: restaurantOpen ? 1 : 0.4,
      child: ProductCard(
        product: product,
        favorited: true,
        onTap: restaurantOpen ? onTap : null,
        onFavoriteChanged: (fav) {
          // Bu yerda faqat "olib tashlash" (fav=false) ma'noga ega —
          // "Istaklarim" sahifasidagi barcha kartochka allaqachon
          // saqlangan.
          if (!fav) onRemoved();
        },
        footer: Row(
          children: [
            if (restaurantLogo.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(right: 4),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(4),
                  child: SizedBox(
                    width: 16,
                    height: 16,
                    child: RemoteImage(url: restaurantLogo),
                  ),
                ),
              ),
            Expanded(
              child: Text(
                restaurantOpen ? restaurantName : '$restaurantName (yopiq)',
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
