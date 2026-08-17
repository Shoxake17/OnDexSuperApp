import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/product_grid.dart' show FavoriteButton;
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
      title: 'Istaklarim',
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
                    ? ListView(
                        children: const [
                          SizedBox(height: 120),
                          Icon(Icons.favorite_border,
                              size: 56, color: Colors.grey),
                          SizedBox(height: 12),
                          Center(
                            child: Text(
                                'Hali hech narsa saqlanmagan\nYoqtirgan taomlaringizni yurak belgisi bilan belgilang',
                                textAlign: TextAlign.center,
                                style: TextStyle(color: Colors.grey)),
                          ),
                        ],
                      )
                    : GridView.builder(
                        padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
                        gridDelegate:
                            const SliverGridDelegateWithFixedCrossAxisCount(
                          crossAxisCount: 2,
                          mainAxisSpacing: 22,
                          crossAxisSpacing: 14,
                          childAspectRatio: 0.62,
                        ),
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

class _FavoriteCard extends StatelessWidget {
  final Map<String, dynamic> product;
  final VoidCallback onTap;
  final VoidCallback onRemoved;
  const _FavoriteCard(
      {required this.product, required this.onTap, required this.onRemoved});

  @override
  Widget build(BuildContext context) {
    final imgUrl = product['image_url'] as String? ?? '';
    final weight = ((product['weight'] ?? 0) as num).toDouble();
    final weightUnit =
        formatWeightUnit((product['weight_unit'] as String?) ?? 'g');
    final name = product['name'] as String? ?? '';
    final price = (product['price_tiyin'] ?? 0) as int;
    final restaurantName = product['restaurant_name'] as String? ?? '';
    final restaurantOpen = product['restaurant_open'] == true;
    final restaurantLogo = product['restaurant_logo_url'] as String? ?? '';
    final surface = Theme.of(context).colorScheme.surfaceContainerHighest;

    return Opacity(
      opacity: restaurantOpen ? 1 : 0.4,
      child: InkWell(
        onTap: restaurantOpen ? onTap : null,
        borderRadius: BorderRadius.circular(18),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AspectRatio(
              aspectRatio: 1,
              child: Stack(
                children: [
                  Positioned.fill(
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(18),
                      child: imgUrl.isEmpty
                          ? Container(
                              color: surface,
                              child: const Icon(Icons.restaurant,
                                  color: Colors.grey, size: 36),
                            )
                          : Image.network(
                              fullImageUrl(imgUrl),
                              fit: BoxFit.cover,
                              loadingBuilder: (context, child, progress) {
                                if (progress == null) return child;
                                return Container(color: surface);
                              },
                              errorBuilder: (_, __, ___) => Container(
                                color: surface,
                                child: const Icon(Icons.restaurant,
                                    color: Colors.grey, size: 36),
                              ),
                            ),
                    ),
                  ),
                  Positioned(
                    left: 8,
                    top: 8,
                    child: FavoriteButton(
                      productId: product['id'] as String,
                      initialFavorited: true,
                      onChanged: (fav) {
                        // Bu yerda faqat "olib tashlash" (fav=false)
                        // ma'noga ega — "Istaklarim" sahifasidagi barcha
                        // kartochka allaqachon saqlangan.
                        if (!fav) onRemoved();
                      },
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 8),
            Text(formatSum(price),
                style:
                    const TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
            const SizedBox(height: 2),
            Text(
              name,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 13),
            ),
            if (weight > 0) ...[
              const SizedBox(height: 2),
              Text('${weight % 1 == 0 ? weight.toInt() : weight} $weightUnit',
                  style: const TextStyle(fontSize: 12, color: Colors.grey)),
            ],
            const SizedBox(height: 4),
            Row(
              children: [
                if (restaurantLogo.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(right: 4),
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(4),
                      child: SizedBox(
                        width: 16,
                        height: 16,
                        child: Image.network(
                          fullImageUrl(restaurantLogo),
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                        ),
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
          ],
        ),
      ),
    );
  }
}
