import 'package:flutter/material.dart';

import '../widgets/ondexmap_surface.dart';

/// OnDexMap muharriri — mahalla va ko'cha nomlarini kiritish.
///
/// ┌─ ARXITEKTURA ──────────────────────────────────────────────────────┐
/// OnDexMap — ALOHIDA loyiha (`F:\OnDexMap`), alohida repo, alohida
/// baza (PostGIS 5433) va alohida server. ChustApp uning bazasiga ham,
/// kodiga ham TEGMAYDI.
///
/// Bu sahifa faqat OYNA: OnDexMap'ning o'z muharririni panel ichida
/// ko'rsatadi. Shu sabab OnDexMap o'zgarganda bu fayl o'zgarmaydi.
///
/// Bu bo'lim ChustApp'ning boshqa bo'limlariga hech qanday ta'sir
/// qilmaydi: yangi so'rov yubormaydi, mavjud holatga tegmaydi va
/// `adminLive` soketidan foydalanmaydi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA 127.0.0.1 ───────────────────────────────────────────────────┐
/// OnDexMap'ning admin serveri YOZISH huquqiga ega, shuning uchun u
/// ataylab faqat lokal manzilga bog'langan va internetga chiqarilmaydi.
/// Ommaviy OnDexMap API'si (`:8090`) esa faqat O'QIY oladi.
///
/// Manzilni o'zgartirish kerak bo'lsa (masalan boshqa port):
///     flutter run --dart-define=ONDEXMAP_ADMIN_URL=http://127.0.0.1:9000
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapPage extends StatelessWidget {
  const OndexMapPage({super.key});

  /// Muharrir manzili. Sir emas — shunchaki lokal URL.
  static const url = String.fromEnvironment(
    'ONDEXMAP_ADMIN_URL',
    defaultValue: 'http://127.0.0.1:8091',
  );

  @override
  Widget build(BuildContext context) {
    return const OndexMapSurface(url: url);
  }
}
