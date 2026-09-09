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

/// Stol nomini ko'rsatishga tayyorlaydi: "5" → "5-stol", "Stol-1" →
/// "Stol-1" (o'zgarishsiz), bo'sh → "Stol".
///
/// ┌─ NEGA SHART TEKSHIRILADI ─────────────────────────────────────────┐
/// Restoran stol nomini ixtiyoriy yozadi: "5", "Stol-1", "VIP zal",
/// "Teras 3". Qo'shimchani SO'ZSIZ ulash ("$label-stol") "Stol-1-stol"
/// va "VIP zal-stol" kabi yozuvlar berardi — restoran panelida aynan shu
/// xato chiqqan edi.
///
/// Shuning uchun "-stol" FAQAT nom yalang'och raqam bo'lganda
/// qo'shiladi; qolgan hamma holatda restoran yozgan nom o'zgarishsiz
/// ko'rsatiladi.
///
/// Bu yerda (`ondex_core`) turishining sababi — bir xil stol nomi
/// affitsiant ilovasida ham, restoran panelida ham KO'RINADI va ikki
/// joyda ikki xil yozilmasligi kerak.
/// └───────────────────────────────────────────────────────────────────┘
String tableText(String label) {
  if (label.isEmpty) return 'Stol';
  if (RegExp(r'^\d+$').hasMatch(label)) return '$label-stol';
  final zoned = RegExp(r'^(.*) · (\d+)$').firstMatch(label);
  if (zoned != null) return '${zoned.group(1)} · ${zoned.group(2)}-stol';
  return label;
}
