import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Kesh ombori — kalit/qiymat (JSON matni).
///
/// ┌─ NEGA ABSTRAKSIYA ────────────────────────────────────────────────┐
/// Ekranlar omborni TO'G'RIDAN-TO'G'RI chaqirmaydi, faqat
/// `Repository` orqali ishlaydi. Shu tufayli ombor turini keyinchalik
/// almashtirish (masalan katalog uchun SQLite/Drift'ga o'tish)
/// birorta ekranga tegmasdan bajariladi.
///
/// Bu MUHIM: hozirgi tanlov (`SharedPreferences`) kichik hajm uchun
/// to'g'ri, lekin menyu kattalashsa yetmay qoladi. O'sha payt faqat
/// shu fayl o'zgaradi.
/// └───────────────────────────────────────────────────────────────────┘
abstract class CacheStore {
  Future<String?> read(String key);
  Future<void> write(String key, String json);
  Future<void> remove(String key);

  /// Chiqishda (logout) chaqiriladi.
  Future<void> clearAll();
}

// ═══════════════════════════════════════════════════════════════════
// XAVFSIZLIK: KESH IKKIGA BO'LINADI
//
// Bu bo'linish butun qatlamning eng muhim qarori. Sabab oddiy:
// hamma narsani shifrlash sekin, hech narsani shifrlamaslik esa
// xavfli. Shuning uchun mezon — MA'LUMOT KIMNIKI:
//
//   PublicCacheStore  — hamma ko'radigan ma'lumot (restoranlar,
//                       menyu, turkumlar). Bu ma'lumot baribir
//                       ochiq API'dan olinadi, shuning uchun uni
//                       shifrlash hech narsa bermaydi va faqat
//                       ochilish tezligini pasaytiradi.
//
//   PersonalCacheStore — FOYDALANUVCHINIKI (buyurtmalar, manzil,
//                       telefon, sevimlilar). Telefon o'g'irlansa
//                       yoki zaxira nusxaga tushsa bu ma'lumot
//                       o'qilmasligi kerak. Android Keystore bilan
//                       shifrlanadi va CHIQISHDA TOZALANADI.
//
// UCHINCHI QOIDA (kod bilan majburlab bo'lmaydi, lekin buzilmasligi
// shart): keshda HECH QACHON avtorizatsiya holati saqlanmaydi.
// Telefondagi bazada "men adminman" degan yozuv turgani hech narsani
// anglatmaydi — har amal serverda qayta tekshiriladi.
// ═══════════════════════════════════════════════════════════════════

/// Ommaviy (shifrlanmagan) kesh — katalog uchun.
class PublicCacheStore implements CacheStore {
  /// Kalit prefiksi — bir xil `SharedPreferences` ichida boshqa
  /// sozlamalar ham turadi (`app_lock_enabled` va h.k.), ular
  /// `clearAll()` da tasodifan o'chib ketmasligi kerak.
  static const _prefix = 'cache.pub.';

  @override
  Future<String?> read(String key) async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString('$_prefix$key');
  }

  @override
  Future<void> write(String key, String json) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('$_prefix$key', json);
  }

  @override
  Future<void> remove(String key) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('$_prefix$key');
  }

  /// FAQAT o'z prefiksidagi kalitlarni o'chiradi.
  ///
  /// `prefs.clear()` ATAYLAB ISHLATILMAYDI: u ilova sozlamalarini
  /// (qulf yoqilganmi, tanlangan til) ham o'chirib yuborardi.
  @override
  Future<void> clearAll() async {
    final prefs = await SharedPreferences.getInstance();
    final keys = prefs.getKeys().where((k) => k.startsWith(_prefix)).toList();
    for (final k in keys) {
      await prefs.remove(k);
    }
  }
}

/// Shaxsiy (shifrlangan) kesh — foydalanuvchi ma'lumoti uchun.
///
/// ┌─ HAJM CHEKLOVI — BILIB TURING ────────────────────────────────────┐
/// `flutter_secure_storage` Android'da Keystore bilan himoyalangan
/// `EncryptedSharedPreferences` ustida ishlaydi. U KICHIK qiymatlar
/// uchun mo'ljallangan: buyurtmalar ro'yxati, manzil, sevimlilar
/// ID'lari — mos. Butun menyu tarixi kabi katta hajm bu yerga
/// SOLINMAYDI (sekinlashadi).
///
/// Shaxsiy ma'lumot kattalashsa — SQLCipher bilan shifrlangan
/// SQLite'ga o'tiladi va faqat shu sinf qayta yoziladi.
/// └───────────────────────────────────────────────────────────────────┘
class PersonalCacheStore implements CacheStore {
  static const _prefix = 'cache.me.';

  // Sozlama BERILMAYDI — standart holat to'g'ri.
  //
  // `AndroidOptions(encryptedSharedPreferences: true)` ATAYLAB
  // qo'yilmagan: `flutter_secure_storage` v10 da u ESKIRGAN va
  // e'tiborsiz qoldiriladi (Google'ning Jetpack Security kutubxonasi
  // to'xtatilgan, plagin endi o'z shifrlarini ishlatadi va mavjud
  // ma'lumotni birinchi murojaatda avtomatik ko'chiradi). Uni
  // qoldirish "biz shifrlashni yoqdik" degan YOLG'ON taassurot
  // berardi — aslida qator hech narsa qilmaydi.
  static const _storage = FlutterSecureStorage();

  @override
  Future<String?> read(String key) async {
    try {
      return await _storage.read(key: '$_prefix$key');
    } catch (_) {
      // Keystore buzilgan bo'lsa (qurilma tiklangan, zaxiradan
      // ko'chirilgan) o'qish yiqiladi. Bu XATO EMAS — shunchaki kesh
      // yo'q, ma'lumot tarmoqdan olinadi.
      return null;
    }
  }

  @override
  Future<void> write(String key, String json) async {
    try {
      await _storage.write(key: '$_prefix$key', value: json);
    } catch (_) {
      // Yozib bo'lmasa ilova ishlashda davom etadi — kesh
      // OPTIMIZATSIYA, majburiyat emas.
    }
  }

  @override
  Future<void> remove(String key) async {
    try {
      await _storage.delete(key: '$_prefix$key');
    } catch (_) {}
  }

  /// Chiqishda MAJBURIY chaqiriladi.
  ///
  /// Busiz keyingi foydalanuvchi (yoki telefonni qo'lga kiritgan odam)
  /// oldingi egasining buyurtmalari va manzilini ko'rardi.
  @override
  Future<void> clearAll() async {
    try {
      final all = await _storage.readAll();
      for (final k in all.keys.where((k) => k.startsWith(_prefix))) {
        await _storage.delete(key: k);
      }
    } catch (_) {
      // O'chirib bo'lmasa ham chiqish DAVOM ETADI: token baribir
      // o'chiriladi va kesh serversiz foydasiz. Lekin bu holat
      // kutilmagan — chaqiruvchi buni logga yozishi mumkin.
    }
  }
}
