import 'dart:async';

import 'package:audioplayers/audioplayers.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import 'api.dart';

/// Push bildirishnomalar — affitsiant ilovasining ENG MUHIM qismi.
///
/// ┌─ NEGA JONLI WS YETARLI EMAS ──────────────────────────────────────┐
/// Affitsiant zal bo'ylab yuradi: telefon cho'ntakda, ekran o'chiq,
/// ilova fonda yoki umuman yopiq. Bu holatda WebSocket ulanishini OS
/// uzib qo'yadi (Android Doze, iOS App Nap) va jonli kanal ishlamaydi.
/// Push esa OS darajasida yetkaziladi — ilova o'lik bo'lsa ham keladi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ TUZATILGAN NOSOZLIK: PUSH OVOZSIZ KELARDI ───────────────────────┐
/// Server FCM xabarida Android kanalini ko'rsatmasdi. Ilova fonda yoki
/// yopiq bo'lganda tizim bildirishnomani o'zining STANDART kanalida
/// chiqarardi — u ko'p telefonlarda ovozsiz va vibratsiyasiz. Zalda
/// yurgan affitsiant uni sezmasdi.
///
/// Endi server "taom tayyor" xabarini aniq [kReadyChannelId] kanaliga
/// yuboradi (`internal/notify/fcm_message.go`), kanal esa 5 soniyalik
/// signal (`res/raw/waiter_ready.ogg`) va 5 soniyalik vibratsiya bilan
/// yaratiladi.
///
/// Android 8+ da ovoz va vibratsiya KANALGA tegishli va kanal
/// yaratilgach o'zgartirib bo'lmaydi. Eski kanal ovozsiz yaratilgan
/// edi — shuning uchun yangi identifikator (`_v2`), eskisi o'chiriladi.
/// └───────────────────────────────────────────────────────────────────┘

/// Serverdagi `waiterReadyChannel` bilan AYNAN bir xil bo'lishi SHART.
const kReadyChannelId = 'ondex_waiter_ready_v2';

/// Profil sozlamasida ovoz o'chirilganda ilova ICHIDAGI signal uchun —
/// faqat vibratsiya.
const _quietChannelId = 'ondex_waiter_ready_quiet';

/// Ovozsiz yaratilgan eski kanal — o'chiriladi.
const _legacyChannelId = 'ondex_waiter_ready';

/// Tovush fayli `android/app/src/main/res/raw/` da (kengaytmasiz nom).
const _soundName = 'waiter_ready';

/// ~5 soniyalik vibratsiya: 0.6 s titrash / 0.3 s pauza, besh marta.
final Int64List _vibration =
    Int64List.fromList([0, 600, 300, 600, 300, 600, 300, 600, 300, 600]);

final _readyChannel = AndroidNotificationChannel(
  kReadyChannelId,
  'Tayyor buyurtmalar',
  description: 'Stol buyurtmasi tayyor bo\'lganda — 5 soniyalik signal va vibratsiya',
  importance: Importance.max,
  playSound: true,
  sound: const RawResourceAndroidNotificationSound(_soundName),
  enableVibration: true,
  vibrationPattern: _vibration,
);

final _quietChannel = AndroidNotificationChannel(
  _quietChannelId,
  'Tayyor buyurtmalar (ovozsiz)',
  description: 'Ilovada ovoz o\'chirilganda — faqat vibratsiya',
  importance: Importance.high,
  playSound: false,
  enableVibration: true,
  vibrationPattern: _vibration,
);

final _local = FlutterLocalNotificationsPlugin();
final _player = AudioPlayer();

String? _currentToken;

/// Bildirishnoma plagini tayyor va ruxsat berilganmi. `false` bo'lsa
/// signal ilova ichidagi ovoz va vibratsiya bilan beriladi.
bool _notificationsReady = false;

/// Fon ishlovchisi TOP-LEVEL funksiya BO'LISHI SHART — Android ilovani
/// alohida izolyatda uyg'otadi va sinf metodiga havola qila olmaydi.
///
/// `@pragma('vm:entry-point')` release build'da MAJBURIY: usiz tree
/// shaking bu funksiyani olib tashlaydi va fon xabari ishlanmay
/// qoladi (debug'da ishlaydi, release'da ishlamaydi).
@pragma('vm:entry-point')
Future<void> _onBackgroundMessage(RemoteMessage message) async {
  await Firebase.initializeApp();
  // Fonda hech narsa qilmaymiz: OS bildirishnomani server ko'rsatgan
  // kanalda (ovoz + vibratsiya bilan) o'zi ko'rsatadi.
}

/// Push'ni ishga tushiradi va tokenni serverga bog'laydi.
///
/// Xatolar YUTILADI: push — qo'shimcha kanal. Firebase sozlanmagan
/// qurilmada (yoki emulyatorda Google Play xizmatlarisiz) ilova
/// baribir ishlashi kerak — jonli WS kanali o'z ishini bajaradi.
Future<void> registerPush() async {
  try {
    await Firebase.initializeApp();

    final messaging = FirebaseMessaging.instance;
    // iOS'da ruxsat MAJBURIY so'raladi; Android 13+ da ham.
    final settings = await messaging.requestPermission(
      alert: true,
      badge: true,
      sound: true,
    );
    if (settings.authorizationStatus == AuthorizationStatus.denied) {
      return;
    }

    final android = _local.resolvePlatformSpecificImplementation<
        AndroidFlutterLocalNotificationsPlugin>();
    await android?.createNotificationChannel(_readyChannel);
    await android?.createNotificationChannel(_quietChannel);
    await android?.deleteNotificationChannel(_legacyChannelId);

    await _local.initialize(
      const InitializationSettings(
        android: AndroidInitializationSettings('@mipmap/ic_launcher'),
        iOS: DarwinInitializationSettings(),
      ),
    );
    _notificationsReady = true;

    FirebaseMessaging.onBackgroundMessage(_onBackgroundMessage);

    // ILOVA OCHIQ bo'lganda OS bildirishnoma KO'RSATMAYDI — uni o'zimiz
    // chiqaramiz. Kalit — buyurtma ID'si: jonli kanal (WS) allaqachon
    // chalgan bo'lsa, ikkinchi marta chalinmaydi.
    FirebaseMessaging.onMessage.listen((m) {
      final n = m.notification;
      if (n == null) return;
      final orderId = m.data['order_id'] ?? '';
      alertReady(
        key: orderId.isNotEmpty ? orderId : (m.messageId ?? '${n.hashCode}'),
        title: n.title ?? 'Buyurtma tayyor',
        body: n.body ?? '',
      );
    });

    final token = await messaging.getToken();
    if (token != null) {
      _currentToken = token;
      await api.savePushToken(token, defaultTargetPlatform.name);
    }

    // Token vaqti-vaqti bilan yangilanadi (OS qayta beradi). Yangisi
    // serverga yozilmasa, push ESKI tokenga ketaverib, hech qayerga
    // yetmasdi.
    FirebaseMessaging.instance.onTokenRefresh.listen((t) async {
      _currentToken = t;
      try {
        await api.savePushToken(t, defaultTargetPlatform.name);
      } catch (_) {}
    });
  } catch (_) {
    // Push yo'q — ilova jonli kanal bilan ishlashda davom etadi.
  }
}

/// Chiqishda: token o'chirilmasa, chiqib ketgan affitsiantning telefoni
/// keyingi xodimning bildirishnomalarini olishda davom etardi.
Future<void> unregisterPush() async {
  final t = _currentToken;
  if (t == null) return;
  try {
    await api.deletePushToken(t);
  } catch (_) {}
  _currentToken = null;
}

/// Ovoz yoqilganmi (Profil ekranidagi sozlama).
///
/// Ba'zi zallarda tinchlik talab qilinadi (masalan kechki smena) —
/// o'shanda ilova ichidagi signal faqat vibratsiya bilan beriladi.
bool _soundEnabled = true;

void setPushSoundEnabled(bool value) => _soundEnabled = value;

/// So'nggi signallar — bitta buyurtma uchun ikki kanaldan (WS + push)
/// ikki marta chalinmasin.
final Map<String, DateTime> _recentAlerts = {};

/// "Taom tayyor" signali: 5 soniyalik ovoz va vibratsiya bilan
/// bildirishnoma.
///
/// Ilova ochiq turganda push tizim tomonidan ko'rsatilmaydi, shuning
/// uchun signal shu yerdan chiqariladi — jonli kanal (WS) ham, ochiq
/// ilovaga kelgan push ham shu funksiyani chaqiradi.
Future<void> alertReady({
  required String key,
  required String title,
  required String body,
}) async {
  final now = DateTime.now();
  _recentAlerts.removeWhere((_, at) => now.difference(at) > const Duration(minutes: 10));
  if (_recentAlerts.containsKey(key)) return;
  _recentAlerts[key] = now;

  if (!_notificationsReady) {
    await _fallbackAlert();
    return;
  }
  final channel = _soundEnabled ? _readyChannel : _quietChannel;
  try {
    await _local.show(
      key.hashCode & 0x7fffffff,
      title,
      body,
      NotificationDetails(
        android: AndroidNotificationDetails(
          channel.id,
          channel.name,
          channelDescription: channel.description,
          importance: channel.importance,
          priority: Priority.max,
          playSound: _soundEnabled,
          sound: _soundEnabled ? const RawResourceAndroidNotificationSound(_soundName) : null,
          enableVibration: true,
          vibrationPattern: _vibration,
          category: AndroidNotificationCategory.reminder,
          visibility: NotificationVisibility.public,
        ),
        iOS: DarwinNotificationDetails(
          presentAlert: true,
          presentSound: _soundEnabled,
        ),
      ),
    );
  } catch (_) {
    await _fallbackAlert();
  }
}

/// Bildirishnoma ko'rsatib bo'lmasa (ruxsat berilmagan): ilova ichida
/// ovoz va qisqa vibratsiya.
Future<void> _fallbackAlert() async {
  unawaited(HapticFeedback.heavyImpact());
  await playReadySound();
}

/// Ilova ichidagi "tayyor" ovozi (bildirishnoma ruxsati bo'lmaganda).
/// Xato yutiladi: ovoz chiqmasa ham ro'yxat yangilanadi.
Future<void> playReadySound() async {
  if (!_soundEnabled) return;
  try {
    await _player.play(AssetSource('sound/ready.mp3'));
  } catch (_) {}
}
