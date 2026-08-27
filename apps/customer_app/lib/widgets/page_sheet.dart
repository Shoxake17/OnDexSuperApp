import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// ┌─ SAHIFA QOBIG'I: QORA TIZIM PANELI + DUMALOQ KARTA ────────────────┐
/// Ilovaning barcha sahifalari bir xil ko'rinadi: eng tepada tizimning
/// o'z paneli (soat, Wi-Fi, batareya) QORA fonda OQ belgilar bilan,
/// ostida esa yuqori burchaklari dumaloqlangan oq karta.
///
/// YAGONA ISTISNO — bosh sahifa, restoranlar ro'yxati
/// (`catalog_screen.dart`). U ilovaning "tagi": ostida boshqa sahifa
/// turmaydi, shuning uchun karta bo'lib ko'rinishi kerak emas va
/// dumaloq burchaksiz chiziladi. Uning tizim paneli ham boshqacha —
/// OQ fon, QORA belgilar ([light]).
/// └───────────────────────────────────────────────────────────────────┘
///
/// Ichkaridagi vidjet odatda oddiy `Scaffold` bo'ladi — sahifalarning
/// ichki tuzilishi (AppBar, pastki tugmalar, ro'yxatlar) o'zgarmaydi.
class PageSheet extends StatelessWidget {
  const PageSheet({
    super.key,
    required this.child,
    this.radius = 20,
    this.backdrop = Colors.black,
  });

  final Widget child;
  final double radius;

  /// Karta ORQASIDAGI rang — tizim paneli chizig'ida ko'rinadi.
  ///
  /// ┌─ NEGA O'ZGARUVCHAN ─────────────────────────────────────────┐
  /// Sahifa joyida turganda bu QORA bo'lishi kerak: tepadagi soat va
  /// Wi-Fi belgilar oq, ular qora fonda ko'rinadi.
  ///
  /// Lekin sahifa PASTGA TORTILGANDA o'sha qora chiziq ham birga
  /// suriladi va ekran o'rtasida qora tasma bo'lib qolardi. Shuning
  /// uchun `SheetPage` tortish boshlanishi bilan uni SHAFFOF qiladi —
  /// ostidagi sahifa ko'rinadi.
  /// └─────────────────────────────────────────────────────────────┘
  final Color backdrop;

  /// Qora tizim paneli — oq belgilar bilan.
  ///
  /// `statusBarColor` eski Android'lar uchun (u yerda panelni tizim
  /// chizadi), qora fon esa Android 15+ uchun — u yerda ilova panel
  /// ostiga ham chizadi va `statusBarColor` e'tiborga olinmaydi.
  static const dark = SystemUiOverlayStyle(
    statusBarColor: Colors.black,
    statusBarIconBrightness: Brightness.light, // Android
    statusBarBrightness: Brightness.dark, // iOS
    // Pastdagi tizim paneli (Orqaga / Uy / Menyu) — OQ fon, qora
    // belgilar. Standart holatda u to'q kulrang chiziq bo'lib turardi
    // va ilovaning oq pastki menyusidan keskin ajralib ketardi.
    systemNavigationBarColor: Colors.white,
    systemNavigationBarIconBrightness: Brightness.dark,
    systemNavigationBarDividerColor: Colors.transparent,
    // Android o'zi qo'shadigan yarim shaffof "kontrast" qatlamini
    // o'chiradi — usiz pastki panel oq so'ralsa ham kulrang chiqadi.
    systemNavigationBarContrastEnforced: false,
    systemStatusBarContrastEnforced: false,
  );

  /// Oq tizim paneli — belgilari QORA.
  ///
  /// Faqat bosh sahifada (`catalog_screen.dart`): u dumaloq kartasiz,
  /// oq fon bilan boshlanadi, shuning uchun ustidagi qora chiziq
  /// ajralib turardi.
  static const light = SystemUiOverlayStyle(
    statusBarColor: Colors.white,
    statusBarIconBrightness: Brightness.dark, // Android
    statusBarBrightness: Brightness.light, // iOS
    systemNavigationBarColor: Colors.white,
    systemNavigationBarIconBrightness: Brightness.dark,
    systemNavigationBarDividerColor: Colors.transparent,
    systemNavigationBarContrastEnforced: false,
    systemStatusBarContrastEnforced: false,
  );

  @override
  Widget build(BuildContext context) {
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: dark,
      child: Container(
        color: backdrop,
        child: SafeArea(
          // Pastdan emas: pastki tugmalar va gesture paneli sahifaning
          // o'z ishi (`Scaffold` uni allaqachon hisobga oladi).
          bottom: false,
          child: ClipRRect(
            borderRadius: BorderRadius.vertical(top: Radius.circular(radius)),
            child: child,
          ),
        ),
      ),
    );
  }
}

/// Sahifalarning YAGONA yuqori paneli.
///
/// ┌─ NEGA ──────────────────────────────────────────────────────────┐
/// Bu sozlamalar to'plami ilovada YETTI marta so'zma-so'z takrorlangan
/// edi (savat, bildirishnomalar, hamyon, turkum, checkout va ikkita
/// qidiruv ekrani):
///
///   backgroundColor: Colors.white,
///   surfaceTintColor: Colors.transparent,   // Material 3 ko'k tusi
///   foregroundColor: Color(0xFF171717),
///   elevation: 0,
///
/// Biri o'zgarsa qolganlari ortda qolardi — masalan savatda matn rangi
/// `Colors.black`, boshqalarida `#171717` edi.
/// └─────────────────────────────────────────────────────────────────┘
class PageAppBar extends StatelessWidget implements PreferredSizeWidget {
  const PageAppBar({
    super.key,
    this.title,
    this.titleWidget,
    this.actions = const [],
    this.leading,
    this.centerTitle = false,
    this.titleSpacing,
    this.leadingWidth,
    this.backgroundColor = Colors.white,
    this.scrolledUnderElevation = 0,
    this.bottom,
  });

  /// Oddiy matnli sarlavha. Murakkabroq bo'lsa [titleWidget] beriladi.
  final String? title;
  final Widget? titleWidget;

  final List<Widget> actions;
  final Widget? leading;
  final bool centerTitle;
  final double? titleSpacing;
  final double? leadingWidth;

  /// Sahifa foni oqdan farq qilsa (masalan checkout `#FAFAFA`) —
  /// panel ham o'sha rangda bo'lishi kerak, aks holda chegara ko'rinadi.
  final Color backgroundColor;

  /// Ro'yxat panel ostiga kirganda paydo bo'ladigan soya.
  final double scrolledUnderElevation;

  final PreferredSizeWidget? bottom;

  @override
  Size get preferredSize => Size.fromHeight(
      kToolbarHeight + (bottom?.preferredSize.height ?? 0));

  @override
  Widget build(BuildContext context) {
    return AppBar(
      backgroundColor: backgroundColor,
      surfaceTintColor: Colors.transparent,
      foregroundColor: const Color(0xFF171717),
      elevation: 0,
      scrolledUnderElevation: scrolledUnderElevation,
      centerTitle: centerTitle,
      titleSpacing: titleSpacing,
      leadingWidth: leadingWidth,
      leading: leading,
      title: titleWidget ??
          (title == null
              ? null
              : Text(title!,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 20, fontWeight: FontWeight.bold))),
      actions: actions,
      bottom: bottom,
    );
  }
}
