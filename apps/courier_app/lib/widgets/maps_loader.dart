/// Google Maps JS kutubxonasini yuklaydigan `ensureGoogleMapsLoaded()`
/// funksiyasi — FAQAT veb'da kerak (Android/iOS'da `google_maps_flutter`
/// xaritani native SDK orqali chizadi, JS skripti umuman kerak emas).
///
/// MUHIM: bu shunchaki `kIsWeb` bilan tekshirilmaydi — `package:web`
/// (`dart:js_interop`) Android/iOS uchun KOMPILYATSIYA vaqtida xato beradi
/// (`toJS`/`jsify` kengaytmalari mobil nishon uchun mavjud emas), runtime
/// tekshiruvi buni oldini ololmaydi. Shuning uchun haqiqiy veb kodi
/// `maps_loader_web.dart`ga chiqarilgan va bu yerda faqat conditional
/// export orqali ulanadi — mobil build paytida `package:web`ga umuman
/// murojaat bo'lmaydi, faqat bo'sh `maps_loader_stub.dart` compile qilinadi.
library;

export 'maps_loader_stub.dart' if (dart.library.html) 'maps_loader_web.dart';
