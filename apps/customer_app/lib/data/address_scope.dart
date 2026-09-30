import 'package:flutter/foundation.dart';

import '../api.dart';
import 'catalog_repository.dart';

/// JORIY YETKAZISH MANZILI — butun ilova uchun BITTA nusxa.
///
/// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
/// Manzil kamida uch joyda ko'rinadi (bosh sahifa sarlavhasi, restoran
/// sahifasi sarlavhasi, rasmiylashtirish) va endi restoran RO'YXATIGA
/// ham ta'sir qiladi: server ro'yxatni manzil shahri bilan cheklaydi
/// (`?for_me=1`).
///
/// Har ekran manzilni o'zi o'qiganda ular bir-biridan XABARSIZ qoladi:
/// manzil ekranidan qaytilganda bittasi yangilanar, ikkinchisi eski
/// qiymat bilan qolardi, restoran ro'yxati esa faqat ilova qayta
/// ishga tushganda o'zgarardi. Aynan shu xato kuzatilgan.
///
/// Endi manzil BITTA joyda turadi va o'zgarganda hamma kuzatuvchiga
/// xabar beradi.
/// └────────────────────────────────────────────────────────────────────┘
class AddressScope extends ChangeNotifier {
  AddressScope._();

  static final AddressScope instance = AddressScope._();

  double _lat = 0;
  double _lng = 0;
  String _text = '';

  double get lat => _lat;
  double get lng => _lng;

  /// Ko'rsatiladigan manzil matni. Bo'sh — manzil tanlanmagan.
  String get text => _text;

  /// Manzil tanlanganmi. `lat/lng == 0` — tanlanmagan (server ham shu
  /// qoidada: `checkout_screen.dart` ga qarang).
  bool get bor => _lat != 0 || _lng != 0;

  /// Restoranlar keshi kalitiga qo'shiladigan yorliq.
  ///
  /// ┌─ NEGA KALITGA KIRADI ────────────────────────────────────────────┐
  /// Server bitta `GET /restaurants` manziliga MANZILGA QARAB har xil
  /// javob qaytaradi. Kalit `restaurants` bo'lib qolsa, Chust ro'yxati
  /// Toshkentga o'tgandan keyin ham diskdan o'qilaverardi — va bu
  /// chiqib-kirgan boshqa akkauntga ham tegishli (katalog keshi
  /// chiqishda tozalanmaydi, `clearOnLogout` izohiga qarang).
  ///
  /// Uch kasr ≈ 110 metr: shaharlarni ajratishga yetadi va GPS
  /// tebranishidan kalit o'zgarib ketmaydi.
  /// └──────────────────────────────────────────────────────────────────┘
  String get cacheTag => bor
      ? '${_lat.toStringAsFixed(3)},${_lng.toStringAsFixed(3)}'
      : 'yoq';

  /// Serverdan qayta o'qiydi.
  ///
  /// Manzil O'ZGARGAN bo'lsa restoranlar keshini tozalaydi va
  /// kuzatuvchilarni xabardor qiladi. O'zgarmagan bo'lsa hech kim
  /// bezovta qilinmaydi — keraksiz qayta yuklanish bo'lmaydi.
  Future<void> refresh() async {
    try {
      final a = await api.getMyAddress();
      _apply(
        lat: (a['lat'] as num?)?.toDouble() ?? 0,
        lng: (a['lng'] as num?)?.toDouble() ?? 0,
        text: (a['text'] ?? '').toString().trim(),
      );
    } catch (_) {
      // Kirmagan foydalanuvchida endpoint 401 qaytaradi — bu NORMAL,
      // manzil shunchaki yo'q deb qoladi.
    }
  }

  /// Manzil ekrani saqlagandan keyin — serverni kutmasdan darhol.
  void set({required double lat, required double lng, required String text}) =>
      _apply(lat: lat, lng: lng, text: text);

  void _apply({
    required double lat,
    required double lng,
    required String text,
  }) {
    final eskiTag = cacheTag;
    if (lat == _lat && lng == _lng && text == _text) return;
    _lat = lat;
    _lng = lng;
    _text = text;
    // Joy o'zgargan bo'lsa (faqat matn emas) — ro'yxat boshqacha
    // bo'ladi, eski keshni tashlaymiz. Kalit ham o'zgaradi, lekin
    // eski yozuv diskda qolib ketmasin.
    //
    // ⚠️ Xatosi YUTILADI va xabar berish TO'XTAMAYDI: bu shunchaki
    // joy tozalash. Kesh do'koni ishlamasa ham (masalan diskda joy
    // yo'q) manzil o'zgargani ekranlarga yetib borishi SHART — aks
    // holda foydalanuvchi eski shaharning ro'yxatini ko'rib qolardi.
    //
    // `catchError` SHART, oddiy `try` yetmaydi: `invalidate()`
    // `Future` qaytaradi va xato ASINXRON keladi — `try/catch` uni
    // ushlamaydi, u "unhandled future error" bo'lib chiqadi.
    if (cacheTag != eskiTag) {
      Repos.restaurants(tag: eskiTag).invalidate().catchError((Object _) {});
    }
    notifyListeners();
  }
}
