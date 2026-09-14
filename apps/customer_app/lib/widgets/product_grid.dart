/// Mijoz ilovasiga XOS menyu bo'laklari.
///
/// ┌─ KARTOCHKA ENDI UMUMIY PAKETDA ───────────────────────────────────┐
/// Taom kartochkasi, to'r, turkum chiplari va narx/aksiya mantig'i
/// `packages/ondex_menu` ga ko'chirildi — affitsiant ilovasining buyurtma
/// ekrani ham AYNI kodni ishlatadi, ya'ni ikki ilovada narx va ko'rinish
/// hech qachon ajralib ketmaydi.
///
/// Bu faylda faqat mijozga xos "Istaklarim" tugmasi qoladi (u
/// `FavoritesStore` ga bog'liq). Paket shu yerdan qayta eksport qilinadi —
/// `widgets/product_grid.dart` ni import qilgan ekranlar o'zgarmaydi.
/// └───────────────────────────────────────────────────────────────────┘
library;

import 'package:flutter/material.dart';

import '../data/favorites_store.dart';

export 'package:ondex_menu/ondex_menu.dart';

// ═══════════════════════════════════════════════════════════════════
// SEVIMLI TUGMASI
// ═══════════════════════════════════════════════════════════════════

/// "Istaklarim" yurak belgisi — oq doira, elevatsiyali; o'z holatini o'zi
/// boshqaradi: bosilganda DARHOL (optimistik) qizarib/oqarib, fon rejimida
/// serverga so'rov yuboradi. Xato bo'lsa (masalan tarmoq uzilsa) holat
/// ORQAGA qaytariladi — foydalanuvchi hech qachon soxta/yolg'on holat
/// ko'rmaydi.
///
/// Umumiy `ProductCard` ga `topLeft` uyasi orqali qo'yiladi.
class FavoriteButton extends StatefulWidget {
  final String productId;

  /// ESKIRGAN: holat endi [FavoritesStore] dan olinadi. Parametr
  /// chaqiruvchilarni buzmaslik uchun qoldirilgan va E'TIBORGA
  /// OLINMAYDI.
  final bool initialFavorited;
  // onChanged — muvaffaqiyatli o'zgarishdan KEYIN chaqiriladi (masalan
  // "Istaklarim" sahifasi shu orqali mahsulotni ro'yxatdan olib tashlaydi).
  final ValueChanged<bool>? onChanged;
  final double size;

  const FavoriteButton({
    super.key,
    required this.productId,
    required this.initialFavorited,
    this.onChanged,
    this.size = 20,
  });

  @override
  State<FavoriteButton> createState() => _FavoriteButtonState();
}

class _FavoriteButtonState extends State<FavoriteButton> {
  bool _busy = false;

  /// ┌─ HOLAT BU YERDA SAQLANMAYDI ────────────────────────────────────┐
  /// Ilgari har tugma o'z `_favorited` ini saqlardi. Shuning uchun
  /// bitta mahsulot ikki joyda ko'rinsa (menyu to'ri va taom tavsifi
  /// paneli) ular bir-biridan bexabar qolardi va biri eskirgan belgini
  /// ko'rsatib turardi.
  ///
  /// Endi yagona manba — [FavoritesStore]. Tugma unga QULOQ SOLADI,
  /// ya'ni qayerda bosilishidan qat'i nazar hamma nusxa bir vaqtda
  /// o'zgaradi.
  /// └─────────────────────────────────────────────────────────────────┘
  Future<void> _toggle() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await FavoritesStore.instance.toggle(widget.productId);
      widget.onChanged
          ?.call(FavoritesStore.instance.contains(widget.productId));
    } catch (_) {
      // Belgi `FavoritesStore` da allaqachon o'z holiga qaytarilgan.
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: FavoritesStore.instance,
      builder: (context, _) {
        final favorited = FavoritesStore.instance.contains(widget.productId);
        return Material(
          color: Colors.white,
          shape: const CircleBorder(),
          elevation: 3,
          child: InkWell(
            customBorder: const CircleBorder(),
            onTap: _toggle,
            child: Padding(
              padding: const EdgeInsets.all(8),
              child: Icon(
                favorited ? Icons.favorite : Icons.favorite_border,
                color: favorited ? const Color(0xFFE53935) : Colors.black,
                size: widget.size,
              ),
            ),
          ),
        );
      },
    );
  }
}
