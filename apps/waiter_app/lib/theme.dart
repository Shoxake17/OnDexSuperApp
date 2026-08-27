import 'package:flutter/material.dart';

/// OnDex affitsiant ilovasining dizayn tokenlari.
///
/// ┌─ BITTA MANBA ─────────────────────────────────────────────────────┐
/// Ranglar EKRANLARDA qo'lda yozilmaydi. Restoran panelida bu qoida
/// buzilgan edi (har sahifa o'z status rangini belgilardi) va natijada
/// bitta holat uch sahifada uch xil ko'rinardi — shuning uchun bu yerda
/// hamma narsa nomlangan token sifatida turadi.
/// └───────────────────────────────────────────────────────────────────┘

/// Brend rangi — barcha OnDex ilovalarida bir xil.
const kBrandColor = Color(0xFFF64E03);

/// Bosilgan holat (tugma bosilganda).
const kBrandPressed = Color(0xFFD84203);

/// Fon — mijoz va kuryer ilovalari bilan AYNAN bir xil (#121212).
const kBackground = Color(0xFF121212);

/// Karta, AppBar, pastki panel yuzasi.
const kSurface = Color(0xFF1C1C1C);

/// Yuza ustidagi yuza: tanlangan qator, ichki blok.
const kSurfaceRaised = Color(0xFF242424);

/// Nozik chegara. Ilgari ekranlarda `Colors.white12` deb qo'lda
/// yozilardi — nomlanmagan qiymat vaqt o'tib bir joyda `white10`,
/// boshqasida `white12` bo'lib ketadi.
const kBorder = Color(0xFF2A2A2A);

/// "Tayyor" holati uchun rang — affitsiant ro'yxatga bir qarashda
/// nima qilish kerakligini ko'rishi uchun. Yashil ATAYLAB: bu yagona
/// harakat talab qiladigan holat.
const kReadyColor = Color(0xFF2E9E4F);

/// "Tayyorlanmoqda" — passiv kutish. Sariq ATAYLAB yashildan farqli:
/// affitsiant hech narsa qilmaydi, faqat kuzatadi.
const kWaitingColor = Color(0xFFB8790C);

/// Bekor qilingan / rad etilgan / xato.
const kDangerColor = Color(0xFFE5484D);

/// Matn ranglari.
const kInk = Color(0xFFF5F0E9);
const kInkDim = Color(0xB3FFFFFF); // white70
const kInkFaint = Color(0x8AFFFFFF); // ~white54
const kInkGhost = Color(0x3DFFFFFF); // ~white24

/// O'lchamlar — panellar (`restaurant_panel/theme.dart`) bilan bir xil,
/// ikkisi yonma-yon ochilganda bitta oila ekani ko'rinsin.
const kRadiusCard = 14.0;
const kRadiusButton = 10.0;
const kRadiusChip = 8.0;

/// Buyurtma holati → (yorliq, asosiy rang, fon rangi).
///
/// Backend'dagi HAQIQIY holatlar (`internal/orders/statemachine.go`:
/// created → accepted → preparing → ready → served). Boshqa hech qanday
/// oraliq holat o'ylab topilmaydi — UI faqat serverda bor narsani
/// ko'rsatadi, aks holda foydalanuvchi hech qachon sodir bo'lmaydigan
/// bosqichni kutib qoladi.
({String label, Color color, Color bg}) statusStyle(String status) {
  switch (status) {
    case 'created':
      return (label: 'Yangi', color: kInkDim, bg: kSurfaceRaised);
    case 'accepted':
      return (label: 'Qabul qilindi', color: kInkDim, bg: kSurfaceRaised);
    case 'preparing':
      return (
        label: 'Tayyorlanmoqda',
        color: kWaitingColor,
        bg: Color(0x1FB8790C),
      );
    case 'ready':
      return (label: 'TAYYOR', color: kReadyColor, bg: Color(0x2E2E9E4F));
    case 'served':
      return (label: 'Berildi', color: kReadyColor, bg: Color(0x1F2E9E4F));
    case 'rejected':
      return (label: 'Rad etildi', color: kDangerColor, bg: Color(0x1FE5484D));
    case 'cancelled':
      return (
        label: 'Bekor qilindi',
        color: kDangerColor,
        bg: Color(0x1FE5484D),
      );
    default:
      // Backend yangi holat qo'shsa — kalitning o'zi ko'rsatiladi,
      // ilova jimgina bo'sh joy chizib qo'ymaydi.
      return (label: status, color: kInkFaint, bg: kSurfaceRaised);
  }
}

/// Butun ilova mavzusi.
///
/// `.copyWith(primary: ...)` — Material3'ning `fromSeed` tonal palitrasi
/// seed rangni ANIQ o'zi sifatida saqlamaydi (biroz o'zgartiradi),
/// shuning uchun brend rangi majburan qayta yoziladi.
ThemeData buildWaiterTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: kBrandColor,
    brightness: Brightness.dark,
  ).copyWith(
    primary: kBrandColor,
    surface: kSurface,
    onSurface: kInk,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: kBackground,
    fontFamily: 'Roboto',
    appBarTheme: const AppBarTheme(
      backgroundColor: kBackground,
      foregroundColor: kInk,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
    ),
    cardTheme: CardThemeData(
      color: kSurface,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(kRadiusCard),
        side: const BorderSide(color: kBorder),
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: kBrandColor,
        foregroundColor: Colors.white,
        disabledBackgroundColor: kSurfaceRaised,
        disabledForegroundColor: kInkFaint,
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(kRadiusButton),
        ),
        textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: kInk,
        side: const BorderSide(color: kBorder),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(kRadiusButton),
        ),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: kSurface,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(kRadiusButton),
        borderSide: const BorderSide(color: kBorder),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(kRadiusButton),
        borderSide: const BorderSide(color: kBorder),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(kRadiusButton),
        borderSide: const BorderSide(color: kBrandColor, width: 1.5),
      ),
      labelStyle: const TextStyle(color: kInkFaint),
    ),
    dividerTheme: const DividerThemeData(color: kBorder, thickness: 1),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: kSurfaceRaised,
      contentTextStyle: const TextStyle(color: kInk),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(kRadiusButton),
      ),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: kSurface,
      surfaceTintColor: Colors.transparent,
    ),
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: kBrandColor,
    ),
  );
}
