import 'package:flutter/material.dart';
import 'package:posthog_flutter/posthog_flutter.dart';

import 'api.dart';
import 'screens/courier_shell.dart';
import 'screens/login_screen.dart';
import 'screens/register_screen.dart';
import 'theme.dart';
import 'session.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Mahsulot tahlili. Kalit (ONDEX_POSTHOG_KEY) berilmagan bo'lsa
  // hech narsa yuborilmaydi va hech qanday kechikish qo'shilmaydi.
  //
  // Xatolik ilovani TO'XTATMAYDI: tahlil hech qachon ishga tushishga
  // to'sqinlik qilmasligi kerak.
  // â”Œâ”€ SEANS YOZUVI (session replay) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
  // Foydalanuvchi ekranini keyin video kabi qayta ko'rish imkonini
  // beradi - nosozlikni "qanday qilib shunday bo'ldi" degan savolga
  // javob topish uchun eng tez yo'l.
  //
  // Bu FAQAT mobil/veb'da mumkin: posthog_flutter Windows'ni
  // qo'llamaydi. Panellarda shuning uchun faqat hodisalar bor.
  //
  // Matn va rasmlar NIQOBLANADI: telefon raqami, manzil, parol
  // yozuvga tushmasligi kerak.
  // â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
  if (posthogEnabled) {
    try {
      final cfg = PostHogConfig(posthogApiKey)
        ..host = posthogHost
        ..sessionReplay = true
        ..sessionReplayConfig.maskAllTexts = true
        ..sessionReplayConfig.maskAllImages = true
        ..captureScreenViews = true
        ..captureApplicationLifecycleEvents = true
        ..debouncerTimeSecs = 5;
      await Posthog().setup(cfg);
      Analytics.instance.onIdentify = ({
        required String userId,
        String? phone,
        String? name,
        String? role,
      }) async {
        try {
          final props = <String, Object?>{};
          if (phone != null && phone.isNotEmpty) props['phone'] = phone;
          if (name != null && name.isNotEmpty) props['name'] = name;
          if (role != null && role.isNotEmpty) props['role'] = role;
          await Posthog().identify(
            userId: userId,
            userProperties: props.isNotEmpty ? props : null,
          );
        } catch (e) {
          if (kDebugMode) debugPrint('[PostHog SDK] identify xatosi: $e');
        }
      };
      Analytics.instance.onReset = (String anonId) async {
        try {
          await Posthog().reset();
        } catch (e) {
          if (kDebugMode) debugPrint('[PostHog SDK] reset xatosi: $e');
        }
      };
    } catch (e) {
      debugPrint('PostHog ishga tushmadi: $e');
    }
  }
  await Analytics.bootstrap();

  runApp(const CourierApp());
}

// Yandex Eats/Wolt kuryer ilovalari uslubida: qorong'i fon, brend rangi
// urg'u sifatida. `.copyWith(primary: kBrandColor)` — Material3'ning
// `fromSeed` tonal palitrasi seed rangni ANIQ o'zi sifatida saqlamaydi
// (qorong'i rejimda biroz o'zgartiradi), shuning uchun `primary` ANIQ
// #F64E03 bo'lishini kafolatlash uchun majburan qayta yoziladi.
final _darkScheme = ColorScheme.fromSeed(
  seedColor: kBrandColor,
  brightness: Brightness.dark,
).copyWith(primary: kBrandColor);

class CourierApp extends StatelessWidget {
  const CourierApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ChustApp Kuryer',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: _darkScheme,
        scaffoldBackgroundColor: const Color(0xFF121212),
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFF121212),
          elevation: 0,
          scrolledUnderElevation: 0,
        ),
      ),
      home: const _Root(),
    );
  }
}

/// Saqlangan token bo'lsa, `/me` orqali rol tekshiriladi: "courier" bo'lsa
/// to'g'ridan-to'g'ri asosiy ekranga, boshqa (masalan hali "customer")
/// bo'lsa kuryerlikka ariza berish ekraniga yo'naltiriladi.
class _Root extends StatefulWidget {
  const _Root();

  @override
  State<_Root> createState() => _RootState();
}

class _RootState extends State<_Root> {
  bool _loading = true;
  Widget _child = const LoginScreen();

  @override
  void initState() {
    super.initState();
    _restore();
  }

  Future<void> _restore() async {
    // Token endi shifrlangan omborda (TokenStore) — eski, shifrlanmagan
    // `SharedPreferences` nusxasi birinchi o'qishda avtomatik ko'chiriladi.
    final token = await tokenStore.read();
    if (token == null || token.isEmpty) {
      setState(() => _loading = false);
      return;
    }
    api.token = token;
    try {
      final user = await api.me();
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';
      _child = (role == 'courier' && entityId.isNotEmpty)
          ? CourierShell(courierId: entityId)
          : const RegisterScreen();
    } catch (_) {
      // Token eskirgan/noto'g'ri — qaytadan login qildiramiz.
      await tokenStore.clear();
      api.token = null;
      _child = const LoginScreen();
    }
    if (mounted) setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return _child;
  }
}
