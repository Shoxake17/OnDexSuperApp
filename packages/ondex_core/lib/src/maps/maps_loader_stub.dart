/// Veb BO'LMAGAN nishonlar uchun stub.
///
/// Android/iOS'da `google_maps_flutter` xaritani native SDK bilan
/// chizadi, Windows'da esa xarita WebView ichida ochiladi — ikkalasida
/// ham JS skriptini yuklashning hojati yo'q.
///
/// [keyProvider] ATAYLAB chaqirilmaydi: kalit uchun serverga behuda
/// so'rov ketmasligi kerak.
Future<void> ensureGoogleMapsLoaded(
    Future<String> Function() keyProvider) async {}
