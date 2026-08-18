// `api.dart` `ondex_core` ni qayta eksport qiladi — `Repository`,
// `CacheStore` va `CachePolicy` shu import orqali keladi. Alohida
// `package:ondex_core` importi ortiqcha bo'lardi (analizator ham
// shuni aytadi).
import '../api.dart';

/// Mijoz ilovasining BARCHA kesh-birinchi repozitoriylari.
///
/// ┌─ NEGA BITTA FAYL ─────────────────────────────────────────────────┐
/// Har ekran o'z keshini o'zi qursa, kalitlar va TTL'lar tarqalib
/// ketardi: birida 30 soniya, boshqasida 5 daqiqa, uchinchisida
/// umuman yo'q. Bu yerda ular BIR joyda ko'rinadi va taqqoslash mumkin.
///
/// Ekranlar `Repository` yaratmaydi — faqat shu yerdagi tayyorini
/// oladi. Yuklanish mantig'i esa `ondex_core` da, bitta
/// implementatsiyada (`Cached.showSpinner`).
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ XAVFSIZLIK: QAYSI OMBOR ─────────────────────────────────────────┐
/// Katalog (restoranlar, menyu, turkumlar) — OMMAVIY ombor. Bu
/// ma'lumot baribir ochiq API'dan olinadi, shifrlash foyda bermaydi.
///
/// Buyurtmalar va sevimlilar — SHAXSIY ombor: shifrlanadi va
/// chiqishda tozalanadi (`clearOnLogout`).
/// └───────────────────────────────────────────────────────────────────┘
class Repos {
  Repos._();

  static final CacheStore _public = PublicCacheStore();
  static final CacheStore _personal = PersonalCacheStore();

  // ── TTL: qachon fonda yangilanadi ────────────────────────────────
  //
  // DIQQAT: TTL "ma'lumot o'chadi" degani EMAS. Muddati o'tgan
  // ma'lumot ham ekranda DARHOL ko'rsatiladi, shunchaki fonda
  // yangilanadi. Ya'ni bu qiymatlar tezlikka emas, TRAFIKKA ta'sir
  // qiladi.

  /// Restoranlar — Go tomonda Redis keshi 30 soniya, shunga moslandi.
  /// Undan qisqa qilishning ma'nosi yo'q: server baribir eski
  /// javobni qaytaradi.
  static const _ttlRestaurants = Duration(seconds: 30);

  /// Menyu — narx va mavjudlik kun davomida o'zgaradi, lekin har
  /// daqiqada emas.
  static const _ttlMenu = Duration(minutes: 5);

  /// Turkumlar — barcha restoranlar uchun umumiy, deyarli
  /// o'zgarmaydi.
  static const _ttlCategories = Duration(hours: 12);

  /// Buyurtmalar — holat o'zgarishi WebSocket orqali JONLI keladi,
  /// shuning uchun so'rov siklining o'zi tez-tez bo'lishi shart emas.
  static const _ttlOrders = Duration(minutes: 2);

  static const _ttlFavorites = Duration(minutes: 10);

  /// Aksiyalar — vaqtga BOG'LIQ (boshlanish/tugash sanasi bor), ya'ni
  /// server javobi soat o'tishi bilan o'z-o'zidan o'zgaradi. Shuning
  /// uchun menyudan qisqa: tugagan aksiya narxi ekranda uzoq turmasin.
  static const _ttlPromotions = Duration(minutes: 1);

  // ── Katalog (ommaviy) ────────────────────────────────────────────

  static Repository<List<dynamic>> restaurants() => _list(
        store: _public,
        key: 'restaurants',
        ttl: _ttlRestaurants,
        fetch: api.restaurants,
      );

  static Repository<List<dynamic>> categories() => _list(
        store: _public,
        key: 'categories',
        ttl: _ttlCategories,
        fetch: api.categories,
      );

  /// Menyu HAR RESTORAN uchun alohida kalit bilan saqlanadi —
  /// aks holda ikkinchi restoran birinchisining keshini bosib
  /// ketardi va mijoz noto'g'ri menyu ko'rardi.
  static Repository<List<dynamic>> menu(String restaurantId) => _list(
        store: _public,
        key: 'menu.$restaurantId',
        ttl: _ttlMenu,
        fetch: () => api.menu(restaurantId),
      );

  /// Restoranning FAOL aksiyalari — menyudagi chegirma narxi va
  /// "Aksiya" lentasi shundan chiziladi.
  ///
  /// DIQQAT: bu FAQAT ko'rsatish uchun. To'lanadigan summa har doim
  /// `POST /restaurants/{id}/quote` dan keladi — vebda ham shunday
  /// (`lib/use-quote.ts`).
  static Repository<List<dynamic>> promotions(String restaurantId) => _list(
        store: _public,
        key: 'promos.$restaurantId',
        ttl: _ttlPromotions,
        fetch: () => api.activePromotions(restaurantId),
      );

  // ── Shaxsiy (shifrlangan) ────────────────────────────────────────

  static Repository<List<dynamic>> myOrders() => _list(
        store: _personal,
        key: 'orders',
        ttl: _ttlOrders,
        fetch: api.myOrders,
      );

  static Repository<List<dynamic>> favorites() => _list(
        store: _personal,
        key: 'favorites',
        ttl: _ttlFavorites,
        fetch: api.favorites,
      );

  // ── Chiqish ──────────────────────────────────────────────────────

  /// Chiqishda MAJBURIY chaqiriladi.
  ///
  /// Katalog keshi ATAYLAB qoldiriladi: u shaxsiy emas va keyingi
  /// kirishda ilova bir zumda ochilishini ta'minlaydi. Shaxsiy
  /// ma'lumot esa butunlay o'chiriladi.
  static Future<void> clearOnLogout() => _personal.clearAll();

  // ── Ichki yordamchi ──────────────────────────────────────────────

  /// Barcha repozitoriylar `List<dynamic>` (xom JSON) bilan ishlaydi.
  ///
  /// NEGA MODEL SINFLARI EMAS: ekranlar hozir ham xom `Map` bilan
  /// ishlaydi (`o['status']`, `p['price_tiyin']`). Model sinflari
  /// kiritish alohida, kattaroq o'zgarish — uni kesh qatlami bilan
  /// ARALASHTIRMASLIK kerak, aks holda ikkala o'zgarish ham bir
  /// vaqtda sinovdan o'tolmay qolardi.
  static Repository<List<dynamic>> _list({
    required CacheStore store,
    required String key,
    required Duration ttl,
    required Future<List<dynamic>> Function() fetch,
  }) {
    return Repository<List<dynamic>>(
      store: store,
      policy: CachePolicy(key, ttl: ttl),
      fetch: fetch,
      encode: (v) => v,
      decode: (j) => (j as List).cast<dynamic>(),
    );
  }
}
