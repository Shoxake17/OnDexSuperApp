import 'package:webview_flutter/webview_flutter.dart';

/// Chiqishda (logout) WebView sessiyasini tozalaydi.
///
/// ┌─ NEGA BU FAYL HALI BOR ───────────────────────────────────────────┐
/// Mobil oqimda WebView UMUMAN ishlatilmaydi (4-bosqich): bosh sahifa,
/// menyu, savat, checkout va buyurtma kuzatuvi — hammasi native.
/// Avvalgi `mini_app_webview.dart` (WebView vidjeti, JS ko'priklari,
/// geolokatsiya kanali, stol seansi uzatish) BUTUNLAY o'chirildi.
///
/// Lekin bu funksiya QOLDI: eski versiyadan yangilanayotgan
/// qurilmalarda WebView `chust_session` cookie'si diskda turgan
/// bo'lishi mumkin va u 30 kun amal qiladi. Chiqishda u tozalanmasa,
/// qurilmani qo'lga kiritgan odam eski sessiyadan foydalanishi mumkin
/// edi.
///
/// Bir necha reliz o'tib, barcha o'rnatmalar yangilangach bu faylni
/// ham, `webview_flutter` bog'liqligini ham olib tashlash mumkin.
/// └───────────────────────────────────────────────────────────────────┘
Future<void> clearMiniAppSession() async {
  try {
    await WebViewCookieManager().clearCookies();
  } catch (_) {
    // Cookie omborini tozalab bo'lmasa ham chiqish DAVOM ETADI —
    // Flutter tomondagi token baribir o'chiriladi.
  }
}
