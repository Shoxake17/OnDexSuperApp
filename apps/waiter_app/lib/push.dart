import 'dart:async';

import 'package:audioplayers/audioplayers.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import 'api.dart';

/// Push bildirishnomalar — affitsiant ilovasining ENG MUHIM qismi.
///
/// ┌─ NEGA JONLI WS YETARLI EMAS ──────────────────────────────────────┐
/// Affitsiant zal bo'ylab yuradi: telefon cho'ntakda, ekran o'chiq,
/// ilova fonda yoki umuman yopiq. Bu holatda WebSocket ulanishini OS
/// uzib qo'yadi (Android Doze, iOS App Nap) va jonli kanal ishlamaydi.
///
/// Push esa OS darajasida yetkaziladi — ilova o'lik bo'lsa ham xabar
/// keladi. Taom oshxonada sovib qolmasligining yagona kafolati shu.
/// └───────────────────────────────────────────────────────────────────┘

/// Android'da fon xabarlari uchun kanal.
///
/// Kanal ILOVA ISHGA TUSHGANDA yaratilishi kerak: Android 8+ da
/// mavjud bo'lmagan kanalga kelgan bildirishnoma JIMGINA tashlanadi.
const _channel = AndroidNotificationChannel(
  'ondex_waiter_ready',
  'Tayyor buyurtmalar',
  description: 'Stol buyurtmasi tayyor bo\'lganda',
  importance: Importance.max,
  playSound: true,
);

final _local = FlutterLocalNotificationsPlugin();
final _player = AudioPlayer();

String? _currentToken;

/// Fon ishlovchisi TOP-LEVEL funksiya BO'LISHI SHART — Android ilovani
/// alohida izolyatda uyg'otadi va sinf metodiga havola qila olmaydi.
///
/// `@pragma('vm:entry-point')` release build'da MAJBURIY: usiz tree
/// shaking bu funksiyani olib tashlaydi va fon xabari ishlanmay
/// qoladi (debug'da ishlaydi, release'da ishlamaydi — topilishi eng
/// qiyin xatolardan).
@pragma('vm:entry-point')
Future<void> _onBackgroundMessage(RemoteMessage message) async {
  await Firebase.initializeApp();
  // Fonda hech narsa qilmaymiz: OS bildirishnomani o'zi ko'rsatadi.
  // Bu funksiya faqat Firebase'ni tirik ushlab turish uchun kerak.
}

/// Push'ni ishga tushiradi va tokenni serverga bog'laydi.
///
/// Xatolar YUTILADI: push — qo'shimcha qulaylik. Firebase sozlanmagan
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

    await _local
        .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin>()
        ?.createNotificationChannel(_channel);

    await _local.initialize(
      const InitializationSettings(
        android: AndroidInitializationSettings('@mipmap/ic_launcher'),
        iOS: DarwinInitializationSettings(),
      ),
    );

    FirebaseMessaging.onBackgroundMessage(_onBackgroundMessage);

    // ILOVA OCHIQ bo'lganda OS bildirishnoma KO'RSATMAYDI — uni o'zimiz
    // chiqaramiz. Busiz affitsiant boshqa ekranda turganda xabarni
    // umuman ko'rmasdi.
    FirebaseMessaging.onMessage.listen((m) {
      final n = m.notification;
      if (n != null) {
        _local.show(
          n.hashCode,
          n.title,
          n.body,
          NotificationDetails(
            android: AndroidNotificationDetails(
              _channel.id,
              _channel.name,
              importance: Importance.max,
              priority: Priority.high,
            ),
            iOS: const DarwinNotificationDetails(),
          ),
        );
      }
      playReadySound();
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

/// "Tayyor" signali.
///
/// Ilova ochiq turganda push kelmaydi (yuqoridagi `onMessage` ni
/// ko'ring), shuning uchun ovoz — affitsiantning e'tiborini tortadigan
/// yagona vosita. Xato yutiladi: ovoz chiqmasa ham ro'yxat yangilanadi.
Future<void> playReadySound() async {
  try {
    await _player.play(AssetSource('sound/ready.mp3'));
  } catch (_) {}
}
