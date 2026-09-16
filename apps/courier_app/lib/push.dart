import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';

import 'api.dart';
import 'offer_alert.dart';

/// Kuryer ilovasi push'i — yangi buyurtma TAKLIFI ilova yopiq bo'lsa ham
/// yetib borishi uchun.
///
/// ┌─ NEGA (2026-09-15) ───────────────────────────────────────────────┐
/// Taklif avval faqat WebSocket orqali kelardi: kuryer ilovani yopsa
/// ("liniyada" bo'lsa ham) taklif hech qayerga yetmasdi. Endi server
/// ilova ulanmaganini ko'rsa FAQAT MA'LUMOT (data-only) push yuboradi
/// (`notify.Live.SendOffer`), ilova esa takrorlanuvchi signal chiqaradi
/// (`offer_alert.dart`).
///
/// Push ichidagi ma'lumotga ISHONILMAYDI: ilova ochilganda taklif SERVERDAN
/// so'raladi — taklif hali ochiqmi, qancha soniya qolgan, faqat server
/// biladi (`onOfferHint`). Push faqat signal va matn beradi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Firebase sozlanmagan bo'lsa (`google-services.json` yo'q) xato yutiladi:
/// ilova push'siz ishlaydi.

/// Push `data` si bo'yicha signalni chiqaradi yoki o'chiradi. Fon izolyatida
/// (ilova yopiq) ham, asosiy izolyatda ham ishlaydi.
Future<void> handleCourierPushData(Map<String, dynamic> data) async {
  switch (data['kind']) {
    case 'courier_offer':
      final alert = offerAlertFromData(data, DateTime.now());
      if (alert == null) return;
      await showOfferAlert(
        orderId: alert.orderId,
        title: alert.title,
        body: alert.body,
        timeout: alert.timeout,
      );
    case 'courier_offer_cancelled':
      final orderId = data['order_id'];
      if (orderId is String && orderId.isNotEmpty) await cancelOfferAlert(orderId);
  }
}

/// Fon ishlovchisi TOP-LEVEL bo'lishi SHART; `vm:entry-point` release'da
/// majburiy (aks holda tree shaking olib tashlaydi).
@pragma('vm:entry-point')
Future<void> _onBackgroundMessage(RemoteMessage message) async {
  await Firebase.initializeApp();
  await handleCourierPushData(message.data);
}

/// Ilova yopiq paytda keladigan push uchun fon ishlovchisi — `main()` da,
/// `runApp` dan OLDIN.
///
/// Avval ishlovchi faqat kuryer ekrani server javoblarini olib bo'lgandan
/// keyin ulanardi. FlutterFire uni `runApp` dan oldin ulashni talab qiladi:
/// aks holda ilova o'rnatilgan/yangilangandan keyin ekran to'liq
/// yuklanmay yopilsa, data-only taklif push'i Dart tomonga yetmasligi mumkin.
Future<void> initCourierPushBackground() async {
  try {
    await Firebase.initializeApp();
    FirebaseMessaging.onBackgroundMessage(_onBackgroundMessage);
  } catch (e) {
    debugPrint('[push] fon ishlovchisi ulanmadi (Firebase sozlanmaganmi?): $e');
  }
}

/// Saqlangan token shuncha vaqtda bir qayta yoziladi: server bazasi
/// tiklangan yoki token o'chirilgan bo'lsa ham o'z-o'zidan tiklansin.
const _tokenResaveAfter = Duration(hours: 6);

String? _token;
DateTime? _tokenSavedAt;
Future<bool>? _setup;
StreamSubscription<String>? _refreshSub;
StreamSubscription<RemoteMessage>? _messageSub;
StreamSubscription<RemoteMessage>? _openedSub;

String get _platform => defaultTargetPlatform.name.toLowerCase();

/// Push'ni ishga tushiradi va qurilma tokenini kuryer akkauntiga bog'laydi.
///
/// Qayta chaqirish xavfsiz (ilova oldinga chiqqanda): tinglovchilar BIR
/// MARTA ulanadi, token esa saqlanmay qolgan bo'lsa (tarmoq yo'q edi) qayta
/// yoziladi.
///
/// [onOfferHint] — ilova jarayoni tirik paytda taklif push'i kelganda:
/// ilova taklifni serverdan tiklaydi va o'zi hal qiladi — ekranda ko'rsatish
/// yoki (fonda bo'lsa) takrorlanuvchi signal.
Future<void> registerCourierPush({required void Function() onOfferHint}) async {
  final setup = _setup ??= _setUp(onOfferHint);
  if (!await setup) {
    // Keyingi chaqiruvda qaytadan uriniladi.
    if (identical(_setup, setup)) _setup = null;
    return;
  }
  await _saveToken();
}

Future<bool> _setUp(void Function() onOfferHint) async {
  try {
    await Firebase.initializeApp();
    final messaging = FirebaseMessaging.instance;
    // Android 13+ da ruxsat so'raladi. Rad etilsa ham token saqlanadi —
    // signal ko'rinmaydi, lekin ilova ochilganda taklif tiklanadi.
    await messaging.requestPermission(alert: true, sound: true, badge: false);
    await initOfferAlerts();

    FirebaseMessaging.onBackgroundMessage(_onBackgroundMessage);
    _messageSub = FirebaseMessaging.onMessage.listen((m) async {
      // Bekor qilish — signal shu zahoti o'chsin; taklif — ilova o'zi
      // (holatiga qarab) ko'rsatadi, ikki marta jiringlamasin.
      if (m.data['kind'] == 'courier_offer_cancelled') await handleCourierPushData(m.data);
      onOfferHint();
    });
    _openedSub = FirebaseMessaging.onMessageOpenedApp.listen((_) => onOfferHint());
    if (await messaging.getInitialMessage() != null) onOfferHint();

    // Token vaqti-vaqti bilan yangilanadi — yangisi yozilmasa push eski
    // tokenga ketib, hech qayerga yetmasdi.
    _refreshSub = messaging.onTokenRefresh.listen((t) async {
      if (api.token == null) return;
      try {
        await api.savePushToken(t, _platform);
        _token = t;
        _tokenSavedAt = DateTime.now();
      } catch (_) {}
    });
    return true;
  } catch (e) {
    await _cancelSubscriptions();
    debugPrint('[push] kuryer push\'i ishga tushmadi (Firebase sozlanmaganmi?): $e');
    return false;
  }
}

/// Tokenni serverga yozadi. Xato yutiladi — keyingi chaqiruvda qayta uriniladi.
Future<void> _saveToken() async {
  if (api.token == null) return;
  try {
    final token = await FirebaseMessaging.instance.getToken();
    if (token == null || token.isEmpty) return;
    final savedAt = _tokenSavedAt;
    if (token == _token && savedAt != null && DateTime.now().difference(savedAt) < _tokenResaveAfter) {
      return;
    }
    await api.savePushToken(token, _platform);
    _token = token;
    _tokenSavedAt = DateTime.now();
  } catch (e) {
    debugPrint('[push] token saqlanmadi — ilova oldinga chiqqanda qayta uriniladi: $e');
  }
}

Future<void> _cancelSubscriptions() async {
  await _refreshSub?.cancel();
  await _messageSub?.cancel();
  await _openedSub?.cancel();
  _refreshSub = null;
  _messageSub = null;
  _openedSub = null;
}

/// Chiqishda — `api.logout()` dan OLDIN (so'rov hali amaldagi token bilan
/// ketadi). Token o'chirilmasa chiqib ketgan kuryerning telefoniga keyingi
/// kuryerning takliflari kelaverardi.
Future<void> unregisterCourierPush() async {
  await _cancelSubscriptions();
  _setup = null;
  _tokenSavedAt = null;
  final t = _token;
  _token = null;
  try {
    if (t != null) await api.deletePushToken(t);
  } catch (_) {}
  try {
    if (Firebase.apps.isNotEmpty) await FirebaseMessaging.instance.deleteToken();
  } catch (_) {}
}
