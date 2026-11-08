import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import 'shell.dart';
import 'store_home_screen.dart';

/// OnDex Merchant ilovasining klient identifikatori (server `DefaultClients`).
const _clientId = 'merchant-desktop';
const _deviceName = 'Merchant panel (Windows)';

/// Kirish bosqichlari (A4b — markaziy sessiya, egasi uchun TOTP majburiy).
///
///   start      — telefon + parol YOKI Telegram kodi
///   code       — Telegram'dan kelgan kod (birinchi bosqich)
///   factorOtp  — TOTP ulashdan oldin: telefon kodi (parol bilan kirgan bo'lsa)
///   factorPwd  — TOTP ulashdan oldin: parol (kod bilan kirgan bo'lsa)
///   passwordSet — parolsiz egasi: kod bilan kirgach yangi parol o'rnatadi
///   enroll     — autentifikator ilovasini ulash (QR)
///   recovery   — 10 ta tiklash kodi (BIR MARTA ko'rsatiladi)
///   totp       — har kirishda: autentifikator kodi yoki tiklash kodi
enum _Stage {
  start,
  code,
  factorOtp,
  factorPwd,
  passwordSet,
  enroll,
  recovery,
  totp
}

/// Restoran/do'kon paneliga kirish. Ro'yxatdan o'tish bu ekranda YO'Q —
/// akkauntni superadmin yaratadi.
class RestaurantLoginScreen extends StatefulWidget {
  const RestaurantLoginScreen({super.key});

  @override
  State<RestaurantLoginScreen> createState() => _RestaurantLoginScreenState();
}

class _RestaurantLoginScreenState extends State<RestaurantLoginScreen> {
  final _phone = TextEditingController(text: '+998');
  final _code = TextEditingController();
  final _password = TextEditingController();
  final _password2 = TextEditingController();
  bool _passwordMode = true;
  bool _busy = false;
  String? _error;
  _Stage _stage = _Stage.start;

  /// Keyingi bosqich tokeni (server: 5–10 daqiqa).
  String? _mfaToken;
  TotpEnrollment? _enrollment;
  List<String> _recoveryCodes = const [];
  bool _recoverySaved = false;

  /// Botning bir martalik havolasi — faqat "Telegramni ochish" tugmasi uchun.
  String? _deepLink;

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    _password.dispose();
    _password2.dispose();
    super.dispose();
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _restart() => setState(() {
        _stage = _Stage.start;
        _mfaToken = null;
        _enrollment = null;
        _deepLink = null;
        _code.clear();
        _password.clear();
        _password2.clear();
        _error = null;
      });

  /// Telegram bot orqali kod so'rash. Havola olingandan keyin `launchUrl`
  /// natijasidan QAT'IY NAZAR kod maydoni ochiladi (so'rov serverda ochilgan).
  Future<void> _requestTelegramCode() async {
    final link = await api.telegramStart(_phone.text.trim());
    setState(() => _deepLink = link);
    var opened = false;
    try {
      opened = await launchUrl(Uri.parse(link),
          mode: LaunchMode.externalApplication);
    } catch (_) {
      opened = false;
    }
    if (!opened && mounted) {
      setState(() => _error =
          'Telegram avtomatik ochilmadi — "Telegramni ochish" tugmasini bosing');
    }
  }

  // ── Birinchi bosqich ────────────────────────────────────────────────

  Future<void> _startTelegram() => _run(() async {
        await _requestTelegramCode();
        setState(() => _stage = _Stage.code);
      });

  Future<void> _loginWithPassword() => _run(() async {
        final s = await api.merchantLoginPassword(
            _phone.text.trim(), _password.text,
            clientId: _clientId, deviceName: _deviceName);
        await _handle(s);
      });

  Future<void> _verifyCode() => _run(() async {
        final s = await api.merchantLoginCode(
            _phone.text.trim(), _code.text.trim(),
            clientId: _clientId, deviceName: _deviceName);
        await _handle(s);
      });

  // ── Keyingi bosqichlar ──────────────────────────────────────────────

  Future<void> _submitFactor() => _run(() async {
        final s = _stage == _Stage.factorOtp
            ? await api.mfaFactor(_mfaToken!, code: _code.text.trim())
            : await api.mfaFactor(_mfaToken!, password: _password.text);
        await _handle(s);
      });

  Future<void> _setPassword() => _run(() async {
        if (_password.text.length < 8) {
          throw ApiException('Parol kamida 8 belgi bo\'lsin');
        }
        if (_password.text != _password2.text) {
          throw ApiException('Parollar mos kelmadi');
        }
        final s = await api.mfaSetPassword(
            _mfaToken!, _password.text, _password2.text);
        _password.clear();
        _password2.clear();
        await _handle(s);
      });

  Future<void> _confirmEnroll() => _run(() async {
        final codes = await api.mfaConfirm(_mfaToken!, _code.text.trim());
        setState(() {
          _recoveryCodes = codes;
          _recoverySaved = false;
          _stage = _Stage.recovery;
          _code.clear();
        });
      });

  Future<void> _verifyTotp() => _run(() async {
        await _handle(await api.mfaVerify(_mfaToken!, _code.text.trim()));
      });

  /// Server javobiga qarab keyingi bosqich. `done` — sessiya o'rnatilgan.
  Future<void> _handle(MerchantStep s) async {
    if (s.done) {
      await _afterSession();
      return;
    }
    _mfaToken = s.mfaToken;
    _code.clear();
    switch (s.method) {
      case 'otp':
        // Parol bilan kirgan egasi TOTP'ni birinchi marta ulayapti —
        // telefon egaligi ham isbotlanishi SHART (faqat parolni bilgan
        // odam o'z TOTP'ini ulab akkauntni egallab olmasin).
        await _requestTelegramCode();
        setState(() => _stage = _Stage.factorOtp);
      case 'password':
        _password.clear();
        setState(() => _stage = _Stage.factorPwd);
      case 'password_set':
        _password.clear();
        _password2.clear();
        setState(() => _stage = _Stage.passwordSet);
      case 'totp_enroll':
        final en = await api.mfaEnroll(_mfaToken!);
        setState(() {
          _enrollment = en;
          _stage = _Stage.enroll;
        });
      case 'totp':
        setState(() => _stage = _Stage.totp);
      default:
        setState(() => _error = 'Noma\'lum bosqich: ${s.method}');
    }
  }

  /// Sessiya ochildi: rolni tekshiradi va mos ekranni ochadi.
  /// Restoran -> panel; do'kon -> do'kon bosh ekrani; boshqalar rad etiladi.
  Future<void> _afterSession() async {
    final user = await api.me();
    if (user['role'] == 'store') {
      if (!mounted) return;
      Navigator.of(context).pushReplacement(MaterialPageRoute(
        builder: (_) => StoreHomeScreen(
          name: (user['name'] as String?) ?? '',
          phone: (user['phone'] as String?) ?? '',
        ),
      ));
      return;
    }
    if (user['role'] != 'restaurant' ||
        (user['entity_id'] as String? ?? '').isEmpty) {
      await api.logout(); // markazda ochilgan sessiya ochiq qolmasin
      await api.clearSession();
      setState(() {
        _stage = _Stage.start;
        _error = 'Bu raqam restoran yoki do\'kon akkaunti emas. '
            'Akkaunt olish uchun OnDex administratsiyasiga murojaat qiling.';
      });
      return;
    }
    api.rid = user['entity_id'] as String;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('rest_rid', api.rid);
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const RestaurantShell()),
    );
  }

  // ── Ko'rinish ───────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      body: Center(
        child: SingleChildScrollView(
          child: Card(
            margin: const EdgeInsets.all(24),
            child: Container(
              width: 420,
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Center(
                    child: Image.asset(
                      'assets/merchant_logo.png',
                      width: 240,
                      fit: BoxFit.contain,
                      filterQuality: FilterQuality.medium,
                      errorBuilder: (_, __, ___) =>
                          const Icon(Icons.storefront, size: 56),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text('OnDex Merchant Panel',
                      textAlign: TextAlign.center,
                      style: theme.textTheme.headlineSmall),
                  const SizedBox(height: 4),
                  Text(_subtitle,
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodySmall),
                  const SizedBox(height: 24),
                  ..._stageBody(theme),
                  const SizedBox(height: 16),
                  if (_error != null)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Text(_error!,
                          style: TextStyle(color: theme.colorScheme.error)),
                    ),
                  FilledButton(
                    onPressed: _busy || !_canSubmit ? null : _submit,
                    child: Text(_busy ? 'Kutilmoqda...' : _submitLabel),
                  ),
                  if (_stage == _Stage.start)
                    TextButton(
                      onPressed: _busy
                          ? null
                          : () => setState(() {
                                _passwordMode = !_passwordMode;
                                _error = null;
                              }),
                      child: Text(_passwordMode
                          ? 'Telegram kodi bilan kirish'
                          : 'Parol bilan kirish'),
                    )
                  else if (_stage != _Stage.recovery)
                    TextButton(
                      onPressed: _busy ? null : _restart,
                      child: const Text('Boshidan boshlash'),
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  String get _subtitle => switch (_stage) {
        _Stage.factorOtp => 'Xavfsizlik: telefon raqamingizni tasdiqlang',
        _Stage.factorPwd => 'Xavfsizlik: parolingizni kiriting',
        _Stage.passwordSet => 'Xavfsizlik: yangi parol o\'rnating',
        _Stage.enroll => 'Ikki bosqichli tasdiqni yoqing (majburiy)',
        _Stage.recovery => 'Tiklash kodlarini saqlab qo\'ying',
        _Stage.totp => 'Ikki bosqichli tasdiq',
        _ => 'O\'z biznesingizni boshqaring',
      };

  String get _submitLabel => switch (_stage) {
        _Stage.start => _passwordMode ? 'Kirish' : 'Telegram orqali kod olish',
        _Stage.enroll => 'Tasdiqlash va yoqish',
        _Stage.recovery => 'Davom etish',
        _ => 'Davom etish',
      };

  bool get _canSubmit => _stage != _Stage.recovery || _recoverySaved;

  Future<void> _submit() => switch (_stage) {
        _Stage.start => _passwordMode ? _loginWithPassword() : _startTelegram(),
        _Stage.code => _verifyCode(),
        _Stage.factorOtp || _Stage.factorPwd => _submitFactor(),
        _Stage.passwordSet => _setPassword(),
        _Stage.enroll => _confirmEnroll(),
        _Stage.recovery => _run(_afterSession),
        _Stage.totp => _verifyTotp(),
      };

  List<Widget> _stageBody(ThemeData theme) {
    Widget codeField(String label) => TextField(
          controller: _code,
          autofocus: true,
          keyboardType: TextInputType.number,
          onSubmitted: (_) => _busy ? null : _submit(),
          decoration: InputDecoration(
              labelText: label, border: const OutlineInputBorder()),
        );
    Widget passwordField(String label) => TextField(
          controller: _password,
          obscureText: true,
          autofocus: _stage != _Stage.start,
          onSubmitted: (_) => _busy ? null : _submit(),
          decoration: InputDecoration(
              labelText: label, border: const OutlineInputBorder()),
        );
    final telegramHint =
        _TelegramHint(deepLink: _deepLink, onRestart: _busy ? null : _restart);

    switch (_stage) {
      case _Stage.start:
        return [
          TextField(
            controller: _phone,
            decoration: const InputDecoration(
                labelText: 'Telefon raqami', border: OutlineInputBorder()),
          ),
          if (_passwordMode) ...[
            const SizedBox(height: 12),
            passwordField('Parol')
          ],
        ];
      case _Stage.code:
        return [
          telegramHint,
          const SizedBox(height: 12),
          codeField('Telegramdan kelgan kod')
        ];
      case _Stage.factorOtp:
        return [
          const Text(
              'Ikki bosqichli tasdiqni yoqishdan oldin telefon raqamingizni tasdiqlang: '
              'Telegram\'ga kelgan kodni kiriting.'),
          const SizedBox(height: 12),
          telegramHint,
          const SizedBox(height: 12),
          codeField('Telegramdan kelgan kod'),
        ];
      case _Stage.factorPwd:
        return [
          const Text(
              'Ikki bosqichli tasdiqni yoqishdan oldin akkaunt parolingizni kiriting.'),
          const SizedBox(height: 12),
          passwordField('Parol'),
        ];
      case _Stage.passwordSet:
        return [
          const Text(
              'Biznes akkauntingizda hali parol yo\'q. Xavfsizlik uchun yangi '
              'parol o\'rnating (kamida 8 belgi; 12345678 kabi oddiy parollar '
              'qabul qilinmaydi).'),
          const SizedBox(height: 12),
          passwordField('Yangi parol'),
          const SizedBox(height: 12),
          TextField(
            controller: _password2,
            obscureText: true,
            onSubmitted: (_) => _busy ? null : _submit(),
            decoration: const InputDecoration(
                labelText: 'Parolni takrorlang', border: OutlineInputBorder()),
          ),
        ];
      case _Stage.enroll:
        final en = _enrollment;
        return [
          const Text('1. Telefoningizga autentifikator ilovasini o\'rnating '
              '(Google Authenticator, Microsoft Authenticator yoki 1Password).\n'
              '2. Ilovada "QR kodni skanerlash" ni tanlang.\n'
              '3. Ilova ko\'rsatgan 6 xonali kodni pastga kiriting.'),
          const SizedBox(height: 16),
          if (en != null) ...[
            Center(
              child: Container(
                color: Colors.white,
                padding: const EdgeInsets.all(8),
                child: QrImageView(data: en.uri, size: 200),
              ),
            ),
            const SizedBox(height: 8),
            Row(children: [
              Expanded(
                child: SelectableText('Kalit: ${en.secret}',
                    style: theme.textTheme.bodySmall
                        ?.copyWith(fontFamily: 'monospace')),
              ),
              IconButton(
                tooltip: 'Kalitni nusxalash',
                icon: const Icon(Icons.copy, size: 18),
                onPressed: () =>
                    Clipboard.setData(ClipboardData(text: en.secret)),
              ),
            ]),
          ],
          const SizedBox(height: 12),
          codeField('Ilovadagi 6 xonali kod'),
        ];
      case _Stage.recovery:
        return [
          const Text(
              'Telefoningiz yo\'qolsa, shu kodlardan biri bilan kira olasiz. '
              'Har bir kod BIR MARTA ishlaydi. Ular boshqa ko\'rsatilmaydi — '
              'hozir xavfsiz joyga (qog\'oz yoki parol menejeri) yozib qo\'ying.'),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: theme.colorScheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(8),
            ),
            child: SelectableText(_recoveryCodes.join('\n'),
                style: const TextStyle(
                    fontFamily: 'monospace', fontSize: 15, height: 1.6)),
          ),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton.icon(
              icon: const Icon(Icons.copy, size: 16),
              label: const Text('Nusxalash'),
              onPressed: () => Clipboard.setData(
                  ClipboardData(text: _recoveryCodes.join('\n'))),
            ),
          ),
          CheckboxListTile(
            contentPadding: EdgeInsets.zero,
            value: _recoverySaved,
            onChanged: (v) => setState(() => _recoverySaved = v ?? false),
            title: const Text('Kodlarni xavfsiz joyga saqlab qo\'ydim'),
          ),
        ];
      case _Stage.totp:
        return [
          const Text('Autentifikator ilovasidagi 6 xonali kodni kiriting. '
              'Telefoningiz yonida bo\'lmasa — tiklash kodlaridan birini kiriting.'),
          const SizedBox(height: 12),
          codeField('Kod'),
        ];
    }
  }
}

/// Telegram oqimi: botni qayta ochish va raqamni almashtirish tugmalari.
///
/// Admin panelidagi ayni vidjet bilan bir xil — ikkala panel mustaqil
/// Flutter ilova bo'lgani uchun umumiy vidjet `packages/ondex_core` ga
/// chiqarilmaguncha nusxa saqlanadi.
class _TelegramHint extends StatelessWidget {
  final String? deepLink;
  final VoidCallback? onRestart;

  const _TelegramHint({required this.deepLink, required this.onRestart});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      // `Wrap`: tor oynada yoki katta shriftda tugmalar keyingi qatorga
      // o'tadi (Row edi — RenderFlex overflow).
      child: Wrap(
        alignment: WrapAlignment.spaceBetween,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          // Havola MATNI ko'rsatilmaydi (foydalanuvchi qarori, 2026-09-15):
          // Telegram avtomatik ochilmasa shu tugma uni qayta ochadi.
          TextButton.icon(
            icon: const Icon(Icons.send, size: 16),
            label: const Text('Telegramni ochish'),
            onPressed: deepLink == null
                ? null
                : () => launchUrl(Uri.parse(deepLink!),
                    mode: LaunchMode.externalApplication),
          ),
          // Havola BIR MARTALIK — "boshqa raqam" yangisini oladi.
          TextButton(onPressed: onRestart, child: const Text('Boshqa raqam')),
        ],
      ),
    );
  }
}
