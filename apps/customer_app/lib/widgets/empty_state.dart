import 'package:flutter/material.dart';

/// Bo'sh ekranlarning YAGONA ko'rinishi: rasm + sarlavha + izoh.
///
/// ┌─ NEGA UMUMIY VIDJET ──────────────────────────────────────────────┐
/// Sevimlilar, savat va bildirishnomalar — uchtasi ham bir xil holatni
/// ko'rsatadi. Har birida alohida `Column` chizilsa, keyin biri
/// o'zgarib qolib ekranlar bir-biridan farq qila boshlaydi. Shuning
/// uchun o'lchamlar, oraliqlar va shriftlar FAQAT shu yerda turadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Rasmlar oq fonli (`assets/empty/`), ilovaning fon rangi ham oq
/// (`scaffoldBackgroundColor: Colors.white`) — shuning uchun ular
/// ekranga qo'shilib ketadi, ramka yoki soya kerak emas.
class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.image,
    required this.title,
    this.subtitle,
    this.action,
    this.imageSize = 220,
  });

  /// `assets/empty/...` ichidagi rasm yo'li.
  final String image;

  /// Qalin, katta sarlavha — bir yoki ikki qator.
  final String title;

  /// Ixtiyoriy izoh — kulrang, sarlavhadan kichik.
  final String? subtitle;

  /// Ixtiyoriy tugma (masalan "Menyuga o'tish").
  final Widget? action;

  final double imageSize;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Image.asset(
            image,
            width: imageSize,
            height: imageSize,
            fit: BoxFit.contain,
            // Rasm yuklanmasa ekran buzilmasligi kerak — bo'sh joy
            // qoladi, matn joyida turadi.
            errorBuilder: (_, __, ___) => SizedBox(height: imageSize),
          ),
          const SizedBox(height: 20),
          Text(
            title,
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: 21,
              fontWeight: FontWeight.w800,
              height: 1.25,
              color: Color(0xFF1A1A1A),
            ),
          ),
          if (subtitle != null) ...[
            const SizedBox(height: 10),
            Text(
              subtitle!,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 15,
                height: 1.45,
                color: Color(0xFF757575),
              ),
            ),
          ],
          if (action != null) ...[
            const SizedBox(height: 24),
            action!,
          ],
        ],
      ),
    );
  }
}

/// Aylantiriladigan (`RefreshIndicator` ichidagi) ro'yxatlar uchun.
///
/// ┌─ NEGA SHUNCHAKI `Center` EMAS ────────────────────────────────────┐
/// Ikkita talab bir-biriga qarshi turadi:
///   1. Rasm va matn ekran O'RTASIDA turishi kerak;
///   2. `RefreshIndicator` ishlashi uchun ichkarida AYLANADIGAN vidjet
///      bo'lishi shart — `Center` ni tortib bo'lmaydi.
///
/// Yechim: aylantiriladigan maydonga ekran balandligiga teng eng kam
/// balandlik beriladi (`minHeight`), ichida esa `Center`. Natijada
/// tarkib markazda turadi va ro'yxat baribir tortiladi.
/// └───────────────────────────────────────────────────────────────────┘
class EmptyStateList extends StatelessWidget {
  const EmptyStateList({
    super.key,
    required this.image,
    required this.title,
    this.subtitle,
    this.action,
  });

  final String image;
  final String title;
  final String? subtitle;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) => SingleChildScrollView(
        // Ro'yxat bo'sh bo'lsa ham pastga tortib yangilash ishlaydi.
        physics: const AlwaysScrollableScrollPhysics(),
        child: ConstrainedBox(
          constraints: BoxConstraints(minHeight: constraints.maxHeight),
          child: Center(
            child: EmptyState(
              image: image,
              title: title,
              subtitle: subtitle,
              action: action,
            ),
          ),
        ),
      ),
    );
  }
}
