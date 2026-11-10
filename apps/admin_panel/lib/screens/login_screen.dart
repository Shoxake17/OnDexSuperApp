import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../widgets/password_field.dart';
import 'shell.dart';

const _deviceName = 'OnDex Superadmin';

/// Kirish bosqichlari (A6 — OnDex ID, platform realmi):
///
///   start       — telefon -> Telegram kodi
///   code        — Telegram'dan kelgan kod
///   password    — parol (HAR safar)
///   passwordSet — parolsiz admin: yangi parol o'rnatish
///   enroll      — autentifikator ilovasini ulash (QR, birinchi marta)
///   recovery    — 10 ta tiklash kodi (BIR MARTA ko'rsatiladi)
///   totp        — autentifikator kodi (HAR safar) yoki tiklash kodi
enum _Stage { start, code, password, passwordSet, enroll, recovery, totp }

class AdminLoginScreen extends StatefulWidget {
  const AdminLoginScreen({super.key});

  @override
  State<AdminLoginScreen> createState() => _AdminLoginScreenState();
}

class _AdminLoginScreenState extends State<AdminLoginScreen> {
  final _phone = TextEditingController(text: '+998');
  final _code = TextEditingController();
  final _password = TextEditingController();
  final _password2 = TextEditingController();
  bool _busy = false;
  String? _error;
  _Stage _stage = _Stage.start;

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

  /// Kod so'rash — Telegram bot orqali. Havola olingach `launchUrl`
  /// natijasidan QAT'IY NAZAR kod maydoni ochiladi.
  Future<void> _startTelegram() => _run(() async {
        final link = await api.telegramStart(_phone.text.trim());
        setState(() {
          _deepLink = link;
          _stage = _Stage.code;
        });
        var opened = false;
        try {
          opened = await launchUrl(Uri.parse(link),
              mode: LaunchMode.externalApplication);
        } catch (_) {
          opened = false;
        }
        if (!opened && mounted) {
          setState(() => _error = 'Telegram avtomatik ochilmadi — '
              '"Telegramni ochish" tugmasini bosing');
        }
      });

  Future<void> _verifyCode() => _run(() async {
        await _handle(await api.adminLoginCode(
            _phone.text.trim(), _code.text.trim(),
            deviceName: _deviceName));
      });

  Future<void> _submitPassword() => _run(() async {
        await _handle(
            await api.mfaFactor(_mfaToken!, password: _password.text));
      });

  Future<void> _setPassword() => _run(() async {
        if (_password.text.length < 8) {
          throw ApiException('Parol kamida 8 belgi bo\'lsin');
        }
        if (_password.text != _password2.text) {
          throw ApiException('Parollar mos kelmadi');
        }
        await _handle(await api.mfaSetPassword(
            _mfaToken!, _password.text, _password2.text));
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
    _password.clear();
    _password2.clear();
    switch (s.method) {
      case 'password':
        setState(() => _stage = _Stage.password);
      case 'password_set':
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

  /// Sessiya ochildi: rol tekshiriladi (server ham tekshiradi) va panel ochiladi.
  Future<void> _afterSession() async {
    final user = await api.me();
    final role = (user['role'] as String?) ?? '';
    if (role != 'admin') {
      await api.logout();
      await api.clearSession();
      setState(() {
        _stage = _Stage.start;
        _error = 'Bu panel faqat superadmin uchun';
      });
      return;
    }
    final userId = (user['id'] as String?) ?? '';
    if (userId.isNotEmpty) {
      Analytics.instance.identify(
        userId: userId,
        phone: user['phone'] as String?,
        name: user['name'] as String?,
        role: role,
      );
    }
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const AdminShell()),
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
                  const Icon(Icons.admin_panel_settings, size: 56),
                  const SizedBox(height: 8),
                  Text('OnDex Superadmin',
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
                  if (_stage != _Stage.start && _stage != _Stage.recovery)
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
        _Stage.password => 'Xavfsizlik: parolingizni kiriting',
        _Stage.passwordSet => 'Xavfsizlik: yangi parol o\'rnating',
        _Stage.enroll => 'Ikki bosqichli tasdiqni yoqing (majburiy)',
        _Stage.recovery => 'Tiklash kodlarini saqlab qo\'ying',
        _Stage.totp => 'Ikki bosqichli tasdiq',
        _ => 'OnDex ID — kod, parol va Authenticator',
      };

  String get _submitLabel => switch (_stage) {
        _Stage.start => 'Telegram orqali kod olish',
        _Stage.enroll => 'Tasdiqlash va yoqish',
        _Stage.totp => 'Kirish',
        _ => 'Davom etish',
      };

  bool get _canSubmit => _stage != _Stage.recovery || _recoverySaved;

  Future<void> _submit() => switch (_stage) {
        _Stage.start => _startTelegram(),
        _Stage.code => _verifyCode(),
        _Stage.password => _submitPassword(),
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
    Widget passwordField(TextEditingController c, String label,
            {bool autofocus = true}) =>
        PasswordField(
          controller: c,
          label: label,
          autofocus: autofocus,
          autofillHints: _stage == _Stage.passwordSet
              ? const [AutofillHints.newPassword]
              : const [AutofillHints.password],
          onSubmitted: (_) => _busy ? null : _submit(),
        );

    switch (_stage) {
      case _Stage.start:
        return [
          TextField(
            controller: _phone,
            onSubmitted: (_) => _busy ? null : _submit(),
            decoration: const InputDecoration(
                labelText: 'Admin telefon raqami',
                border: OutlineInputBorder()),
          ),
        ];
      case _Stage.code:
        return [
          _TelegramHint(
              deepLink: _deepLink, onRestart: _busy ? null : _restart),
          const SizedBox(height: 12),
          codeField('Telegramdan kelgan kod'),
        ];
      case _Stage.password:
        return [passwordField(_password, 'Parol')];
      case _Stage.passwordSet:
        return [
          const Text('Admin akkauntida hali parol yo\'q. Yangi parol '
              'o\'rnating (kamida 8 belgi; oddiy parollar qabul qilinmaydi).'),
          const SizedBox(height: 12),
          passwordField(_password, 'Yangi parol'),
          const SizedBox(height: 12),
          passwordField(_password2, 'Parolni takrorlang', autofocus: false),
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
      // Havola MATNI ko'rsatilmaydi (foydalanuvchi qarori, 2026-09-15).
      // Havola BIR MARTALIK — "Boshqa raqam" yangisini oladi.
      child: Wrap(
        alignment: WrapAlignment.spaceBetween,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          TextButton.icon(
            icon: const Icon(Icons.send, size: 16),
            label: const Text('Telegramni ochish'),
            onPressed: deepLink == null
                ? null
                : () => launchUrl(Uri.parse(deepLink!),
                    mode: LaunchMode.externalApplication),
          ),
          TextButton(onPressed: onRestart, child: const Text('Boshqa raqam')),
        ],
      ),
    );
  }
}
