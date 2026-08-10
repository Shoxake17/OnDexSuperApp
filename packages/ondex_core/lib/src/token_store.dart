import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Sessiya tokenini SAQLASH joyi — barcha ilovalar uchun bitta nusxa.
///
/// XAVFSIZLIK: token avval oddiy `SharedPreferences` da (shifrlanmagan
/// XML fayl) saqlanardi — root qilingan qurilmada yoki ADB backup orqali
/// uni o'qib olish mumkin edi, u esa uzoq muddat amal qiladigan to'liq
/// huquqli bearer token. Endi Android Keystore bilan shifrlangan
/// `flutter_secure_storage` ishlatiladi.
///
/// MIGRATSIYA: eski o'rnatishlarda token hali ham `SharedPreferences` da
/// bo'lishi mumkin — birinchi o'qishda uni xavfsiz omborga ko'chirib,
/// eski nusxani O'CHIRAMIZ. Foydalanuvchi qayta login qilmaydi.
class TokenStore {
  /// [key] — ilova boshiga alohida (`token`, `courier_token`, ...),
  /// shunda bitta qurilmada ikki ilova bir-birining sessiyasini
  /// almashtirib yubormaydi.
  const TokenStore(this.key);

  final String key;

  // `AndroidOptions` ATAYLAB berilmaydi: `encryptedSharedPreferences`
  // paketning yangi versiyasida eskirgan (Jetpack Security Google
  // tomonidan to'xtatilgan) va standart holatda ma'lumot o'z-o'zidan
  // shifrlangan omborga ko'chiriladi.
  static const _secure = FlutterSecureStorage();

  Future<String?> read() async {
    try {
      final v = await _secure.read(key: key);
      if (v != null && v.isNotEmpty) return v;
    } catch (_) {
      // Keystore ishlamasa — pastdagi zaxira yo'lga tushamiz.
    }

    // Eski joy — bir martalik ko'chirish.
    //
    // `SharedPreferences.getInstance()` ATAYLAB try/catch ICHIDA:
    // test muhitida (plugin registratsiyasisiz) u `MissingPluginException`
    // tashlaydi va bu kutilmagan joyda ilovani yiqitardi.
    try {
      final prefs = await SharedPreferences.getInstance();
      final legacy = prefs.getString(key);
      if (legacy != null && legacy.isNotEmpty) {
        try {
          await _secure.write(key: key, value: legacy);
          await prefs.remove(key); // shifrlanmagan nusxa qoldirilmaydi
        } catch (_) {}
        return legacy;
      }
    } catch (_) {}
    return null;
  }

  Future<void> write(String token) async {
    try {
      await _secure.write(key: key, value: token);
    } catch (_) {
      // Keystore mavjud bo'lmagan kamdan-kam holatda ilova ishlashda
      // davom etsin — lekin ATAYLAB eski shifrlanmagan joyga
      // qaytmaymiz, aks holda tuzatish ma'nosini yo'qotadi.
    }
  }

  Future<void> clear() async {
    try {
      await _secure.delete(key: key);
    } catch (_) {}
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(key); // eski qoldiq ham tozalanadi
    } catch (_) {}
  }
}
