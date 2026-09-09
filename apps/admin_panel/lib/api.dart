import 'package:ondex_core/ondex_core.dart';

// Umumiy yadro qayta eksport qilinadi - sahifalar api.dart ni import
// qilgani uchun formatSum, ApiException va boshqalarga o'zgarishsiz
// kirishda davom etadi. Ular endi 4 ta ilovada takrorlanmaydi.
export 'package:ondex_core/ondex_core.dart';



/// Backend manzili — BUILD vaqtida `--dart-define-from-file` orqali keladi,
/// bu yerga qattiq yozilmaydi (`config/dev.json`, `config/dev-tunnel.json`,
/// `config/prod.json`). Sozlama berilmasa `http://localhost:8080`.
const baseUrl = apiBaseUrl;

/// Admin paneli endpointlari.
///
/// Transport (`send`, timeout, tarmoq xatosi, 401, JSON guard) va
/// umumiy endpointlar (`telegramStart`/`verify`/`me`/`wsTicket`/
/// `mapsApiKey`/`reverseGeocode`/`uploadImage`) `ondex_core.ApiClient`
/// da. Ilgari bu yerda ULARNING NUSXASI turardi va u yomonroq edi:
/// timeout yo'q, tarmoq xatosi ushlanmaydi, nginx HTML javobi
/// `FormatException` bilan yiqiladi, 401 ishlanmaydi.
class AdminApi extends ApiClient {
  AdminApi() : super(baseUrl: apiBaseUrl);

  Future<Map<String, dynamic>> stats() async =>
      Map<String, dynamic>.from(await send('GET', '/admin/stats'));

  Future<List<dynamic>> orders() async =>
      (await send('GET', '/admin/orders')) as List<dynamic>? ?? [];

  Future<List<dynamic>> couriers() async =>
      (await send('GET', '/admin/couriers')) as List<dynamic>? ?? [];

  /// Rol bo'yicha akkauntlar: `entity_id` -> {userId, phone, name}.
  ///
  /// PostHog seans yozuviga havola qurish uchun kerak (userId).
  /// Telefon raqam ham — admin panel restoranlar jadvalida
  /// ko'rsatish uchun. Ism — "mas'ul shaxs" ko'rsatish uchun.
  ///
  /// Backend `GET /admin/accounts` har bir yozuvda `entity_id`,
  /// `user_id`, `name`, `phone` ni hammaisini qaytaradi.
  Future<Map<String, ({String userId, String phone, String name})>>
      accountsByEntity(String role) async {
    final list =
        (await send('GET', '/admin/accounts?role=$role')) as List<dynamic>? ??
            [];
    final map = <String, ({String userId, String phone, String name})>{};
    for (final row in list) {
      if (row is! Map) continue;
      final e = (row['entity_id'] as String?) ?? '';
      final u = (row['user_id'] as String?) ?? '';
      final p = (row['phone'] as String?) ?? '';
      final n = (row['name'] as String?) ?? '';
      if (e.isNotEmpty && u.isNotEmpty) {
        map.putIfAbsent(e, () => (userId: u, phone: p, name: n));
      }
    }
    return map;
  }

  Future<void> approveCourier(String id, bool approved) =>
      send('POST', '/admin/couriers/$id/approve', {'approved': approved});

  /// Mijozlar ro'yxati: `{'count': int, 'items': [...]}`.
  ///
  /// Har bir yozuvda `devices` — foydalanuvchi qaysi ilovadan
  /// kirgani (`tma`, `android`, `ios`, `web`, ...) va ilova versiyasi.
  /// Server buni har so'rovdagi `X-Ondex-Client` sarlavhasidan yig'adi.
  Future<Map<String, dynamic>> customers() async =>
      Map<String, dynamic>.from(await send('GET', '/admin/customers'));

  /// Affitsiantlar ro'yxati — mijozlar bilan bir xil shaklda, ustiga
  /// `restaurant_id`/`restaurant_name` qo'shiladi.
  Future<Map<String, dynamic>> waiters() async =>
      Map<String, dynamic>.from(await send('GET', '/admin/waiters'));

  /// Akkauntni o'chiradi.
  ///
  /// Server tomonda bu bitta amalda uch ishni bajaradi: sessiyani bekor
  /// qiladi (qo'lidagi token darhol yaroqsiz bo'ladi), ochiq ilovaga
  /// WebSocket orqali `account_deleted` yuboradi va shaxsiy
  /// ma'lumotlarni o'chiradi. Yakunlanmagan buyurtmasi bor akkaunt
  /// uchun 409 qaytadi — bu XATO EMAS, ataylab qo'yilgan to'siq.
  Future<void> deleteUser(String id) => send('DELETE', '/admin/users/$id');

  // ── Kafe kutubxonasi ────────────────────────────────────────────
  //
  // Kitob 3D maketdagi javondan olib o'qiladi. Sotilmaydi va savatga
  // tushmaydi - narx maydoni umuman yo'q, shuning uchun bu yo'nalishda
  // moliyaviy xavf ham yo'q.

  /// Restoran kitoblari. Matn QAYTMAYDI - ro'yxat uchun kerak emas
  /// va javobni o'nlab barobar shishirardi.
  Future<List<dynamic>> books(String restaurantId) async =>
      (await send('GET', '/restaurants/$restaurantId/books'))
          as List<dynamic>? ?? [];

  /// Boshqaruv ro'yxati - O'CHIRILGAN kitoblar ham qaytadi.
  ///
  /// Panel ilgari ochiq ro'yxatni (`books`) ishlatardi va u o'chirilgan
  /// kitobni yashirardi: "faol" tugmasi o'chirilgan zahoti kitob
  /// ro'yxatdan yo'qolib, uni qayta yoqib bo'lmay qolardi.
  Future<List<dynamic>> booksAll(String restaurantId) async =>
      (await send('GET', '/restaurants/$restaurantId/books/all'))
          as List<dynamic>? ?? [];

  /// Bitta kitob, matni bilan.
  Future<Map<String, dynamic>> book(String id) async =>
      Map<String, dynamic>.from(await send('GET', '/books/$id') as Map);

  Future<Map<String, dynamic>> createBook({
    required String restaurantId,
    required String title,
    String author = '',
    String coverUrl = '',
    String pdfUrl = '',
    String text = '',
  }) async =>
      Map<String, dynamic>.from(
          await send('POST', '/restaurants/$restaurantId/books', {
        'title': title,
        'author': author,
        'cover_url': coverUrl,
        'pdf_url': pdfUrl,
        'text': text,
      }) as Map);

  /// Kitob muqovasini yuklaydi va manzilini qaytaradi.
  ///
  /// Server rasmni tik (2:3) muqova nisbatiga kesib, WebP ga o'giradi -
  /// javondagi muqovalar orasida bo'sh oq chiziqlar qolmasligi uchun.
  Future<String> uploadBookCover(List<int> bytes, String filename) async {
    final data = await sendMultipart(
        'POST', '/uploads/book-cover', bytes, filename);
    return (data is Map ? data['url'] as String? : null) ?? '';
  }

  /// Kitob PDF ini yuklaydi.
  ///
  /// Javobda R2 dagi manzil BILAN BIRGA ajratilgan matn keladi: maketdagi
  /// o'quvchi PDF ni ocha olmaydi, shuning uchun matn serverda bir marta
  /// ajratiladi. `warning` - skanerlangan (matnsiz) kitob belgisi.
  Future<Map<String, dynamic>> uploadBookPdf(
      List<int> bytes, String filename) async {
    final data =
        await sendMultipart('POST', '/uploads/book-pdf', bytes, filename);
    return data is Map ? Map<String, dynamic>.from(data) : <String, dynamic>{};
  }

  /// Kitobni yangilaydi.
  ///
  /// FAQAT berilgan maydonlar yuboriladi: server yuborilmagan
  /// maydonga tegmaydi. Hammasini yuborish yuborilmaganini
  /// o'chirib yuborardi.
  Future<void> updateBook(
    String id, {
    String? title,
    String? author,
    String? coverUrl,
    String? pdfUrl,
    String? text,
    bool? active,
  }) =>
      send('PATCH', '/books/$id', {
        if (title != null) 'title': title,
        if (author != null) 'author': author,
        if (coverUrl != null) 'cover_url': coverUrl,
        if (pdfUrl != null) 'pdf_url': pdfUrl,
        if (text != null) 'text': text,
        if (active != null) 'active': active,
      });

  Future<void> deleteBook(String id) => send('DELETE', '/books/$id');
  Future<List<dynamic>> restaurants() async =>
      (await send('GET', '/restaurants')) as List<dynamic>? ?? [];

  Future<Map<String, dynamic>> createRestaurant({
    required String name,
    required String address,
    required String phone,
    required String staffName,
    double lat = 41.0030,
    double lng = 71.2360,
  }) async =>
      Map<String, dynamic>.from(await send('POST', '/admin/restaurants', {
        'name': name,
        'address': address,
        'phone': phone,
        'staff_name': staffName,
        'lat': lat,
        'lng': lng,
      }));

  Future<void> setRestaurantOpen(String id, bool open) =>
      send('POST', '/admin/restaurants/$id/open', {'open': open});

  Future<void> deleteRestaurant(String id) =>
      send('DELETE', '/admin/restaurants/$id');

  /// Restoran ma'lumotlarini tahrirlaydi. Har doim TO'LIQ holatni
  /// yuboradi (edit oynasi mavjud qiymatlar bilan oldindan to'ldirilgan).
  ///
  /// ┌─ REYTING/ETA PARAMETRLARI NEGA `required` ────────────────────────┐
  /// Server bu maydonlarni SHARTSIZ o'zlashtiradi. Ya'ni ular JSON'da
  /// bo'lmasa Go nol qiymat oladi va mavjud reyting/vaqt JIMGINA
  /// NOLLANADI — tahrir oynasida faqat nomni o'zgartirgan admin
  /// buni sezmasdi. `required` shu xatoni kompilyatsiya vaqtida
  /// to'xtatadi (standart qiymat qo'yilsa, kelajakdagi chaqiruvchi
  /// uni jimgina o'tkazib yuborardi).
  /// └───────────────────────────────────────────────────────────────────┘
  Future<Map<String, dynamic>> editRestaurant({
    required String id,
    required String name,
    required String address,
    required double lat,
    required double lng,
    required double rating,
    required int ratingCount,
    required int etaMinMinutes,
    required int etaMaxMinutes,
    String logoUrl = '',
    String coverUrl = '',
    String tags = '',
  }) async =>
      Map<String, dynamic>.from(await send('POST', '/admin/restaurants/$id', {
        'name': name,
        'address': address,
        'lat': lat,
        'lng': lng,
        'logo_url': logoUrl,
        'cover_url': coverUrl,
        'tags': tags,
        'rating': rating,
        'rating_count': ratingCount,
        'eta_min_minutes': etaMinMinutes,
        'eta_max_minutes': etaMaxMinutes,
      }));


}

final api = AdminApi();

/// Sessiya tokeni SAQLANADIGAN joy.
///
/// ┌─ TUZATILGAN NOSOZLIK (bug.md 2-band) ──────────────────────────────┐
/// Panel tokenni AVVAL to'g'ridan-to'g'ri `SharedPreferences` ga
/// yozardi — shifrlanmagan faylga. Windows'da bu `%APPDATA%` ichidagi
/// oddiy JSON: foydalanuvchi nomidan ishlayotgan istalgan jarayon,
/// zaxira nusxa yoki fayl menejeri uni o'qiy oladi.
///
/// Bu eng imtiyozli kalit edi: admin tokeni butun platformaga kirish
/// beradi (restoran yaratish/o'chirish, kuryer tasdiqlash, mijozlar
/// ro'yxati) va 30 kun amal qiladi. Mijoz, kuryer va affitsiant
/// ilovalari esa allaqachon shifrlangan omborni ishlatardi.
///
/// `TokenStore` `ondex_core` da va u `flutter_secure_storage` ga
/// bog'liq — ya'ni paket panelga TRANZITIV ravishda allaqachon
/// yetib kelgan, `pubspec.yaml` ga hech narsa qo'shish shart emas.
///
/// Eski `SharedPreferences` yozuvi birinchi o'qishda avtomatik
/// ko'chiriladi va shifrlanmagan nusxa O'CHIRILADI — foydalanuvchi
/// qayta login qilmaydi (`TokenStore.read()` izohiga qarang).
/// └────────────────────────────────────────────────────────────────────┘
const adminTokenStore = TokenStore('admin_token');

/// Rasm manzilini ko'rsatishga tayyorlaydi.
///
/// Mantiq `ondex_core` da (`coreFullImageUrl`) — bu yerda faqat shu
/// ilovaning `baseUrl` i bog'lanadi. Ilgari bu funksiya to'rt ilovada
/// qo'lda takrorlangan edi.
String imageUrl(String? path) => coreFullImageUrl(path, baseUrl);
