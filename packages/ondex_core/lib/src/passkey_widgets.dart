import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import 'api_client.dart';
import 'passkey.dart';

/// "Passkey bilan kirish" tugmasi — qurilma qo'llamasa UMUMAN ko'rinmaydi
/// (foydalanuvchini ishlamaydigan tugma bilan chalg'itmaslik).
///
/// Muvaffaqiyatda [onSignedIn] (sessiya `api` da o'rnatilgan). Xato —
/// [onError] ga tushunarli matn; bekor qilish xato hisoblanmaydi.
class PasskeyLoginButton extends StatefulWidget {
  const PasskeyLoginButton({
    super.key,
    required this.passkeys,
    required this.clientId,
    required this.onSignedIn,
    required this.onError,
    this.deviceName,
    this.enabled = true,
  });

  final OnDexPasskeys passkeys;
  final String clientId;
  final String? deviceName;
  final bool enabled;
  final Future<void> Function(MerchantStep step) onSignedIn;
  final void Function(String message) onError;

  @override
  State<PasskeyLoginButton> createState() => _PasskeyLoginButtonState();
}

class _PasskeyLoginButtonState extends State<PasskeyLoginButton> {
  bool? _supported;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    widget.passkeys.isSupported().then((v) {
      if (mounted) setState(() => _supported = v);
    });
  }

  Future<void> _go() async {
    setState(() => _busy = true);
    try {
      final step = await widget.passkeys.signIn(
        clientId: widget.clientId,
        deviceName: widget.deviceName,
      );
      await widget.onSignedIn(step);
    } on PasskeyException catch (e) {
      if (e.reason != PasskeyFailure.cancelled) widget.onError(e.message);
    } on ApiException catch (e) {
      widget.onError(
        e.statusCode == 401 || e.statusCode == 400
            ? 'Passkey tasdiqlanmadi. Telefon kodi bilan kiring.'
            : e.message,
      );
    } catch (_) {
      widget.onError("Serverga ulanib bo'lmadi");
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_supported != true) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        FilledButton.icon(
          key: const Key('passkey-login'),
          onPressed: _busy || !widget.enabled ? null : _go,
          icon: const Icon(Icons.fingerprint),
          label: Text(_busy ? 'Kutilmoqda...' : 'Passkey bilan kirish'),
        ),
        const SizedBox(height: 6),
        Text(
          'Windows Hello, barmoq izi yoki telefon qulfi — parol va kodsiz.',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodySmall,
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            const Expanded(child: Divider()),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              child: Text('yoki', style: Theme.of(context).textTheme.bodySmall),
            ),
            const Expanded(child: Divider()),
          ],
        ),
        const SizedBox(height: 12),
      ],
    );
  }
}

/// Akkaunt sozlamalaridagi "Passkey'lar" kartasi: ro'yxat, qo'shish,
/// o'chirish. Qo'shish/o'chirish — [codeLabel] dagi kod bilan (egasi/admin:
/// Authenticator; xodim/mijoz: Telegram kodi — [requestCode] uni yuboradi).
class PasskeySettingsCard extends StatefulWidget {
  const PasskeySettingsCard({
    super.key,
    required this.passkeys,
    this.codeLabel = 'Authenticator kodi',
    this.requestCode,
  });

  final OnDexPasskeys passkeys;
  final String codeLabel;

  /// Telegram kodini yuborish (TOTP'siz akkauntlar); null — kod Authenticator'dan.
  final Future<void> Function()? requestCode;

  @override
  State<PasskeySettingsCard> createState() => _PasskeySettingsCardState();
}

class _PasskeySettingsCardState extends State<PasskeySettingsCard> {
  List<Map<String, dynamic>>? _list;
  bool _supported = false;
  bool _busy = false;
  String? _msg;
  bool _msgOk = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final sup = await widget.passkeys.isSupported();
    try {
      final l = await widget.passkeys.api.myPasskeys();
      if (mounted)
        setState(() {
          _supported = sup;
          _list = l;
        });
    } on ApiException catch (e) {
      if (mounted)
        setState(() {
          _supported = sup;
          _list = const [];
          _msg = e.statusCode == 503 ? 'Passkey hali yoqilmagan' : e.message;
        });
    }
  }

  void _say(String m, {bool ok = false}) => setState(() {
    _msg = m;
    _msgOk = ok;
  });

  Future<String?> _askCode(String title) async {
    if (widget.requestCode != null) {
      try {
        await widget.requestCode!();
      } on ApiException catch (e) {
        _say(e.message);
        return null;
      }
    }
    if (!mounted) return null;
    final v = await showDialog<String>(
      context: context,
      builder: (_) => _CodeDialog(title: title, label: widget.codeLabel),
    );
    return (v == null || v.isEmpty) ? null : v;
  }

  Future<void> _add() async {
    final code = await _askCode("Passkey qo'shish");
    if (code == null) return;
    setState(() => _busy = true);
    try {
      await widget.passkeys.register(code: code, name: 'Ushbu qurilma');
      _say("Passkey qo'shildi. Keyingi safar bir tegishda kirasiz.", ok: true);
      await _load();
    } on PasskeyException catch (e) {
      if (e.reason != PasskeyFailure.cancelled) _say(e.message);
    } on ApiException catch (e) {
      _say(e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _delete(Map<String, dynamic> p) async {
    final code = await _askCode("Passkey'ni o'chirish");
    if (code == null) return;
    setState(() => _busy = true);
    try {
      await widget.passkeys.api.passkeyDelete(p['id'] as String, code);
      _say("Passkey o'chirildi.", ok: true);
      await _load();
    } on ApiException catch (e) {
      _say(e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final list = _list;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.fingerprint),
                const SizedBox(width: 8),
                Text("Passkey'lar", style: theme.textTheme.titleMedium),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              "Parol va kodsiz, fishingga chidamli kirish. Kamida ikkita qurilmada "
              "passkey bo'lgani yaxshi; yo'qolsa — telefon kodi bilan kirib yangisini qo'shasiz.",
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
            if (list == null)
              const LinearProgressIndicator()
            else if (list.isEmpty)
              Text("Hozircha passkey yo'q", style: theme.textTheme.bodyMedium)
            else
              for (final p in list)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: Icon(
                    p['synced'] == true
                        ? Icons.cloud_done_outlined
                        : Icons.devices,
                  ),
                  title: Text((p['name'] ?? 'Passkey').toString()),
                  subtitle: Text(
                    p['synced'] == true ? 'Bulutda sinxron' : 'Shu qurilmada',
                  ),
                  trailing: IconButton(
                    tooltip: "O'chirish",
                    onPressed: _busy ? null : () => _delete(p),
                    icon: const Icon(Icons.delete_outline),
                  ),
                ),
            if (_msg != null)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(
                  _msg!,
                  style: TextStyle(
                    color: _msgOk
                        ? Colors.green.shade700
                        : theme.colorScheme.error,
                  ),
                ),
              ),
            const SizedBox(height: 8),
            if (_supported)
              OutlinedButton.icon(
                key: const Key('passkey-add'),
                onPressed: _busy ? null : _add,
                icon: const Icon(Icons.add),
                label: const Text("Shu qurilmaga passkey qo'shish"),
              )
            else if (list != null)
              Text(
                "Bu qurilma passkey'ni qo'llamaydi.",
                style: theme.textTheme.bodySmall,
              ),
          ],
        ),
      ),
    );
  }
}

/// Kod so'rash dialogi — controller dialogning O'Z holatida (yopilish
/// animatsiyasi tugaguncha yashaydi; oldin pop'dan keyin darhol dispose
/// qilinib, "used after being disposed" xatosi bo'lardi).
class _CodeDialog extends StatefulWidget {
  const _CodeDialog({required this.title, required this.label});
  final String title;
  final String label;

  @override
  State<_CodeDialog> createState() => _CodeDialogState();
}

class _CodeDialogState extends State<_CodeDialog> {
  final _c = TextEditingController();

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text(widget.title),
    content: TextField(
      key: const Key('passkey-code'),
      controller: _c,
      autofocus: true,
      keyboardType: TextInputType.number,
      autocorrect: false,
      enableSuggestions: false,
      decoration: InputDecoration(labelText: widget.label),
      onSubmitted: (v) => Navigator.pop(context, v.trim()),
    ),
    actions: [
      TextButton(
        onPressed: () => Navigator.pop(context),
        child: const Text('Bekor qilish'),
      ),
      FilledButton(
        onPressed: () => Navigator.pop(context, _c.text.trim()),
        child: const Text('Tasdiqlash'),
      ),
    ],
  );
}

/// TOTP'siz akkaunt (xodim, mijoz) uchun step-up kodi: joriy akkaunt
/// raqamiga Telegram orqali kod yuboriladi va bot ochiladi. Raqam
/// SERVERDAN olinadi (`/me`) — foydalanuvchi kiritmaydi, begona raqamga
/// kod so'rab bo'lmaydi.
Future<void> sendTelegramStepUpCode(ApiClient api) async {
  final me = await api.me();
  final phone = (me['phone'] ?? '').toString();
  if (phone.isEmpty) {
    throw ApiException("Akkauntda telefon raqami yo'q", statusCode: 400);
  }
  final link = await api.telegramStart(phone);
  final uri = Uri.tryParse(link);
  if (uri != null && uri.scheme == 'https' && uri.host == 't.me') {
    try {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    } catch (_) {
      // Telegram ochilmasa ham kod yuborilgan — foydalanuvchi o'zi ochadi.
    }
  }
}