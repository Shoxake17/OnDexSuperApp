import 'package:ondex_core/ondex_core.dart';

// Umumiy yadro qayta eksport qilinadi - sahifalar api.dart ni import
// qilgani uchun formatSum, fullImageUrl, ApiException va boshqalarga
// o'zgarishsiz kirishda davom etadi. Endi 4 ta ilovada takrorlanmaydi.
export 'package:ondex_core/ondex_core.dart';

/// Backend manzili — BUILD vaqtida `--dart-define-from-file` orqali keladi,
/// bu yerga qattiq yozilmaydi (`config/dev.json`, `config/dev-tunnel.json`,
/// `config/prod.json`). Sozlama berilmasa `http://localhost:8080`.
///
/// Production: `flutter build windows --release --dart-define-from-file=config/prod.json`
const baseUrl = apiBaseUrl;

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

  /// "Restoran sozlamalari". Nomi, turi, telefoni, manzili — faqat ko'rish
  /// uchun (`locked`): ularni faqat OnDex administratori o'zgartiradi.
  Future<Map<String, dynamic>> restaurantSettings() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/settings') as Map);

  /// Faqat ruxsat etilgan maydonlar: `logo_url`, `cover_url`, `description`,
  /// `working_hours` (null — cheklanmagan), `payment_methods`. Boshqa maydon
  /// yuborilsa server 403/400 qaytaradi.
  Future<Map<String, dynamic>> updateRestaurantSettings(Map<String, Object?> patch) async =>
      Map<String, dynamic>.from(
          await send('PATCH', '/restaurants/$rid/settings', patch) as Map);

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

  /// "Kuryer topilmadi" holatida kuryer qidiruvini qayta boshlaydi (yangi
  /// 10 daqiqa). Bekor qilish — oddiy `transition(id, 'cancelled')`:
  /// server uni tayyor buyurtmada FAQAT shu holatda ruxsat etadi.
  Future<void> retryCourierSearch(String orderId) =>
      send('POST', '/orders/$orderId/dispatch/retry', <String, Object?>{});

  // ---------- Stollar (QR kod) ----------
  //
  // Javobdagi `qr_token` — SIR. U faqat shu endpointlarda beriladi
  // (restoran egasiga) va QR kod chizish uchun kerak. Mijoz
  // ilovasidagi proksi bu yo'llarni UMUMAN o'tkazmaydi
  // (`apps/web/app/api/proxy` — ruxsat etilgan ro'yxat).

  Future<List<dynamic>> tables() async =>
      (await send('GET', '/restaurants/$rid/tables')) as List<dynamic>? ?? [];

  /// Bitta joy. [kind] — `table` | `cabin` | `vip_room` | `tapchan` |
  /// `bar_counter` | `lounge` | `banquet_hall` (`pages/tables/table_models.dart`).
  Future<Map<String, dynamic>> createTable({
    required String label,
    required String zone,
    required String kind,
    int? capacity,
  }) async =>
      Map<String, dynamic>.from(await send('POST', '/restaurants/$rid/tables', {
        'label': label,
        'zone': zone,
        'kind': kind,
        if (capacity != null) 'capacity': capacity,
      }));

  /// Bir nechta ketma-ket raqamli joy birdaniga (hammasi yoki hech biri).
  Future<List<dynamic>> createTablesBatch({
    required String zone,
    required String kind,
    required String prefix,
    required int from,
    required int count,
    int? capacity,
  }) async =>
      (await send('POST', '/restaurants/$rid/tables/batch', {
        'zone': zone,
        'kind': kind,
        'prefix': prefix,
        'from': from,
        'count': count,
        if (capacity != null) 'capacity': capacity,
      })) as List<dynamic>? ??
      [];

  /// Qisman tahrir: `label`, `zone`, `kind`, `capacity` (null — olib
  /// tashlash), `active`, `cleaning`. Server hammasini BITTA yozuvda saqlaydi.
  Future<Map<String, dynamic>> updateTable(String tableId, Map<String, Object?> patch) async =>
      Map<String, dynamic>.from(await send('PATCH', '/tables/$tableId', patch));

  // Eslatma: QR tokenni YANGILASH metodi ATAYLAB yo'q. QR kod menyu
  // varaqasiga chop etilgan va stolda abadiy turadi — tokenni
  // almashtirish butun zaldagi varaqalarni qayta chop etishni talab
  // qilardi. Serverda ham bunday endpoint yo'q.

  Future<void> deleteTable(String tableId) =>
      send('DELETE', '/tables/$tableId');

  // ---------- Affitsiantlar ----------

  /// "Xodimlar": ro'yxat, kartalar, lavozimlar taqsimoti va so'nggi
  /// faoliyat — bitta javobda.
  Future<Map<String, dynamic>> staffOverview() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/staff') as Map);

  /// Yangi xodim. Ofitsiantga `app_access: true` berilsa "OnDex Affitsiant"
  /// akkaunti ochiladi — parolsiz, xodim SMS/Telegram kod bilan kiradi.
  Future<Map<String, dynamic>> createStaff(Map<String, dynamic> body) async =>
      Map<String, dynamic>.from(await send('POST', '/restaurants/$rid/staff', body) as Map);

  /// Faqat o'zgargan maydonlar (`schedule: null` — jadvalni olib tashlash).
  Future<Map<String, dynamic>> updateStaff(String id, Map<String, dynamic> patch) async =>
      Map<String, dynamic>.from(await send('PATCH', '/restaurants/$rid/staff/$id', patch) as Map);

  /// `active` / `on_leave` / `dismissed`. Ta'til va ishdan bo'shatishda
  /// ilovaga kirish serverda DARHOL yopiladi; yozuv o'chirilmaydi.
  Future<Map<String, dynamic>> setStaffStatus(String id, String status) async =>
      Map<String, dynamic>.from(
          await send('POST', '/restaurants/$rid/staff/$id/status', {'status': status}) as Map);

  /// "Bildirishnomalar" ro'yxati. `after` — jonli kanal uzilganda
  /// o'tkazib yuborilganlar (o'sish tartibida), `before` — sahifalash.
  Future<Map<String, dynamic>> notifications({
    String category = '',
    String period = '',
    String query = '',
    int? before,
    int? after,
    int limit = 30,
  }) async {
    final params = <String, String>{
      if (category.isNotEmpty) 'category': category,
      if (period.isNotEmpty) 'period': period,
      if (query.trim().isNotEmpty) 'q': query.trim(),
      if (before != null && before > 0) 'before': '$before',
      if (after != null && after > 0) 'after': '$after',
      'limit': '$limit',
    };
    final qs = Uri(queryParameters: params).query;
    return Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/notifications?$qs') as Map);
  }

  /// O'qilmaganlar soni va oxirgi tartib raqami (qo'ng'iroq belgisi).
  Future<Map<String, dynamic>> notificationsSummary() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/notifications/summary') as Map);

  Future<Map<String, dynamic>> markNotificationRead(String id) async => Map<String, dynamic>.from(
      await send('POST', '/restaurants/$rid/notifications/${Uri.encodeComponent(id)}/read') as Map);

  /// Faqat `upToSeq` gacha — bosish paytida kelgan yangi xabar o'qilmagan qoladi.
  Future<Map<String, dynamic>> markAllNotificationsRead(int upToSeq) async => Map<String, dynamic>.from(
      await send('POST', '/restaurants/$rid/notifications/read-all', {'up_to_seq': upToSeq}) as Map);

  Future<List<dynamic>> staffActivity(String id) async {
    final res = await send('GET', '/restaurants/$rid/staff/$id/activity');
    return (res is Map ? res['items'] as List<dynamic>? : null) ?? const [];
  }

  /// Oylik hisobot (`month` — "2026-09"): har bir xodimning ish kunlari,
  /// soatlari, ta'tili va hisoblangan maoshi — ish jadvali va holatlar
  /// tarixi bo'yicha.
  Future<Map<String, dynamic>> staffReport(String month) async => Map<String, dynamic>.from(
      await send('GET', '/restaurants/$rid/staff/report?month=${Uri.encodeQueryComponent(month)}') as Map);

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

  /// "Statistika" sahifasi — butun hisob-kitob SERVERDA
  /// (`internal/stats`).
  ///
  /// NEGA `orders()` dan hisoblanmaydi: u eng so'nggi 100 ta buyurtmani
  /// qaytaradi, ya'ni bir oylik statistika jimgina kesilgan ro'yxatdan
  /// chiqib, yolg'on raqam ko'rsatardi.
  ///
  /// [from] va [to] — kalendar kunlari (ikkalasi ham davrga kiradi),
  /// [granularity] — `day` | `week` | `month`.
  Future<Map<String, dynamic>> stats({
    required DateTime from,
    required DateTime to,
    String granularity = 'day',
  }) async =>
      Map<String, dynamic>.from(await send(
          'GET',
          '/restaurants/$rid/stats'
              '?from=${_apiDate(from)}&to=${_apiDate(to)}'
              '&granularity=${Uri.encodeQueryComponent(granularity)}') as Map);

  /// "Barcha buyurtmalar" — eng yangisidan, sahifalab. [status] — `all` |
  /// `in_progress` | `completed` | `cancelled`, [cursor] — oldingi
  /// javobning `next_cursor` i. [from]/[to] — kalendar kunlari (ikkalasi
  /// ham kiradi); berilmasa restoran ochilgandan beri.
  ///
  /// Birinchi sahifada (kursorsiz) javobga shu davr xulosasi (`summary`)
  /// ham qo'shiladi.
  Future<Map<String, dynamic>> orderHistory({
    String status = 'all',
    String? cursor,
    int limit = 30,
    DateTime? from,
    DateTime? to,
  }) async =>
      Map<String, dynamic>.from(await send(
          'GET',
          '/restaurants/$rid/orders/history'
              '?status=${Uri.encodeQueryComponent(status)}&limit=$limit'
              '${from == null || to == null ? '' : '&from=${_apiDate(from)}&to=${_apiDate(to)}'}'
              '${cursor == null || cursor.isEmpty ? '' : '&cursor=${Uri.encodeQueryComponent(cursor)}'}') as Map);

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

  // ---------- Qo'llab-quvvatlash: Yordam markazi va Chat markazi ----------

  /// OnDex admini kiritgan aloqa ma'lumotlari (barcha panellarda bir xil).
  Future<Map<String, dynamic>> supportContacts() async =>
      Map<String, dynamic>.from(await send('GET', '/support/contacts') as Map);

  /// Chat xabarlari (eskisidan yangisiga) + suhbat xulosasi. [before] —
  /// eskiroq sahifa, [after] — uzilishdan keyingi to'ldirish.
  Future<Map<String, dynamic>> supportMessages({int? before, int? after, int limit = 50}) async {
    final q = [
      'limit=$limit',
      if (before != null && before > 0) 'before=$before',
      if (after != null && after > 0) 'after=$after',
    ].join('&');
    return Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/support/messages?$q') as Map);
  }

  Future<Map<String, dynamic>> supportSummary() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/support/summary') as Map);

  /// [clientId] — qayta urinishda AYNI qiymat: server ikkinchi xabar yaratmaydi.
  Future<Map<String, dynamic>> sendSupportMessage(String body, String clientId) async =>
      Map<String, dynamic>.from(await send('POST', '/restaurants/$rid/support/messages',
          {'body': body, 'client_id': clientId}) as Map);

  Future<Map<String, dynamic>> markSupportRead(int upToSeq) async =>
      Map<String, dynamic>.from(
          await send('POST', '/restaurants/$rid/support/read', {'up_to_seq': upToSeq}) as Map);

  /// Rasm + ixtiyoriy izoh (multipart). Server rasmni tekshiradi va qayta
  /// kodlaydi; [clientId] qayta urinishda AYNI qiymat.
  Future<Map<String, dynamic>> sendSupportImage(
      String body, String clientId, List<int> bytes, String filename) async {
    final data = await sendMultipart('POST', '/restaurants/$rid/support/messages', bytes, filename,
        fields: {'client_id': clientId, if (body.isNotEmpty) 'body': body},
        errorText: 'Rasm yuborilmadi — internetni tekshiring');
    return data is Map ? Map<String, dynamic>.from(data) : <String, dynamic>{};
  }

  /// Chatdagi rasm manzili — faqat token bilan ochiladi (ommaviy emas).
  String supportAttachmentUrl(String attachmentId) =>
      '$baseUrl/restaurants/$rid/support/attachments/${Uri.encodeComponent(attachmentId)}';
}

/// Sana `YYYY-MM-DD` shaklida (server `internal/stats.ParseQuery` shuni kutadi).
String _apiDate(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}'
    '-${d.day.toString().padLeft(2, '0')}';

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
