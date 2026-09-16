/// Auth ekranlarining UMUMIY qurilish bloklari.
///
/// NEGA AJRATILDI: `register_screen`, `login_screen` va OTP ekrani
/// dizaynda deyarli bir xil — bir xil sarlavha, bir xil tab'lar, bir xil
/// kiritish maydonlari, bir xil tugma va ijtimoiy qator. Ular har bir
/// faylda qaytadan yozilsa, keyingi har bir tuzatishni uch marta qilish
/// kerak bo'lardi (loyihada bu xato allaqachon uchragan: `401` ishlash
/// va HTTP timeout to'rtta ilovada ham yo'q edi, chunki API klienti 4
/// marta ko'chirilgan edi).
///
/// Endi ranglar, o'lchamlar va xatti-harakat BITTA joyda.
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'brand_icons.dart';

// ---------- Ranglar (bitta manba) ----------
const authBrand = Color(0xFFF64E03);
const authBg = Color(0xFFFDFBFA);
const authSurface = Colors.white;
const authBorder = Color(0xFFEDE5DF);
const authHint = Color(0xFFA8A29D);
const authText = Color(0xFF1A1A1A);
const authMuted = Color(0xFF7C7671);

/// Kirish usuli — telefon yoki email.
enum AuthMethod { phone, email }

/// Ekran tepasi: orqaga tugmasi, brend nomi, illyustratsiya, sarlavha.
class AuthHeader extends StatelessWidget {
  final String title;
  final String subtitle;
  const AuthHeader({super.key, required this.title, required this.subtitle});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        // `Expanded` — matn kengligi + rasm kengligi ekranga sig'masa,
        // matn qisqaradi. Busiz o'ng chekkada ortiqcha bo'lak chiqadi.
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Material(
                color: authSurface,
                borderRadius: BorderRadius.circular(11),
                child: InkWell(
                  borderRadius: BorderRadius.circular(11),
                  onTap: () => Navigator.of(context).maybePop(),
                  child: const SizedBox(
                    width: 38,
                    height: 38,
                    child: Icon(Icons.arrow_back, color: authText, size: 20),
                  ),
                ),
              ),
              const SizedBox(height: 10),
              const Text.rich(
                TextSpan(
                  style: TextStyle(
                      fontSize: 30,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.8),
                  children: [
                    TextSpan(text: 'On', style: TextStyle(color: authText)),
                    TextSpan(text: 'Dex', style: TextStyle(color: authBrand)),
                  ],
                ),
              ),
              // Logotip ostidagi kichik tavsif — brend shiori. Ilovaning
              // BIRINCHI ko'rinadigan ekrani shu, ya'ni bu yozuv OnDex
              // nima ekanini bir qatorda aytadi.
              const Text('Xalq ilovasi',
                  style: TextStyle(
                      fontSize: 12,
                      color: authMuted,
                      fontWeight: FontWeight.w500)),
              const SizedBox(height: 10),
              Text(title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 21,
                      fontWeight: FontWeight.w800,
                      color: authText)),
              const SizedBox(height: 2),
              Text(subtitle,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, color: authMuted)),
            ],
          ),
        ),
        const SizedBox(width: 6),
        Image.asset(
          'assets/ondex_hero.png',
          width: 126,
          fit: BoxFit.contain,
          errorBuilder: (_, __, ___) => const SizedBox(width: 126),
        ),
      ],
    );
  }
}

/// "Telefon raqami orqali" / "Email orqali" — YAGONA ramka ichida
/// yonma-yon. Chegara faqat tashqi chekkada.
class AuthTabs extends StatelessWidget {
  final AuthMethod value;
  final ValueChanged<AuthMethod> onChanged;
  const AuthTabs({super.key, required this.value, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    Widget tab(AuthMethod m, IconData icon, String label) {
      final active = value == m;
      return Expanded(
        child: GestureDetector(
          onTap: () => onChanged(m),
          child: Container(
            height: 42,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color:
                  active ? authBrand.withValues(alpha: 0.10) : Colors.transparent,
              borderRadius: BorderRadius.circular(9),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, size: 16, color: active ? authBrand : authHint),
                const SizedBox(width: 5),
                // `Flexible` + ellipsis — matn toshib chiqmasligi
                // kafolatlanadi (avval "RIGHT OVERFLOWED" chizig'i chiqardi).
                Flexible(
                  child: Text(label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                          color: active ? authBrand : authMuted,
                          fontWeight: FontWeight.w600,
                          fontSize: 12.5)),
                ),
              ],
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: authSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: authBorder),
      ),
      child: Row(children: [
        tab(AuthMethod.phone, Icons.smartphone, 'Telefon raqami orqali'),
        tab(AuthMethod.email, Icons.mail_outline, 'Email orqali'),
      ]),
    );
  }
}

/// Yorliq + maydon juftligi.
class AuthLabelled extends StatelessWidget {
  final String label;
  final Widget child;
  const AuthLabelled({super.key, required this.label, required this.child});

  @override
  Widget build(BuildContext context) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Padding(
            padding: const EdgeInsets.only(bottom: 5, left: 2),
            child: Text(label,
                style: const TextStyle(
                    fontSize: 12.5,
                    fontWeight: FontWeight.w600,
                    color: authText)),
          ),
          child,
        ],
      );
}

BoxDecoration _fieldBox() => BoxDecoration(
      color: authSurface,
      borderRadius: BorderRadius.circular(11),
      border: Border.all(color: authBorder),
    );

/// Oddiy kiritish maydoni (ism, email, parol).
class AuthField extends StatelessWidget {
  final TextEditingController controller;
  final String hint;
  final IconData icon;
  final bool obscure;
  final Widget? trailing;
  final TextInputType? keyboardType;
  final TextCapitalization textCapitalization;

  const AuthField({
    super.key,
    required this.controller,
    required this.hint,
    required this.icon,
    this.obscure = false,
    this.trailing,
    this.keyboardType,
    this.textCapitalization = TextCapitalization.none,
  });

  @override
  Widget build(BuildContext context) => Container(
        height: 46,
        decoration: _fieldBox(),
        child: TextField(
          controller: controller,
          obscureText: obscure,
          keyboardType: keyboardType,
          textCapitalization: textCapitalization,
          maxLength: 128,
          style: const TextStyle(fontSize: 14.5, color: authText),
          decoration: InputDecoration(
            counterText: '',
            hintText: hint,
            hintStyle: const TextStyle(color: authHint, fontSize: 14.5),
            prefixIcon: Icon(icon, color: authHint, size: 19),
            prefixIconConstraints:
                const BoxConstraints(minWidth: 36, minHeight: 36),
            suffixIcon: trailing,
            suffixIconConstraints:
                const BoxConstraints(minWidth: 40, minHeight: 40),
            border: InputBorder.none,
            isDense: true,
            contentPadding: const EdgeInsets.symmetric(vertical: 12),
          ),
        ),
      );
}

/// Parolni ko'rsatish/yashirish tugmasi.
class AuthEyeButton extends StatelessWidget {
  final bool visible;
  final VoidCallback onTap;
  const AuthEyeButton({super.key, required this.visible, required this.onTap});

  @override
  Widget build(BuildContext context) => IconButton(
        padding: EdgeInsets.zero,
        constraints: const BoxConstraints(minWidth: 40, minHeight: 40),
        icon: Icon(visible ? Icons.visibility_off : Icons.visibility,
            color: authHint, size: 19),
        onPressed: onTap,
      );
}

/// Bayroq + `+998` + raqam maydoni. Faqat 9 ta raqam qabul qiladi.
class AuthPhoneField extends StatelessWidget {
  final TextEditingController controller;
  const AuthPhoneField({super.key, required this.controller});

  /// Kiritilgan raqamdan to'liq xalqaro formatni quradi.
  static String fullPhone(TextEditingController c) =>
      '+998${c.text.replaceAll(RegExp(r'\D'), '')}';

  @override
  Widget build(BuildContext context) => Container(
        height: 46,
        decoration: _fieldBox(),
        child: Row(children: [
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 10),
            child: Row(children: [
              UzFlag(width: 22),
              SizedBox(width: 6),
              Text('+998',
                  style: TextStyle(
                      fontSize: 14.5,
                      fontWeight: FontWeight.w700,
                      color: authText)),
            ]),
          ),
          Container(width: 1, height: 24, color: authBorder),
          Expanded(
            child: TextField(
              controller: controller,
              keyboardType: TextInputType.phone,
              inputFormatters: [
                FilteringTextInputFormatter.digitsOnly,
                LengthLimitingTextInputFormatter(9),
              ],
              style: const TextStyle(fontSize: 14.5, color: authText),
              decoration: const InputDecoration(
                hintText: '90 123 45 67',
                hintStyle: TextStyle(color: authHint, fontSize: 14.5),
                border: InputBorder.none,
                isDense: true,
                contentPadding: EdgeInsets.symmetric(horizontal: 10),
              ),
            ),
          ),
        ]),
      );
}

/// Asosiy (to'q sariq) amal tugmasi.
class AuthButton extends StatelessWidget {
  final String label;
  final bool busy;
  final VoidCallback? onPressed;
  const AuthButton(
      {super.key, required this.label, required this.busy, this.onPressed});

  @override
  Widget build(BuildContext context) => SizedBox(
        width: double.infinity,
        height: 50,
        child: FilledButton(
          style: FilledButton.styleFrom(
            backgroundColor: authBrand,
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          ),
          // Ikki marta yuborishga qarshi: `busy` birinchi `await` dan
          // OLDIN sinxron o'rnatiladi (chaqiruvchi tomonda).
          onPressed: busy ? null : onPressed,
          child: busy
              ? const SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(
                      strokeWidth: 2.2, color: Colors.white))
              : Text(label,
                  style: const TextStyle(
                      fontSize: 15.5,
                      fontWeight: FontWeight.w700,
                      color: Colors.white)),
        ),
      );
}

/// "───── yoki ─────" ajratgichi.
class AuthOrDivider extends StatelessWidget {
  const AuthOrDivider({super.key});

  @override
  Widget build(BuildContext context) => const Row(children: [
        Expanded(child: Divider(color: authBorder, height: 1)),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 10),
          child: Text('yoki', style: TextStyle(color: authMuted, fontSize: 12.5)),
        ),
        Expanded(child: Divider(color: authBorder, height: 1)),
      ]);
}

/// Google / Apple / Telegram qatori.
///
/// Bu usullar hali ULANMAGAN (OAuth kalitlari kerak) — bosilganda
/// ANIQ xabar chiqadi, jimgina hech narsa qilmaydigan tugma bo'lmaydi.
class AuthSocialRow extends StatelessWidget {
  /// Google tugmasi bosilganda. `null` bo'lsa "hozircha ulanmagan"
  /// xabari chiqadi (Apple hozir shunday).
  final VoidCallback? onGoogle;

  /// Telegram tugmasi bosilganda.
  final VoidCallback? onTelegram;

  /// Ijtimoiy kirish jarayoni ketyaptimi — tugmalar bloklanadi.
  final bool busy;

  /// Qaysi tugmada aylanma ko'rsatiladi ('google' yoki 'telegram').
  final String? busyOn;

  const AuthSocialRow({
    super.key,
    this.onGoogle,
    this.onTelegram,
    this.busy = false,
    this.busyOn,
  });

  void _notReady(BuildContext context, String name) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text('$name orqali kirish hozircha ulanmagan'),
      behavior: SnackBarBehavior.floating,
    ));
  }

  @override
  Widget build(BuildContext context) {
    Widget btn(String label, Widget icon, {VoidCallback? onTap}) => Expanded(
          child: SizedBox(
            height: 46,
            child: OutlinedButton(
              style: OutlinedButton.styleFrom(
                backgroundColor: authSurface,
                side: const BorderSide(color: authBorder),
                padding: EdgeInsets.zero,
                // Material tugmalari standart holatda 48x48 minimal
                // teginish maydonini majburlaydi — busiz tashqi
                // balandlik 46 bo'lsa "BOTTOM OVERFLOWED" chiqadi.
                minimumSize: Size.zero,
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
              ),
              onPressed:
                  busy ? null : (onTap ?? () => _notReady(context, label)),
              child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                icon,
                const SizedBox(width: 6),
                Flexible(
                  child: Text(label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          color: authText,
                          fontWeight: FontWeight.w600,
                          fontSize: 12.5)),
                ),
              ]),
            ),
          ),
        );

    const spinner = SizedBox(
        width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2));

    return Row(children: [
      btn(
        'Google',
        busyOn == 'google' ? spinner : const GoogleIcon(size: 17),
        onTap: onGoogle,
      ),
      const SizedBox(width: 8),
      btn('Apple', const Icon(Icons.apple, size: 19, color: authText)),
      const SizedBox(width: 8),
      btn(
        'Telegram',
        busyOn == 'telegram' ? spinner : const TelegramIcon(size: 17),
        onTap: onTelegram,
      ),
    ]);
  }
}

/// Pastdagi "Savol? Havola" qatori.
class AuthBottomLink extends StatelessWidget {
  final String question;
  final String action;
  final VoidCallback onTap;
  const AuthBottomLink(
      {super.key,
      required this.question,
      required this.action,
      required this.onTap});

  @override
  Widget build(BuildContext context) => Center(
        child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
          Text('$question ',
              style: const TextStyle(color: authMuted, fontSize: 13.5)),
          GestureDetector(
            onTap: onTap,
            child: Text(action,
                style: const TextStyle(
                    color: authBrand,
                    fontWeight: FontWeight.w700,
                    fontSize: 13.5)),
          ),
        ]),
      );
}

/// 6 ta ALOHIDA katakli OTP kiritish maydoni (`image/forgetpassword.png`).
///
/// Xatti-harakat: raqam kiritilganda keyingi katakka avtomatik o'tadi,
/// Backspace bosilganda oldingisiga qaytadi, to'liq kiritilganda
/// [onCompleted] chaqiriladi. Bitta uzun maydonga qaraganda tez va
/// xatosiz — foydalanuvchi qaysi raqamda ekanini ko'rib turadi.
class OtpInput extends StatefulWidget {
  final int length;
  final ValueChanged<String> onChanged;
  final ValueChanged<String>? onCompleted;

  /// Birinchi katak ochilishi bilan fokus olsinmi.
  ///
  /// Kirish/tasdiqlash ekranida — HA: kod o'sha yerdagi yagona
  /// to'ldiriladigan narsa. Parolni tiklash ekranida — YO'Q: u yerda
  /// avval telefon raqami kerak, va ekran scroll bo'lgani uchun
  /// avtomatik fokus sahifani pastga surib, sarlavhani ko'rinmas
  /// qilib qo'yardi (klaviatura ham keraksiz ochilardi).
  final bool autofocus;

  const OtpInput({
    super.key,
    this.length = 6,
    required this.onChanged,
    this.onCompleted,
    this.autofocus = true,
  });

  @override
  State<OtpInput> createState() => OtpInputState();
}

class OtpInputState extends State<OtpInput> {
  late final List<TextEditingController> _c;
  late final List<FocusNode> _f;

  @override
  void initState() {
    super.initState();
    _c = List.generate(widget.length, (_) => TextEditingController());
    _f = List.generate(widget.length, (_) => FocusNode());
  }

  @override
  void dispose() {
    for (final x in _c) {
      x.dispose();
    }
    for (final x in _f) {
      x.dispose();
    }
    super.dispose();
  }

  String get value => _c.map((x) => x.text).join();

  /// Dev rejimda serverdan kelgan kodni dasturiy to'ldirish uchun.
  void fill(String code) {
    for (var i = 0; i < widget.length; i++) {
      _c[i].text = i < code.length ? code[i] : '';
    }
    setState(() {});
    widget.onChanged(value);
  }

  void _onChanged(int i, String v) {
    if (v.length > 1) {
      // Kodni yopishtirib qo'yish (paste) — hammasini tarqatamiz.
      final digits = v.replaceAll(RegExp(r'\D'), '');
      for (var k = 0; k < widget.length; k++) {
        _c[k].text = k < digits.length ? digits[k] : '';
      }
      _f[(digits.length.clamp(1, widget.length)) - 1].requestFocus();
    } else if (v.isNotEmpty && i < widget.length - 1) {
      _f[i + 1].requestFocus();
    }
    setState(() {});
    widget.onChanged(value);
    if (value.length == widget.length) {
      widget.onCompleted?.call(value);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: List.generate(widget.length, (i) {
        final filled = _c[i].text.isNotEmpty;
        final focused = _f[i].hasFocus;
        return Expanded(
          child: Padding(
            padding: EdgeInsets.only(right: i == widget.length - 1 ? 0 : 8),
            child: KeyboardListener(
              focusNode: FocusNode(skipTraversal: true),
              // Bo'sh katakda Backspace bosilsa oldingisiga qaytadi —
              // busiz foydalanuvchi qo'lda orqaga bosishga majbur bo'lardi.
              onKeyEvent: (e) {
                if (e is KeyDownEvent &&
                    e.logicalKey == LogicalKeyboardKey.backspace &&
                    _c[i].text.isEmpty &&
                    i > 0) {
                  _c[i - 1].clear();
                  _f[i - 1].requestFocus();
                  setState(() {});
                  widget.onChanged(value);
                }
              },
              child: Container(
                height: 54,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: authSurface,
                  borderRadius: BorderRadius.circular(11),
                  border: Border.all(
                    color: focused || filled ? authBrand : authBorder,
                    width: focused ? 1.6 : 1,
                  ),
                ),
                child: TextField(
                  controller: _c[i],
                  focusNode: _f[i],
                  autofocus: widget.autofocus && i == 0,
                  textAlign: TextAlign.center,
                  keyboardType: TextInputType.number,
                  inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                  style: const TextStyle(
                      fontSize: 21,
                      fontWeight: FontWeight.w700,
                      color: authText),
                  decoration: const InputDecoration(
                    counterText: '',
                    hintText: '–',
                    hintStyle: TextStyle(color: authHint, fontSize: 18),
                    border: InputBorder.none,
                    isDense: true,
                    contentPadding: EdgeInsets.zero,
                  ),
                  onChanged: (v) => _onChanged(i, v),
                  onTap: () => setState(() {}),
                ),
              ),
            ),
          ),
        );
      }),
    );
  }
}
