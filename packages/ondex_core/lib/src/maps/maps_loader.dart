/// Google Maps JS kutubxonasini yuklaydigan `ensureGoogleMapsLoaded()` —
/// FAQAT veb'da kerak (Android/iOS va Windows'da xarita native SDK yoki
/// WebView orqali chiziladi, JS skripti umuman kerak emas).
///
/// ┌─ NEGA SHARTLI EKSPORT, `kIsWeb` EMAS ─────────────────────────────┐
/// `package:web` (`dart:js_interop`) veb BO'LMAGAN nishonlar uchun
/// KOMPILYATSIYA vaqtida xato beradi (`toJS` kengaytmasi mavjud emas).
/// Runtime tekshiruvi buni to'sa olmaydi — shuning uchun haqiqiy veb
/// kodi alohida faylga chiqarilgan va bu yerda faqat shartli eksport
/// bilan ulanadi. Mobil/desktop build'da bo'sh stub kompilyatsiya
/// qilinadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Ilgari bu uchlik (`maps_loader` + stub + web) UCHTA ilovada —
/// mijoz, kuryer va admin panelida — baytma-bayt takrorlangan edi.
library;

export 'maps_loader_stub.dart'
    if (dart.library.js_interop) 'maps_loader_web.dart';
