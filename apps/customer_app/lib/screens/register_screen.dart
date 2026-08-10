import 'package:flutter/material.dart';

import '../api.dart';
import '../services/firebase_phone.dart';
import '../services/otp_delivery.dart';
import '../widgets/auth_flow.dart';
import '../widgets/auth_ui.dart';

/// Ro'yxatdan o'tish ekrani — `image/register.png` dizayni bo'yicha.
///
/// BUTUN KO'RINISH `widgets/auth_ui.dart` dagi UMUMIY bloklardan
/// yig'ilgan — kirish ekrani bilan bir xil sarlavha, tab'lar, maydonlar
/// va tugmalar. Bu yerda faqat SHU ekranga xos mantiq bor.
///
/// TUZILISH QOIDASI: butun forma BITTA ekranga sig'adi, scroll YO'Q.
/// Shu sabab Ism va Familiya yonma-yon joylashtirilgan va xatolar
/// SnackBar orqali ko'rsatiladi (inline xato qutisi forma balandligini
/// o'zgartirib, "BOTTOM OVERFLOWED" xatosini keltirib chiqarardi).
///
/// OQIM (xavfsizlik uchun MUHIM):
///   forma -> `POST /auth/register` -> akkaunt TASDIQLANMAGAN holatda
///   yaratiladi, token BERILMAYDI -> SMS kod ekrani -> `POST /auth/verify`
///   -> token. Busiz istalgan odam begona raqam bilan akkaunt ochib,
///   raqam egasining ro'yxatdan o'tishini to'sib qo'yardi.
class RegisterScreen extends StatefulWidget {
  const RegisterScreen({super.key});

  @override
  State<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends State<RegisterScreen>
    with SocialAuthMixin<RegisterScreen> {
  final _phone = TextEditingController();
  final _email = TextEditingController();
  final _firstName = TextEditingController();
  final _lastName = TextEditingController();
  final _password = TextEditingController();
  final _passwordConfirm = TextEditingController();

  // `final` EMAS: email tabi vaqtincha izohga olingani uchun hozir
  // hech kim o'zgartirmaydi, lekin blok qaytarilganda `AuthTabs` uni
  // yana o'zgartiradi. `final` qilib qo'yilsa qaytarish paytida bu
  // qator ham tahrirlanishi kerak bo'lardi.
  // ignore: prefer_final_fields
  AuthMethod _method = AuthMethod.phone;
  bool _showPassword = false;
  bool _showPasswordConfirm = false;
  bool _busy = false;

  @override
  void dispose() {
    for (final c in [
      _phone,
      _email,
      _firstName,
      _lastName,
      _password,
      _passwordConfirm
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  void _snack(String msg, {bool error = false}) {
    if (!mounted) return;
    authSnack(context, msg, error: error);
  }

  /// Formani SERVERGA YUBORISHDAN OLDIN tekshiradi.
  ///
  /// NEGA SHART: `POST /auth/register` SMS cheklovi ostida — IP
  /// bo'yicha bor-yo'g'i 5 ta portlash, tiklanish ~5 daqiqada bitta.
  /// Avval bu ekranda tekshiruv UMUMAN yo'q edi, ya'ni "parollar mos
  /// kelmadi" kabi eng oddiy xato ham bitta tokenni yoqardi va
  /// formani besh marta xato to'ldirgan odam ~25 daqiqaga
  /// bloklanardi. Server tomonda ham tartib to'g'irlandi (tekshiruv
  /// cheklovdan oldin), bu yerda esa so'rov umuman yuborilmaydi.
  ///
  /// Xabar matnlari server bilan bir xil bo'lishi shart emas —
  /// yakuniy hakam har doim server (bu tekshiruv faqat qulaylik
  /// uchun, himoya emas).
  String? _validate() {
    if (_method == AuthMethod.phone) {
      if (_phone.text.trim().length < 9) return 'Telefon raqamini to\'liq kiriting';
    } else {
      final email = _email.text.trim();
      if (email.isEmpty) return 'Email manzilini kiriting';
      // Ataylab sodda: haqiqiy tekshiruv serverda. Bu yerda faqat
      // aniq noto'g'ri kiritma to'siladi.
      if (!looksLikeEmail(email)) return 'Email manzili noto\'g\'ri';
    }
    if (_firstName.text.trim().isEmpty) return 'Ismingizni kiriting';
    if (_password.text.isEmpty) return 'Parol yarating';
    // 8 — serverdagi `MinPasswordLength` bilan bir xil. `runes` —
    // server `utf8.RuneCountInString` bilan sanaydi, ya'ni kirill/
    // o'zbek harflari uchun natija bir xil bo'ladi.
    if (_password.text.runes.length < 8) {
      return 'Parol kamida 8 belgidan iborat bo\'lsin';
    }
    if (_password.text != _passwordConfirm.text) return 'Parollar mos kelmadi';
    return null;
  }

  // Google / Telegram oqimlari `SocialAuthMixin` da — kirish va parol
  // tiklash ekranlari bilan BITTA nusxa. U yerda Telegram tasdiq soni
  // ham ko'rsatiladi (fishingga qarshi to'siq), shuning uchun bu
  // mantiqni ekranlarga ko'chirib yozish XAVFLI.
  //
  // Forma bu oqimlar uchun UMUMAN to'ldirilmasa ham bo'ladi: ism
  // Google/Telegramdan keladi, parol esa kerak emas.

  Future<void> _submit() async {
    final problem = _validate();
    if (problem != null) {
      _snack(problem, error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      final byPhone = _method == AuthMethod.phone;
      final phone = AuthPhoneField.fullPhone(_phone);

      if (byPhone) {
        // TELEFON YO'LI — kod ZANJIR bo'ylab yuboriladi:
        // Telegram -> Firebase -> server SMS (`OtpDelivery`).
        //
        // Bizning `/auth/register` bu yerda CHAQIRILMAYDI: akkaunt
        // tasdiqdan keyin yaratiladi, ism va parol esa undan keyin
        // qo'yiladi. Sabab o'sha invariant: tasdiqlanmagan raqamga
        // akkaunt/parol yozilmasin.
        final ticket = await OtpDelivery.send(phone);
        if (!mounted) return;
        Navigator.of(context).pop({
          'phone': phone,
          // Kanal — kodni QAYSI usulda tekshirishni belgilaydi
          // (`OtpDelivery` izohiga qarang).
          'otp_channel': ticket.channel.name,
          'verification_id': ticket.verificationId,
          'password': _password.text,
          'first_name': _firstName.text.trim(),
          'last_name': _lastName.text.trim(),
        });
        return;
      }

      final devCode = await api.register(
        // Email rejimi — telefon UMUMAN so'ralmaydi.
        phone: '',
        email: _email.text.trim(),
        firstName: _firstName.text.trim(),
        lastName: _lastName.text.trim(),
        password: _password.text,
        passwordConfirm: _passwordConfirm.text,
      );
      if (!mounted) return;
      // Alohida OTP sahifasi YO'Q: manzil va dev kod kirish ekraniga
      // QAYTARILADI va kod maydoni o'sha yerda, parol maydonining
      // ostida ochiladi. Telefon va email uchun oqim BIR XIL.
      //
      // PAROL HAM QAYTARILADI: server uni ro'yxatdan o'tishda
      // ATAYLAB saqlamaydi (begona odam sizning raqamingiz/emailingiz
      // bilan ro'yxatdan o'tib, o'z parolini qo'yib, keyin kirib
      // olishi mumkin edi). Kirish ekrani tasdiqdan keyin uni
      // `POST /me/password` orqali o'rnatadi — ya'ni parolni faqat
      // manzilni tasdiqlagan odam qo'yadi.
      Navigator.of(context).pop({
        'email': _email.text.trim(),
        'dev_code': devCode,
        'password': _password.text,
      });
    } on OtpDeliveryFailure catch (e) {
      // Zanjirning HAMMA pog'onasi qulagan — sabab allaqachon
      // foydalanuvchi tilida.
      _snack(e.message, error: true);
    } on PhoneAuthFailure catch (e) {
      _snack(e.message, error: true);
    } on ApiException catch (e) {
      _snack(e.message, error: true);
    } catch (_) {
      _snack('Serverga ulanib bo\'lmadi — internetni tekshiring', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
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
              const AuthHeader(
                title: 'Ro\'yxatdan o\'tish',
                subtitle: 'OnDex super appda hisob yarating',
              ),
              const SizedBox(height: 10),
              // ── EMAIL BILAN RO'YXATDAN O'TISH — VAQTINCHA O'CHIQ ──
              //
              // Sabab `login_screen.dart` dagi o'sha izohda: email
              // bilan ochilgan akkauntda telefon bo'sh qoladi va
              // kuryer mijozga qo'ng'iroq qila olmaydi. Backend
              // (`/auth/email/*`) tegilmagan — u chek/bildirishnoma va
              // xodim panellari uchun qoladi.
              //
              // QAYTARISH: shu blokni izohdan chiqarib, pastdagi
              // yakka `AuthLabelled` ni olib tashlash yetarli.
              //
              // AuthTabs(
              //   value: _method,
              //   onChanged: (m) => setState(() => _method = m),
              // ),
              // const SizedBox(height: 12),
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
              const SizedBox(height: 10),
              // Ism va Familiya YONMA-YON — bitta ekranga sig'ishi uchun
              // eng katta yutuq shu (60 logik piksel tejaladi).
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: AuthLabelled(
                      label: 'Ism',
                      child: AuthField(
                        controller: _firstName,
                        hint: 'Ismingiz',
                        icon: Icons.person_outline,
                        textCapitalization: TextCapitalization.words,
                      ),
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: AuthLabelled(
                      label: 'Familiya',
                      child: AuthField(
                        controller: _lastName,
                        hint: 'Familiyangiz',
                        icon: Icons.person_outline,
                        textCapitalization: TextCapitalization.words,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              AuthLabelled(
                label: 'Parol',
                child: AuthField(
                  controller: _password,
                  hint: 'Parol yarating',
                  icon: Icons.lock_outline,
                  obscure: !_showPassword,
                  trailing: AuthEyeButton(
                    visible: _showPassword,
                    onTap: () => setState(() => _showPassword = !_showPassword),
                  ),
                ),
              ),
              const SizedBox(height: 10),
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
              const SizedBox(height: 14),
              AuthButton(
                  label: 'Ro\'yxatdan o\'tish',
                  busy: _busy,
                  onPressed: _submit),
              const SizedBox(height: 12),
              const AuthOrDivider(),
              const SizedBox(height: 10),
              AuthSocialRow(
                busy: socialBusy != null,
                busyOn: socialBusy,
                onGoogle: signInWithGoogle,
                onTelegram: signInWithTelegram,
              ),
              const SizedBox(height: 16),
              AuthBottomLink(
                question: 'Hisobingiz bormi?',
                action: 'Kirish',
                onTap: () => Navigator.of(context).maybePop(),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
