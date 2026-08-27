import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/material.dart';
// `SystemChrome` / `SystemUiMode` uchun.
import 'package:flutter/services.dart';

import 'api.dart';
import 'session.dart';
import 'screens/home_shell.dart';
import 'screens/lock_gate.dart';
import 'screens/login_screen.dart';

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
final _lightScheme = ColorScheme.fromSeed(
  seedColor: kBrand,
  brightness: Brightness.light,
);

class ChustApp extends StatelessWidget {
  const ChustApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ChustApp',
      debugShowCheckedModeBanner: false,
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

  @override
  void initState() {
    super.initState();
    _restore();
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
