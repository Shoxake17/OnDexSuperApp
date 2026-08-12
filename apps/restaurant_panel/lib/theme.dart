import 'package:flutter/material.dart';

/// OnDex brend ranglari — sabzi (apelsin) rang asosida. Bitta joyda
/// saqlanadi (sidebar, dashboard, status chip'lar barchasi shu yerdan
/// oladi) — rang qayerdadir noto'g'ri qo'lda yozilib, boshqa joydan farq
/// qilib qolishining oldini oladi.
class OnDexColors {
  OnDexColors._();

  static const primary = Color(0xFFF2650F); // asosiy sabzi rang
  static const primaryPressed = Color(0xFFD9540A);
  static const primaryDark = Color(0xFFFF7A29); // qorong'i fon ustida
  static const primaryTint = Color(0xFFFDE9DA); // yengil fon (badge)

  static const sidebarBg = Color(0xFF1B140F);
  static const sidebarBgActive = Color(0xFF2A2018);
  static const sidebarText = Color(0xFFF5ECE2);
  static const sidebarTextDim = Color(0xFFB7A896);

  static const pageBg = Color(0xFFF7F1E7);
  static const cardBg = Color(0xFFFFFFFF);
  static const cardBorder = Color(0xFFEAD9C8);

  static const ink = Color(0xFF1F1710);
  static const inkDim = Color(0xFF7A6B5C);
  static const inkFaint = Color(0xFFA6987F);

  static const success = Color(0xFF2F9E58);
  static const successBg = Color(0xFFE1F1E6);
  static const warning = Color(0xFF93551A);
  static const warningBg = Color(0xFFFBEAD6);
  static const danger = Color(0xFFE5484D);
  static const dangerBg = Color(0xFFFBE2E2);
  static const info = Color(0xFF3B5FA8);
  static const infoBg = Color(0xFFE3E9F6);

  // Statistika kartochkalari va mini-ko'rsatkichlar uchun ikonka-doira
  // aksentlari (buyurtma holati ranglaridan ATAYLAB alohida — status
  // pill'lar bilan aralashib ketmasligi uchun bitta yangi rang: amber).
  static const amber = Color(0xFFB8790C);
  static const amberBg = Color(0xFFFAECD2);
  static const purple = Color(0xFF7C5CFC);
  static const purpleBg = Color(0xFFECE8FE);
  static const pink = Color(0xFFE23B84);
  static const pinkBg = Color(0xFFFCE4EF);
  static const teal = Color(0xFF11897E);
  static const tealBg = Color(0xFFDFF3F1);
}

ThemeData buildOnDexTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: OnDexColors.primary,
    primary: OnDexColors.primary,
    onPrimary: Colors.white,
    surface: OnDexColors.cardBg,
  );
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: OnDexColors.pageBg,
    fontFamily: 'Roboto',
    cardTheme: CardThemeData(
      color: OnDexColors.cardBg,
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: const BorderSide(color: OnDexColors.cardBorder),
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: OnDexColors.primary,
        foregroundColor: Colors.white,
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
    ),
    appBarTheme: const AppBarTheme(
      backgroundColor: OnDexColors.pageBg,
      foregroundColor: OnDexColors.ink,
      elevation: 0,
      surfaceTintColor: Colors.transparent,
    ),
  );
}

/// Buyurtma holati → (yorlik matni, rang) — sidebar, buyurtmalar va bosh
/// sahifa BARCHASI shu bitta xaritalashdan foydalanadi (ilgari har sahifa
/// o'z rangini alohida belgilardi, endi bitta manba).
(String, Color, Color) orderStatusStyle(String status) {
  return switch (status) {
    'created' => ('Yangi', OnDexColors.primary, OnDexColors.primaryTint),
    'accepted' => ('Qabul qilindi', OnDexColors.info, OnDexColors.infoBg),
    'preparing' => (
        'Tayyorlanmoqda',
        OnDexColors.warning,
        OnDexColors.warningBg
      ),
    'ready' => ('Tayyor', OnDexColors.warning, OnDexColors.warningBg),
    'picked_up' => ('Yo\'lda', OnDexColors.info, OnDexColors.infoBg),
    'delivered' => ('Yetkazildi', OnDexColors.success, OnDexColors.successBg),
    // Stol buyurtmasining tugashi: affitsiant taomni stolga olib bordi.
    'served' => ('Berildi', OnDexColors.success, OnDexColors.successBg),
    'rejected' => ('Rad etildi', OnDexColors.danger, OnDexColors.dangerBg),
    'cancelled' => (
        'Bekor qilindi',
        OnDexColors.danger,
        OnDexColors.dangerBg
      ),
    _ => (status, OnDexColors.inkDim, OnDexColors.pageBg),
  };
}
