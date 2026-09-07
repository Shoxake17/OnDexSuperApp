import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import 'shell.dart';

class AdminLoginScreen extends StatefulWidget {
  const AdminLoginScreen({super.key});

  @override
  State<AdminLoginScreen> createState() => _AdminLoginScreenState();
}

class _AdminLoginScreenState extends State<AdminLoginScreen> {
  final _phone = TextEditingController(text: '+998');
  final _code = TextEditingController();
  bool _codeSent = false;
  bool _busy = false;
  String? _error;

  /// Botning bir martalik havolasi. Ekranda KO'RSATILADI, chunki
  /// `launchUrl` ishonchli emas: web'da brauzer qalqib chiquvchi oynani
  /// to'sishi mumkin, desktopda esa Telegram Desktop o'rnatilmagan
  /// bo'lishi mumkin. Havola ko'rinib turgani uchun foydalanuvchi uni
  /// har doim qo'lda nusxalab ocha oladi.
  String? _deepLink;

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

  /// Kod so'rash — Telegram bot orqali.
  ///
  /// Havola OLINGANDAN keyin darhol `_codeSent` ga o'tamiz, `launchUrl`
  /// natijasidan QAT'IY NAZAR: so'rov serverda allaqachon ochilgan va
  /// foydalanuvchi botni qo'lda ochsa ham kod keladi. Aks holda Telegram
  /// ochilmaganda ekran birinchi qadamda qotib qolardi va kod kiritish
  /// maydoni umuman chiqmasdi.
  Future<void> _startTelegram() => _run(() async {
        final link = await api.telegramStart(_phone.text.trim());
        setState(() {
          _codeSent = true;
          _deepLink = link;
        });
        bool opened = false;
        try {
          opened = await launchUrl(Uri.parse(link),
              mode: LaunchMode.externalApplication);
        } catch (_) {
          opened = false;
        }
        if (!opened && mounted) {
          setState(() => _error = 'Telegram avtomatik ochilmadi — '
              'pastdagi havolani nusxalab oching');
        }
      });

  Future<void> _verify() => _run(() async {
        final d = await api.verify(_phone.text.trim(), _code.text.trim());
        final role = (d['user'] as Map)['role'];
        if (role != 'admin') {
          api.token = null;
          setState(() => _error = 'Bu panel faqat superadmin uchun');
          return;
        }
        await adminTokenStore.write(api.token!);
        if (!mounted) return;
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => const AdminShell()),
        );
      });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Card(
          margin: const EdgeInsets.all(24),
          child: Container(
            width: 400,
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const Icon(Icons.admin_panel_settings, size: 56),
                const SizedBox(height: 8),
                Text('OnDex Superadmin',
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.headlineSmall),
                const SizedBox(height: 24),
                TextField(
                  controller: _phone,
                  enabled: !_codeSent,
                  decoration: const InputDecoration(
                    labelText: 'Admin telefon raqami',
                    border: OutlineInputBorder(),
                  ),
                ),
                if (_codeSent) ...[
                  const SizedBox(height: 16),
                  _TelegramHint(
                    deepLink: _deepLink,
                    onRestart: _busy
                        ? null
                        : () => setState(() {
                              _codeSent = false;
                              _deepLink = null;
                              _code.clear();
                              _error = null;
                            }),
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _code,
                    autofocus: true,
                    keyboardType: TextInputType.number,
                    decoration: const InputDecoration(
                      labelText: 'Telegramdan kelgan kod',
                      border: OutlineInputBorder(),
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Text(_error!,
                        style: TextStyle(
                            color: Theme.of(context).colorScheme.error)),
                  ),
                FilledButton(
                  onPressed:
                      _busy ? null : (_codeSent ? _verify : _startTelegram),
                  child: Text(_busy
                      ? 'Kutilmoqda...'
                      : (_codeSent ? 'Kirish' : 'Telegram orqali kod olish')),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Telegram oqimining ko'rsatmasi va zaxira havolasi.
///
/// Ikkala panelda bir xil — nusxalanmasligi uchun alohida vidjet.
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
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Telegramda:', style: theme.textTheme.labelLarge),
          const SizedBox(height: 4),
          Text(
            '1. "Start" tugmasini bosing\n'
            '2. "Raqamni ulashish" tugmasini bosing\n'
            '3. Bot yuborgan kodni pastga kiriting',
            style: theme.textTheme.bodySmall,
          ),
          if (deepLink != null) ...[
            const SizedBox(height: 8),
            SelectableText(
              deepLink!,
              style: theme.textTheme.bodySmall
                  ?.copyWith(color: theme.colorScheme.primary),
            ),
            const SizedBox(height: 4),
            Row(
              children: [
                TextButton.icon(
                  icon: const Icon(Icons.copy, size: 16),
                  label: const Text('Havolani nusxalash'),
                  onPressed: () {
                    Clipboard.setData(ClipboardData(text: deepLink!));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Havola nusxalandi')),
                    );
                  },
                ),
                const Spacer(),
                // Havola BIR MARTALIK: bot uni ishlatgach server tokenni
                // o'chiradi (`verifier.go` — `store.drop`). Shuning uchun
                // "boshqa raqam" yangi havola oladi, eskisini qayta
                // ishlatmaydi.
                TextButton(
                  onPressed: onRestart,
                  child: const Text('Boshqa raqam'),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
