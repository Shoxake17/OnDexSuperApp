import 'dart:async';

import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';

import '../api.dart';

/// Push bildirishnomalar (FCM) — qurilma manzilini serverga ro'yxatdan
/// o'tkazish va chiqishda uni O'CHIRISH.
///
/// ┌─ NEGA PUSH KERAK, WEBSOCKET YETMAYDIMI ───────────────────────────┐
/// WebSocket FAQAT ilova ochiq turganda ishlaydi. Foydalanuvchi
/// telefonni cho'ntagiga solganda esa soket o'ladi — "kuryer yo'lga
/// chiqdi" xabari hech qachon ko'rinmasdi. Backend shu sababli
/// bildirishnomani DB'ga yozadi va foydalanuvchi ONLAYN EMASLIGINI
/// ko'rsa push yuboradi (`internal/notify/service.go`).
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ XAVFSIZLIK: TOKEN — QURILMA MANZILI, HISOBGA BOG'LANGAN ─────────┐
/// FCM tokeni foydalanuvchining EMAS, QURILMANING manzili. Bitta
/// telefonda ikki kishi navbat bilan kirsa va token eski egasida
/// qolib ketsa — chiqib ketgan odamning telefoniga yangi egasining
/// buyurtmalari haqidagi xabarlar kelaverardi. Bu ma'lumot sizishi.
///
/// Uch qatlamli himoya:
///   1. Kirishda `POST /me/push-token` — backend tokenni YANGI egasiga
///      o'tkazadi (token PRIMARY KEY, migration 0030);
///   2. Chiqishda `DELETE /me/push-token` — yozuv o'chiriladi;
///   3. Chiqishda `deleteToken()` — token GOOGLE tomonida ham
///      o'ldiriladi. Bu 2-qadam tarmoqsizlik sababli bajarilmay
///      qolsa ham himoya qiladi: eski tokenga yuborilgan har qanday
///      push FCM'dan `UNREGISTERED` qaytaradi va hech qayerga
///      yetmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class PushService {
  PushService._();
  static final instance = PushService._();

  /// Serverga ro'yxatdan o'tkazilgan token. Chiqishda AYNAN SHUNI
  /// o'chirish kerak — orada token yangilangan bo'lsa `getToken()`
  /// boshqasini qaytaradi va serverda eski yozuv qolib ketardi.
  String? _registered;

  StreamSubscription<String>? _refreshSub;
  bool _starting = false;

  /// Sessiya ochilgandan KEYIN chaqiriladi (`HomeShell`).
  ///
  /// Hech qachon otilmaydi: push — qo'shimcha qulaylik, uning
  /// ishlamasligi ilovaning qolgan qismini to'xtatmasligi kerak.
  /// Google Play Services yo'q qurilmalarda `getToken()` xato beradi,
  /// Firebase umuman ishga tushmagan bo'lishi ham mumkin
  /// (`main.dart` uni ataylab yutadi).
  Future<void> start() async {
    if (_starting || _registered != null) return;
    _starting = true;
    try {
      final messaging = FirebaseMessaging.instance;

      // Android 13+ da POST_NOTIFICATIONS — ish vaqtidagi ruxsat.
      // Ataylab SHU YERDA so'raladi (ilova ochilishida emas): ruxsat
      // oynasi foydalanuvchi nima uchun kerakligini tushungan paytda,
      // ya'ni hisobga kirgandan keyin chiqadi. Rad etilsa ham davom
      // etamiz — token baribir olinadi, xabar shunchaki ko'rinmaydi.
      await messaging.requestPermission();

      final token = await messaging.getToken();
      if (token == null || token.isEmpty) return;
      await _sendToServer(token);

      // Token vaqti-vaqti bilan yangilanadi (ilova qayta o'rnatilganda,
      // ma'lumotlar tozalanganda, Google o'zi yangilaganda). Kuzatilmasa
      // push jimgina yetib kelmay qo'yardi.
      _refreshSub?.cancel();
      _refreshSub = messaging.onTokenRefresh.listen(
        (t) async {
          // Sessiya yopilgan bo'lsa ro'yxatdan o'tkazmaymiz — aks holda
          // chiqib ketgan foydalanuvchining tokeni qayta tiklanardi.
          if (api.token == null) return;
          await _sendToServer(t);
        },
        onError: (_) {},
      );
    } catch (e) {
      debugPrint('Push ro\'yxatdan o\'tmadi (bildirishnomalar kelmaydi): $e');
    } finally {
      _starting = false;
    }
  }

  Future<void> _sendToServer(String token) async {
    await api.send('POST', '/me/push-token', {
      'token': token,
      // "android" / "ios" — backend shu satrni o'zgartirmasdan saqlaydi,
      // shuning uchun kichik harfga keltiriladi (`TargetPlatform.iOS.name`
      // aks holda "iOS" bo'lib ketardi).
      'platform': defaultTargetPlatform.name.toLowerCase(),
    });
    _registered = token;
  }

  /// Chiqishdan OLDIN chaqiriladi — `api.logout()` dan ham oldin,
  /// chunki `DELETE` so'rovi hali AMALDAGI token bilan yuboriladi.
  /// Logout barcha sessiyalarni bekor qilgandan keyin bu so'rov 401
  /// olardi va yozuv serverda abadiy qolib ketardi.
  Future<void> stop() async {
    await _refreshSub?.cancel();
    _refreshSub = null;
    final token = _registered;
    _registered = null;
    try {
      if (token != null) {
        await api.send(
            'DELETE', '/me/push-token?token=${Uri.encodeQueryComponent(token)}');
      }
      // Google tomonida ham o'ldiramiz — yuqoridagi 3-qatlam himoya.
      // Yuqoridagi so'rov muvaffaqiyatsiz bo'lsa ham BU BAJARILADI.
      await FirebaseMessaging.instance.deleteToken();
    } catch (e) {
      debugPrint('Push tokenini o\'chirib bo\'lmadi: $e');
    }
  }
}

// ─── NEGA `onMessage` TINGLANMAYDI ───────────────────────────────────
// Ilova OCHIQ bo'lganda Android push bildirishnomasini o'zi
// ko'rsatmaydi — buni ilova qilishi kerak. Lekin biz ATAYLAB
// qilmaymiz: ochiq ilovada bir xil voqea ALLAQACHON WebSocket orqali
// keladi va ekranda jonli yangilanadi. Ikkalasi ham ko'rsatilsa
// foydalanuvchi har bir holat o'zgarishini IKKI MARTA ko'rardi.
//
// Backend push'ni faqat foydalanuvchi onlayn BO'LMAGANDA yuboradi
// (`notify.Service.deliver`), shuning uchun bu holat kamdan-kam
// uchraydi — lekin soket uzilib, backend buni hali bilmagan qisqa
// oraliqda mumkin.
//
// ─── NEGA FON ISHLOVCHISI (background handler) YO'Q ──────────────────
// Backend har bir push'da `notification` blokini yuboradi
// (`internal/notify/fcm.go`), shuning uchun ilova yopiq yoki fonda
// bo'lganda bildirishnomani ANDROID O'ZI ko'rsatadi. Fon ishlovchisi
// faqat `data`-only xabarlar uchun kerak bo'lardi.
