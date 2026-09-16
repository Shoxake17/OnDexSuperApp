import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:posthog_flutter/posthog_flutter.dart';

import 'api.dart';
import 'push.dart';
import 'screens/courier_shell.dart';
import 'screens/login_screen.dart';
import 'screens/register_screen.dart';
import 'theme.dart';
import 'session.dart';

/// Butun ilova uchun navigator — sessiya istalgan ekranda (masalan fon
/// so'rovida) yaroqsiz bo'lsa ham login ekraniga qaytarish uchun.
final _navigatorKey = GlobalKey<NavigatorState>();

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Mahsulot tahlili. Kalit (ONDEX_POSTHOG_KEY) berilmagan bo'lsa
  // hech narsa yuborilmaydi va hech qanday kechikish qo'shilmaydi.
  //
  // Xatolik ilovani TO'XTATMAYDI: tahlil hech qachon ishga tushishga
  // to'sqinlik qilmasligi kerak.
  //
  // ┌─ SEANS YOZUVI (session replay) ───────────────────────────────────┐
  // Matn va rasmlar NIQOBLANADI: telefon raqami, manzil yozuvga
  // tushmasligi kerak. posthog_flutter Windows'ni qo'llamaydi — bu
  // faqat mobil ilovalarda.
  // └───────────────────────────────────────────────────────────────────┘
  if (posthogEnabled) {
    try {
      final cfg = PostHogConfig(posthogApiKey)
        ..host = posthogHost
        ..sessionReplay = true
        ..sessionReplayConfig.maskAllTexts = true
        ..sessionReplayConfig.maskAllImages = true
        // posthog_flutter 5.39 da `captureScreenViews`/`debouncerTimeSecs`
        // YO'Q (build aynan shu sababli yiqilardi). Ekran nomlari
        // `Analytics.instance.screen(...)` orqali yuboriladi.
        ..captureApplicationLifecycleEvents = true;
      await Posthog().setup(cfg);
      Analytics.instance.onIdentify = ({
        required String userId,
        String? phone,
        String? name,
        String? role,
      }) async {
        try {
          final props = <String, Object>{};
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

  // ┌─ 401 — LOGIN EKRANIGA (kuryer ilovasi auditi, 5-band) ─────────────┐
  // Avval bu ilgak ULANMAGAN edi. Restoran kirishni yopsa (ta'til, ishdan
  // bo'shatish) yoki token muddati tugasa, ilova ochiq qolib har so'rovda
  // xato ko'rsatardi, WebSocket esa har 2 soniyada 401 olib qayta urinardi
  // (bitta telefondan soatiga ~1800 so'rov).
  // └────────────────────────────────────────────────────────────────────┘
  api.onUnauthorized = _handleUnauthorized;

  // Ilova yopiq paytdagi taklif push'i uchun fon ishlovchisi — runApp dan
  // OLDIN (`push.dart`).
  await initCourierPushBackground();

  runApp(const CourierApp());
}

bool _loggingOut = false;

void _handleUnauthorized() {
  // Token yo'q bo'lsa (masalan login jarayonidagi xato kod) — bu sessiya
  // tugashi emas, hech narsa qilinmaydi.
  if (_loggingOut || api.token == null) return;
  _loggingOut = true;
  api.token = null;
  unawaited(tokenStore.clear().whenComplete(() {
    _loggingOut = false;
    final nav = _navigatorKey.currentState;
    if (nav == null) return;
    nav.pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (_) => false,
    );
    final ctx = _navigatorKey.currentContext;
    if (ctx != null && ctx.mounted) {
      ScaffoldMessenger.maybeOf(ctx)?.showSnackBar(const SnackBar(
        content: Text('Sessiya tugadi yoki ilovaga kirishingiz yopildi — qaytadan kiring'),
      ));
    }
  }));
}

// Yorug' mavzu (image/Kuryer.png, 2026-09-15). `.copyWith(primary: ...)` —
// Material3'ning `fromSeed` palitrasi seed rangni ANIQ o'zi sifatida
// saqlamaydi, `primary` aynan #F64E03 bo'lishi uchun qayta yoziladi.
final _lightScheme = ColorScheme.fromSeed(
  seedColor: kBrandColor,
  brightness: Brightness.light,
).copyWith(primary: kBrandColor, surface: kSurface);

class CourierApp extends StatelessWidget {
  const CourierApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'OnDexGO',
      navigatorKey: _navigatorKey,
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: _lightScheme,
        scaffoldBackgroundColor: kSurface,
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: kSurface,
          foregroundColor: kInk,
          elevation: 0,
          scrolledUnderElevation: 0,
        ),
        navigationBarTheme: NavigationBarThemeData(
          backgroundColor: kSurface,
          indicatorColor: kBrandColor.withValues(alpha: 0.14),
        ),
      ),
      home: const _Root(),
    );
  }
}

/// Saqlangan token bo'lsa, `/me` orqali rol tekshiriladi: "courier" bo'lsa
/// to'g'ridan-to'g'ri asosiy ekranga, aks holda "akkauntni restoran beradi"
/// ekraniga yo'naltiriladi.
class _Root extends StatefulWidget {
  const _Root();

  @override
  State<_Root> createState() => _RootState();
}

class _RootState extends State<_Root> {
  bool _loading = true;
  String? _error;
  Widget _child = const LoginScreen();

  @override
  void initState() {
    super.initState();
    _restore();
  }

  Future<void> _restore() async {
    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    // Token shifrlangan omborda (TokenStore) — eski, shifrlanmagan
    // `SharedPreferences` nusxasi birinchi o'qishda avtomatik ko'chiriladi.
    final token = await tokenStore.read();
    if (token == null || token.isEmpty) {
      if (mounted) setState(() => _loading = false);
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
    } on ApiException catch (e) {
      if (e.isUnauthorized) {
        // Token haqiqatan yaroqsiz — faqat SHU holatda o'chiriladi.
        await tokenStore.clear();
        api.token = null;
        _child = const LoginScreen();
      } else {
        // ┌─ TARMOQ/SERVER XATOSI — TOKEN O'CHIRILMAYDI (auditi, 6-band) ─┐
        // Avval har qanday xato tokenni o'chirardi: signal yomon joyda
        // ilovani ochgan kuryer har safar Telegram orqali qaytadan
        // kirishga majbur bo'lardi.
        // └──────────────────────────────────────────────────────────────┘
        _error = e.message;
      }
    } catch (_) {
      _error = 'Serverga ulanib bo\'lmadi — internetni tekshiring';
    }
    if (mounted) setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    final error = _error;
    if (error != null) {
      return Scaffold(
        body: SafeArea(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.wifi_off_rounded, size: 56),
                  const SizedBox(height: 16),
                  Text(error, textAlign: TextAlign.center),
                  const SizedBox(height: 20),
                  FilledButton.icon(
                    onPressed: _restore,
                    icon: const Icon(Icons.refresh),
                    label: const Text('Qayta urinish'),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }
    return _child;
  }
}
