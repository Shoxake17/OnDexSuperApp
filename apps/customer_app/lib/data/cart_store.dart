import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../api.dart';

/// Savat — butun ilova uchun BITTA.
///
/// ┌─ QOIDA: BITTA BUYURTMA = BITTA RESTORAN ──────────────────────────┐
/// Boshqa restoran taomi qo'shilsa, eski savat JIMGINA tozalanadi.
/// Bu server qoidasining aksi: `internal/orders` da aralash savat
/// `ErrMixedRestaurants` bilan rad etiladi.
///
/// Klient uni OLDINDAN qo'llaydi — mijoz savatni to'ldirib, checkout'da
/// rad javob olmasligi uchun. Lekin HAQIQIY tekshiruv baribir serverda:
/// bu yerdagi qoida faqat qulaylik, himoya emas.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA SHIFRLANGAN OMBOR ──────────────────────────────────────────┐
/// Savat — foydalanuvchining nima buyurtma qilmoqchi ekani. Bu shaxsiy
/// ma'lumot, shuning uchun `PersonalCacheStore` da saqlanadi va
/// chiqishda boshqa shaxsiy ma'lumot bilan birga tozalanadi.
/// └───────────────────────────────────────────────────────────────────┘
class CartStore extends ChangeNotifier {
  CartStore._();

  /// Yagona nusxa — menyu, savat va checkout ekranlari shunga obuna.
  static final instance = CartStore._();

  static const _key = 'cart';
  static final CacheStore _store = PersonalCacheStore();

  String? _restaurantId;
  String? _restaurantName;
  final Map<String, int> _items = {};

  /// Stol (dine-in) seansi — QR skanerlangan bo'lsa to'ladi.
  ///
  /// Buyurtma yaratilganda `table_token` sifatida yuboriladi va server
  /// buyurtmani `dine_in` deb belgilaydi (`routes_orders.go`).
  String? _tableToken;
  String? _tableLabel;

  String? get restaurantId => _restaurantId;
  String? get restaurantName => _restaurantName;
  String? get tableToken => _tableToken;
  String? get tableLabel => _tableLabel;
  bool get isDineIn => _tableToken != null;

  /// O'zgarmas nusxa — tashqaridan tahrirlab bo'lmaydi.
  Map<String, int> get items => Map.unmodifiable(_items);

  bool get isEmpty => _items.isEmpty;

  int qtyOf(String productId) => _items[productId] ?? 0;

  /// Savatdagi jami dona soni (pastki panelda ko'rsatiladi).
  int get totalQty => _items.values.fold(0, (a, b) => a + b);

  // ── O'zgartirish ──────────────────────────────────────────────────

  /// Mahsulot miqdorini o'rnatadi. `qty <= 0` — savatdan olib tashlaydi.
  ///
  /// `restaurantId` HAR CHAQIRUVDA beriladi: aynan shu yerda "boshqa
  /// restoran" holati aniqlanadi va eski savat tozalanadi.
  void setQty({
    required String restaurantId,
    required String productId,
    required int qty,
    String? restaurantName,
  }) {
    if (_restaurantId != null && _restaurantId != restaurantId) {
      // Boshqa restoran — eski savat butunlay tashlanadi.
      _items.clear();
      // Stol seansi ham shu restoranga tegishli edi, u ham ketadi.
      _tableToken = null;
      _tableLabel = null;
    }
    _restaurantId = restaurantId;
    if (restaurantName != null) _restaurantName = restaurantName;

    if (qty <= 0) {
      _items.remove(productId);
    } else {
      _items[productId] = qty;
    }

    // Oxirgi taom olib tashlansa savat butunlay bo'shaydi — restoran
    // bog'lanishi ham qoldirilmaydi, aks holda keyingi restoranga
    // o'tishda "boshqa restoran" tekshiruvi keraksiz ishlagan bo'lardi.
    if (_items.isEmpty) {
      _restaurantId = null;
      _restaurantName = null;
    }

    _persist();
    notifyListeners();
  }

  void increment({
    required String restaurantId,
    required String productId,
    String? restaurantName,
  }) =>
      setQty(
        restaurantId: restaurantId,
        productId: productId,
        qty: qtyOf(productId) + 1,
        restaurantName: restaurantName,
      );

  void decrement({required String restaurantId, required String productId}) =>
      setQty(
        restaurantId: restaurantId,
        productId: productId,
        qty: qtyOf(productId) - 1,
      );

  /// Stol QR kodi skanerlangandan keyin.
  ///
  /// Savat BOSHQA restoranniki bo'lsa tozalanadi — QR skanerlash aniq
  /// niyat: "men SHU restoranning SHU stolidaman"
  /// (`apps/web/lib/open-table.ts` dagi qoida bilan bir xil).
  void startTableSession({
    required String restaurantId,
    required String token,
    String? tableLabel,
    String? restaurantName,
  }) {
    if (_restaurantId != null && _restaurantId != restaurantId) {
      _items.clear();
    }
    _restaurantId = restaurantId;
    if (restaurantName != null) _restaurantName = restaurantName;
    _tableToken = token;
    _tableLabel = tableLabel;
    _persist();
    notifyListeners();
  }

  /// Buyurtma yuborilgach yoki foydalanuvchi tozalaganda.
  void clear() {
    _items.clear();
    _restaurantId = null;
    _restaurantName = null;
    _tableToken = null;
    _tableLabel = null;
    _persist();
    notifyListeners();
  }

  // ── Saqlash ───────────────────────────────────────────────────────

  /// Ilova ishga tushganda bir marta chaqiriladi.
  Future<void> restore() async {
    try {
      final raw = await _store.read(_key);
      if (raw == null) return;
      final m = jsonDecode(raw) as Map<String, dynamic>;
      _restaurantId = m['rid'] as String?;
      _restaurantName = m['rname'] as String?;
      _tableToken = m['ttoken'] as String?;
      _tableLabel = m['tlabel'] as String?;
      _items
        ..clear()
        ..addAll((m['items'] as Map).map(
          (k, v) => MapEntry(k as String, (v as num).toInt()),
        ));
      notifyListeners();
    } catch (_) {
      // Buzilgan yoki eski formatdagi yozuv — savat bo'sh boshlanadi.
      // Ilova YIQILMAYDI (sxema o'zgarganda ham shu naqsh ishlaydi).
      await _store.remove(_key);
    }
  }

  /// Yozish `await` QILINMAYDI: interfeys diskni kutib turmasligi kerak.
  /// Xato bo'lsa savat xotirada baribir to'g'ri qoladi.
  void _persist() {
    unawaited(() async {
      try {
        if (_items.isEmpty) {
          await _store.remove(_key);
          return;
        }
        await _store.write(
          _key,
          jsonEncode({
            'rid': _restaurantId,
            'rname': _restaurantName,
            'ttoken': _tableToken,
            'tlabel': _tableLabel,
            'items': _items,
          }),
        );
      } catch (_) {}
    }());
  }
}
