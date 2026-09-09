import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../screens/lock_gate.dart';
import '../services/firebase_phone.dart' show PhoneAuthFailure;
import '../services/google_auth.dart';
import '../services/otp_delivery.dart' show OtpDeliveryFailure;
import '../services/telegram_auth.dart';
import '../session.dart';
import 'auth_ui.dart';

/// Kirish / ro'yxatdan o'tish / parol tiklash ekranlarining UMUMIY
/// qismlari.
///
/// ── NEGA ALOHIDA FAYL ───────────────────────────────────────────────
/// Uch ekranda bir xil kod uch nusxada yotardi: `_snack`, qayta yuborish
/// taymeri (`_startTimer` + `_clock` + `_left` + `_timer`), email
/// regexi, taymer qatori va ijtimoiy kirish `try/catch` bloklari.
///
/// Bu shunchaki chiroyli emaslik emas edi — XAVFSIZLIK masalasi ham:
/// ijtimoiy kirish oqimlarida (`SocialAuthMixin`) fishingga qarshi
/// himoya bor va uni ekranlardan birida noto'g'ri ko'chirib yozish
/// o'sha ekrandagi butun himoyani bekor qilardi. Endi u BITTA joyda va
/// har uchala ekran shu yerdan foydalanadi.

/// Xato/xabar ko'rsatish — uchala ekranda bir xil ko'rinish.
/// Auth oqimidagi XATOLARNI foydalanuvchi matniga aylantiradi.
///
/// ┌─ NEGA UMUMIY ─────────────────────────────────────────────────────┐
/// Kod yuborish va tasdiqlash oltita joyda chaqiriladi (kirish,
/// ro'yxatdan o'tish, parolni tiklash, PIN tiklash…). Har birida AYNAN
/// bir xil to'rt bosqichli `catch` zanjiri qo'lda yozilgan edi:
///
///   on OtpDeliveryFailure / on PhoneAuthFailure / on ApiException /
///   catch (_) -> 'Serverga ulanib bo'lmadi — internetni tekshiring'
///
/// Oxirgi satr o'nta joyda so'zma-so'z takrorlanardi. Endi bitta joyda.
/// └───────────────────────────────────────────────────────────────────┘
String authErrorText(Object e) {
  if (e is OtpDeliveryFailure) return e.message;
  if (e is PhoneAuthFailure) return e.message;
  if (e is ApiException) return e.message;
  return 'Serverga ulanib bo\'lmadi — internetni tekshiring';
}

void authSnack(BuildContext context, String msg, {bool error = false}) {
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
    content: Text(msg),
    backgroundColor: error ? const Color(0xFFC62828) : null,
    behavior: SnackBarBehavior.floating,
  ));
}

/// Ataylab sodda — yakuniy hakam har doim server
/// (`users.emailRe` bilan bir xil naqsh).
bool looksLikeEmail(String v) =>
    RegExp(r'^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$').hasMatch(v);

/// Kodni qayta yuborish taymeri.
///
/// `dispose` da taymer MAJBURIY bekor qilinadi — aks holda ekran
/// yopilgandan keyin ham ishlab, `setState` chaqirishga urinadi.
mixin ResendTimerMixin<T extends StatefulWidget> on State<T> {
  static const resendSeconds = 60;

  int resendLeft = 0;
  Timer? _resendTimer;

  /// `mm:ss` ko'rinishidagi qolgan vaqt.
  String get resendClock {
    final m = (resendLeft ~/ 60).toString().padLeft(2, '0');
    final s = (resendLeft % 60).toString().padLeft(2, '0');
    return '$m:$s';
  }

  void startResendTimer() {
    _resendTimer?.cancel();
    setState(() => resendLeft = resendSeconds);
    _resendTimer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) {
        t.cancel();
        return;
      }
      setState(() => resendLeft--);
      if (resendLeft <= 0) t.cancel();
    });
  }

  void stopResendTimer() {
    _resendTimer?.cancel();
    setState(() => resendLeft = 0);
  }

  @override
  void dispose() {
    _resendTimer?.cancel();
    super.dispose();
  }
}

/// "Kod 00:45 da qayta yuboriladi" + "Kod qayta yuborish" qatori.
class ResendRow extends StatelessWidget {
  const ResendRow({
    super.key,
    required this.left,
    required this.clock,
    required this.onResend,
    this.idleLabel = 'Kodni qayta yuborish mumkin',
    this.actionLabel = 'Kod qayta yuborish',
  });

  final int left;
  final String clock;

  /// `null` — amal hozir mumkin emas (band yoki taymer ishlayapti).
  final VoidCallback? onResend;
  final String idleLabel;
  final String actionLabel;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Flexible(
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            const Icon(Icons.schedule, size: 15, color: authMuted),
            const SizedBox(width: 6),
            Flexible(
              child: Text(
                left > 0 ? 'Kod $clock da qayta yuboriladi' : idleLabel,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: authMuted, fontSize: 12.5),
              ),
            ),
          ]),
        ),
        GestureDetector(
          onTap: onResend,
          child: Text(
            actionLabel,
            style: TextStyle(
              color: left > 0 ? authHint : authBrand,
              fontWeight: FontWeight.w700,
              fontSize: 12.5,
            ),
          ),
        ),
      ],
    );
  }
}

/// Google / Telegram orqali kirish — uchala ekran uchun bitta nusxa.
mixin SocialAuthMixin<T extends StatefulWidget> on State<T> {
  /// 'google' / 'telegram' / `null`. Asosiy formaning `busy` holatidan
  /// MUSTAQIL: foydalanuvchi maydonlarni to'ldirmasdan ham bosishi
  /// mumkin.
  String? socialBusy;

  Future<void> signInWithGoogle() =>
      _run('google', () => GoogleAuth.signIn());

  Future<void> signInWithTelegram() async {
    if (!await TelegramAuth.isInstalled()) {
      if (!mounted) return;
      authSnack(context, 'Telegram o\'rnatilmagan', error: true);
      return;
    }
    if (!mounted) return;
    await _run('telegram', TelegramAuth.signIn);
  }

  /// SOVUQ START: foydalanuvchi Telegramda tasdiqlagan payt Android
  /// ilovani fonda o'ldirgan bo'lishi mumkin. Unda "OnDex'ga qaytish"
  /// tugmasi ilovani QAYTADAN ishga tushiradi va `signIn()` ni kutib
  /// turgan oqim endi mavjud bo'lmaydi.
  ///
  /// Busiz foydalanuvchi hamma narsani to'g'ri qilib turib, kirish
  /// ekranida qolib ketardi. Ekran ochilishida bir marta chaqiriladi.
  Future<void> resumeTelegramLoginIfAny() async {
    Map<String, dynamic>? res;
    try {
      res = await TelegramAuth.completeIfReturning();
    } catch (_) {
      return; // eskirgan/uzilgan so'rov jimgina tashlab yuboriladi
    }
    if (res == null || !mounted) return;
    await tokenStore.write(api.token!);
    // ┌─ POSTHOG IDENTIFY TELEGRAM RESUME UCHUN ─────────────────────┐
    // `_goHome()` da api.me() orqali identify() chaqiriladi — bu yerda
    // ham xuddi shu amal bajarilishi shart. Aks holda bu yo'ldan
    // kirgan mijoz uchun PostHog'da HECH QACHON person yaratilmaydi
    // va admin panel play button "Person not found" chiqaradi.
    //
    // Xato ilova ishlashiga to'sqin qilmasligi kerak.
    // └───────────────────────────────────────────────────────────────┘
    try {
      await api.me();
    } catch (_) {}
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LockedHome()),
      (route) => false,
    );
  }

  /// Ikkala oqim uchun umumiy qism: natija `null` bo'lsa (foydalanuvchi
  /// bekor qildi yoki tasdiqlamadi) hech qanday xato ko'rsatilmaydi —
  /// u fikridan qaytgan bo'lishi mumkin.
  Future<void> _run(
      String name, Future<Map<String, dynamic>?> Function() run) async {
    setState(() => socialBusy = name);
    try {
      final res = await run();
      if (res == null) {
        if (name == 'telegram' && mounted) {
          authSnack(context, 'Telegramda tasdiqlanmadi');
        }
        return;
      }
      await tokenStore.write(api.token!);
      // ┌─ POSTHOG IDENTIFY GOOGLE/TELEGRAM LOGIN UCHUN ──────────────┐
      // `api.me()` ichida identify() chaqiriladi va PostHog person
      // yaratiladi. Bu qilinmasa Google yoki Telegram orqali kirgan
      // mijozlar PostHog'da umuman yo'q bo'ladi — admin uchun bular
      // "play icon bosilganda person topilmadi" xatosi beradi.
      //
      // Bu _goHome() funksiyasidagi aniq mantiqning AYNAN NUSXASI.
      // └───────────────────────────────────────────────────────────────┘
      try {
        await api.me();
      } catch (_) {}
      if (!mounted) return;
      Navigator.of(context).pushAndRemoveUntil(
        MaterialPageRoute(builder: (_) => const LockedHome()),
        (route) => false,
      );
    } on GoogleAuthFailure catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } on TelegramAuthFailure catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } on ApiException catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } catch (_) {
      if (mounted) {
        authSnack(context, 'Serverga ulanib bo\'lmadi — internetni tekshiring',
            error: true);
      }
    } finally {
      if (mounted) setState(() => socialBusy = null);
    }
  }

}
