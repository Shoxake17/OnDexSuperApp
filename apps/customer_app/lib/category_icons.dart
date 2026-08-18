/// Turkum nomidan rasm assetini topadi.
///
/// ┌─ NEGA ALOHIDA FAYL ───────────────────────────────────────────────┐
/// `apps/web/lib/categoryIcons.ts` aynan shu faylga havola qiladi va
/// "bir xil xarita va normalizatsiya mantig'i" deb yozadi — lekin fayl
/// YO'Q edi: native ko'chirishda jadval `catalog_screen.dart` ichiga
/// ko'chirilgan va normalizatsiya YO'QOLGAN.
///
/// Natija qurilmada ko'rindi: "Burgerlar", "Steyklar", "Salatlar",
/// "Desertlar", "Gazaklar" turkumlarida rasm umuman chizilmasdi.
/// Sabab — jadvalda `burger`, `steyk`, `salat` bor, kelayotgan nom esa
/// KO'PLIK shaklda va qat'iy solishtiruv uni topa olmasdi. Vebda esa
/// qism-satr mosligi bor va o'sha turkumlar to'g'ri ko'rinardi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Bu — STEKLAR ORASIDAGI dublikat (Dart va TypeScript), ruxsat
/// etilgan. Ikkalasi o'zgarganda BIRGA o'zgarishi shart.
library;

/// Turkum nomi -> asset fayli.
///
/// Kalitlar `apps/web/lib/categoryIcons.ts` bilan bir xil tartibda:
/// qism-satr qidiruvi tartibga sezgir, shuning uchun ikkala tomon bir
/// xil turkumga bir xil rasmni bog'lashi kerak.
const _categoryIcons = <String, String>{
  'burger': 'burger',
  'kfc': 'kfc',
  'pizza': 'pizza',
  'lavash': 'lavash',
  'sushi': 'sushi',
  'kabob': 'kabob',
  'somsa': 'somsa',
  'lag\'mon': 'lag\'mon',
  'lagmon': 'lag\'mon',
  'hotdog': 'hotdog',
  'steyk': 'steyk',
  'sandvich': 'sandvich',
  'desert': 'dessert',
  'pishiriq': 'pishiriq',
  // `nonushta` ATAYLAB yo'q: veb jadvalida bor, lekin Flutter
  // `assets/categories/` ichida bunday fayl YO'Q. Mavjud bo'lmagan
  // assetga havola qilish `errorBuilder` ni ishga tushirib, baribir
  // zaxira belgiga tushardi — ya'ni foyda bermaydi.
  'bolalar': 'bolalar',
  'salat': 'salat',
  'milliy taomlar': 'milliy',
  'turk taomlari': 'turkcha',
  'yevropa taomlar': 'yevropa',
  'yapon taomlari': 'yapon',
  'italyan taomlari': 'italya',
  'fast food': 'fastfood',
  'ichimlik': 'ichimlik',
  'shirinlik': 'shirinlik',
  'norin': 'norin',
  'gazaklar': 'gazak',
  'quyuq ovqatlar': 'quyuq-ovqatlar',
  'suyuq ovqatlar': 'suyuq-ovqat',
};

/// Faqat harf va raqamni qoldiradi.
///
/// Shu tufayli `Fast Food`, `fast-food` va `FASTFOOD` bir xil kalitga
/// aylanadi; `lag'mon` dagi apostrof ham yo'qoladi. Backend'dagi
/// `catalog.NormalizeForSearch` bilan mos.
String _normalize(String s) {
  final b = StringBuffer();
  for (final ch in s.toLowerCase().codeUnits) {
    final isDigit = ch >= 0x30 && ch <= 0x39;
    final isLower = ch >= 0x61 && ch <= 0x7A;
    if (isDigit || isLower) b.writeCharCode(ch);
  }
  return b.toString();
}

final _normalized = <String, String>{
  for (final e in _categoryIcons.entries) _normalize(e.key): e.value,
};

/// Turkum nomiga mos asset yo'li. Topilmasa `null` — chaqiruvchi
/// zaxira belgi chizadi.
///
/// Avval ANIQ moslik, keyin qism-satr: "Burgerlar" -> `burger`,
/// "Suyuq ovqatlar" -> `suyuqovqatlar`. Ko'plik qo'shimchalari
/// (`-lar`, `-lari`) aynan shu bosqichda hal bo'ladi.
String? categoryIconFor(String rawLabel) {
  final key = _normalize(rawLabel);
  if (key.isEmpty) return null;

  final exact = _normalized[key];
  if (exact != null) return 'assets/categories/$exact.png';

  for (final e in _normalized.entries) {
    if (key.contains(e.key) || e.key.contains(key)) {
      return 'assets/categories/${e.value}.png';
    }
  }
  return null;
}
