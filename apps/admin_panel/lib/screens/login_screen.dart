import 'package:flutter/material.dart';
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

  /// Botning bir martalik havolasi. Ekranda KO'RSATILMAYDI — faqat
  /// "Telegramni ochish" tugmasi uni qayta ochish uchun saqlaydi, chunki
  /// avtomatik `launchUrl` ishlamasligi mumkin (brauzer qalqib chiquvchi
  /// oynani to'sishi yoki Telegram Desktop o'rnatilmagan bo'lishi).
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
              '"Telegramni ochish" tugmasini bosing');
        }
      });

  Future<void> _verify() => _run(() async {
        final d = await api.verify(_phone.text.trim(), _code.text.trim());
        final user = Map<String, dynamic>.from(d['user'] as Map);
        final role = (user['role'] as String?) ?? '';
        if (role != 'admin') {
          api.token = null;
          setState(() => _error = 'Bu panel faqat superadmin uchun');
          return;
        }
        await adminTokenStore.write(api.token!);
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

/// Telegram oqimi: botni qayta ochish va raqamni almashtirish tugmalari.
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
          // ┌─ QADAMMA-QADAM KO'RSATMA OLIB TASHLANDI ──────────────────┐
          // Ilgari bu yerda "1. Start bosing, 2. Raqamni ulashing,
          // 3. Kodni kiriting" degan uch qatorli o'rgatish turardi.
          //
          // Bot oqimining o'zi allaqachon tushunarli: havola ochilganda
          // Telegram Start tugmasini, so'ng raqam so'rovini o'zi
          // ko'rsatadi. Ya'ni ko'rsatma ekranda joy egallab, hech
          // qanday yangi ma'lumot bermasdi.
          // └────────────────────────────────────────────────────────────┘
          if (deepLink != null) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                // Havola MATNI ko'rsatilmaydi va nusxalanmaydi
                // (foydalanuvchi qarori, 2026-09-15): u odam uchun
                // ma'nosiz uzun URL. Telegram avtomatik ochilmasa shu
                // tugma uni qayta ochadi; desktopda `https://t.me`
                // havolasi Telegram Desktop bo'lmasa brauzerda ochiladi.
                TextButton.icon(
                  icon: const Icon(Icons.send, size: 16),
                  label: const Text('Telegramni ochish'),
                  onPressed: () => launchUrl(Uri.parse(deepLink!),
                      mode: LaunchMode.externalApplication),
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
