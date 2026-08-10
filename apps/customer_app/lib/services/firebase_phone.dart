import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart';

/// Firebase Phone Auth ustidagi yupqa qatlam.
///
/// ── NIMA UCHUN ALOHIDA FAYL ────────────────────────────────────────
/// Telefon tasdiqlash IKKI ekranda kerak (ro'yxatdan o'tish va parolni
/// tiklash). Firebase API'si callback'lar bilan ishlaydi va uni har
/// ekranda qayta yozish — xato qilishning eng qisqa yo'li. Bu yerda u
/// bir marta `Future` ga o'raladi.
///
/// ── OQIM ───────────────────────────────────────────────────────────
///   1. `sendCode(phone)`  -> Firebase SMS yuboradi, `verificationId`
///                            qaytadi (kod BIZDA emas, Firebase'da);
///   2. foydalanuvchi kodni kiritadi;
///   3. `idTokenFor(verificationId, code)` -> Firebase kodni tekshiradi
///      va IMZOLANGAN ID token beradi;
///   4. token backendga yuboriladi (`POST /auth/firebase`), u yerda
///      IMZO tekshiriladi va telefon raqami TOKEN ICHIDAN olinadi.
///
/// Ya'ni kodni na yuboramiz, na tekshiramiz — bu butunlay Firebase
/// zimmasida. Bizning serverimiz faqat natijaga (imzolangan tokenga)
/// ishonadi.
class FirebasePhoneAuth {
  FirebasePhoneAuth._();

  /// SMS yuboradi va `verificationId` qaytaradi.
  ///
  /// `timeout` — Android'da kodni AVTOMATIK o'qish uchun kutish vaqti.
  /// Bu qiymat SMS muddatiga aloqador emas.
  static Future<String> sendCode(String phone,
      {Duration timeout = const Duration(seconds: 60)}) {
    final completer = Completer<String>();
    FirebaseAuth.instance.verifyPhoneNumber(
      phoneNumber: phone,
      timeout: timeout,
      // Android ba'zan SMS'ni o'zi o'qib, kodsiz tasdiqlaydi. Biz baribir
      // `codeSent` ni kutamiz va foydalanuvchidan kod so'raymiz — oqim
      // ikkala holatda ham BIR XIL bo'ladi (kamroq holat = kamroq xato).
      verificationCompleted: (_) {},
      verificationFailed: (FirebaseAuthException e) {
        if (!completer.isCompleted) {
          completer.completeError(PhoneAuthFailure(_message(e)));
        }
      },
      codeSent: (String verificationId, int? _) {
        if (!completer.isCompleted) completer.complete(verificationId);
      },
      // Avtomatik o'qish muddati tugadi — bu XATO EMAS, foydalanuvchi
      // kodni qo'lda kiritadi. `codeSent` allaqachon ishlagan bo'ladi.
      codeAutoRetrievalTimeout: (_) {},
    );
    return completer.future;
  }

  /// Kodni tekshiradi va backendga yuborish uchun ID token qaytaradi.
  static Future<String> idTokenFor(String verificationId, String smsCode) async {
    try {
      final cred = PhoneAuthProvider.credential(
        verificationId: verificationId,
        smsCode: smsCode,
      );
      final res = await FirebaseAuth.instance.signInWithCredential(cred);
      final token = await res.user?.getIdToken();
      if (token == null || token.isEmpty) {
        throw PhoneAuthFailure('Tasdiqlash amalga oshmadi');
      }
      return token;
    } on FirebaseAuthException catch (e) {
      throw PhoneAuthFailure(_message(e));
    }
  }

  /// Firebase sessiyasini yopadi.
  ///
  /// MUHIM: Firebase O'ZINING sessiyasini saqlaydi. Uni tozalamasak,
  /// ilovadan chiqqan foydalanuvchi Firebase'da kirgan bo'lib qolardi
  /// va keyingi tasdiqlashda eski hisob ishlatilishi mumkin edi.
  static Future<void> signOut() async {
    try {
      await FirebaseAuth.instance.signOut();
    } catch (_) {
      // Chiqishga xalaqit bermaydi.
    }
  }

  /// Firebase xatolarini foydalanuvchi tilida tushuntiradi.
  ///
  /// Xom kodlar (`invalid-verification-code`) foydalanuvchiga hech
  /// narsa aytmaydi va ingliz tilida chiqadi.
  static String _message(FirebaseAuthException e) {
    switch (e.code) {
      case 'invalid-phone-number':
        return 'Telefon raqami noto\'g\'ri';
      case 'invalid-verification-code':
        return 'Kod noto\'g\'ri';
      case 'session-expired':
      case 'invalid-verification-id':
        return 'Kod muddati tugadi — qayta yuboring';
      case 'too-many-requests':
        // Firebase kodi 17010. Bu BIZNING tezlik cheklovimiz EMAS —
        // Firebase g'ayritabiiy faollik deb QURILMANI bloklaydi
        // (raqamni emas!). Shu sabab boshqa raqam bilan urinish ham
        // yordam bermaydi. Xabar buni aniq aytishi kerak, aks holda
        // foydalanuvchi qayta-qayta urinaveradi.
        return 'Bir necha soatdan keyin urinib ko\'ring.';
      case 'quota-exceeded':
        return 'SMS chegarasi tugadi — keyinroq urinib ko\'ring';
      case 'network-request-failed':
        return 'Tarmoqqa ulanib bo\'lmadi';
      case 'app-not-authorized':
        // Odatda SHA-1/SHA-256 barmoq izi Firebase Console'da
        // qo'shilmagan bo'lganda chiqadi.
        return 'Ilova Firebase\'da tasdiqlanmagan (SHA barmoq izi qo\'shilmagan)';
      default:
        return e.message ?? 'Tasdiqlash amalga oshmadi';
    }
  }
}

/// Foydalanuvchiga ko'rsatish uchun tayyor xato.
class PhoneAuthFailure implements Exception {
  final String message;
  PhoneAuthFailure(this.message);
  @override
  String toString() => message;
}
