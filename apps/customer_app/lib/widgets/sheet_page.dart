import 'package:flutter/material.dart';

import 'page_sheet.dart';

/// Pastdan suzib chiquvchi sahifa — ochilishi, yopilishi va PASTGA
/// TORTIB yopish imo-ishorasi bilan.
///
/// ┌─ NEGA UMUMIY ─────────────────────────────────────────────────────┐
/// Menyu, savat, rasmiylashtirish va bildirishnomalar — hammasi bir xil
/// ishlashi kerak: pastdan chiqadi, orqaga tugmasi bilan ham, pastga
/// tortib ham yopiladi. Bu mantiq har ekranda qayta yozilsa, biri
/// o'zgarib qolib ular bir-biridan farq qila boshlardi.
///
/// Shuning uchun ikkita narsa shu yerda:
///   [sheetRoute] — marshrut (animatsiya va shaffoflik);
///   [SheetPage]  — sahifa qobig'i (tortish + dumaloq karta).
/// └───────────────────────────────────────────────────────────────────┘

/// Sahifani pastdan suzib chiqaradigan marshrut.
///
/// `opaque: false` — ostidagi sahifa CHIZILAVERADI. Busiz sahifani
/// pastga tortganda ochilgan joyda qop-qora oyna ko'rinardi va nima
/// ustida turganini bilib bo'lmasdi.
Route<T> sheetRoute<T>(Widget page) {
  return PageRouteBuilder<T>(
    pageBuilder: (_, __, ___) => page,
    opaque: false,
    // Sekin va sezilarli: 320 ms da harakat "chaqnab" o'tib ketardi.
    transitionDuration: const Duration(milliseconds: 520),
    reverseTransitionDuration: const Duration(milliseconds: 440),
    transitionsBuilder: (_, animation, __, child) {
      final curved = CurvedAnimation(
        parent: animation,
        curve: Curves.easeOutCubic,
        reverseCurve: Curves.easeInCubic,
      );
      return SlideTransition(
        position: Tween<Offset>(begin: const Offset(0, 1), end: Offset.zero)
            .animate(curved),
        child: child,
      );
    },
  );
}

/// Sahifa qobig'i: dumaloq yuqori burchaklar + pastga tortib yopish.
///
/// ┌─ TORTISH QAYERDAN BOSHLANADI ─────────────────────────────────────┐
/// `deferToChild` — imo-ishora avval BOLAGA taklif qilinadi. Ro'yxat
/// ustida boshlangan tortish skroll (yoki "tortib yangilash") bo'lib
/// qolaveradi; qotirilgan sarlavha ustida boshlangani esa bu yerga
/// keladi, chunki u aylanmaydi.
///
/// MUHIM: sarlavha AYLANUVCHI ro'yxatning ichida bo'lmasligi kerak
/// (`SliverAppBar` emas). Aks holda tortishni skroll o'zi olib qo'yadi —
/// bu jonli sinovda tasdiqlangan.
/// └───────────────────────────────────────────────────────────────────┘
class SheetPage extends StatefulWidget {
  const SheetPage({super.key, required this.child});

  final Widget child;

  @override
  State<SheetPage> createState() => _SheetPageState();
}

class _SheetPageState extends State<SheetPage> {
  /// Barmoq bilan surilgan masofa (piksel). Faqat pastga.
  double _dy = 0;
  bool _dragging = false;

  /// Shu masofadan oshsa — sahifa yopiladi.
  static const _closeDistance = 110.0;

  /// Yoki shu tezlikdan oshib "otib yuborilsa" (piksel/soniya).
  static const _closeVelocity = 700.0;

  void _onUpdate(DragUpdateDetails d) {
    _dragging = true;
    final next =
        (_dy + d.delta.dy).clamp(0.0, MediaQuery.of(context).size.height);
    if (next != _dy) setState(() => _dy = next);
  }

  void _onEnd(DragEndDetails d) {
    _dragging = false;
    final fast = d.velocity.pixelsPerSecond.dy > _closeVelocity;
    if (_dy > _closeDistance || fast) {
      // `maybePop` — sahifa allaqachon yopilayotgan bo'lsa ikkinchi
      // marta yopilib ketmasin.
      Navigator.of(context).maybePop();
    } else {
      setState(() => _dy = 0);
    }
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.deferToChild,
      onVerticalDragUpdate: _onUpdate,
      onVerticalDragEnd: _onEnd,
      child: AnimatedSlide(
        // Tortish paytida animatsiya YO'Q: barmoq bilan bir xil
        // tezlikda yurishi kerak. Qo'yib yuborilgach — joyiga yumshoq
        // qaytadi.
        duration: _dragging ? Duration.zero : const Duration(milliseconds: 240),
        curve: Curves.easeOutCubic,
        offset: Offset(0, _dy / MediaQuery.of(context).size.height),
        child: PageSheet(
          // Joyida turganda tizim paneli chizig'i QORA (belgilar oq).
          // Tortish boshlanishi bilan SHAFFOF — aks holda o'sha qora
          // chiziq sahifa bilan birga pastga tushib, ekran o'rtasida
          // tasma bo'lib qolardi.
          backdrop: _dy > 0 ? Colors.transparent : Colors.black,
          child: widget.child,
        ),
      ),
    );
  }
}
