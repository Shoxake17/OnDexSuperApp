import 'dart:typed_data';

import 'package:flutter_local_notifications/flutter_local_notifications.dart';

/// Taklif SIGNALI — ilova yopiq yoki fonda bo'lganda.
///
/// ┌─ NEGA (foydalanuvchi sinovi, 2026-09-15) ─────────────────────────┐
/// Ilova ochiq bo'lsa taklif ovozi (`kuryersound.mp3`) taklif tugaguncha
/// takrorlanadi. Ilova yopiq/fonda bo'lsa esa oddiy push tovushi bir marta
/// qisqa chalinardi — kuryer sezmasdi.
///
/// Endi shu holatda TAKRORLANUVCHI (insistent) bildirishnoma chiqadi: o'sha
/// mp3 kuryer bosguncha, taklif boshqaga ketguncha yoki muddati tugaguncha
/// qayta-qayta chalinadi, qo'ng'iroq ovozi darajasida.
/// └───────────────────────────────────────────────────────────────────┘

/// Kanal. Android 8+ da ovoz va ustuvorlik kanalga tegishli va yaratilgach
/// o'zgarmaydi — shuning uchun yangi identifikator (`_v2`), eskisi o'chiriladi.
const kOfferChannelId = 'ondex_courier_offer_v2';
const _legacyChannelIds = ['ondex_courier_offer_v1'];

/// `android/app/src/main/res/raw/courier_offer.mp3` (kengaytmasiz).
const _offerSound = 'courier_offer';

/// Signal eng ko'pi bilan shuncha chalinadi (server noto'g'ri muddat
/// yuborsa ham telefon cheksiz jiringlamasin).
const kMaxOfferAlert = Duration(seconds: 60);

/// Android `Notification.FLAG_INSISTENT` — ovoz bosilguncha takrorlanadi.
const _flagInsistent = 4;

final _vibration = Int64List.fromList([0, 800, 400, 800, 400, 800]);

final _offerChannel = AndroidNotificationChannel(
  kOfferChannelId,
  'Yangi buyurtmalar',
  description: 'Buyurtma taklifi — ilova yopiq bo\'lsa ham, qabul qilinguncha takrorlanadi',
  importance: Importance.max,
  playSound: true,
  sound: const RawResourceAndroidNotificationSound(_offerSound),
  enableVibration: true,
  vibrationPattern: _vibration,
  // Qo'ng'iroq ovozi darajasi: bildirishnoma ovozi odatda pastroq qo'yiladi.
  audioAttributesUsage: AudioAttributesUsage.notificationRingtone,
);

final _plugin = FlutterLocalNotificationsPlugin();
bool _ready = false;

/// Kanalni yaratadi (va eskisini o'chiradi). Ham asosiy, ham fon (push)
/// izolyatida chaqirilishi mumkin — takroriy chaqiruv xavfsiz.
Future<void> initOfferAlerts() async {
  if (_ready) return;
  await _plugin.initialize(
    const InitializationSettings(android: AndroidInitializationSettings('@mipmap/ic_launcher')),
  );
  final android = _plugin.resolvePlatformSpecificImplementation<AndroidFlutterLocalNotificationsPlugin>();
  await android?.createNotificationChannel(_offerChannel);
  for (final id in _legacyChannelIds) {
    await android?.deleteNotificationChannel(id);
  }
  _ready = true;
}

/// Buyurtma ID'sidan BARQAROR bildirishnoma raqami (FNV-1a, 31 bit).
///
/// `String.hashCode` ishlatilmaydi: u izolyatlar/ishga tushishlar orasida
/// bir xil bo'lishi kafolatlanmagan — fon izolyatida chiqqan signalni asosiy
/// izolyat o'chira olmay qolardi.
int offerNotificationId(String orderId) {
  var hash = 0x811c9dc5;
  for (final unit in orderId.codeUnits) {
    hash ^= unit;
    hash = (hash * 0x01000193) & 0xffffffff;
  }
  return hash & 0x7fffffff;
}

/// Push `data` sidan signal parametrlari. Noto'g'ri yoki eskirgan bo'lsa `null`.
({String orderId, String title, String body, Duration timeout})? offerAlertFromData(
  Map<String, dynamic> data,
  DateTime now,
) {
  if (data['kind'] != 'courier_offer') return null;
  final orderId = data['order_id'];
  if (orderId is! String || orderId.isEmpty) return null;
  final expiresAt = int.tryParse('${data['expires_at'] ?? ''}');
  if (expiresAt == null) return null;
  var timeout = DateTime.fromMillisecondsSinceEpoch(expiresAt).difference(now);
  if (timeout <= const Duration(seconds: 1)) return null;
  if (timeout > kMaxOfferAlert) timeout = kMaxOfferAlert;
  String text(Object? v, String fallback) {
    final s = v is String ? v.trim() : '';
    return s.isEmpty ? fallback : s;
  }

  return (
    orderId: orderId,
    title: text(data['title'], 'Yangi buyurtma'),
    body: text(data['body'], 'Qabul qilish uchun ilovani oching'),
    timeout: timeout,
  );
}

/// Takrorlanuvchi signal. [timeout] o'tgach o'zi yo'qoladi.
Future<void> showOfferAlert({
  required String orderId,
  required String title,
  required String body,
  required Duration timeout,
}) async {
  if (timeout <= Duration.zero) return;
  final limited = timeout > kMaxOfferAlert ? kMaxOfferAlert : timeout;
  try {
    await initOfferAlerts();
    await _plugin.show(
      offerNotificationId(orderId),
      title,
      body,
      NotificationDetails(
        android: AndroidNotificationDetails(
          _offerChannel.id,
          _offerChannel.name,
          channelDescription: _offerChannel.description,
          importance: Importance.max,
          priority: Priority.max,
          category: AndroidNotificationCategory.call,
          // Qulflangan ekranda ham: matnda faqat restoran nomi.
          visibility: NotificationVisibility.public,
          playSound: true,
          sound: const RawResourceAndroidNotificationSound(_offerSound),
          audioAttributesUsage: AudioAttributesUsage.notificationRingtone,
          enableVibration: true,
          vibrationPattern: _vibration,
          additionalFlags: Int32List.fromList([_flagInsistent]),
          timeoutAfter: limited.inMilliseconds,
          autoCancel: true,
        ),
      ),
      payload: orderId,
    );
  } catch (_) {
    // Signal chiqmasa ham taklif ilova ochilganda serverdan tiklanadi.
  }
}

Future<void> cancelOfferAlert(String orderId) async {
  try {
    await initOfferAlerts();
    await _plugin.cancel(offerNotificationId(orderId));
  } catch (_) {}
}
