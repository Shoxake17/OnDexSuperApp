import '../api.dart';

/// OnDex POS (do'kon / qurt / dorixona) endpointlari — `routes_shop.go`.
///
/// Do'kon SERVERDA sessiyadan aniqlanadi: URL'da do'kon ID yoki login YO'Q
/// (ShoxPos'dagi `/:login` IDOR sinfi bu yerda umuman mavjud emas).
/// Pul — tiyin (int), miqdor — kasr (kg uchun).
extension ShopApi on RestaurantApi {
  /// `{store_id, category, shop_type, sections}` — sidebar shu bo'yicha.
  Future<Map<String, dynamic>> shopMe() async =>
      Map<String, dynamic>.from(await send('GET', '/merchant/shop/me') as Map);

  // ── Mahsulotlar ──
  Future<List<Map<String, dynamic>>> shopProducts() async =>
      _list(await send('GET', '/merchant/shop/products'));

  Future<List<Map<String, dynamic>>> shopIncoming() async =>
      _list(await send('GET', '/merchant/shop/products/incoming'));

  Future<List<Map<String, dynamic>>> shopSearch(String q, {String category = ''}) async =>
      _list(await send('GET',
          '/merchant/shop/products/search?q=${Uri.encodeQueryComponent(q)}&category=${Uri.encodeQueryComponent(category)}'));

  /// Avval do'kon, keyin global baza. Topilmasa `ApiException` (404).
  Future<Map<String, dynamic>> shopScan({String barcode = '', String name = ''}) async =>
      Map<String, dynamic>.from(await send('GET',
          '/merchant/shop/products/scan?barcode=${Uri.encodeQueryComponent(barcode)}&name=${Uri.encodeQueryComponent(name)}') as Map);

  /// OpenFoodFacts: `{source, name?, barcode}`.
  Future<Map<String, dynamic>> shopLookup(String barcode) async => Map<String, dynamic>.from(
      await send('GET', '/merchant/shop/products/lookup?barcode=${Uri.encodeQueryComponent(barcode)}') as Map);

  Future<int> shopBaseCount() async =>
      ((await send('GET', '/merchant/shop/products/base-count') as Map)['count'] as num?)?.toInt() ?? 0;

  Future<List<String>> shopCategories() async =>
      ((await send('GET', '/merchant/shop/categories')) as List? ?? const []).cast<String>();

  /// Qo'shish yoki tahrirlash. Qurt maydonlari `null` — "o'zgarmaydi".
  Future<Map<String, dynamic>> shopUpsert(
    String barcode, {
    required String name,
    required String category,
    required int priceTiyin,
    required int basePriceTiyin,
    required double quantity,
    int? weightGrams,
    String? description,
    String? composition,
    String? imageUrl,
  }) async =>
      Map<String, dynamic>.from(await send('PUT', '/merchant/shop/products/${Uri.encodeComponent(barcode)}', {
        'name': name,
        'category': category,
        'price_tiyin': priceTiyin,
        'base_price_tiyin': basePriceTiyin,
        'quantity': quantity,
        if (weightGrams != null) 'weight_grams': weightGrams,
        if (description != null) 'description': description,
        if (composition != null) 'composition': composition,
        if (imageUrl != null) 'image_url': imageUrl,
      }) as Map);

  /// [field]: `price` | `base_price` (tiyin) | `quantity`.
  Future<Map<String, dynamic>> shopUpdateField(String barcode, String field, num value) async =>
      Map<String, dynamic>.from(await send('PATCH', '/merchant/shop/products/${Uri.encodeComponent(barcode)}',
          {'field': field, 'value': value}) as Map);

  Future<void> shopDelete(String barcode) =>
      send('DELETE', '/merchant/shop/products/${Uri.encodeComponent(barcode)}');

  // ── Sotuv ──
  /// [items]: `{barcode, name, quantity, price_tiyin, base_price_tiyin, total_tiyin, category}`.
  Future<List<Map<String, dynamic>>> shopSell(List<Map<String, Object?>> items, String paymentMethod) async {
    final r = await send('POST', '/merchant/shop/sales', {'items': items, 'payment_method': paymentMethod});
    return _list((r as Map)['sales']);
  }

  Future<List<Map<String, dynamic>>> shopSales({DateTime? from, DateTime? to}) async =>
      _list(await send('GET', '/merchant/shop/sales${_range(from, to)}'));

  Future<void> shopDeleteSale(String id) => send('DELETE', '/merchant/shop/sales/${Uri.encodeComponent(id)}');

  Future<List<Map<String, dynamic>>> shopSoldLines({DateTime? from, DateTime? to}) async =>
      _list(await send('GET', '/merchant/shop/sold-lines${_range(from, to)}'));

  // ── Qarzlar ──
  Future<List<Map<String, dynamic>>> shopDebts() async => _list(await send('GET', '/merchant/shop/debts'));

  Future<Map<String, dynamic>> shopAddDebt(String name, String phone, int amountTiyin) async =>
      Map<String, dynamic>.from(await send('POST', '/merchant/shop/debts',
          {'name': name, 'phone': phone, 'amount_tiyin': amountTiyin}) as Map);

  /// `{fully_paid, debt}`.
  Future<Map<String, dynamic>> shopPayDebt(String id, int paidTiyin) async => Map<String, dynamic>.from(
      await send('POST', '/merchant/shop/debts/${Uri.encodeComponent(id)}/pay', {'paid_tiyin': paidTiyin}) as Map);

  // ── Dorixona ──
  Future<List<Map<String, dynamic>>> pharmProducts() async => _list(await send('GET', '/merchant/shop/pharm'));

  Future<List<Map<String, dynamic>>> pharmSearch(String q, {String category = ''}) async => _list(await send('GET',
      '/merchant/shop/pharm/search?q=${Uri.encodeQueryComponent(q)}&category=${Uri.encodeQueryComponent(category)}'));

  Future<Map<String, dynamic>> pharmInfo(String q) async => Map<String, dynamic>.from(
      await send('GET', '/merchant/shop/pharm/info?q=${Uri.encodeQueryComponent(q)}') as Map);

  Future<List<Map<String, dynamic>>> pharmBase() async => _list(await send('GET', '/merchant/shop/pharm/base'));

  Future<int> pharmBaseCount() async =>
      ((await send('GET', '/merchant/shop/pharm/base-count') as Map)['count'] as num?)?.toInt() ?? 0;

  Future<Map<String, dynamic>> pharmAddBase(Map<String, Object?> body) async =>
      Map<String, dynamic>.from(await send('POST', '/merchant/shop/pharm/base', body) as Map);

  /// Qayta qo'shilsa qoldiq va kirim oshadi (ShoxPos bilan bir xil).
  Future<Map<String, dynamic>> pharmAdd(Map<String, Object?> body) async =>
      Map<String, dynamic>.from(await send('POST', '/merchant/shop/pharm', body) as Map);

  /// Faqat yuborilgan maydonlar o'zgaradi; `expiry_date: null` — tozalash.
  Future<Map<String, dynamic>> pharmUpdate(String barcode, Map<String, Object?> patch) async =>
      Map<String, dynamic>.from(
          await send('PATCH', '/merchant/shop/pharm/${Uri.encodeComponent(barcode)}', patch) as Map);

  Future<void> pharmDelete(String barcode) => send('DELETE', '/merchant/shop/pharm/${Uri.encodeComponent(barcode)}');

  /// Hammasi yoki hech narsa: qoldiq yetmasa 409.
  Future<Map<String, dynamic>> pharmSell(List<Map<String, Object?>> items, String paymentMethod) async =>
      Map<String, dynamic>.from(await send('POST', '/merchant/shop/pharm/sales',
          {'items': items, 'payment_method': paymentMethod}) as Map);

  Future<List<Map<String, dynamic>>> pharmSales() async => _list(await send('GET', '/merchant/shop/pharm/sales'));

  Future<Map<String, dynamic>> pharmStats() async =>
      Map<String, dynamic>.from(await send('GET', '/merchant/shop/pharm/stats') as Map);

  // ── Qurt buyurtmalari va logistika (`routes_qurt*.go`) ──
  Future<List<Map<String, dynamic>>> qurtOrders({String status = ''}) async {
    final r = await send('GET', '/merchant/qurt/orders${status.isEmpty ? '' : '?status=${Uri.encodeQueryComponent(status)}'}');
    return _list(r is Map ? r['items'] ?? r['orders'] : r);
  }

  Future<Map<String, dynamic>> qurtOrder(String id) async =>
      Map<String, dynamic>.from(await send('GET', '/merchant/qurt/orders/${Uri.encodeComponent(id)}') as Map);

  Future<void> qurtTransition(String id, String to, {String reason = ''}) =>
      send('POST', '/merchant/qurt/orders/${Uri.encodeComponent(id)}/transition',
          {'to': to, if (reason.isNotEmpty) 'reason': reason});

  Future<List<Map<String, dynamic>>> qurtCouriers() async {
    final r = await send('GET', '/merchant/qurt/couriers');
    return _list(r is Map ? r['items'] ?? r['couriers'] : r);
  }
}

String _range(DateTime? from, DateTime? to) {
  String d(DateTime v) =>
      '${v.year.toString().padLeft(4, '0')}-${v.month.toString().padLeft(2, '0')}-${v.day.toString().padLeft(2, '0')}';
  final q = [if (from != null) 'from=${d(from)}', if (to != null) 'to=${d(to)}'];
  return q.isEmpty ? '' : '?${q.join('&')}';
}

List<Map<String, dynamic>> _list(Object? v) =>
    (v as List? ?? const []).map((e) => Map<String, dynamic>.from(e as Map)).toList();

/// Tiyin -> "12 500 so'm" (`ondex_core.formatSum`, kasrsiz).
String somFromTiyin(num tiyin) => formatSum(tiyin.round());

/// So'm matni ("12 500" / "12500.5") -> tiyin.
int tiyinFromSomText(String s) {
  final v = double.tryParse(s.replaceAll(RegExp(r'[\s,]'), '').replaceAll("'", '')) ?? 0;
  return (v * 100).round();
}

/// Miqdor: butun bo'lsa kasrsiz, aks holda 3 xonagacha.
String fmtQty(num q) {
  if (q == q.roundToDouble()) return q.toInt().toString();
  return q.toStringAsFixed(3).replaceFirst(RegExp(r'0+$'), '');
}
