/// Xarita yuzasi — platformaga qarab ikki xil chiziladi.
///
/// ┌─ NEGA CONDITIONAL EXPORT ──────────────────────────────────────────┐
/// `google_maps_flutter` faqat android/ios/web platformalarini e'lon
/// qiladi (paket `pubspec.yaml` idagi `platforms:` bloki). Windows
/// desktopda u ishlamaydi va xarita o'rniga
///     "TargetPlatform.windows is not yet supported by the maps plugin"
/// yozuvi chiqadi — tugmalar ko'rinadi (ular Flutter vidjetlari), lekin
/// xarita maydoni bo'sh qoladi.
///
/// Bu `kIsWeb` bilan hal bo'lmaydi: `webview_windows` esa web'da
/// KOMPILYATSIYA bo'lmaydi. Shuning uchun ikkala implementatsiya
/// alohida fayllarda va faqat kerakligi compile qilinadi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// Ikkalasi ham bir xil shartnomani bajaradi: `MapSurface` vidjeti,
/// bosilgan nuqtani `onPick(lat, lng)` orqali qaytaradi.
///
/// Teskari geokodlash ikkala yo'lda ham BACKEND orqali
/// (`/geocode/reverse`) — Google Geocoding API brauzerdan chaqirilganda
/// CORS bilan bloklanadi va kalit frontendga chiqib ketardi.
library;

export 'map_surface_webview.dart'
    if (dart.library.html) 'map_surface_gmaps.dart';
