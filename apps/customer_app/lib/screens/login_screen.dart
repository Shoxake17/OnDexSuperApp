import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../services/firebase_phone.dart';
import '../services/otp_delivery.dart';
import '../session.dart';
import '../widgets/auth_flow.dart';
import '../widgets/auth_ui.dart';
import 'forgot_password_screen.dart';
import 'lock_gate.dart';
import 'register_screen.dart';

/// Kirish ekrani — `image/login.png` va `image/forgetpassword.png`
/// dizaynlari BITTA ekranga birlashtirilgan.
///
/// NEGA BITTA EKRAN: OTP alohida sahifa bo'lganda foydalanuvchi
/// kontekstni yo'qotadi (qaysi raqamga kod ketdi, orqaga qanday
/// qaytaman). Endi kod maydoni AYNAN parol maydonining o'rnida,
/// shu yerda ochiladi — hech qanday o'tish yo'q.
///
/// IKKI HOLAT:
///   * `_smsMode == false` — parol bilan kirish (`POST /auth/login`);
///   * `_smsMode == true`  — SMS kod bilan kirish (`POST /auth/verify`).
///     Bu holat "Parolni unutdingizmi?" bosilganda yoki ro'yxatdan
///     o'tishdan qaytilganda yoqiladi.
///
/// Barcha ko'rinish bloklari `widgets/auth_ui.dart` dan — ro'yxatdan
/// o'tish ekrani bilan bir xil, hech narsa ko'chirilmagan.
class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen>
    with ResendTimerMixin<LoginScreen>, SocialAuthMixin<LoginScreen> {
  final _phone = TextEditingController();
  final _email = TextEditingController();
  final _password = TextEditingController();
  final _otpKey = GlobalKey<OtpInputState>();

  AuthMethod _method = AuthMethod.phone;
  bool _showPassword = false;
  bool _busy = false;

  /// Kod tasdiqlash holati.
  ///
  /// FAQAT RO'YXATDAN O'TISHDAN keyin yoqiladi — yangi telefon yoki
  /// emailni tasdiqlash uchun. KIRISHDA umuman ishlatilmaydi: kirish
  /// endi sof telefon/email + parol (foydalanuvchi qarori). Shu sabab
  /// bu holatga boshqa hech qanday yo'l yo'q.
  bool _smsMode = false;
  String _code = '';

  /// Tasdiqlash EMAIL orqali ketyaptimi (telefon emas). Ro'yxatdan
  /// o'tish ekrani qaysi usulni tanlaganiga qarab o'rnatiladi.
  bool _verifyByEmail = false;

  /// Firebase `verificationId` — FAQAT Firebase kanalida to'ladi.
  String? _verificationId;

  /// Kod QAYSI kanal orqali yuborilgani.
  ///
  /// Tekshirish usuli shunga bog'liq: Telegram va server SMS'da kod
  /// BIZNING do'konimizda (`/auth/verify`), Firebase'da esa
  /// Firebase'ning o'zida (`verificationId` + `/auth/firebase`).
  /// Qarang: `services/otp_delivery.dart`.
  OtpChannel? _otpChannel;

  /// Ro'yxatdan o'tishda kiritilgan ism — tasdiqdan KEYIN saqlanadi
  /// (`POST /me`), chunki akkaunt shu paytda yaratiladi.
  String _pendingFirstName = '';
  String _pendingLastName = '';

  @override
  void initState() {
    super.initState();
    // Telegramda tasdiqlab qaytgan bo'lsa, kirishni SHU YERDA
    // yakunlaymiz. Kerak bo'lish sababi `resumeTelegramLoginIfAny`
    // izohida: ilova fonda o'ldirilgan bo'lishi mumkin.
    resumeTelegramLoginIfAny();
  }

  @override
  void dispose() {
    for (final c in [_phone, _email, _password]) {
      c.dispose();
    }
    // Taymer `ResendTimerMixin.dispose` da bekor qilinadi.
    super.dispose();
  }

  String get _login => _method == AuthMethod.phone
      ? AuthPhoneField.fullPhone(_phone)
      : _email.text.trim();


  Future<void> _goHome() async {
    await tokenStore.write(api.token!);
    // ┌─ POSTHOG IDENTIFY: BARCHA LOGIN USULLARI UCHUN ──────────┐
    // identify() faqat `ApiClient.me()` ichida chaqiriladi va u
    // PostHog person yaratadi. Password/SMS/Firebase/email har
    // qaysi usul bilan kirganidan qat'iy nazar, shu yerda bitta
    // so'rov orqali identify() bajariladi. Aks holda mijoz keyin
    // ilovani qayta ochmaguncha admin panel PostHog da "Person
    // not found" chiqadi.
    //
    // Xato chiqsa ilova ishlashiga ta'sir qilmasligi kerak.
    // └──────────────────────────────────────────────────────────┘
    try {
      await api.me();
    } catch (_) {}
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LockedHome()),
      (route) => false,
    );
  }

  /// Parol bilan kirish.
  ///
  /// Bo'sh maydonlar SERVERGA UMUMAN yuborilmaydi. Sabab: bo'sh parol
  /// bilan yuborilgan so'rov ham tezlik cheklovidan bitta "urinish"
  /// yeb qo'yardi va foydalanuvchi hech nima qilmasdan turib
  /// "juda ko'p urinish" xatosiga qamalardi. Ustiga server javobi
  /// ("telefon/email yoki parol noto'g'ri") bu holatda chalg'ituvchi —
  /// aslida shunchaki maydon to'ldirilmagan.
  Future<void> _loginWithPassword() async {
    if (_method == AuthMethod.phone && _phone.text.trim().length < 9) {
      snack('Telefon raqamini to\'liq kiriting');
      return;
    }
    if (_method == AuthMethod.email && _email.text.trim().isEmpty) {
      snack('Email manzilini kiriting');
      return;
    }
    if (_password.text.isEmpty) {
      snack('Parolni kiriting');
      return;
    }
    setState(() => _busy = true);
    try {
      await api.loginWithPassword(_login, _password.text);
      await _goHome();
    } catch (e) {
      snack(authErrorText(e), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// SMS kodni QAYTA yuborish (ro'yxatdan o'tishni tasdiqlash ekrani).
  ///
  /// FAQAT `_smsMode` ichidan chaqiriladi — kirish oqimida SMS umuman
  /// ishlatilmaydi. "Parolni unutdingizmi?" havolasi ATAYLAB olib
  /// tashlangan: u SMS kod yuborardi, ya'ni parolni bilmasdan ham
  /// kirish yo'lini ochardi va parolning ma'nosini yo'qotardi.
  /// Parolni tiklash alohida funksiya sifatida keyinroq quriladi.
  Future<void> _requestSmsCode() async {
    if (!_verifyByEmail && _phone.text.trim().length < 9) {
      snack('Telefon raqamini to\'liq kiriting');
      return;
    }
    setState(() => _busy = true);
    try {
      if (_verifyByEmail) {
        final devCode = await api.requestEmailCode(_email.text.trim());
        if (!mounted) return;
        setState(() {
          _smsMode = true;
          _code = '';
        });
        startResendTimer();
        if (devCode != null) {
          // Dev qulayligi (SMTP sozlanmagan holat) — kodni qo'lda
          // ko'chirish shart emas. Resend ulangach server buni
          // qaytarmaydi va bu blok ishlamaydi.
          WidgetsBinding.instance.addPostFrameCallback(
              (_) => _otpKey.currentState?.fill(devCode));
        }
        return;
      }
      // TELEFON — kod ZANJIR bo'ylab yuboriladi (Telegram -> Firebase
      // -> server SMS). Qayta yuborishda kanal boshqacha bo'lishi
      // mumkin, shuning uchun u har safar yangilanadi.
      final ticket = await OtpDelivery.send(_login);
      if (!mounted) return;
      setState(() {
        _smsMode = true;
        _code = '';
        _verificationId = ticket.verificationId;
        _otpChannel = ticket.channel;
      });
      startResendTimer();
      _otpKey.currentState?.fill('');
    } catch (e) {
      snack(authErrorText(e), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// RO'YXATDAN O'TISHNI tasdiqlash: SMS kod + parolni o'rnatish.
  ///
  /// Bu ekran FAQAT ro'yxatdan o'tishdan keyin ochiladi — kirish
  /// oqimida SMS umuman ishlatilmaydi.
  ///
  /// NEGA PAROL AYNAN SHU YERDA: server uni ro'yxatdan o'tishda
  /// ATAYLAB saqlamaydi — begona odam sizning raqamingiz bilan
  /// ro'yxatdan o'tib, o'z parolini qo'yib qo'ya olardi va siz SMS
  /// kodni kiritganingizdan keyin u sizning akkauntingizga o'sha
  /// parol bilan kirardi. Parolni faqat tasdiqlangan token egasi
  /// qo'yadi, ya'ni tasdiqlashdan KEYIN.
  Future<void> _verifyCode() async {
    if (_code.length != 6) {
      snack('6 xonali kodni to\'liq kiriting');
      return;
    }
    final password = _password.text;
    // Parol MAJBURIY: busiz akkaunt parolsiz qolib, keyin kirishning
    // yo'li bo'lmaydi (kirish endi faqat parol bilan).
    if (password.isEmpty) {
      snack('Parolni kiriting');
      return;
    }
    // 8 — serverdagi `MinPasswordLength`. Kodni bekor sarflab,
    // keyin 400 olishdan ko'ra shu yerda aytgan ma'qul: kod bir
    // martalik va tekshirilgach o'chadi.
    if (password.runes.length < 8) {
      snack('Parol kamida 8 belgidan iborat bo\'lsin');
      return;
    }
    setState(() => _busy = true);
    try {
      if (_verifyByEmail) {
        await api.verifyEmail(_email.text.trim(), _code);
      } else {
        // Tekshirish usuli KANALGA bog'liq (`otp_delivery.dart`):
        //   * Firebase — kod Firebase'da, natijada ID token olinadi;
        //   * Telegram / server SMS — kod BIZNING do'konimizda,
        //     ya'ni odatdagi `/auth/verify`.
        if (_otpChannel == OtpChannel.firebase) {
          final vid = _verificationId;
          if (vid == null) {
            snack('Kod muddati tugadi — qayta yuboring', error: true);
            return;
          }
          final idToken = await FirebasePhoneAuth.idTokenFor(vid, _code);
          await api.loginWithFirebase(idToken);
        } else {
          await api.verify(_login, _code);
        }
        // Ism akkaunt YARATILGANDAN keyin saqlanadi: telefon oqimida
        // akkaunt aynan shu qadamda paydo bo'ladi.
        if (_pendingFirstName.isNotEmpty) {
          try {
            await api.updateName(_pendingFirstName, _pendingLastName);
          } on ApiException catch (e) {
            snack('Kirdingiz, lekin ism saqlanmadi: ${e.message}',
                error: true);
          }
        }
      }
      try {
        await api.setPassword(password: password, passwordConfirm: password);
      } on ApiException catch (e) {
        // Kirish MUVAFFAQIYATLI bo'ldi — parol o'rnatilmagani uni
        // bekor qilmaydi. Foydalanuvchini tizimdan quvib
        // chiqarmaymiz, shunchaki aniq aytamiz.
        snack('Kirdingiz, lekin parol o\'rnatilmadi: ${e.message}',
            error: true);
      }
      await _goHome();
    } on PhoneAuthFailure catch (e) {
      snack(e.message, error: true);
    } catch (e) {
      snack(authErrorText(e), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Ro'yxatdan o'tishga o'tadi. Telefon bilan ro'yxatdan o'tilsa,
  /// u ekran raqam va dev kodni QAYTARADI — shunda kod maydoni SHU
  /// yerda ochiladi, alohida OTP sahifasi kerak bo'lmaydi.
  Future<void> _openRegister() async {
    final result = await Navigator.of(context).push<Map<String, String?>>(
      MaterialPageRoute(builder: (_) => const RegisterScreen()),
    );
    if (!mounted || result == null) return;
    final phone = result['phone'];
    final email = result['email'];
    if (phone == null && email == null) return;
    if (phone != null) {
      // "+998" prefiksini olib tashlab, maydonga faqat raqamni qo'yamiz.
      _phone.text = phone.replaceFirst('+998', '');
    } else {
      _email.text = email!;
    }
    // Ro'yxatdan o'tishda kiritilgan parol maydonga OLDINDAN
    // qo'yiladi — foydalanuvchi uni qayta terishi shart emas, lekin
    // ko'rib turadi va xohlasa o'zgartiradi. Tasdiqlashdan keyin
    // aynan shu qiymat o'rnatiladi (`_verifyCode` izohiga qarang).
    _password.text = result['password'] ?? '';
    setState(() {
      _verifyByEmail = phone == null;
      _method = _verifyByEmail ? AuthMethod.email : AuthMethod.phone;
      _smsMode = true;
      _code = '';
      _verificationId = result['verification_id'];
      _otpChannel = OtpChannel.values
          .where((c) => c.name == result['otp_channel'])
          .firstOrNull;
      _pendingFirstName = result['first_name'] ?? '';
      _pendingLastName = result['last_name'] ?? '';
    });
    startResendTimer();
    final dev = result['dev_code'];
    if (dev != null) {
      WidgetsBinding.instance
          .addPostFrameCallback((_) => _otpKey.currentState?.fill(dev));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: authBg,
      resizeToAvoidBottomInset: false,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(18, 4, 18, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AuthHeader(
                title: _smsMode ? 'Tasdiqlash kodi' : 'Xush kelibsiz!',
                subtitle: _smsMode
                    ? 'SMS orqali yuborilgan 6 xonali kodni kiriting'
                    : 'OnDex super appga kiring',
              ),
              const SizedBox(height: 14),
              // ── EMAIL ORQALI KIRISH — VAQTINCHA O'CHIRILGAN ──────
              //
              // Telefon YAGONA kimlik bo'lib qoldi. Sabab amaliy:
              // email bilan ochilgan akkauntda `users.phone` bo'sh
              // qoladi (migration 0027), kuryer ilovasidagi
              // "Qo'ng'iroq qilish" esa aynan shu maydonni oladi
              // (`httpapi.customerPhoneFor`). Ya'ni email registratsiya
              // XIZMAT KO'RSATIB BO'LMAYDIGAN akkaunt yaratardi.
              //
              // Backend (`/auth/email/*`, SMTP/Resend) TEGILMAGAN — u
              // chek/bildirishnoma va kelajakdagi xodim panellari
              // uchun kerak. Emailni yaxshi ko'radigan foydalanuvchi
              // pastdagi Google tugmasi orqali kiradi (manzil
              // tasdiqlangan holda keladi).
              //
              // QAYTARISH: shu blokni va `AuthTabs` ni izohdan
              // chiqarish yetarli — `_method`, `_email`,
              // `_verifyByEmail` mantig'i joyida turibdi.
              //
              // if (!_smsMode) ...[
              //   AuthTabs(
              //     value: _method,
              //     onChanged: (m) => setState(() => _method = m),
              //   ),
              //   const SizedBox(height: 14),
              // ],
              // if (_method == AuthMethod.phone)
              //   AuthLabelled(
              //       label: 'Telefon raqami',
              //       child: AuthPhoneField(controller: _phone))
              // else
              //   AuthLabelled(
              //     label: 'Email',
              //     child: AuthField(
              //       controller: _email,
              //       hint: 'email@example.com',
              //       icon: Icons.mail_outline,
              //       keyboardType: TextInputType.emailAddress,
              //     ),
              //   ),
              AuthLabelled(
                label: 'Telefon raqami',
                child: AuthPhoneField(controller: _phone),
              ),
              const SizedBox(height: 12),

              // ---- Parol maydoni HAR IKKALA holatda ham ----
              //
              // Kod kutilayotganda ham ko'rsatiladi: tartib
              // Telefon -> Parol -> Kod. Sabab shunchaki tartib emas —
              // server parolni ro'yxatdan o'tishda SAQLAMAYDI, u
              // tasdiqlashdan KEYIN o'rnatiladi, ya'ni bu maydon aynan
              // shu ekranda o'z ishini bajaradi (`_verifyCode` izohiga
              // qarang). Avval u yashirilgan edi va parol ko'rinmas
              // holda tashilardi.
              AuthLabelled(
                label: 'Parol',
                child: AuthField(
                  controller: _password,
                  hint: 'Parolni kiriting',
                  icon: Icons.lock_outline,
                  obscure: !_showPassword,
                  trailing: AuthEyeButton(
                    visible: _showPassword,
                    onTap: () => setState(() => _showPassword = !_showPassword),
                  ),
                ),
              ),
              // "Parolni unutdingizmi?" — SHU YERDA hech qanday SMS
              // YUBORILMAYDI (avval shunday edi va parolni bilmasdan
              // kirish yo'lini ochib qo'yardi). Endi u faqat alohida
              // tiklash ekranini ochadi.
              if (!_smsMode) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerRight,
                  child: GestureDetector(
                    onTap: _busy
                        ? null
                        : () => Navigator.of(context).push(MaterialPageRoute(
                            builder: (_) => const ForgotPasswordScreen())),
                    child: const Padding(
                      padding: EdgeInsets.symmetric(vertical: 4),
                      child: Text('Parolni unutdingizmi?',
                          style: TextStyle(color: authMuted, fontSize: 13)),
                    ),
                  ),
                ),
              ],
              if (_smsMode) ...[
                const SizedBox(height: 12),
                AuthLabelled(
                  label: '6 xonali kodni kiriting',
                  child: OtpInput(
                    key: _otpKey,
                    onChanged: (v) => setState(() => _code = v),
                    // To'liq kiritilishi bilan avtomatik yuboriladi.
                    onCompleted: (_) => _verifyCode(),
                  ),
                ),
                const SizedBox(height: 10),
                ResendRow(
                  left: resendLeft,
                  clock: resendClock,
                  onResend:
                      (_busy || resendLeft > 0) ? null : _requestSmsCode,
                ),
              ],

              const SizedBox(height: 10),
              AuthButton(
                label: _smsMode ? 'Davom etish' : 'Kirish',
                busy: _busy,
                onPressed: _smsMode ? _verifyCode : _loginWithPassword,
              ),
              const SizedBox(height: 16),
              const AuthOrDivider(),
              const SizedBox(height: 12),
              AuthSocialRow(
                busy: socialBusy != null,
                busyOn: socialBusy,
                onGoogle: signInWithGoogle,
                onTelegram: signInWithTelegram,
              ),
              const SizedBox(height: 18),
              AuthBottomLink(
                question: 'Hisobingiz yo\'qmi?',
                action: 'Ro\'yxatdan o\'tish',
                onTap: _openRegister,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
