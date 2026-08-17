/// Muhit sozlamalari — BUILD vaqtida beriladi, kodga yozilmaydi.
///
/// Avval har bir ilovada manzil `const baseUrl = 'http://192.168.x.x:8080'`
/// shaklida QATTIQ yozilgan edi. Oqibatlari:
///   * Wi-Fi IP o'zgarganda 4 ta faylni qo'lda tahrirlash kerak edi;
///   * release build'ga LAN IP va cleartext HTTP tushib ketardi;
///   * to'rtta ilovada IP uch xil bo'lib qolgan holat kuzatilgan.
///
/// Endi:
///   flutter run --dart-define=ONDEX_API_URL=http://192.168.45.231:8080 \
///               --dart-define=ONDEX_WEB_URL=http://192.168.45.231:3000
///
/// TAVSIYA ETILADIGAN LOKAL YO'L — Cloudflare tunnel
/// (`scripts/dev_tunnel.ps1`), LAN IP o'rniga:
///
///   flutter run --dart-define=ONDEX_API_URL=https://dev-api-ondex.shoxpro.uz \
///               --dart-define=ONDEX_WEB_URL=https://dev-web-ondex.shoxpro.uz
///
/// Nima uchun afzal:
///   * Wi-Fi IP o'zgarishi (kuniga ikki marta bo'lardi) endi ta'sir
///     qilmaydi — qiymat barqaror;
///   * HTTPS bo'lgani uchun Android cleartext bloki umuman paydo
///     bo'lmaydi (`network_security_config.xml` ga tegish shart emas);
///   * telefon kompyuter bilan bir xil Wi-Fi'da bo'lishi shart emas —
///     mobil internet ham ishlaydi.
///
/// Production'da CI shu qiymatlarni domen bilan beradi.
library;

import 'package:flutter/foundation.dart';

/// Go backend manzili.
const String apiBaseUrl = String.fromEnvironment(
  'ONDEX_API_URL',
  defaultValue: 'http://localhost:8080',
);

/// Next.js mini-app'lari manzili (WebView shu yerdan yuklanadi).
const String webAppUrl = String.fromEnvironment(
  'ONDEX_WEB_URL',
  defaultValue: 'http://localhost:3000',
);

/// Ilova versiyasi — `--dart-define=ONDEX_APP_VERSION=1.4.0`.
///
/// NEGA `package_info_plus` EMAS: u yangi platforma plagini, ya'ni
/// to'rtala ilovaga qo'shimcha bog'liqlik va Android/iOS/Windows
/// tomonida qo'shimcha sozlash demak — bir satr matn olish uchun juda
/// qimmat. CI build buyrug'ida bitta `--dart-define` yetadi; berilmasa
/// `dev` qoladi va superadmin panelida aynan shunday ko'rinadi
/// (ya'ni "qo'lda yig'ilgan build" darhol ajralib turadi).
const String appVersion = String.fromEnvironment(
  'ONDEX_APP_VERSION',
  defaultValue: 'dev',
);

/// Mijoz platformasi — `X-Ondex-Client` sarlavhasi uchun.
///
/// Qiymatlar serverdagi YOPIQ ro'yxatga mos bo'lishi shart
/// (`internal/users/device.go`): notanish qiymat u yerda `unknown`
/// ga aylantiriladi.
///
/// DIQQAT: bu yerda `tma` (Telegram Mini App) YO'Q va bo'lishi ham
/// mumkin emas — TMA bu Flutter ilova emas, `apps/web` sahifasi.
/// U o'z sarlavhasini o'zi qo'yadi (`apps/web/lib/api.ts`).
String get clientPlatform {
  // `kIsWeb` AVVAL tekshiriladi: brauzerda `defaultTargetPlatform`
  // baribir biror qiymat qaytaradi (masalan Windows'dagi Chrome uchun
  // `windows`) va usiz Flutter Web paneli "desktop ilova" bo'lib
  // ko'rinardi.
  if (kIsWeb) return 'web';
  return switch (defaultTargetPlatform) {
    TargetPlatform.android => 'android',
    TargetPlatform.iOS => 'ios',
    TargetPlatform.windows => 'windows',
    TargetPlatform.macOS => 'macos',
    TargetPlatform.linux => 'linux',
    _ => 'unknown',
  };
}

/// `X-Ondex-Client` sarlavhasining qiymati: `<platforma>/<versiya>`.
///
/// Server buni superadmin panelidagi "Qurilma" ustuni uchun yozib
/// boradi. Hech qanday huquq shu qiymatga bog'lanmaydi (mijoz uni
/// o'zgartirishi mumkin), shuning uchun u faqat ma'lumot.
String get clientHeaderValue => '$clientPlatform/$appVersion';

/// WebSocket manzili — `apiBaseUrl` dan olinadi.
///
/// `replaceFirst('http', 'ws')` ATAYLAB: `https://` dan `wss://` chiqadi
/// (`https` -> `wss` + `s` qoldig'i emas, chunki almashtiriladigan qism
/// aynan boshidagi `http`). Bu production HTTPS'da to'g'ri ishlaydi.
String wsUrl(String ticket, {String? restaurantId, String? orderId}) {
  var url = '${apiBaseUrl.replaceFirst('http', 'ws')}/ws?ticket=$ticket';
  if (restaurantId != null) url += '&restaurant_id=$restaurantId';
  if (orderId != null) url += '&order_id=$orderId';
  return url;
}
