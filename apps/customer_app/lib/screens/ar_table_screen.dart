import 'package:flutter/material.dart';

import '../ar/ar_camera_screen.dart';

/// Buyurtma qatoridagi AR uchun yetarli ma'lumot.
class ArDish {
  const ArDish({required this.name, required this.modelUrl, required this.qty});

  final String name;
  final String modelUrl;
  final int qty;

  /// Buyurtma qatoridan yasaydi. 3D modeli yo'q qator uchun `null`.
  static ArDish? fromItem(Map<String, dynamic> item) {
    final url = ((item['model_3d_url'] as String?) ?? '').trim();
    if (url.isEmpty) return null;
    return ArDish(
      name: (item['name'] as String?) ?? '',
      modelUrl: url,
      qty: (item['qty'] as num?)?.toInt() ?? 1,
    );
  }

  /// Buyurtma tarkibidan 3D modeli bor qatorlarni ajratadi.
  static List<ArDish> fromOrder(Map<String, dynamic> order) {
    final items = (order['items'] as List?) ?? const [];
    return items
        .whereType<Map>()
        .map((e) => ArDish.fromItem(Map<String, dynamic>.from(e)))
        .whereType<ArDish>()
        .toList();
  }
}

/// "Stolni kameraga tuting" — buyurtma qilingan taomlarni kamera
/// orqali ko'rish.
///
/// Taom tanlangach ILOVA ICHIDA ochiladi (`ArCameraScreen`) — mijoz
/// OnDex'dan chiqib ketmaydi.
class ArTableScreen extends StatelessWidget {
  const ArTableScreen({super.key, required this.dishes});

  final List<ArDish> dishes;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        title: const Text('Stolda ko\'rish',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
      ),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
        children: [
          const _Hint(),
          const SizedBox(height: 18),
          for (final d in dishes) ...[
            _DishTile(dish: d),
            const SizedBox(height: 12),
          ],
        ],
      ),
    );
  }
}

class _Hint extends StatelessWidget {
  const _Hint();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF3E9),
        borderRadius: BorderRadius.circular(14),
      ),
      child: const Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.photo_camera_rounded, color: kArBrand, size: 22),
          SizedBox(width: 12),
          Expanded(
            child: Text(
              'Taomni tanlang — kamera ochiladi va taom ekranda paydo '
              'bo\'ladi. Telefonni stolga qarating.',
              style: TextStyle(fontSize: 13, height: 1.45),
            ),
          ),
        ],
      ),
    );
  }
}

class _DishTile extends StatelessWidget {
  const _DishTile({required this.dish});

  final ArDish dish;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFFFAFAFA),
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        onTap: () => Navigator.of(context).push(MaterialPageRoute(
          builder: (_) => ArCameraScreen(
            modelUrl: dish.modelUrl,
            title: dish.name,
          ),
        )),
        borderRadius: BorderRadius.circular(14),
        child: Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: const Color(0xFFE5E5E5)),
          ),
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: kArBrand.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: const Icon(Icons.view_in_ar_rounded,
                    color: kArBrand, size: 22),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(dish.name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 15, fontWeight: FontWeight.w700)),
                    const SizedBox(height: 2),
                    const Text('Kamerada ko\'rish',
                        style: TextStyle(
                            fontSize: 12.5, color: Color(0xFF757575))),
                  ],
                ),
              ),
              if (dish.qty > 1)
                Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: Text('${dish.qty}×',
                      style: const TextStyle(
                          fontSize: 13, color: Color(0xFF9E9E9E))),
                ),
              const Icon(Icons.chevron_right, color: Color(0xFFBDBDBD)),
            ],
          ),
        ),
      ),
    );
  }
}

/// Brend rangi — mijoz ilovasidagi bilan bir xil.
const kArBrand = Color(0xFFF4511E);
