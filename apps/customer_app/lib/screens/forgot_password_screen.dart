import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../session.dart';
import '../services/firebase_phone.dart';
import '../services/otp_delivery.dart';
import '../widgets/auth_flow.dart';
import '../widgets/auth_ui.dart';
import 'lock_gate.dart';

/// Parolni tiklash — `image/forgetpassword.png` va
/// `image/email-forgetpassword.png` dizaynlari bo'yicha.
///
/// ── NEGA ALOHIDA EKRAN ─────────────────────────────────────────────
/// Avval "Parolni unutdingizmi?" kirish ekranining o'zida SMS kod
/// yuborardi va kod bilan kirib olish mumkin edi — ya'ni parolni
/// bilmasdan ham kirish yo'li ochiq qolardi va parolning ma'nosi
/// yo'qolardi. Endi kirish FAQAT telefon/email + parol, tiklash esa
/// mana shu alohida oqim.
///
/// ── XAVFSIZLIK ─────────────────────────────────────────────────────
/// Oqim mavjud, sinalgan chaqiruvlardan yig'ilgan — yangi hujum
/// yuzasi qo'shilmagan:
///
///   TELEFON:
///     1. Firebase SMS yuboradi va kodni O'ZI tekshiradi (kod bizning
///        serverimizga umuman yetib bormaydi);
///     2. `POST /auth/firebase` — backend ID tokenning IMZOSINI
///        tekshirib, raqamni TOKEN ICHIDAN oladi.
///   EMAIL:
///     1. `POST /auth/email/request-code` (IP + global cheklov ostida)
///     2. `POST /auth/email/verify`
///
///   IKKALASIDA HAM:
///     3. `POST /me/password` — yangi parol. Server joriy parolni
///        SO'RAMAYDI, chunki token tasdiqlangan va 15 daqiqadan yangi
///        (`users.PhoneProofWindow`).
///
/// TIKLASH HAVOLASI YARATILMAYDI — email'dagi "reset link" naqshi
/// ataylab ishlatilmadi: o'g'irlanadigan/qayta ishlatiladigan havola
/// umuman mavjud emas, faqat bir martalik kod.
///
/// MUVAFFAQIYATLI TIKLASHDAN KEYIN server BARCHA eski sessiyalarni
/// bekor qiladi (`/me/password` xatti-harakati) — bu aynan to'g'ri:
/// "parolimni unutdim" holatining ortida ko'pincha "akkauntim
/// buzilgan" turadi.
///
/// FOYDALANUVCHINI SANAB OLISH YO'Q: kod raqam ro'yxatda bor-yo'qligiga
/// QARAMASDAN yuboriladi va javob bir xil bo'ladi.
class ForgotPasswordScreen extends StatefulWidget {
  const ForgotPasswordScreen({super.key});

  @override
  State<ForgotPasswordScreen> createState() => _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends State<ForgotPasswordScreen>
    with ResendTimerMixin<ForgotPasswordScreen> {
  final _phone = TextEditingController();
  final _email = TextEditingController();
  final _password = TextEditingController();
  final _passwordConfirm = TextEditingController();
  final _otpKey = GlobalKey<OtpInputState>();

  // `final` EMAS — sabab `register_screen.dart` dagi o'sha izohda
  // (email tabi qaytarilganda `AuthTabs` uni yana o'zgartiradi).
  // ignore: prefer_final_fields
  AuthMethod _method = AuthMethod.phone;
  bool _showPassword = false;
  bool _showPasswordConfirm = false;
  bool _busy = false;

  String _code = '';

  /// Kod YUBORILGANMI. Busiz foydalanuvchi kodni kutmasdan
  /// "Parolni tiklash" ni bosib, tushunarsiz xato olardi.
  bool _codeSent = false;

  /// Kod ALLAQACHON tasdiqlanganmi (`/auth/verify` muvaffaqiyatli
  /// o'tgan).
  ///
  /// NEGA KERAK: SMS kod BIR MARTALIK — tasdiqlangach serverda
  /// o'chadi. Agar shundan keyin parol o'rnatish rad etilsa (masalan
  /// server "bu parol juda oson topiladi" desa), qayta urinishda
  /// kodni yana yuborish MUMKIN EMAS. Busiz foydalanuvchi yangi kod
  /// so'rashga majbur bo'lardi — OTP cheklovi esa juda qattiq
  /// (IP bo'yicha 5 ta, tiklanish ~5 daqiqada bitta). Endi tasdiq
  /// bir marta bajariladi va qayta urinishda faqat parol yuboriladi.
  bool _verified = false;

  /// Firebase `verificationId` — FAQAT Firebase kanalida to'ladi.
  String? _verificationId;

  /// Kod qaysi kanal orqali ketgani — tekshirish usulini belgilaydi.
  OtpChannel? _otpChannel;

  @override
  void dispose() {
    for (final c in [_phone, _email, _password, _passwordConfirm]) {
      c.dispose();
    }
    // Taymer `ResendTimerMixin.dispose` da bekor qilinadi.
    super.dispose();
  }

  String get _phoneFull => AuthPhoneField.fullPhone(_phone);

  /// SMS kod so'rash.
  ///
  /// Raqam to'liqligi SHU YERDA tekshiriladi: bu endpoint SMS cheklovi
  /// ostida (IP bo'yicha 5 ta portlash, tiklanish ~5 daqiqada bitta),
  /// ya'ni har bir bekor so'rov haqiqiy pul va foydalanuvchining
  /// keyingi urinishini bloklash demakdir.
  Future<void> _sendCode() async {
    final byPhone = _method == AuthMethod.phone;
    if (byPhone && _phone.text.trim().length < 9) {
      snack('Telefon raqamini to\'liq kiriting', error: true);
      return;
    }
    if (!byPhone && !looksLikeEmail(_email.text.trim())) {
      snack('Email manzilini to\'g\'ri kiriting', error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      if (byPhone) {
        // Kod ZANJIR bo'ylab yuboriladi: Telegram -> Firebase ->
        // server SMS (`services/otp_delivery.dart`).
        final ticket = await OtpDelivery.send(_phoneFull);
        if (!mounted) return;
        setState(() {
          _codeSent = true;
          _code = '';
          _verified = false;
          _verificationId = ticket.verificationId;
          _otpChannel = ticket.channel;
        });
        startResendTimer();
        snack(ticket.channel == OtpChannel.telegram
            ? 'Kod Telegram botga yuborildi'
            : 'Kod yuborildi');
        return;
      }
      // SMTP ulanmagan bo'lsa server ANIQ xato qaytaradi
      // ("email yuborish hali ulanmagan...") — u pastdagi
      // `ApiException` bloki orqali foydalanuvchiga ko'rsatiladi.
      final devCode = await api.requestEmailCode(_email.text.trim());
      if (!mounted) return;
      setState(() {
        _codeSent = true;
        _code = '';
        // Yangi kod — eski tasdiq kuchini yo'qotadi.
        _verified = false;
      });
      startResendTimer();
      if (devCode != null) {
        WidgetsBinding.instance.addPostFrameCallback(
            (_) => _otpKey.currentState?.fill(devCode));
      }
      snack('Kod yuborildi');
    } on PhoneAuthFailure catch (e) {
      snack(e.message, error: true);
    } catch (e) {
      snack(authErrorText(e), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Yakuniy qadam: kodni tasdiqlash + yangi parolni o'rnatish.
  String? _validate() {
    if (_method == AuthMethod.phone) {
      if (_phone.text.trim().length < 9) {
        return 'Telefon raqamini to\'liq kiriting';
      }
    } else if (!looksLikeEmail(_email.text.trim())) {
      return 'Email manzilini to\'g\'ri kiriting';
    }
    if (!_codeSent) return 'Avval "Kod yuborish" tugmasini bosing';
    if (_password.text.isEmpty) return 'Yangi parolni kiriting';
    // 8 — serverdagi `MinPasswordLength`. `runes` — server
    // `utf8.RuneCountInString` bilan sanaydi.
    if (_password.text.runes.length < 8) {
      return 'Parol kamida 8 belgidan iborat bo\'lsin';
    }
    if (_password.text != _passwordConfirm.text) return 'Parollar mos kelmadi';
    if (!_verified && _code.length != 6) return '6 xonali kodni to\'liq kiriting';
    return null;
  }

  Future<void> _reset() async {
    final problem = _validate();
    if (problem != null) {
      snack(problem, error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      // 1) Kod -> tasdiqlangan token (manzil egaligi isbotlandi).
      //    Kod bir martalik, shuning uchun faqat BIR MARTA yuboriladi.
      if (!_verified) {
        if (_method == AuthMethod.phone) {
          // Tekshirish usuli KANALGA bog'liq (`otp_delivery.dart`).
          if (_otpChannel == OtpChannel.firebase) {
            final vid = _verificationId;
            if (vid == null) {
              snack('Kod muddati tugadi — qayta yuboring', error: true);
              return;
            }
            final idToken = await FirebasePhoneAuth.idTokenFor(vid, _code);
            await api.loginWithFirebase(idToken);
          } else {
            await api.verify(_phoneFull, _code);
          }
        } else {
          await api.verifyEmail(_email.text.trim(), _code);
        }
        if (!mounted) return;
        setState(() => _verified = true);
      }
      // 2) Yangi parol. Joriy parol SO'RALMAYDI — token SMS bilan
      //    tasdiqlangan va yangi. Server barcha eski sessiyalarni
      //    bekor qilib, javobda yangi token beradi.
      await api.setPassword(
        password: _password.text,
        passwordConfirm: _passwordConfirm.text,
      );
      await tokenStore.write(api.token!);
      if (!mounted) return;
      snack('Parol yangilandi');
      Navigator.of(context).pushAndRemoveUntil(
        MaterialPageRoute(builder: (_) => const LockedHome()),
        (route) => false,
      );
    } catch (e) {
      snack(authErrorText(e), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final byPhone = _method == AuthMethod.phone;
    return Scaffold(
      backgroundColor: authBg,
      body: SafeArea(
        // Bu ekranda maydon ko'p (raqam + ikki parol + kod + banner),
        // shuning uchun kirish/ro'yxatdan o'tish ekranlaridan farqli
        // o'laroq SCROLL bor — aks holda kichik ekranlarda va
        // klaviatura ochilganda toshib ketardi.
        //
        // DIQQAT: bu yerda `Spacer` ISHLATILMAYDI — u cheksiz
        // balandlikdagi scroll ichida ekranni butunlay bo'sh qoldiradi
        // (avval aynan shu xato bo'lgan, `mijozapp.md` 6-bo'lim).
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(18, 4, 18, 18),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const AuthHeader(
                title: 'Parolni unutdingizmi?',
                subtitle:
                    'Parolni tiklash uchun telefon raqamingizni kiriting, sizga SMS orqali 6 xonali kod yuboriladi.',
              ),
              const SizedBox(height: 14),
              // ── EMAIL ORQALI TIKLASH — VAQTINCHA O'CHIRILGAN ──────
              //
              // Sabab `login_screen.dart` dagi izohda. Backend
              // (`/auth/email/request-code`, `/auth/email/verify`)
              // tegilmagan.
              //
              // QAYTARISH: shu blokni izohdan chiqarib, pastdagi
              // yakka telefon maydonini olib tashlash yetarli
              // (`_method`, `_email`, `byPhone` mantig'i joyida).
              //
              // AuthTabs(
              //   value: _method,
              //   onChanged: (m) => setState(() {
              //     _method = m;
              //     // Usul almashsa kod endi tegishli emas.
              //     _codeSent = false;
              //     _code = '';
              //     _verified = false;
              //     stopResendTimer();
              //   }),
              // ),
              // const SizedBox(height: 14),
              //
              // if (byPhone) ...[ ... ] else ...[
              //   AuthLabelled(
              //     label: 'Email manzilingiz',
              //     child: AuthField(
              //       controller: _email,
              //       hint: 'email@example.com',
              //       icon: Icons.mail_outline,
              //       keyboardType: TextInputType.emailAddress,
              //     ),
              //   ),
              // ],
              AuthLabelled(
                label: 'Telefon raqami',
                child: AuthPhoneField(controller: _phone),
              ),
              const Padding(
                padding: EdgeInsets.only(top: 5, left: 2),
                child: Text('Kod telefon raqamiga yuboriladi',
                    style: TextStyle(fontSize: 12, color: authMuted)),
              ),
              const SizedBox(height: 12),

              AuthLabelled(
                label: 'Yangi parol',
                child: AuthField(
                  controller: _password,
                  hint: 'Yangi parolni kiriting',
                  icon: Icons.lock_outline,
                  obscure: !_showPassword,
                  trailing: AuthEyeButton(
                    visible: _showPassword,
                    onTap: () => setState(() => _showPassword = !_showPassword),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              AuthLabelled(
                label: 'Parolni tasdiqlang',
                child: AuthField(
                  controller: _passwordConfirm,
                  hint: 'Parolni qayta kiriting',
                  icon: Icons.lock_outline,
                  obscure: !_showPasswordConfirm,
                  trailing: AuthEyeButton(
                    visible: _showPasswordConfirm,
                    onTap: () => setState(
                        () => _showPasswordConfirm = !_showPasswordConfirm),
                  ),
                ),
              ),
              const SizedBox(height: 12),

              AuthLabelled(
                label: '6 xonali kodni kiriting',
                child: OtpInput(
                  key: _otpKey,
                  // Bu ekranda AVTOMATIK FOKUS YO'Q: u sahifani darhol
                  // pastga surib, sarlavhani ko'rinmas qilardi va
                  // klaviaturani keraksiz ochardi. Bu yerda birinchi
                  // qadam — telefon raqamini kiritish.
                  autofocus: false,
                  onChanged: (v) => setState(() => _code = v),
                ),
              ),
              Padding(
                padding: const EdgeInsets.only(top: 5, left: 2),
                child: Text(
                  byPhone
                      ? 'SMS orqali yuborilgan kodni kiriting'
                      : 'Email manzilingizga yuborilgan kodni kiriting',
                  style: const TextStyle(fontSize: 12, color: authMuted),
                ),
              ),
              const SizedBox(height: 10),
              ResendRow(
                left: resendLeft,
                clock: resendClock,
                onResend: (_busy || resendLeft > 0) ? null : _sendCode,
                idleLabel:
                    _codeSent ? 'Kodni qayta yuborish mumkin' : 'Kod hali yuborilmadi',
                // Dizaynda faqat "Kod qayta yuborish" bor, lekin kodni
                // BIRINCHI marta yuborish uchun ham amal kerak —
                // shuning uchun yozuv holatga qarab o'zgaradi.
                actionLabel: _codeSent ? 'Kod qayta yuborish' : 'Kod yuborish',
              ),
              const SizedBox(height: 14),

              AuthButton(
                label: 'Parolni tiklash',
                busy: _busy,
                onPressed: _reset,
              ),
              // Dizayndagi "yoki" bloki (Google/Email orqali tiklash)
              // va pastdagi xavfsizlik banneri ATAYLAB QO'YILMADI —
              // foydalanuvchi so'rovi. Google OAuth baribir ulanmagan
              // edi, email tabi esa yuqoridagi tab orqali ochiladi.
            ],
          ),
        ),
      ),
    );
  }
}

