/// Google Maps JS kutubxonasini yuklaydigan `ensureGoogleMapsLoaded()`
/// funksiyasi — FAQAT veb'da kerak.
///
/// MUHIM: bu shunchaki `kIsWeb` bilan tekshirilmaydi — `package:web`
/// (`dart:js_interop`) web bo'lmagan nishonlar uchun KOMPILYATSIYA vaqtida
/// xato beradi, runtime tekshiruvi buni oldini ololmaydi. Shuning uchun
/// haqiqiy veb kodi `maps_loader_web.dart`ga chiqarilgan va bu yerda faqat
/// conditional export orqali ulanadi — desktop build paytida `package:web`ga
/// umuman murojaat bo'lmaydi, faqat `maps_loader_stub.dart` compile qilinadi.
///
/// Bu naqsh `customer_app` va `courier_app` da allaqachon ishlatilgan; bu
/// yerga ko'chirilmagani uchun `flutter build windows` yuzlab
/// "Dart library 'dart:js_interop' is not available on this platform"
/// xatosi bilan yiqilardi.
library;

export 'maps_loader_stub.dart' if (dart.library.html) 'maps_loader_web.dart';
