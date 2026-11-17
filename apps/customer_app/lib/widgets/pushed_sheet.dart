import 'package:flutter/material.dart';

import 'page_sheet.dart';
import 'sheet_page.dart';

/// ALOHIDA OCHILADIGAN (push qilinadigan) sahifa qobig'i — `TrackingScreen`
/// bilan bir xil naqsh: `SheetPage` (pastga tortib yopish) + o'z `Scaffold`i.
///
/// ┌─ NEGA `SheetScaffold` EMAS ───────────────────────────────────────────┐
/// `SheetScaffold` faqat TAB ekranlar uchun: ular ona qobiqning `Scaffold`i
/// (Material) ichida turadi. Push qilingan sahifada Material ajdodi bo'lmaydi
/// va matnlar Flutter'ning "Material yo'q" ko'rinishida chiziladi — QIZIL
/// harflar, SARIQ ikki qavat tagchiziq, monoshrift (bron ekranida shunday
/// bo'ldi, 2026-11-17). Bu qobiq o'z Material qatlamini beradi.
/// └──────────────────────────────────────────────────────────────────────┘
class PushedSheet extends StatelessWidget {
  const PushedSheet({super.key, required this.title, required this.child, this.actions = const []});

  final String title;
  final Widget child;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) => SheetPage(
        child: Scaffold(
          backgroundColor: Colors.white,
          appBar: PageAppBar(title: title, actions: actions),
          body: child,
        ),
      );
}
