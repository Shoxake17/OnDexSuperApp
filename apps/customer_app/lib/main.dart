import 'dart:async';

import 'package:app_links/app_links.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/foundation.dart' show kDebugMode;
import 'package:flutter/material.dart';
// `SystemChrome` / `SystemUiMode` uchun.
import 'package:flutter/services.dart';
// `Analytics`, `posthog*` — `api.dart` orqali (u `ondex_core` ni
// qayta eksport qiladi).
import 'package:posthog_flutter/posthog_flutter.dart';

import 'api.dart';
import 'session.dart';
import 'data/agent_driver.dart';
import 'screens/connected_apps_screen.dart';
import 'screens/home_shell.dart';
import 'screens/lock_gate.dart';
import 'screens/login_screen.dart';
import 'widgets/agent_overlay.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // ┌─ TIZIM PANELLARI SHAFFOF ─────────────────────────────────────┐
  // Ilova status-bar va pastki navigatsiya paneli ORQASIGA ham
  // chizadi. Busiz Android o'z fonini qo'yadi va biz so'ragan rang
  // (oq pastki panel, qora status chizig'i) e'tiborga olinmasdi —
  // Back/Home/Menu paneli to'q kulrang bo'lib qolardi.
  //
  // Chekinishlarni `SafeArea` va `Scaffold` hisobga oladi, ya'ni
  // tarkib panellar ostida qolib ketmaydi.
  // └───────────────────────────────────────────────────────────────┘
  SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);

  // Firebase Phone Auth uchun. Sozlamalar `google-services.json` dan
  // (Android) build vaqtida olinadi.
  //
  // XATO ILOVANI TO'XTATMAYDI: Firebase faqat telefon tasdiqlash
  // uchun kerak — u ishga tushmasa ham katalog, savat, buyurtma va
  // email bilan kirish ishlashda davom etishi kerak. Aks holda
  // konfiguratsiya muammosi butun ilovani o'ldirardi.
  try {
    await Firebase.initializeApp();
  } catch (e) {
    debugPrint('Firebase ishga tushmadi (telefon tasdiqlash ishlamaydi): $e');
  }

  // ┌─ MAHSULOT TAHLILI VA SEANS YOZUVI ────────────────────────────┐
  // Ikki qatlam ATAYLAB:
  //
  //   Analytics (`ondex_core`) — HODISALAR. Sof HTTP, hamma
  //   platformada ishlaydi, panellar ham shundan foydalanadi.
  //
  //   PostHog SDK — SEANS YOZUVI (foydalanuvchi ekranini video kabi
  //   qayta ko'rish). Bu faqat Android/iOS/veb'da mumkin.
  //
  // Kalit berilmagan bo'lsa (`ONDEX_POSTHOG_KEY` bo'sh) ikkalasi ham
  // JIM turadi — dev build hech qayerga ma'lumot yubormaydi.
  //
  // XATO ILOVANI TO'XTATMAYDI: tahlil hech qachon buyurtma berishga
  // to'sqinlik qilmasligi kerak.
  // └───────────────────────────────────────────────────────────────┘
  if (posthogEnabled) {
    try {
      final cfg = PostHogConfig(posthogApiKey)
        ..host = posthogHost
        ..sessionReplay = true
        // Session Replay uchun to'liq config:
        //   1) maskAllTexts/maskAllImages — shaxsiy ma'lumot niqoblash
        //   2) captureApplicationLifecycleEvents — ilova foreground/background
        //      o'tganda ham hodisa yozish
        // (`captureScreenViews`/`debouncerTimeSecs` posthog_flutter 5.39.0
        // da olib tashlangan — endi ekran ko'rinishini avtomatik yozish
        // uchun `PosthogObserver` navigator observer sifatida ulanishi
        // kerak, hodisalar esa `flushAt`/`flushInterval` standart
        // qiymatlari bilan to'planadi.)
        ..sessionReplayConfig.maskAllTexts = true
        ..sessionReplayConfig.maskAllImages = true
        ..captureApplicationLifecycleEvents = true;
      await Posthog().setup(cfg);
      // ┌─ SDK BRIDGE: ondex_core Analytics bilan sinxronlash ─────┐
      // `api.me()` da `Analytics.instance.identify()` chaqirilganda,
      // shu callback orqali posthog_flutter SDK ga ham aytamiz — va
      // SDK Session Replay yozuvini shu PERSON ga biriktiradi.
      //
      // Agar bu bridge bo'lmasa:
      //   * ondex_core odam yaratadi (Properties tayyor) ✅
      //   * SDK Recording yozadi, lekin ANONIM holatda ❌
      //   * Natija: PostHog da person bor, Recordings bo'sh turadi
      // └───────────────────────────────────────────────────────────┘
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

  runApp(const ChustApp());
}

/// OnDex brend rangi — logotipdagi "Dex", faol menyu elementi va QR
/// tugmasi. `apps/web/tailwind.config.ts` dagi `brand.DEFAULT` va
/// `screens/home_shell.dart` dagi `_kBrand` bilan AYNAN bir xil.
const kBrand = Color(0xFFF4511E);

// ┌─ YORUG' MAVZU (2026-08-17) ────────────────────────────────────────┐
// Avval ilova qorong'i edi (`#121212` fon, `#1B873F` yashil urg'u).
// Endi maketdagidek (`image/restarant.png`) — OQ fon, brend to'q
// sariq urg'u.
//
// NEGA URUG' RANG (seed) ham o'zgardi: yashil `#1B873F` Telegram'ning
// standart rangi edi va OnDex brendiga aloqasi yo'q. Tugmalar,
// belgilar va tanlangan elementlar shu rangdan kelib chiqadi —
// natijada ilova ichida ikkita rang (yashil va to'q sariq) yonma-yon
// turardi.
//
// Qorong'i rejim KEYINCHALIK qo'shiladi: o'shanda shu yerga
// `darkTheme:` va `themeMode:` qo'shiladi. Veb tomonidagi mos
// almashtirgich — `apps/web/tailwind.config.ts` dagi `darkMode` izohi.
// └────────────────────────────────────────────────────────────────────┘
// ┌─ `primary` URUG'DAN EMAS, BREND RANGINING O'ZI ────────────────────┐
// `ColorScheme.fromSeed` Material 3 qoidasi bo'yicha urug'dan TONAL
// palitra hosil qiladi va `primary` urug'ning AYNAN o'zi bo'lmaydi.
// `#F4511E` (to'q sariq) dan chiqqan `primary` jigarrang edi — va
// `FilledButton` lar (masalan manzil ekranidagi "Tayyor") ilovaning
// qolgan qismidan butunlay boshqa rangda ko'rinardi.
//
// Shuning uchun `primary` va `onPrimary` QO'LDA qo'yiladi. Qolgan
// tonal ranglar (`primaryContainer`, `surfaceTint` va h.k.) urug'dan
// hosil bo'lishda davom etadi — ular fon va soyalar uchun, ularda
// aniq brend rangi talab qilinmaydi.
// └────────────────────────────────────────────────────────────────────┘
final _lightScheme = ColorScheme.fromSeed(
  seedColor: kBrand,
  brightness: Brightness.light,
).copyWith(
  primary: kBrand,
  onPrimary: Colors.white,
);

class ChustApp extends StatelessWidget {
  const ChustApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ChustApp',
      debugShowCheckedModeBanner: false,
      // Shaddiy ilovani boshqarganda ekranlar orasida yurishi kerak —
      // yordamchi oynasi yopilgach uning `BuildContext` i o'ladi,
      // shuning uchun ildiz navigatoriga kalit qo'yiladi
      // (`lib/data/agent_driver.dart`).
      navigatorKey: appNavigatorKey,
      // Holat lentasi va "barmoq" BUTUN ilova ustida turadi: boshqaruv
      // bitta ekranda emas, ekranlar orasida ketadi.
      builder: (context, child) =>
          AgentOverlay(child: child ?? const SizedBox.shrink()),
      theme: ThemeData(
        colorScheme: _lightScheme,
        // OQ — WebView ichidagi sahifaning foni bilan AYNAN bir xil
        // (`apps/web/app/globals.css` dagi `body`). Bu shart: WebView
        // `SafeArea` ichida turadi va tepa/pastda Flutter'ning o'z foni
        // ko'rinadi — ranglar farq qilsa chok (seam) sezilib qoladi.
        scaffoldBackgroundColor: Colors.white,
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: Colors.white,
          foregroundColor: Color(0xFF171717),
          elevation: 0,
          scrolledUnderElevation: 0,
        ),
        // Kartalar oq fonda ajralib turishi uchun yengil chegara —
        // qorong'i mavzuda buni rang farqi (`#1E1E1E` va `#121212`)
        // bajarardi, oq fonda esa u ishlamaydi.
        cardTheme: CardThemeData(
          color: Colors.white,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
            side: const BorderSide(color: Color(0xFFE5E5E5)),
          ),
        ),
        // Pastki navigatsiya paneli ixchamlashtirildi: Material 3'ning
        // standart balandligi 80dp — bu kichik ekranlarda kontent uchun
        // juda ko'p joy oladi. 58dp + kichikroq ikonka/matn bilan
        // ancha yig'iq, lekin barmoq bilan bosish uchun hali ham qulay
        // (Material'ning 48dp minimal teginish maydonidan katta).
        navigationBarTheme: NavigationBarThemeData(
          height: 58,
          backgroundColor: Colors.white,
          surfaceTintColor: Colors.transparent,
          indicatorColor: _lightScheme.primary.withValues(alpha: 0.14),
          labelTextStyle: const WidgetStatePropertyAll(
            TextStyle(fontSize: 11, fontWeight: FontWeight.w500),
          ),
          iconTheme: const WidgetStatePropertyAll(
            IconThemeData(size: 22),
          ),
        ),
      ),
      home: const _Root(),
    );
  }
}

/// Saqlangan token bo'lsa to'g'ri restoranlarga, bo'lmasa login ekraniga.
class _Root extends StatefulWidget {
  const _Root();

  @override
  State<_Root> createState() => _RootState();
}

class _RootState extends State<_Root> {
  bool _loading = true;
  bool _loggedIn = false;

  /// Deep link tinglagichi (`ondex://agent-link?code=...`).
  ///
  /// ┌─ NEGA SHU YERDA, EKRAN ICHIDA EMAS ────────────────────────────┐
  /// Havola ilova YOPIQ bo'lganda ham kelishi mumkin — o'shanda hech
  /// bir ekran hali qurilmagan bo'ladi. Ildizda tinglash ikkala
  /// holatni ham qamrab oladi: "sovuq" ishga tushish (`getInitialLink`)
  /// va ilova ochiq turganda kelgan havola (`uriLinkStream`).
  ///
  /// Telegram qaytish havolasi (`ondex://auth`) bu yerda O'QILMAYDI —
  /// uni `telegram_auth.dart` o'z oqimida kutadi.
  /// └────────────────────────────────────────────────────────────────┘
  final _links = AppLinks();
  StreamSubscription<Uri>? _linkSub;

  @override
  void initState() {
    super.initState();
    _restore();
    _listenLinks();
  }

  @override
  void dispose() {
    _linkSub?.cancel();
    super.dispose();
  }

  Future<void> _listenLinks() async {
    _linkSub = _links.uriLinkStream.listen(_handleLink, onError: (_) {});
    try {
      final initial = await _links.getInitialLink();
      if (initial != null) _handleLink(initial);
    } catch (_) {
      // Havolani o'qib bo'lmasa ilova odatdagidek ochilaveradi.
    }
  }

  void _handleLink(Uri uri) {
    if (uri.host != 'agent-link') return;
    final code = uri.queryParameters['code'] ?? '';
    if (code.trim().isEmpty) return;
    // Kirmagan foydalanuvchi rozilik BERA OLMAYDI: kimga ruxsat
    // berilayotgani noma'lum bo'lardi. Havola shunchaki e'tiborsiz
    // qoldiriladi — kirgandan keyin kodni qo'lda kiritishi mumkin.
    if (!_loggedIn) return;
    // Ekran qurilib bo'lgach ochiladi.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => ConnectedAppsScreen(initialCode: code),
      ));
    });
  }

  Future<void> _restore() async {
    // TokenStore — Keystore bilan shifrlangan ombor (eski, shifrlanmagan
    // SharedPreferences nusxasini avtomatik ko'chirib o'chiradi).
    final token = await tokenStore.read();
    if (token != null && token.isNotEmpty) {
      api.token = token;
      _loggedIn = true;
    }
    setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    // Qulf FAQAT sessiyali holatni o'raydi — sabab `LockGate`
    // izohida (kirish oqimlari ilovani ataylab fonga chiqaradi).
    return _loggedIn
        ? const LockGate(child: HomeShell())
        : const LoginScreen();
  }
}
