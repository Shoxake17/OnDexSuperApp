import 'dart:math';

import 'package:ondex_core/ondex_core.dart';

// Umumiy yadro qayta eksport qilinadi — ekranlar faqat `api.dart` ni
// import qiladi (kuryer/mijoz ilovalaridagi bilan bir xil naqsh).
export 'package:ondex_core/ondex_core.dart';

/// Backend manzili — `--dart-define=ONDEX_API_URL=...` orqali beriladi.
const baseUrl = apiBaseUrl;

/// Buyurtma uchun bir martalik tasodifiy kalit (idempotency key).
///
/// Zalda tarmoq zaif: javob kelmay qolsa affitsiant qayta bosadi. Server
/// AYNI kalitni ko'rib, ikkinchi buyurtma yaratmaydi. Shuning uchun kalit
/// savat o'zgarganda yangilanadi, qayta urinishda esa O'ZGARMAYDI.
String newIdempotencyKey() {
  final rnd = Random.secure();
  final bytes = List<int>.generate(16, (_) => rnd.nextInt(256));
  return bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
}

/// Affitsiant ilovasiga XOS endpointlar.
///
/// Umumiy qismi (`send`, xato ishlash, timeout, 401, `requestCode`/
/// `verify`/`me`/`logout`/`wsTicket`) `ondex_core.ApiClient` da.
class WaiterApi extends ApiClient {
  WaiterApi() : super(baseUrl: apiBaseUrl);

  /// Affitsiant qaysi restoranda ishlaydi.
  Future<Map<String, dynamic>> waiterMe() async =>
      Map<String, dynamic>.from(await send('GET', '/waiter/me'));

  /// Restoranning FAOL stol buyurtmalari.
  ///
  /// Server allaqachon filtrlab beradi: faqat `dine_in` va faqat
  /// yakunlanmaganlari. Mijoz telefoni/manzili bu javobda UMUMAN yo'q
  /// (`routes_waiter.go` — eng kam huquq prinsipi).
  Future<List<Map<String, dynamic>>> orders() => _list('/waiter/orders');

  /// Restoranning BARCHA joylari va jonli holati. QR tokenlar bu
  /// javobda yo'q.
  Future<List<Map<String, dynamic>>> tables() => _list('/waiter/tables');

  /// Restoran menyusi (ochiq endpoint — mijoz ilovasi ham shuni ko'radi).
  Future<List<Map<String, dynamic>>> menu(String restaurantId) =>
      _list('/restaurants/${Uri.encodeComponent(restaurantId)}/menu');

  /// Restoranning FAOL aksiyalari (ochiq endpoint) — menyu kartochkasida
  /// mijoz ko'radigan chegirma va lentani ko'rsatish uchun.
  Future<List<Map<String, dynamic>>> activePromotions(String restaurantId) =>
      _list('/restaurants/${Uri.encodeComponent(restaurantId)}/active-promotions');

  /// Stolga buyurtma kiritish. Narxni SERVER hisoblaydi — ilova faqat
  /// taom ID'si va miqdorini yuboradi.
  Future<Map<String, dynamic>> createOrder({
    required String tableId,
    required int partySize,
    required List<Map<String, dynamic>> items,
    required String idempotencyKey,
  }) async =>
      Map<String, dynamic>.from(await send('POST', '/waiter/orders', {
        'table_id': tableId,
        'party_size': partySize,
        'items': items,
        'idempotency_key': idempotencyKey,
      }) as Map);

  /// "Berildi" — taom stolga olib borildi (terminal holat).
  ///
  /// Server tomonda holat mashinasi buni FAQAT `ready` holatidagi
  /// stol buyurtmasi uchun ruxsat beradi.
  Future<void> markServed(String orderId) =>
      send('POST', '/orders/$orderId/transition', {'to': 'served'});

  /// Push tokenini serverga bog'laydi (`routes_notifications.go`).
  Future<void> savePushToken(String token, String platform) =>
      send('POST', '/me/push-token', {'token': token, 'platform': platform});

  /// Chiqishda chaqiriladi: token o'chirilmasa, chiqib ketgan
  /// affitsiantning telefoni keyingi xodimning bildirishnomalarini
  /// olishda davom etardi.
  ///
  /// Token QUERY orqali ketadi — server uni aynan shu yerdan o'qiydi
  /// (`routes_notifications.go`), `ApiClient.send` esa DELETE uchun
  /// tana yubormaydi.
  Future<void> deletePushToken(String token) => send(
        'DELETE',
        '/me/push-token?token=${Uri.encodeQueryComponent(token)}',
      );

  Future<List<Map<String, dynamic>>> _list(String path) async {
    final d = await send('GET', path);
    if (d is! List) return [];
    return d
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList();
  }
}

final api = WaiterApi();
