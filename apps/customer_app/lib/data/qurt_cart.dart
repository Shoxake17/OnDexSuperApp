import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../api.dart';

/// Bitta mahsulotdan savatga solinadigan eng ko'p dona (server ham shuni qo'llaydi:
/// `qurtorders.MaxQtyPerLine`).
const kQurtMaxQtyPerLine = 100;

/// Savatdagi pozitsiya. Narx va nom — mijozga KO'RSATISH uchun nusxa; buyurtma
/// yaratilganda server mahsulot ID'sidan HAQIQIY narxni o'zi oladi.
class QurtCartLine {
  QurtCartLine({
    required this.productId,
    required this.name,
    required this.priceTiyin,
    required this.qty,
    this.imageUrl = '',
    this.weightGrams = 0,
    this.storeName = '',
    this.maxQty = kQurtMaxQtyPerLine,
  });

  final String productId;
  String name;
  int priceTiyin;
  String imageUrl;
  int weightGrams;
  String storeName;

  /// Serverdagi qoldiq (`stock_left`) bo'yicha yuqori chegara.
  int maxQty;
  int qty;

  int get totalTiyin => priceTiyin * qty;

  Map<String, dynamic> toJson() => {
    'id': productId,
    'name': name,
    'price': priceTiyin,
    'qty': qty,
    'img': imageUrl,
    'g': weightGrams,
    'store': storeName,
    'max': maxQty,
  };

  static QurtCartLine? fromJson(Object? j) {
    if (j is! Map) return null;
    final id = j['id'];
    final price = j['price'];
    final qty = j['qty'];
    if (id is! String || id.isEmpty || price is! num || qty is! num) {
      return null;
    }
    final q = qty.toInt();
    if (price <= 0 || q < 1) return null;
    return QurtCartLine(
      productId: id,
      name: (j['name'] ?? '').toString(),
      priceTiyin: price.toInt(),
      qty: q.clamp(1, kQurtMaxQtyPerLine),
      imageUrl: (j['img'] ?? '').toString(),
      weightGrams: (j['g'] as num?)?.toInt() ?? 0,
      storeName: (j['store'] ?? '').toString(),
      maxQty: ((j['max'] as num?)?.toInt() ?? kQurtMaxQtyPerLine).clamp(
        0,
        kQurtMaxQtyPerLine,
      ),
    );
  }
}

/// Qurt savati — butun ilova uchun BITTA (restoran savati `CartStore`dan ALOHIDA:
/// qurt buyurtmasi boshqa logistika va boshqa to'lov oqimiga ega).
///
/// Bir nechta do'kondan mahsulot solish MUMKIN: server buyurtmani do'konlar bo'yicha
/// bo'ladi. Qoldiq chegarasi faqat qulaylik uchun; HAQIQIY tekshiruv serverda.
///
/// Shaxsiy ma'lumot bo'lgani uchun shifrlangan `PersonalCacheStore`da saqlanadi
/// va chiqishda tozalanadi ([reset]).
class QurtCart extends ChangeNotifier {
  QurtCart._() : _storeOverride = null;

  /// Yagona nusxa.
  static final instance = QurtCart._();

  /// Sinov uchun: yangi, mustaqil savat (singletonga tegmaydi).
  @visibleForTesting
  QurtCart.forTest({CacheStore? store}) : _storeOverride = store;

  static const _key = 'qurt_cart';
  final CacheStore? _storeOverride;
  CacheStore get _store => _storeOverride ?? _sharedStore;
  static final CacheStore _sharedStore = PersonalCacheStore();

  final Map<String, QurtCartLine> _lines = {};
  bool _restored = false;
  String? _checkoutKey;

  List<QurtCartLine> get lines => List.unmodifiable(_lines.values);
  bool get isEmpty => _lines.isEmpty;
  int get totalQty => _lines.values.fold(0, (a, l) => a + l.qty);
  int get totalTiyin => _lines.values.fold(0, (a, l) => a + l.totalTiyin);
  int qtyOf(String productId) => _lines[productId]?.qty ?? 0;

  /// Mahsulot shu miqdorgacha ko'tarilishi mumkin.
  int limitOf(String productId) {
    final l = _lines[productId];
    return l == null
        ? kQurtMaxQtyPerLine
        : l.maxQty.clamp(0, kQurtMaxQtyPerLine);
  }

  /// Buyurtma yuborish uchun BIR MARTALIK kalit. Tarmoq uzilib qayta urinilsa
  /// AYNAN SHU kalit ketadi (server dublikat yaratmaydi); savat o'zgarsa yoki
  /// buyurtma muvaffaqiyatli bo'lsa YANGI kalit olinadi.
  String checkoutKey() => _checkoutKey ??= newIdempotencyKey();

  void _changed() {
    _checkoutKey =
        null; // savat o'zgardi — avvalgi urinish endi boshqa buyurtma
    _persist();
    notifyListeners();
  }

  /// Serverdan kelgan mahsulot (`GET /qurt/products` elementi) ni savatga qo'shadi
  /// yoki miqdorini 1 ga oshiradi. Qoldiq/chegara to'lgan bo'lsa `false`.
  bool add(Map<String, dynamic> product, {int qty = 1}) {
    final id = (product['id'] ?? '').toString();
    final price = (product['price_tiyin'] as num?)?.toInt() ?? 0;
    if (id.isEmpty || price <= 0 || qty < 1) return false;
    final left = (product['stock_left'] as num?)?.toInt() ?? kQurtMaxQtyPerLine;
    final max = left.clamp(0, kQurtMaxQtyPerLine);
    final line = _lines[id];
    final next = (line?.qty ?? 0) + qty;
    if (max < 1 || next > max) return false;
    if (line == null) {
      _lines[id] = QurtCartLine(
        productId: id,
        name: (product['name'] ?? '').toString(),
        priceTiyin: price,
        qty: next,
        imageUrl: (product['image_url'] ?? '').toString(),
        weightGrams: (product['weight_grams'] as num?)?.toInt() ?? 0,
        storeName: (product['store_name'] ?? '').toString(),
        maxQty: max,
      );
    } else {
      line
        ..qty = next
        ..maxQty = max
        ..priceTiyin = price;
    }
    _changed();
    return true;
  }

  /// Miqdorni o'rnatadi (chegaraga qirqiladi); `qty <= 0` — olib tashlaydi.
  void setQty(String productId, int qty) {
    final line = _lines[productId];
    if (line == null) return;
    if (qty <= 0) {
      _lines.remove(productId);
    } else {
      line.qty = qty.clamp(1, line.maxQty < 1 ? 1 : line.maxQty);
    }
    _changed();
  }

  void remove(String productId) => setQty(productId, 0);

  /// Buyurtma yuborilgach yoki mijoz tozalaganda.
  void clear() {
    if (_lines.isEmpty) return;
    _lines.clear();
    _changed();
  }

  /// Chiqishda: xotira ham, disk ham, kalit ham.
  Future<void> reset() async {
    _lines.clear();
    _checkoutKey = null;
    _restored = true;
    try {
      await _store.remove(_key);
    } catch (_) {}
    notifyListeners();
  }

  /// Serverdagi joriy mahsulotlar bilan solishtiradi: narx/nom/qoldiq yangilanadi,
  /// sotuvdan chiqqanlari olib tashlanadi. Qaytadi: o'zgarish haqida mijozga
  /// aytiladigan xabarlar (bo'sh — o'zgarish yo'q).
  List<String> syncWith(List<Map<String, dynamic>> fresh) {
    final byId = {for (final p in fresh) (p['id'] ?? '').toString(): p};
    final notes = <String>[];
    for (final line in _lines.values.toList()) {
      final p = byId[line.productId];
      final left = (p?['stock_left'] as num?)?.toInt();
      if (p == null || (left != null && left < 1)) {
        _lines.remove(line.productId);
        notes.add('«${line.name}» sotuvda qolmagan — savatdan olib tashlandi');
        continue;
      }
      final price = (p['price_tiyin'] as num?)?.toInt() ?? line.priceTiyin;
      if (price != line.priceTiyin) {
        notes.add('«${line.name}» narxi o\'zgardi');
        line.priceTiyin = price;
      }
      line
        ..name = (p['name'] ?? line.name).toString()
        ..imageUrl = (p['image_url'] ?? line.imageUrl).toString()
        ..storeName = (p['store_name'] ?? line.storeName).toString();
      if (left != null) {
        line.maxQty = left.clamp(1, kQurtMaxQtyPerLine);
        if (line.qty > line.maxQty) {
          line.qty = line.maxQty;
          notes.add('«${line.name}» dan faqat ${line.maxQty} dona qolgan');
        }
      }
    }
    if (notes.isNotEmpty) _changed();
    return notes;
  }

  /// Savatni serverdagi HOZIRGI narx/qoldiq bilan yangilaydi (GET /qurt/products?ids=).
  /// Qaytadi: mijozga aytiladigan o'zgarishlar (bo'sh — savat serverga mos).
  /// Tarmoq xatosida istisno tashlaydi — chaqiruvchi hal qiladi.
  Future<List<String>> refresh() async {
    if (isEmpty) return [];
    final fresh = await api.qurtProductsByIds(_lines.keys.toList());
    return syncWith(fresh);
  }

  /// Serverga yuboriladigan pozitsiyalar: FAQAT mahsulot ID va miqdor.
  List<Map<String, dynamic>> toOrderItems() => [
    for (final l in _lines.values) {'product_id': l.productId, 'qty': l.qty},
  ];

  // ── Saqlash ───────────────────────────────────────────────────────

  /// Diskdan bir marta o'qiydi (takroriy chaqiruv zararsiz).
  Future<void> ensureRestored() async {
    if (_restored) return;
    _restored = true;
    try {
      final raw = await _store.read(_key);
      if (raw == null) return;
      final list = (jsonDecode(raw) as Map)['lines'] as List;
      for (final j in list) {
        final l = QurtCartLine.fromJson(j);
        if (l != null) _lines.putIfAbsent(l.productId, () => l);
      }
      if (_lines.isNotEmpty) notifyListeners();
    } catch (_) {
      // Buzilgan yozuv — savat bo'sh boshlanadi, ilova yiqilmaydi.
      try {
        await _store.remove(_key);
      } catch (_) {}
    }
  }

  void _persist() {
    unawaited(() async {
      try {
        if (_lines.isEmpty) {
          await _store.remove(_key);
          return;
        }
        await _store.write(
          _key,
          jsonEncode({'lines': _lines.values.map((l) => l.toJson()).toList()}),
        );
      } catch (_) {}
    }());
  }
}
