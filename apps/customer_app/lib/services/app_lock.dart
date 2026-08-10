import 'package:flutter/foundation.dart';
import 'package:local_auth/local_auth.dart';
import 'package:local_auth_android/local_auth_android.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Ilova qulfi — barmoq izi / yuz / grafik kalit / PIN.
///
/// ── NIMANI HIMOYA QILADI VA NIMANI YO'Q ────────────────────────────
/// Bu qulf QURILMA QO'LGA TUSHGAN holatga qarshi: telefon ochiq qolgan
/// yoki o'g'irlangan bo'lsa, begona odam OnDex'ni ochib buyurtma bera
/// olmasin, manzil va telefon raqamini ko'ra olmasin.
///
/// U TARMOQ hujumlariga qarshi EMAS: kirish tokeni baribir qurilmada
/// saqlanadi (Android Keystore bilan shifrlangan — `ondex_core`
/// `TokenStore`) va qulf uni o'chirmaydi. Ya'ni bu qatlam serverdagi
/// himoyaning O'RNINI BOSMAYDI, ustiga qo'shiladi.
///
/// ── NEGA `biometricOnly: false` ────────────────────────────────────
/// `true` qilinsa faqat barmoq izi/yuz qabul qilinadi va barmoq izi
/// sozlanmagan telefondagi foydalanuvchi ilovaga UMUMAN kira olmay
/// qolardi. `false` bo'lganda Android o'zi zaxira sifatida grafik
/// kalit/PIN/parolni taklif qiladi — foydalanuvchi qurilmasida nima
/// bo'lsa, o'sha ishlaydi. Global ilovalar (bank, pochta) aynan
/// shunday qiladi.
class AppLock {
  AppLock._();

  static final _auth = LocalAuthentication();

  /// Sozlama kaliti — foydalanuvchi qulfni o'chirib qo'yishi mumkin.
  static const _prefKey = 'app_lock_enabled';

  /// Fonga chiqqandan keyin qayta qulflanishgacha bo'lgan muhlat.
  ///
  /// NEGA NOL EMAS: ilova xarita, rasm tanlash, WebView mini-app yoki
  /// Telegram uchun bir necha soniyaga fonga chiqadi. Har safar qulf
  /// so'ralsa ilova ishlatib bo'lmas holga kelardi. 30 soniya —
  /// "cho'ntakka soldim" bilan "boshqa ilovaga o'tdim" orasidagi
  /// amaliy chegara.
  static const lockAfterBackground = Duration(seconds: 30);

  /// Qurilmada umuman qulf sozlanganmi (barmoq izi YOKI PIN/grafik).
  ///
  /// MUHIM: `getAvailableBiometrics()` YETARLI EMAS — u barmoq izi
  /// yo'q, lekin PIN qo'yilgan telefonda bo'sh ro'yxat qaytaradi va
  /// biz qulfni noto'g'ri "imkonsiz" deb hisoblardik.
  /// `isDeviceSupported()` aynan "qurilmada biror himoya bormi" ni
  /// aytadi (barmoq izi YOKI qurilma kaliti).
  static Future<bool> isAvailable() async {
    try {
      return await _auth.isDeviceSupported();
    } catch (_) {
      // Plagin ishga tushmagan yoki platforma qo'llab-quvvatlamaydi.
      return false;
    }
  }

  /// Foydalanuvchi qulfni yoqganmi. STANDART — YOQILGAN.
  ///
  /// Qurilmada hech qanday himoya bo'lmasa `false` qaytadi: aks holda
  /// ilova ochilmaydigan bo'lib qolardi.
  static Future<bool> isEnabled() async {
    if (!await isAvailable()) return false;
    final prefs = await SharedPreferences.getInstance();
    return prefs.getBool(_prefKey) ?? true;
  }

  static Future<void> setEnabled(bool value) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_prefKey, value);
  }

  /// Qulfni ochish so'rovi. `true` — ochildi.
  ///
  /// `persistAcrossBackgrounding: true` — tasdiqlash paytida ilova
  /// fonga chiqib qaytsa (masalan bildirishnoma bosilsa) so'rov xato
  /// bilan tugamaydi, balki qaytadan ko'rsatiladi. Busiz qulf oynasi
  /// yo'qolib qolardi va foydalanuvchi bo'sh ekranda qolardi.
  ///
  /// XATO YUTILADI, LEKIN "OCHILDI" DEB HISOBLANMAYDI: har qanday
  /// nosozlikda `false` qaytadi. Teskarisi (xato bo'lsa kiritib
  /// yuborish) qulfni butunlay ma'nosiz qilardi.
  static Future<bool> unlock() async {
    try {
      return await _auth.authenticate(
        localizedReason: 'OnDex\'ga kirish uchun shaxsingizni tasdiqlang',
        biometricOnly: false, // grafik kalit / PIN ham qabul qilinadi
        persistAcrossBackgrounding: true,
        authMessages: const <AuthMessages>[
          AndroidAuthMessages(
            signInTitle: 'OnDex qulfi',
            signInHint: 'Shaxsingizni tasdiqlang',
            cancelButton: 'Bekor qilish',
          ),
        ],
      );
    } catch (e) {
      // XATO YOZILADI. Telegram oqimida aynan "jimgina yutilgan xato"
      // tufayli tugma hech qachon ko'rinmagani va buni sezishning
      // yo'li bo'lmagani uchun bu yerda ham jim qolmaymiz: qulf
      // ochilmasa, sababi logda turadi.
      debugPrint('AppLock: qulfni ochib bo\'lmadi: $e');
      return false;
    }
  }
}
