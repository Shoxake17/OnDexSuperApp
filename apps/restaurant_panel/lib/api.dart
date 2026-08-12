import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:ondex_core/ondex_core.dart';

// Umumiy yadro qayta eksport qilinadi - sahifalar api.dart ni import
// qilgani uchun formatSum, fullImageUrl, ApiException va boshqalarga
// o'zgarishsiz kirishda davom etadi. Endi 4 ta ilovada takrorlanmaydi.
export 'package:ondex_core/ondex_core.dart';

/// Restoran paneli manzili - ONDEX_API_URL dart-define orqali beriladi.
const baseUrl = apiBaseUrl;

/// GET /ws uchun manzil (bilet bilan).
String wsUrl(String ticket) =>
    '${baseUrl.replaceFirst('http', 'ws')}/ws?ticket=$ticket';


class RestaurantApi {
  String? token;

  /// Tokenga bog'langan restoran IDsi (login'da user.entity_id dan olinadi).
  /// Server baribir har so'rovda egalikni o'zi tekshiradi — bu faqat qulaylik.
  String rid = '';

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (token != null) 'Authorization': 'Bearer $token',
      };

  Future<dynamic> _send(String method, String path,
      [Map<String, dynamic>? body]) async {
    final uri = Uri.parse('$baseUrl$path');
    final http.Response r;
    if (method == 'GET') {
      r = await http.get(uri, headers: _headers);
    } else if (method == 'DELETE') {
      r = await http.delete(uri, headers: _headers);
    } else if (method == 'PATCH') {
      // Stol nomini/holatini qisman yangilash uchun
      // (`PATCH /tables/{id}`). Busiz stollarni tahrirlash POST bilan
      // qilinishi kerak bo'lardi va bu "yaratish" bilan chalkashardi.
      r = await http.patch(uri, headers: _headers, body: jsonEncode(body ?? {}));
    } else {
      r = await http.post(uri, headers: _headers, body: jsonEncode(body ?? {}));
    }
    final data = r.body.isEmpty ? null : jsonDecode(utf8.decode(r.bodyBytes));
    if (r.statusCode >= 400) {
      throw ApiException((data is Map && data['error'] != null)
          ? data['error']
          : 'Server xatosi');
    }
    return data;
  }

  Future<String?> requestCode(String phone) async {
    final d = await _send('POST', '/auth/request-code', {'phone': phone});
    return d['dev_code'] as String?;
  }

  Future<Map<String, dynamic>> verify(String phone, String code) async {
    final d =
        await _send('POST', '/auth/verify', {'phone': phone, 'code': code});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// GET /ws uchun qisqa muddatli, bir martalik ulanish bileti (wsUrl()ga
  /// qarang) — Authorization header orqali (oddiy HTTP, xavfsiz) yuboriladi.
  Future<String> wsTicket() async {
    final d = await _send('POST', '/ws/ticket');
    return d['ticket'] as String;
  }

  Future<Map<String, dynamic>> myRestaurant() async =>
      Map<String, dynamic>.from(await _send('GET', '/restaurants/$rid'));

  /// Login qilgan xodimning o'zi haqida (ism, telefon, rol) — sidebar/AppBar
  /// avatariga chiqariladi.
  Future<Map<String, dynamic>> me() async =>
      Map<String, dynamic>.from(await _send('GET', '/me'));

  Future<void> setOpen(bool open) =>
      _send('POST', '/restaurants/$rid/open', {'open': open});

  Future<List<dynamic>> orders() async =>
      (await _send('GET', '/restaurants/$rid/orders')) as List<dynamic>? ?? [];

  /// [preparationMinutes] — "accepted"ga o'tishda MAJBURIY: taxminiy
  /// tayyorlash vaqti (daqiqada). Backend shu payt kuryer qidirishni
  /// AVTOMATIK boshlaydi (ETA-asoslangan matching engine) — qo'lda
  /// "Kuryer chaqirish" tugmasi endi shart emas.
  Future<void> transition(String orderId, String to,
          {int? preparationMinutes}) =>
      _send('POST', '/orders/$orderId/transition', {
        'to': to,
        if (preparationMinutes != null)
          'preparation_minutes': preparationMinutes,
      });

  // ---------- Stollar (QR kod) ----------
  //
  // Javobdagi `qr_token` — SIR. U faqat shu endpointlarda beriladi
  // (restoran egasiga) va QR kod chizish uchun kerak. Mijoz
  // ilovasidagi proksi bu yo'llarni UMUMAN o'tkazmaydi
  // (`apps/web/app/api/proxy` — ruxsat etilgan ro'yxat).

  Future<List<dynamic>> tables() async =>
      (await _send('GET', '/restaurants/$rid/tables')) as List<dynamic>? ?? [];

  Future<Map<String, dynamic>> createTable(String label) async =>
      Map<String, dynamic>.from(
          await _send('POST', '/restaurants/$rid/tables', {'label': label}));

  Future<Map<String, dynamic>> renameTable(String tableId, String label) async =>
      Map<String, dynamic>.from(
          await _send('PATCH', '/tables/$tableId', {'label': label}));

  Future<Map<String, dynamic>> setTableActive(
          String tableId, bool active) async =>
      Map<String, dynamic>.from(
          await _send('PATCH', '/tables/$tableId', {'active': active}));

  // Eslatma: QR tokenni YANGILASH metodi ATAYLAB yo'q. QR kod menyu
  // varaqasiga chop etilgan va stolda abadiy turadi — tokenni
  // almashtirish butun zaldagi varaqalarni qayta chop etishni talab
  // qilardi. Serverda ham bunday endpoint yo'q.

  Future<void> deleteTable(String tableId) =>
      _send('DELETE', '/tables/$tableId');

  // ---------- Affitsiantlar ----------

  Future<List<dynamic>> waiters() async =>
      (await _send('GET', '/restaurants/$rid/waiters')) as List<dynamic>? ?? [];

  /// Affitsiant akkauntini yaratadi. Parol o'rnatilmaydi — xodim o'z
  /// ilovasida SMS kod bilan kiradi, ya'ni restoran uning parolini
  /// hech qachon bilmaydi.
  Future<Map<String, dynamic>> addWaiter(String phone, String name) async =>
      Map<String, dynamic>.from(await _send(
          'POST', '/restaurants/$rid/waiters', {'phone': phone, 'name': name}));

  /// Ishdan bo'shatish: rol `customer` ga qaytariladi va sessiya
  /// darhol bekor qilinadi (akkauntning o'zi o'chirilmaydi).
  Future<void> removeWaiter(String waiterId) =>
      _send('DELETE', '/restaurants/$rid/waiters/$waiterId');

  Future<List<dynamic>> menu() async =>
      (await _send('GET', '/restaurants/$rid/menu')) as List<dynamic>? ?? [];

  /// Barcha restoranlar uchun umumiy, standart taom turkumlari (backend'da
  /// bitta joyda saqlanadi — mijoz ilovasi ham xuddi shu ro'yxatdan
  /// foydalanadi, ikkalasi orasida yozilishi farq qilib ketmasligi uchun).
  Future<List<dynamic>> categories() async =>
      (await _send('GET', '/categories')) as List<dynamic>? ?? [];

  Future<void> deleteProduct(String productId) =>
      _send('DELETE', '/restaurants/$rid/products/$productId');

  Future<void> saveProduct({
    String? id,
    required String name,
    required int priceTiyin,
    required bool available,
    String category = '',
    int discountPriceTiyin = 0,
    int stock = 0,
    double weight = 0,
    String weightUnit = 'g',
    String description = '',
    String prepTimeText = '',
    String imageUrl = '',
  }) =>
      _send('POST', '/restaurants/$rid/products', {
        if (id != null) 'id': id,
        'name': name,
        'price_tiyin': priceTiyin,
        'discount_price_tiyin': discountPriceTiyin,
        'available': available,
        'category': category,
        'stock': stock,
        'weight': weight,
        'weight_unit': weightUnit,
        'description': description,
        'prep_time_text': prepTimeText,
        'image_url': imageUrl,
      });

  Future<List<dynamic>> promotions() async =>
      (await _send('GET', '/restaurants/$rid/promotions')) as List<dynamic>? ??
      [];

  Future<void> deletePromotion(String promotionId) =>
      _send('DELETE', '/restaurants/$rid/promotions/$promotionId');

  /// [startAt]/[endAt] — RFC3339 (masalan "2026-08-02T10:00:00.000Z"),
  /// backend shu formatni kutadi. [endAt] indefinite=true bo'lsa e'tiborga
  /// olinmaydi, lekin baribir yuborilishi kerak (backend uni o'qimaydi).
  /// Javob {..., "stopped_promotion_names": [...], "adjusted_promotion_names":
  /// [...]} ni o'z ichiga oladi — shu bilan bir mahsulot/turkumga
  /// to'qnashgani uchun backend avtomatik butunlay to'xtatgan (stopped) yoki
  /// qisman moslashtirgan (adjusted — faol qoladi, lekin to'qnashgan
  /// mahsulot/turkumga endi tegishli emas) ESKI aksiyalar nomlari
  /// (promotions_page.dart shu ro'yxatlarni foydalanuvchiga snackbar orqali
  /// ko'rsatadi, aks holda eski aksiya "sababsiz" o'zgargandek chalkash
  /// tuyulardi).
  Future<Map<String, dynamic>> savePromotion({
    String? id,
    required String name,
    String description = '',
    required String type,
    required String discountUnit,
    required int discountValue,
    int minOrderAmountTiyin = 0,
    int maxDiscountAmountTiyin = 0,
    int minPreviousOrders = 0,
    required String startAt,
    String endAt = '',
    bool indefinite = false,
    bool active = true,
    bool appliesToProducts = false,
    bool appliesToOrders = false,
    bool appliesToCategories = false,
    List<String> targetProductIds = const [],
    List<String> targetCategories = const [],
    String imageUrl = '',
  }) async {
    final d = await _send('POST', '/restaurants/$rid/promotions', {
      if (id != null) 'id': id,
      'name': name,
      'description': description,
      'type': type,
      'discount_unit': discountUnit,
      'discount_value': discountValue,
      'min_order_amount_tiyin': minOrderAmountTiyin,
      'max_discount_amount_tiyin': maxDiscountAmountTiyin,
      'min_previous_orders': minPreviousOrders,
      'start_at': startAt,
      'end_at': endAt,
      'indefinite': indefinite,
      'active': active,
      'applies_to_products': appliesToProducts,
      'applies_to_orders': appliesToOrders,
      'applies_to_categories': appliesToCategories,
      'target_product_ids': targetProductIds,
      'target_categories': targetCategories,
      'image_url': imageUrl,
    });
    return d as Map<String, dynamic>;
  }

  /// Mahsulot rasmini yuklaydi, saqlangan faylning yo'lini qaytaradi
  /// (masalan "/uploads/abc123.jpg") — shu yo'l saveProduct'ga beriladi.
  Future<String> uploadImage(Uint8List bytes, String filename) async {
    final req = http.MultipartRequest('POST', Uri.parse('$baseUrl/uploads'))
      ..headers['Authorization'] = 'Bearer $token'
      ..files
          .add(http.MultipartFile.fromBytes('file', bytes, filename: filename));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    final data =
        resp.body.isEmpty ? null : jsonDecode(utf8.decode(resp.bodyBytes));
    if (resp.statusCode >= 400) {
      throw ApiException((data is Map && data['error'] != null)
          ? data['error']
          : 'Rasm yuklashda xato');
    }
    return data['url'] as String;
  }
}

final api = RestaurantApi();

/// Server qaytargan rasm manzilini ko'rsatish uchun tayyorlaydi.
/// Ikki holat bor: lokal disk rejimida server nisbiy yo'l qaytaradi
/// ("/uploads/..." — baseUrl qo'shilishi kerak); R2 rejimida esa to'liq,

/// Rasm manzili - ondex_core.coreFullImageUrl ustidagi yupqa o'ram.
String fullImageUrl(String? path) => coreFullImageUrl(path, apiBaseUrl);
