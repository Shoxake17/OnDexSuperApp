/// OnDexMap muharriri yuzasi — platformaga qarab ikki xil chiziladi.
///
/// `map_surface.dart` bilan AYNAN bir xil naqsh: `webview_windows`
/// web'da KOMPILYATSIYA bo'lmaydi, shuning uchun `kIsWeb` tekshiruvi
/// yetarli emas va ikkala implementatsiya alohida fayllarda turadi.
///
/// Ikkalasi ham bir xil shartnomani bajaradi: `OndexMapSurface`
/// vidjeti, `url` ni ko'rsatadi.
library;

export 'ondexmap_surface_webview.dart'
    if (dart.library.html) 'ondexmap_surface_web.dart';
