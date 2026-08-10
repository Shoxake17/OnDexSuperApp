import 'package:firebase_auth/firebase_auth.dart';
import 'package:google_sign_in/google_sign_in.dart';

import '../api.dart';

/// Google hisobi bilan kirish / ro'yxatdan o'tish.
///
/// ── OQIM ───────────────────────────────────────────────────────────
///   1. Google hisob tanlash oynasi ochiladi (`authenticate`);
///   2. Google ID token olinadi;
///   3. u Firebase'ga beriladi va Firebase O'Z ID tokenini qaytaradi;
///   4. Firebase tokeni backendga yuboriladi (`POST /auth/google`).
///
/// ── NEGA FIREBASE ORQALI ───────────────────────────────────────────
/// Google tokenini to'g'ridan-to'g'ri backendga yuborish ham mumkin
/// edi, lekin u holda serverda IKKINCHI, alohida tekshiruvchi yozish
/// kerak bo'lardi (boshqa `aud`, boshqa kalitlar). Firebase orqali
/// o'tkazilganda esa MAVJUD `internal/firebaseauth` tekshiruvchisi
/// o'zgarishsiz ishlaydi — imzo, `aud`, `iss`, `exp`, algoritm
/// tekshiruvlari telefon oqimi bilan AYNAN bir xil kod bo'lib qoladi.
/// Kamroq kod = kamroq xato qilinadigan joy.
///
/// ── XAVFSIZLIK ─────────────────────────────────────────────────────
/// Email ilovadan YUBORILMAYDI. Backend uni imzosi tekshirilgan token
/// ICHIDAN oladi va qo'shimcha ravishda `sign_in_provider == google.com`
/// hamda `email_verified == true` ekanini talab qiladi
/// (`firebaseauth.Token.RequireGoogleEmail`).
class GoogleAuth {
  GoogleAuth._();

  static bool _initialized = false;

  /// Google hisobini tanlaydi va backendga kiradi.
  ///
  /// Bekor qilinsa (foydalanuvchi oynani yopsa) `null` qaytaradi —
  /// bu XATO EMAS, shuning uchun chaqiruvchi xabar ko'rsatmaydi.
  static Future<Map<String, dynamic>?> signIn() async {
    try {
      if (!_initialized) {
        // `serverClientId` ATAYLAB berilmaydi: Android'da plagin uni
        // `google-services.json` dan (`default_web_client_id`)
        // avtomatik oladi. Uni qo'lda yozish — yana bitta joyda
        // qotib qolgan qiymat va yana bitta mos kelmaslik manbai.
        await GoogleSignIn.instance.initialize();
        _initialized = true;
      }

      final account = await GoogleSignIn.instance.authenticate();
      final idToken = account.authentication.idToken;
      if (idToken == null || idToken.isEmpty) {
        throw GoogleAuthFailure(
            'Google hisobidan token olinmadi. Firebase Console\'da '
            'Google provayderi yoqilganini tekshiring.');
      }

      // Google tokeni -> Firebase hisobi -> Firebase ID tokeni.
      final cred = GoogleAuthProvider.credential(idToken: idToken);
      final res = await FirebaseAuth.instance.signInWithCredential(cred);
      final firebaseToken = await res.user?.getIdToken();
      if (firebaseToken == null || firebaseToken.isEmpty) {
        throw GoogleAuthFailure('Tasdiqlash amalga oshmadi');
      }

      return await api.loginWithGoogle(firebaseToken);
    } on GoogleSignInException catch (e) {
      // Foydalanuvchi oynani yopdi — bu xato emas.
      if (e.code == GoogleSignInExceptionCode.canceled) return null;
      throw GoogleAuthFailure(_message(e));
    } on FirebaseAuthException catch (e) {
      throw GoogleAuthFailure(_firebaseMessage(e));
    } on ApiException {
      rethrow; // backend xabari allaqachon o'zbekcha
    }
  }

  static String _message(GoogleSignInException e) {
    switch (e.code) {
      case GoogleSignInExceptionCode.canceled:
        return 'Bekor qilindi';
      case GoogleSignInExceptionCode.interrupted:
        return 'Ulanish uzildi — qayta urinib ko\'ring';
      case GoogleSignInExceptionCode.clientConfigurationError:
        // Odatda: Firebase Console'da Google provayderi yoqilmagan
        // yoki SHA-1 barmoq izi qo'shilmagan.
        return 'Google kirish sozlanmagan (Firebase Console\'da '
            'Google provayderini yoqing va SHA-1 qo\'shing)';
      default:
        return e.description ?? 'Google orqali kirib bo\'lmadi';
    }
  }

  static String _firebaseMessage(FirebaseAuthException e) {
    switch (e.code) {
      case 'account-exists-with-different-credential':
        return 'Bu email boshqa usul bilan ro\'yxatdan o\'tgan';
      case 'network-request-failed':
        return 'Tarmoqqa ulanib bo\'lmadi';
      default:
        return e.message ?? 'Tasdiqlash amalga oshmadi';
    }
  }

  /// Chiqishda Google sessiyasini ham yopadi — aks holda keyingi
  /// safar hisob tanlash oynasi ko'rsatilmay, eskisi ishlatilardi.
  static Future<void> signOut() async {
    try {
      await GoogleSignIn.instance.signOut();
    } catch (_) {
      // Chiqishga xalaqit bermaydi.
    }
  }
}

/// Foydalanuvchiga ko'rsatish uchun tayyor xato.
class GoogleAuthFailure implements Exception {
  final String message;
  GoogleAuthFailure(this.message);
  @override
  String toString() => message;
}
