import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../services/app_pin.dart';
import '../widgets/auth_ui.dart';

/// Barmoq izi VA yuz tanish belgisi — bitta tugmada.
///
/// Flutter'da "face + fingerprint" uchun yagona ikonka yo'q, shuning
/// uchun ikkalasi yonma-yon chiziladi. Bu ataylab: qurilmada qaysi biri
/// sozlanganini ilova oldindan bila olmaydi (`isDeviceSupported` faqat
/// "biror himoya bor" deydi), shuning uchun ikkala imkoniyat ham
/// ko'rsatiladi — Click/Payme'dagi kabi.
class BiometricIcon extends StatelessWidget {
  const BiometricIcon({super.key, this.size = 26, this.color = authBrand});

  final double size;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.face_outlined, size: size * 0.92, color: color),
        SizedBox(width: size * 0.16),
        Icon(Icons.fingerprint, size: size, color: color),
      ],
    );
  }
}

/// PIN klaviaturasi — kiritish va yaratish ekranlari uchun umumiy.
///
/// TIZIM KLAVIATURASI ISHLATILMAYDI: o'z klaviaturamiz
///   * uchinchi tomon klaviaturalarini (ular kiritilgan matnni ko'radi
///     va ba'zilari uni bulutga yuboradi) chetlab o'tadi;
///   * ekranni surib yubormaydi;
///   * bank ilovalaridagi ko'rinishni beradi.
class _PinPad extends StatelessWidget {
  const _PinPad({required this.onDigit, required this.onBackspace, this.extra});

  final ValueChanged<String> onDigit;
  final VoidCallback onBackspace;

  /// Chap pastdagi qo'shimcha tugma (biometrika).
  final Widget? extra;

  @override
  Widget build(BuildContext context) {
    Widget key(String d) => _PinKey(label: d, onTap: () => onDigit(d));
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final row in const [
          ['1', '2', '3'],
          ['4', '5', '6'],
          ['7', '8', '9'],
        ])
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [for (final d in row) key(d)],
          ),
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            SizedBox(width: 92, height: 74, child: Center(child: extra)),
            key('0'),
            _PinKey(icon: Icons.backspace_outlined, onTap: onBackspace),
          ],
        ),
      ],
    );
  }
}

class _PinKey extends StatelessWidget {
  const _PinKey({this.label, this.icon, required this.onTap});

  final String? label;
  final IconData? icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 92,
      height: 74,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(40),
          onTap: () {
            HapticFeedback.lightImpact();
            onTap();
          },
          child: Center(
            child: icon != null
                ? Icon(icon, size: 26, color: authText)
                : Text(
                    label!,
                    style: const TextStyle(
                      fontSize: 34,
                      fontWeight: FontWeight.w400,
                      color: authText,
                    ),
                  ),
          ),
        ),
      ),
    );
  }
}

/// Nuqtalar + "ko'z" tugmasi.
///
/// KO'Z NEGA BOR: 6 xonali kodni terishda xato qilish oson va
/// foydalanuvchi nimani terganini ko'ra olmasa, butun kodni o'chirib
/// qaytadan boshlashga majbur bo'ladi. Click/Payme'da ham shunday.
///
/// XAVFSIZLIK: ochish FAQAT foydalanuvchining o'z harakati bilan
/// bo'ladi va holat hech qayerda saqlanmaydi — har safar yopiq
/// boshlanadi.
class _PinDots extends StatelessWidget {
  const _PinDots({
    required this.pin,
    required this.error,
    required this.revealed,
    this.onToggleReveal,
  });

  final String pin;
  final bool error;
  final bool revealed;
  final VoidCallback? onToggleReveal;

  @override
  Widget build(BuildContext context) {
    final color = error ? const Color(0xFFC62828) : authBrand;
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Ko'z tugmasi kengligicha bo'sh joy — nuqtalar ekran
        // MARKAZIDA qolsin (aks holda ular chapga siljib ketardi).
        const SizedBox(width: 44),
        ...List.generate(AppPin.pinLength, (i) {
          final on = i < pin.length;
          if (revealed && on) {
            return SizedBox(
              width: 26,
              child: Text(
                pin[i],
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w600,
                  color: color,
                ),
              ),
            );
          }
          return Container(
            width: 13,
            height: 13,
            margin: const EdgeInsets.symmetric(horizontal: 6.5),
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: on ? color : Colors.transparent,
              border: Border.all(
                color: error ? const Color(0xFFC62828) : authBorder,
                width: 1.6,
              ),
            ),
          );
        }),
        SizedBox(
          width: 44,
          child: onToggleReveal == null
              ? null
              : IconButton(
                  padding: EdgeInsets.zero,
                  visualDensity: VisualDensity.compact,
                  icon: Icon(
                    revealed ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                    size: 21,
                    color: authMuted,
                  ),
                  tooltip: revealed ? 'Yashirish' : 'Ko\'rsatish',
                  onPressed: onToggleReveal,
                ),
        ),
      ],
    );
  }
}

/// PIN KIRITISH — qulflangan holatdagi asosiy ekran.
///
/// Biometrika oynasi SHU EKRAN USTIDA ochiladi (`LockGate` boshqaradi).
/// Foydalanuvchi uni yopsa, orqada shu ekran qoladi va PIN terib
/// kiradi — aynan Click/Payme'dagi xatti-harakat.
class PinEntryScreen extends StatefulWidget {
  const PinEntryScreen({
    super.key,
    required this.onSubmit,
    required this.onForgot,
    this.onBiometric,
    this.onLogout,
    this.attemptsLeft,
    this.title = 'PIN kodni kiriting',
    this.subtitle = 'OnDex hisobingizga kirish uchun',
    this.forgotLabel = 'PIN kodni unutdingizmi?',
  });

  /// PIN to'liq terilganda chaqiriladi. `false` qaytsa maydon
  /// tozalanadi va xato ko'rsatiladi.
  final Future<bool> Function(String pin) onSubmit;

  /// "PIN kodni unutdingizmi?" — tiklash oqimi.
  final VoidCallback onForgot;

  /// Biometrikani QAYTA so'rash (foydalanuvchi oynani yopgan bo'lsa).
  /// `null` — qurilmada biometrika yo'q.
  final VoidCallback? onBiometric;

  /// O'ng yuqoridagi chiqish tugmasi. `null` — ko'rsatilmaydi.
  final VoidCallback? onLogout;

  final int? attemptsLeft;
  final String title;
  final String subtitle;
  final String forgotLabel;

  @override
  State<PinEntryScreen> createState() => _PinEntryScreenState();
}

class _PinEntryScreenState extends State<PinEntryScreen> {
  String _pin = '';
  bool _busy = false;
  bool _error = false;
  bool _reveal = false;

  Future<void> _add(String d) async {
    if (_busy || _pin.length >= AppPin.pinLength) return;
    setState(() {
      _pin += d;
      _error = false;
    });
    if (_pin.length != AppPin.pinLength) return;

    setState(() => _busy = true);
    final ok = await widget.onSubmit(_pin);
    if (!mounted) return;
    setState(() {
      _busy = false;
      if (!ok) {
        _pin = '';
        _error = true;
        _reveal = false;
      }
    });
    if (!ok) HapticFeedback.heavyImpact();
  }

  void _backspace() {
    if (_busy || _pin.isEmpty) return;
    setState(() {
      _pin = _pin.substring(0, _pin.length - 1);
      _error = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    final left = widget.attemptsLeft;
    return Scaffold(
      backgroundColor: authBg,
      body: SafeArea(
        child: Column(
          children: [
            // Chiqish — o'ng yuqori burchak.
            Align(
              alignment: Alignment.centerRight,
              child: widget.onLogout == null
                  ? const SizedBox(height: 48)
                  : IconButton(
                      icon: const Icon(Icons.logout, color: authMuted),
                      tooltip: 'Hisobdan chiqish',
                      onPressed: widget.onLogout,
                    ),
            ),
            const Spacer(flex: 2),
            Text(
              widget.title,
              style: const TextStyle(
                  fontSize: 22, fontWeight: FontWeight.w700, color: authText),
            ),
            const SizedBox(height: 10),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 40),
              child: Text(
                _error && left != null
                    ? 'PIN noto\'g\'ri. Yana $left ta urinish qoldi.'
                    : widget.subtitle,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 13.5,
                  color: _error ? const Color(0xFFC62828) : authMuted,
                ),
              ),
            ),
            const SizedBox(height: 30),
            SizedBox(
              height: 30,
              child: _busy
                  ? const Center(
                      child: SizedBox(
                        height: 16,
                        width: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                    )
                  : _PinDots(
                      pin: _pin,
                      error: _error,
                      revealed: _reveal,
                      onToggleReveal: _pin.isEmpty
                          ? null
                          : () => setState(() => _reveal = !_reveal),
                    ),
            ),
            const Spacer(flex: 2),
            _PinPad(
              onDigit: _add,
              onBackspace: _backspace,
              extra: widget.onBiometric == null
                  ? null
                  : IconButton(
                      icon: const BiometricIcon(),
                      tooltip: 'Yuz yoki barmoq izi',
                      onPressed: widget.onBiometric,
                    ),
            ),
            const SizedBox(height: 4),
            TextButton(
              onPressed: widget.onForgot,
              child: Text(widget.forgotLabel,
                  style: const TextStyle(color: authBrand, fontSize: 13.5)),
            ),
            const SizedBox(height: 6),
          ],
        ),
      ),
    );
  }
}

/// PIN YARATISH — ikki qadam: kiritish va tasdiqlash.
class PinSetupScreen extends StatefulWidget {
  const PinSetupScreen({super.key, required this.onDone, this.onCancel});

  /// PIN muvaffaqiyatli o'rnatilgandan keyin.
  final VoidCallback onDone;

  /// `null` bo'lsa bekor qilib bo'lmaydi (kirishdan keyingi majburiy
  /// yaratish). Profildan o'zgartirishda beriladi.
  final VoidCallback? onCancel;

  @override
  State<PinSetupScreen> createState() => _PinSetupScreenState();
}

class _PinSetupScreenState extends State<PinSetupScreen> {
  String _first = '';
  String _pin = '';
  bool _error = false;
  bool _busy = false;
  bool _reveal = false;
  String? _message;

  bool get _confirming => _first.isNotEmpty;

  Future<void> _add(String d) async {
    if (_busy || _pin.length >= AppPin.pinLength) return;
    setState(() {
      _pin += d;
      _error = false;
      _message = null;
    });
    if (_pin.length != AppPin.pinLength) return;

    if (!_confirming) {
      final weak = AppPin.weakness(_pin);
      if (weak != null) {
        setState(() {
          _message = weak;
          _error = true;
          _pin = '';
          _reveal = false;
        });
        HapticFeedback.heavyImpact();
        return;
      }
      setState(() {
        _first = _pin;
        _pin = '';
        _reveal = false;
      });
      return;
    }

    if (_pin != _first) {
      setState(() {
        _message = 'PIN kodlar mos kelmadi. Qaytadan boshlang.';
        _error = true;
        _pin = '';
        _first = '';
        _reveal = false;
      });
      HapticFeedback.heavyImpact();
      return;
    }

    setState(() => _busy = true);
    await AppPin.set(_pin);
    if (!mounted) return;
    widget.onDone();
  }

  void _backspace() {
    if (_busy || _pin.isEmpty) return;
    setState(() {
      _pin = _pin.substring(0, _pin.length - 1);
      _error = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: authBg,
      body: SafeArea(
        child: Column(
          children: [
            Align(
              alignment: Alignment.centerLeft,
              child: widget.onCancel == null
                  ? const SizedBox(height: 48)
                  : IconButton(
                      icon: const Icon(Icons.arrow_back, color: authText),
                      onPressed: widget.onCancel,
                    ),
            ),
            const Spacer(flex: 2),
            Text(
              _confirming ? 'PIN kodni takrorlang' : 'PIN kod yarating',
              style: const TextStyle(
                  fontSize: 22, fontWeight: FontWeight.w700, color: authText),
            ),
            const SizedBox(height: 10),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 40),
              child: Text(
                _message ??
                    (_confirming
                        ? 'Xatolik bo\'lmasligi uchun yana bir marta kiriting'
                        : 'Ilovaga kirishda shu kod so\'raladi. '
                            'Uni hech kimga aytmang.'),
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 13.5,
                  color: _error ? const Color(0xFFC62828) : authMuted,
                ),
              ),
            ),
            const SizedBox(height: 30),
            SizedBox(
              height: 30,
              child: _busy
                  ? const Center(
                      child: SizedBox(
                        height: 16,
                        width: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                    )
                  : _PinDots(
                      pin: _pin,
                      error: _error,
                      revealed: _reveal,
                      onToggleReveal: _pin.isEmpty
                          ? null
                          : () => setState(() => _reveal = !_reveal),
                    ),
            ),
            const Spacer(flex: 2),
            _PinPad(onDigit: _add, onBackspace: _backspace),
            const SizedBox(height: 34),
          ],
        ),
      ),
    );
  }
}
