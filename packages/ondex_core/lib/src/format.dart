/// Ko'rsatish uchun formatlash — barcha ilovalar uchun BITTA nusxa.
///
/// Avval `formatSum` BESH joyda (4 Dart + web/lib/format.ts), `fullImageUrl`
/// TO'RT joyda takrorlangan edi. Ular birdek ko'rinsa ham ajralib ketishi
/// oson: bittasida "so'm" qo'shiladi, boshqasida yo'q; bittasi yaxlitlaydi,
/// boshqasi kesadi — va foydalanuvchi ikki ekranda ikki xil summa ko'radi.
library;

/// Tiyinni so'mga aylantirib, minglik ajratgich bilan formatlaydi.
///
/// DIQQAT: so'mdan kichik qoldiq KESILADI (yaxlitlanmaydi) — server ham
/// foizli chegirmani butun bo'lish bilan hisoblaydi
/// (`internal/promotions/apply.go`), shuning uchun ikki taraf bir xil
/// natija beradi.
String formatSum(int tiyin) {
  final sum = tiyin ~/ 100;
  return '${sum.toString().replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (m) => '${m[1]} ')} so\'m';
}

/// Server qaytargan rasm manzilini ko'rsatishga tayyorlaydi.
///
/// Backend ikki xil qiymat qaytaradi: lokal disk rejimida NISBIY yo'l
/// (`/uploads/...`), Cloudflare R2 rejimida esa TO'LIQ URL. Shuning uchun
/// hech qachon `'$baseUrl$path'` deb sodda birlashtirib bo'lmaydi — R2
/// rejimida bu `http://server/https://...` beradi.
/// Nomi ATAYLAB `core` prefiksi bilan: har bir ilova o'z `baseUrl` ini
/// bilgani uchun ustidan bir argumentli `fullImageUrl(path)` o'ramini
/// e'lon qiladi va chaqiruv joylari o'zgarmasdan qoladi.
String coreFullImageUrl(String? path, String baseUrl) {
  if (path == null || path.isEmpty) return '';
  if (path.startsWith('http://') || path.startsWith('https://')) return path;
  return '$baseUrl$path';
}

/// O'lchov birligi — faqat "l" (litr) "L" ga aylantiriladi (sonli "1"
/// bilan chalkashmasligi uchun; SI belgisi ham shunday tavsiya qiladi).
String formatWeightUnit(String unit) => unit == 'l' ? 'L' : unit;
