import 'package:flutter/foundation.dart';

import '../api.dart';

/// Sevimli mahsulotlarning YAGONA holati.
///
/// ┌─ NIMA UCHUN ──────────────────────────────────────────────────────┐
/// Ilgari har ekran o'zining `Set<String> _favorites` ini saqlardi va
/// uni `api.favoriteIds()` bilan alohida yuklardi: menyu, savat, turkum
/// ekrani, sevimlilar tab'i. Natijada:
///   * menyuda yurakcha bosilsa, taom tavsifi panelida u ESKI holatda
///     qolardi va faqat qayta yuklangach yangilanardi;
///   * "Sevimlilar" tab'i o'zgarishni ko'rish uchun tarmoqqa qayta
///     murojaat qilardi.
///
/// Endi holat bitta joyda. Yurakcha bosilishi bilan hamma joyda
/// o'zgaradi — kutishsiz, qayta yuklashsiz.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ OPTIMISTIK YANGILANISH ──────────────────────────────────────────┐
/// Belgi DARHOL o'zgaradi, so'rov esa fonda ketadi. Server rad etsa —
/// belgi o'z holiga QAYTARILADI va xato chaqiruvchiga uzatiladi.
/// Shu sababli sekin internetda ham ilova "qotib" turmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class FavoritesStore extends ChangeNotifier {
  FavoritesStore._();

  static final instance = FavoritesStore._();

  final Set<String> _ids = <String>{};

  /// Serverdan bir marta yuklanganmi. Anonim foydalanuvchida so'rov 401
  /// qaytaradi — bu XATO EMAS, shunchaki hech narsa belgilanmagan.
  bool _loaded = false;

  /// Bir vaqtning o'zida ikkita yuklash ketmasligi uchun.
  Future<void>? _loading;

  Set<String> get ids => Set.unmodifiable(_ids);

  bool contains(String productId) => _ids.contains(productId);

  /// Serverdagi ro'yxatni oladi. Takroriy chaqiruv xavfsiz: bir marta
  /// yuklangach qayta so'ralmaydi (`force` bilan majburlash mumkin).
  Future<void> load({bool force = false}) {
    if (_loaded && !force) return Future.value();
    return _loading ??= _doLoad().whenComplete(() => _loading = null);
  }

  Future<void> _doLoad() async {
    try {
      final ids = await api.favoriteIds();
      _loaded = true;
      if (_sameAs(ids)) return;
      _ids
        ..clear()
        ..addAll(ids);
      notifyListeners();
    } catch (_) {
      // Kirmagan foydalanuvchi yoki tarmoq xatosi — ro'yxat bo'sh
      // qoladi. Ilova baribir ishlayveradi.
    }
  }

  bool _sameAs(Set<String> other) =>
      _ids.length == other.length && _ids.containsAll(other);

  /// Yurakchani almashtiradi va YANGI holatni qaytaradi.
  ///
  /// Xato bo'lsa belgi o'z holiga qaytariladi va xato uzatiladi —
  /// chaqiruvchi mijozga xabar ko'rsatishi mumkin.
  Future<bool> toggle(String productId) async {
    final next = !_ids.contains(productId);
    _set(productId, next);
    try {
      if (next) {
        await api.addFavorite(productId);
      } else {
        await api.removeFavorite(productId);
      }
      return next;
    } catch (_) {
      _set(productId, !next);
      rethrow;
    }
  }

  void _set(String productId, bool favorited) {
    final changed =
        favorited ? _ids.add(productId) : _ids.remove(productId);
    if (changed) notifyListeners();
  }
}
