// DEV vositasi: faqat «OnDexMap» bo'limini ChustApp'ga kirmasdan ochadi
// (ko'rinishni tekshirish uchun). Production build'ga KIRMAYDI (`lib/` da emas).
//
//     flutter run -d windows -t tool/ondexmap_preview.dart
import 'package:chust_admin/pages/ondexmap_page.dart';
import 'package:flutter/material.dart';

void main() {
  runApp(MaterialApp(
    debugShowCheckedModeBanner: false,
    theme: ThemeData(
      colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)),
      useMaterial3: true,
    ),
    home: const Scaffold(body: OndexMapPage()),
  ));
}
