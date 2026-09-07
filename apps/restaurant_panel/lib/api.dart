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


/// Restoran paneli endpointlari.
///
/// Transport (`send`, timeout, tarmoq xatosi, 401, JSON guard) va
/// umumiy endpointlar (`telegramStart`/`verify`/`me`/`wsTicket`/
/// `uploadImage`) `ondex_core.ApiClient` da. Ilgari bu yerda
/// ULARNING NUSXASI turardi va u yomonroq edi: timeout yo'q, tarmoq
/// xatosi ushlanmaydi, nginx HTML javobi `FormatException` bilan
/// yiqiladi, 401 ishlanmaydi, `X-Ondex-Client` sarlavhasi yo'q.
class RestaurantApi extends ApiClient {
  RestaurantApi() : super(baseUrl: apiBaseUrl);

  /// Tokenga bog'langan restoran IDsi (login'da user.entity_id dan
  /// olinadi). Server baribir har so'rovda egalikni o'zi tekshiradi.
  String rid = '';

  Future<Map<String, dynamic>> myRestaurant() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid'));

  Future<void> setOpen(bool open) =>
      send('POST', '/restaurants/$rid/open', {'open': open});

  Future<List<dynamic>> orders() async =>
      (await send('GET', '/restaurants/$rid/orders')) as List<dynamic>? ?? [];

  /// [preparationMinutes] — "accepted"ga o'tishda MAJBURIY: taxminiy
  /// tayyorlash vaqti (daqiqada). Backend shu payt kuryer qidirishni
  /// AVTOMATIK boshlaydi (ETA-asoslangan matching engine) — qo'lda
  /// "Kuryer chaqirish" tugmasi endi shart emas.
  Future<void> transition(String orderId, String to,
          {int? preparationMinutes}) =>
      send('POST', '/orders/$orderId/transition', {
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
      (await send('GET', '/restaurants/$rid/tables')) as List<dynamic>? ?? [];

  Future<Map<String, dynamic>> createTable(String label) async =>
      Map<String, dynamic>.from(
          await send('POST', '/restaurants/$rid/tables', {'label': label}));

  Future<Map<String, dynamic>> renameTable(String tableId, String label) async =>
      Map<String, dynamic>.from(
          await send('PATCH', '/tables/$tableId', {'label': label}));

  Future<Map<String, dynamic>> setTableActive(
          String tableId, bool active) async =>
      Map<String, dynamic>.from(
          await send('PATCH', '/tables/$tableId', {'active': active}));

  // Eslatma: QR tokenni YANGILASH metodi ATAYLAB yo'q. QR kod menyu
  // varaqasiga chop etilgan va stolda abadiy turadi — tokenni
  // almashtirish butun zaldagi varaqalarni qayta chop etishni talab
  // qilardi. Serverda ham bunday endpoint yo'q.

  Future<void> deleteTable(String tableId) =>
      send('DELETE', '/tables/$tableId');

  // ---------- Affitsiantlar ----------

  Future<List<dynamic>> waiters() async =>
      (await send('GET', '/restaurants/$rid/waiters')) as List<dynamic>? ?? [];

  /// Affitsiant akkauntini yaratadi. Parol o'rnatilmaydi — xodim o'z
  /// ilovasida SMS kod bilan kiradi, ya'ni restoran uning parolini
  /// hech qachon bilmaydi.
  Future<Map<String, dynamic>> addWaiter(String phone, String name) async =>
      Map<String, dynamic>.from(await send(
          'POST', '/restaurants/$rid/waiters', {'phone': phone, 'name': name}));

  /// Ishdan bo'shatish: rol `customer` ga qaytariladi va sessiya
  /// darhol bekor qilinadi (akkauntning o'zi o'chirilmaydi).
  Future<void> removeWaiter(String waiterId) =>
      send('DELETE', '/restaurants/$rid/waiters/$waiterId');

  /// Menyu — panel uchun TO'LIQ ro'yxat.
  ///
  /// Ochiq `/menu` emas, avtorizatsiya talab qiladigan `/products`:
  /// restoranning ICHKI maydonlari (ulgurji narx) faqat shu yerda
  /// qaytadi — mijozga ko'rinadigan javobdan ular kesilgan
  /// (`catalog.Product.PublicView`).
  Future<List<dynamic>> menu() async =>
      (await send('GET', '/restaurants/$rid/products')) as List<dynamic>? ??
      [];

  /// Barcha restoranlar uchun umumiy, standart taom turkumlari (backend'da
  /// bitta joyda saqlanadi — mijoz ilovasi ham xuddi shu ro'yxatdan
  /// foydalanadi, ikkalasi orasida yozilishi farq qilib ketmasligi uchun).
  Future<List<dynamic>> categories() async =>
      (await send('GET', '/categories')) as List<dynamic>? ?? [];

  Future<void> deleteProduct(String productId) =>
      send('DELETE', '/restaurants/$rid/products/$productId');

  // ---------- 3D model (AI generatsiyasi) ----------
  //
  // Tripo API kaliti bu yerda YO'Q va bo'lishi ham mumkin emas: panel
  // — ish stoli ilovasi, uning faylidan kalitni ajratib olish oson.
  // Panel faqat o'z serveriga murojaat qiladi, tashqi xizmat bilan
  // server gaplashadi.

  /// Generatsiyani boshlaydi. TEZ qaytadi — model bir necha daqiqada
  /// tayyor bo'ladi va natija WebSocket (`model3d_updated`) orqali
  /// keladi, ya'ni ro'yxatni qayta so'rash shart emas.
  Future<void> generateModel3D(String productId) =>
      send('POST', '/restaurants/$rid/products/$productId/model3d');

  /// Modelni taomdan uzadi (natija sifatsiz chiqqan bo'lsa).
  Future<void> removeModel3D(String productId) =>
      send('DELETE', '/restaurants/$rid/products/$productId/model3d');

  /// Saqlangan mahsulotni QAYTARADI.
  ///
  /// Javob kerak, chunki YANGI mahsulotning ID si faqat serverda
  /// yaratiladi — 3D model generatsiyasini boshlash uchun esa aynan
  /// shu ID kerak bo'ladi.
  Future<Map<String, dynamic>> saveProduct({
    String? id,
    required String name,
    required int priceTiyin,
    required bool available,
    String category = '',
    // Ulgurji narx — ixtiyoriy ICHKI maydon (0 = kiritilmagan), mijozga
    // ko'rsatilmaydi va narxlashga ta'sir qilmaydi.
    //
    // `discount_price_tiyin` ATAYLAB yuborilmaydi: mahsulot darajasidagi
    // chegirma narxi formadan olib tashlandi, chegirmalar endi faqat
    // "Aksiyalar" bo'limi orqali beriladi. Ya'ni mahsulot qayta
    // saqlanganda eski chegirma narxi tozalanadi.
    int wholesalePriceTiyin = 0,
    int stock = 0,
    double weight = 0,
    String weightUnit = 'g',
    String description = '',
    String prepTimeText = '',
    String imageUrl = '',
  }) async =>
      Map<String, dynamic>.from(
          await send('POST', '/restaurants/$rid/products', {
        if (id != null) 'id': id,
        'name': name,
        'price_tiyin': priceTiyin,
        'wholesale_price_tiyin': wholesalePriceTiyin,
        'available': available,
        'category': category,
        'stock': stock,
        'weight': weight,
        'weight_unit': weightUnit,
        'description': description,
        'prep_time_text': prepTimeText,
        'image_url': imageUrl,
      }) as Map);

  Future<List<dynamic>> promotions() async =>
      (await send('GET', '/restaurants/$rid/promotions')) as List<dynamic>? ??
      [];

  Future<void> deletePromotion(String promotionId) =>
      send('DELETE', '/restaurants/$rid/promotions/$promotionId');

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
    final d = await send('POST', '/restaurants/$rid/promotions', {
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

}

final api = RestaurantApi();

/// Sessiya tokeni SAQLANADIGAN joy — shifrlangan ombor.
///
/// Sabab va migratsiya tafsiloti `admin_panel/lib/api.dart` dagi bir
/// xil izohda (bug.md 2-band): panel tokenni avval shifrlanmagan
/// `SharedPreferences` ga yozardi, mijoz/kuryer/affitsiant ilovalari
/// esa allaqachon `TokenStore` ni ishlatardi.
const restTokenStore = TokenStore('rest_token');

/// Server qaytargan rasm manzilini ko'rsatish uchun tayyorlaydi.
/// Ikki holat bor: lokal disk rejimida server nisbiy yo'l qaytaradi
/// ("/uploads/..." — baseUrl qo'shilishi kerak); R2 rejimida esa to'liq,

/// Rasm manzili - ondex_core.coreFullImageUrl ustidagi yupqa o'ram.
String fullImageUrl(String? path) => coreFullImageUrl(path, apiBaseUrl);
