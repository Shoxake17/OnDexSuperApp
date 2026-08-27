import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../session.dart';
import 'courier_shell.dart';
import 'register_screen.dart';

/// Kuryer kirishi — telefon raqami + tasdiqlash kodi.
///
/// ┌─ KOD TELEGRAM ORQALI KELADI ──────────────────────────────────────┐
/// Ilgari bu yerda faqat `/auth/request-code` (SMS) chaqirilardi.
/// Production'da SMS provayderi (Eskiz.uz) hali ulanmagan — Eskiz
/// akkaunti `role: test` da turibdi va ixtiyoriy matnni umuman
/// yubormaydi. Bunday holatda server `LogSms` ga tushadi: kodni FAQAT
/// server logiga yozadi va baribir "yuborildi" deb javob beradi. Ya'ni
/// kuryer hech qachon kelmaydigan SMS ni kutardi, ekranda esa hech
/// qanday xato ko'rinmasdi.
///
/// Endi birinchi tanlov — Telegram bot (bepul, allaqachon ishlaydi;
/// admin, restoran va affitsiant ilovalari ham shu yo'ldan yuradi).
/// SMS zaxira bo'lib qoladi: Eskiz bilan shartnoma tuzilgan kunda u
/// kodni HAQIQATAN yetkazadi va bu yerda hech narsa o'zgartirish
/// kerak bo'lmaydi.
///
/// Ikkala kanal ham oxirida BIR XIL joyga tushadi — kod `CodeStore` ga
/// yoziladi va `POST /auth/verify` bilan tekshiriladi. Shuning uchun
/// `_verify()` umuman o'zgarmadi.
/// └───────────────────────────────────────────────────────────────────┘
class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _phone = TextEditingController(text: '+998');
  final _code = TextEditingController();
  bool _codeSent = false;
  bool _busy = false;
  String? _error;

  /// Botning bir martalik havolasi. Ekranda KO'RSATILADI, chunki
  /// `launchUrl` ishonchli emas: Telegram o'rnatilmagan bo'lishi yoki
  /// tizim havolani boshqa ilovaga yo'naltirishi mumkin.
  String? _deepLink;

  /// Kod qaysi kanal orqali yuborilgani — faqat MATNNI to'g'ri
  /// ko'rsatish uchun. Tekshirish ikkalasida bir xil (`verify`).
  bool _viaTelegram = false;

  /// Kod so'rash: avval Telegram, u ishlamasa SMS.
  ///
  /// Telegram pog'onasi FAQAT server bot bilan bog'lana olmaganda
  /// (`/auth/telegram/start` xato bersa — masalan bot sozlanmagan
  /// bo'lsa 503) tashlab ketiladi. `launchUrl` ning muvaffaqiyati
  /// SHART EMAS: so'rov serverda allaqachon ochilgan va kuryer botni
  /// qo'lda ochsa ham kod keladi. Aks holda Telegram ochilmaganda
  /// ekran birinchi qadamda qotib qolardi.
  Future<void> _requestCode() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      try {
        final link = await api.telegramStart(_phone.text.trim());
        if (!mounted) return;
        setState(() {
          _codeSent = true;
          _viaTelegram = true;
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
              'pastdagi havolani bosing yoki nusxalab oching');
        }
        return;
      } on ApiException catch (e) {
        // Bot sozlanmagan yoki vaqtincha ishlamayapti — SMS'ga
        // o'tamiz. Sabab tushib qolmasin: SMS ham ishlamasa kuryer
        // faqat oxirgi xatoni ko'radi.
        debugPrint('Telegram pog\'onasi ishlamadi: ${e.message}');
      }

      final devCode = await api.requestCode(_phone.text.trim());
      if (!mounted) return;
      setState(() {
        _codeSent = true;
        _viaTelegram = false;
        _deepLink = null;
        // Dev rejimda server kodni qaytaradi — qo'lda yozmaslik uchun to'ldirib qo'yamiz.
        if (devCode != null) _code.text = devCode;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await api.verify(_phone.text.trim(), _code.text.trim());
      await tokenStore.write(api.token!);
      final user = await api.me();
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(
          builder: (_) => (role == 'courier' && entityId.isNotEmpty)
              ? CourierShell(courierId: entityId)
              : const RegisterScreen(),
        ),
      );
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Kuryer ilovasi logotipi (`image/kuryer.png` dan).
              // Telefondagi ilova belgisi ham AYNAN shu fayldan
              // generatsiya qilinadi (pubspec: flutter_launcher_icons).
              Image.asset(
                'assets/logo.png',
                height: 96,
                // Rasm topilmasa kirish ekrani YIQILMASLIGI kerak.
                errorBuilder: (_, _, _) =>
                    const Icon(Icons.delivery_dining, size: 64),
              ),
              const SizedBox(height: 8),
              Text('OnDex Kuryer',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.headlineMedium),
              const SizedBox(height: 32),
              TextField(
                controller: _phone,
                enabled: !_codeSent,
                keyboardType: TextInputType.phone,
                decoration: const InputDecoration(
                  labelText: 'Telefon raqam',
                  hintText: '+998901234567',
                  border: OutlineInputBorder(),
                ),
              ),
              if (_codeSent) ...[
                if (_viaTelegram) ...[
                  const SizedBox(height: 16),
                  _TelegramHint(deepLink: _deepLink),
                ],
                const SizedBox(height: 16),
                TextField(
                  controller: _code,
                  autofocus: true,
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(
                    labelText:
                        _viaTelegram ? 'Telegramdan kelgan kod' : 'SMS kod',
                    border: const OutlineInputBorder(),
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
                onPressed: _busy ? null : (_codeSent ? _verify : _requestCode),
                child: Text(_busy
                    ? 'Kutilmoqda...'
                    : (_codeSent ? 'Kirish' : 'Telegram orqali kod olish')),
              ),
              if (_codeSent)
                TextButton(
                  onPressed: _busy
                      ? null
                      : () => setState(() {
                            _codeSent = false;
                            // Havola BIR MARTALIK: bot uni ishlatgach
                            // server tokenni o'chiradi (`verifier.go`).
                            // Boshqa raqam YANGI havola oladi.
                            _deepLink = null;
                            _viaTelegram = false;
                            _code.clear();
                            _error = null;
                          }),
                  child: const Text('Raqamni o\'zgartirish'),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Telegram oqimining ko'rsatmasi va zaxira havolasi.
class _TelegramHint extends StatelessWidget {
  final String? deepLink;

  const _TelegramHint({required this.deepLink});

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
          // Raqam MOS KELMASA bot kodni umuman yubormaydi — bu
          // xavfsizlik qoidasi, nosozlik emas.
          const SizedBox(height: 6),
          Text(
            'Diqqat: Telegramdagi raqamingiz yuqorida yozilgan raqam '
            'bilan bir xil bo\'lishi kerak.',
            style: theme.textTheme.bodySmall
                ?.copyWith(color: theme.colorScheme.outline),
          ),
          if (deepLink != null) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                TextButton.icon(
                  icon: const Icon(Icons.open_in_new, size: 16),
                  label: const Text('Botni ochish'),
                  onPressed: () => launchUrl(Uri.parse(deepLink!),
                      mode: LaunchMode.externalApplication),
                ),
                const Spacer(),
                TextButton.icon(
                  icon: const Icon(Icons.copy, size: 16),
                  label: const Text('Havola'),
                  onPressed: () {
                    Clipboard.setData(ClipboardData(text: deepLink!));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Havola nusxalandi')),
                    );
                  },
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
