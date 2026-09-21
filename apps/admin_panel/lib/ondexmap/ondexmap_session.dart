/// OnDexMap lokal sessiyasi — platformaga qarab.
///
/// `dart:io` web'da KOMPILYATSIYA bo'lmaydi, shuning uchun ikki fayl
/// (`map_surface.dart` bilan bir xil naqsh). Ikkalasi ham bir xil
/// shartnomani bajaradi: `OndexMapSession`, `readOndexMapSession`,
/// `isLoopbackHttp`.
library;

export 'ondexmap_session_io.dart'
    if (dart.library.html) 'ondexmap_session_stub.dart';
