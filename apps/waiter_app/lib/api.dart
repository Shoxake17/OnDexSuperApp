import 'package:ondex_core/ondex_core.dart';

// Umumiy yadro qayta eksport qilinadi — ekranlar faqat `api.dart` ni
// import qiladi (kuryer/mijoz ilovalaridagi bilan bir xil naqsh).
export 'package:ondex_core/ondex_core.dart';

/// Backend manzili — `--dart-define=ONDEX_API_URL=...` orqali beriladi.
const baseUrl = apiBaseUrl;

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
  Future<List<Map<String, dynamic>>> orders() async {
    final d = await send('GET', '/waiter/orders');
    if (d is! List) return [];
    return d.map((e) => Map<String, dynamic>.from(e as Map)).toList();
  }

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
}

final api = WaiterApi();
