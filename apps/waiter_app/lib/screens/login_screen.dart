import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../session.dart';
import '../theme.dart';
import 'waiter_shell.dart';

/// Affitsiant kirishi — telefon raqami + tasdiqlash kodi.
///
/// ┌─ KOD TELEGRAM ORQALI KELADI ──────────────────────────────────────┐
/// Ilgari bu yerda faqat `/auth/request-code` (SMS) chaqirilardi.
/// Production'da SMS provayderi (Eskiz.uz) hali ulanmagan, server esa
/// bunday holatda `LogSms` ga tushadi — kodni FAQAT server logiga
/// yozadi va baribir "yuborildi" deb javob beradi. Ya'ni affitsiant
/// hech qachon kelmaydigan SMS ni kutib o'tirardi, ekranda esa hech
/// qanday xato ko'rinmasdi.
///
/// Endi birinchi tanlov — Telegram bot (bepul, allaqachon ishlaydi;
/// admin va restoran panellari ham shu yo'ldan yuradi). SMS zaxira
/// bo'lib qoladi: Eskiz ulangan kunda u kodni HAQIQATAN yetkazadi va
/// bu yerda hech narsa o'zgartirish kerak bo'lmaydi.
///
/// Ikkala kanal ham oxirida BIR XIL joyga tushadi — kod `CodeStore` ga
/// yoziladi va `POST /auth/verify` bilan tekshiriladi. Shuning uchun
/// `_verify()` umuman o'zgarmadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA RO'YXATDAN O'TISH YO'Q ─────────────────────────────────────┐
/// Affitsiant akkauntini RESTORAN o'z panelidan yaratadi (foydalanuvchi
/// qarori, 2026-08-12). Bu ataylab: aks holda istalgan odam o'zini
/// affitsiant deb ro'yxatdan o'tkazib, restoranning buyurtmalarini
/// ko'ra olardi.
///
/// Shuning uchun bu yerda faqat KIRISH bor. Raqam ro'yxatda bo'lmasa
/// yoki roli `waiter` bo'lmasa — aniq xabar ko'rsatiladi.
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
  /// tizim havolani boshqa ilovaga yo'naltirishi mumkin. Havola
  /// ko'rinib turgani uchun uni har doim qo'lda ochib bo'ladi.
  String? _deepLink;

  /// Kod qaysi kanal orqali yuborilgani — faqat MATNNI to'g'ri
  /// ko'rsatish uchun. Tekshirish ikkalasida bir xil (`verify`).
  bool _viaTelegram = false;

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  /// Kod so'rash: avval Telegram, u ishlamasa SMS.
  ///
  /// Telegram pog'onasi FAQAT server bot bilan bog'lana olmaganda
  /// (`/auth/telegram/start` xato bersa — masalan bot sozlanmagan
  /// bo'lsa 503) tashlab ketiladi. `launchUrl` ning muvaffaqiyati
  /// SHART EMAS: so'rov serverda allaqachon ochilgan va foydalanuvchi
  /// botni qo'lda ochsa ham kod keladi. Aks holda Telegram
  /// ochilmaganda ekran birinchi qadamda qotib qolardi va kod
  /// kiritish maydoni umuman chiqmasdi.
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
        // o'tamiz. Sabab tushib qolmasin: SMS ham ishlamasa
        // foydalanuvchi faqat oxirgi xatoni ko'radi, shuning uchun
        // Telegram sababi shu yerda saqlanadi.
        debugPrint('Telegram pog\'onasi ishlamadi: ${e.message}');
      }

      final devCode = await api.requestCode(_phone.text.trim());
      if (!mounted) return;
      setState(() {
        _codeSent = true;
        _viaTelegram = false;
        _deepLink = null;
        // Dev rejimda server kodni javobda qaytaradi — qo'lda
        // yozishning hojati yo'q.
        if (devCode != null) _code.text = devCode;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
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
      final res = await api.verify(_phone.text.trim(), _code.text.trim());
      final user = Map<String, dynamic>.from(res['user'] as Map);
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';

      // ┌─ ROL TEKSHIRUVI ─────────────────────────────────────────┐
      // Token muvaffaqiyatli olindi, lekin bu odam affitsiant
      // bo'lmasligi mumkin (masalan oddiy mijoz o'z raqami bilan
      // kirdi). Server baribir `/waiter/*` endpointlariga 403
      // beradi, lekin foydalanuvchi buni tushunarsiz xato sifatida
      // ko'rardi. Shuning uchun sabab SHU YERDA aytiladi.
      //
      // Tokenni SAQLAMAYMIZ ham — aks holda ilova keyingi ochilishda
      // shu yaroqsiz sessiya bilan urinaverardi.
      // └───────────────────────────────────────────────────────────┘
      if (role != 'waiter' || entityId.isEmpty) {
        api.token = null;
        if (mounted) {
          setState(() => _error =
              'Bu raqam affitsiant sifatida ro\'yxatdan o\'tmagan.\n'
              'Restoran ma\'muriyatiga murojaat qiling.');
        }
        return;
      }

      await tokenStore.write(res['token'] as String);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const WaiterShell()),
      );
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Ilova logotipi (`image/affitsiant.png` dan). Ilova
                // belgisi (launcher icon) ham AYNAN shu fayldan
                // generatsiya qilinadi — telefondagi belgi va ekrandagi
                // logotip bir xil ko'rinadi.
                Image.asset(
                  'assets/logo.png',
                  height: 96,
                  // Rasm topilmasa ilova YIQILMASLIGI kerak: kirish
                  // ekrani — foydalanuvchi ko'radigan birinchi narsa.
                  errorBuilder: (_, _, _) =>
                      const Icon(Icons.room_service, size: 64, color: kBrandColor),
                ),
                const SizedBox(height: 16),
                const Text(
                  'OnDex Affitsiant',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 32),
                TextField(
                  controller: _phone,
                  enabled: !_codeSent,
                  keyboardType: TextInputType.phone,
                  decoration: const InputDecoration(
                    labelText: 'Telefon raqam',
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
                      labelText: _viaTelegram
                          ? 'Telegramdan kelgan kod'
                          : 'SMS kod',
                      border: const OutlineInputBorder(),
                    ),
                  ),
                ],
                if (_error != null) ...[
                  const SizedBox(height: 16),
                  Text(
                    _error!,
                    style: const TextStyle(color: Colors.redAccent),
                    textAlign: TextAlign.center,
                  ),
                ],
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: _busy ? null : (_codeSent ? _verify : _requestCode),
                  style: FilledButton.styleFrom(
                    backgroundColor: kBrandColor,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                  ),
                  child: _busy
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(_codeSent
                          ? 'Kirish'
                          : 'Telegram orqali kod olish'),
                ),
                if (_codeSent)
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () => setState(() {
                              _codeSent = false;
                              // Havola BIR MARTALIK: bot uni ishlatgach
                              // server tokenni o'chiradi
                              // (`verifier.go` — `store.drop`). Shuning
                              // uchun boshqa raqam YANGI havola oladi,
                              // eskisi qayta ishlatilmaydi.
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
      ),
    );
  }
}

/// Telegram oqimining ko'rsatmasi va zaxira havolasi.
///
/// Panellardagi shunga o'xshash vidjetdan farqi — bu MOBIL uchun:
/// havola bosiladigan qilingan (telefonda uzun URL'ni qo'lda terish
/// amalda imkonsiz) va matn Telegram ILOVASI haqida gapiradi.
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
          // xavfsizlik qoidasi, nosozlik emas. Affitsiant buni
          // bilmasa "bot ishlamayapti" deb o'ylardi.
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
