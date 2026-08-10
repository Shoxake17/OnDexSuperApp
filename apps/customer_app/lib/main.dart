import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/material.dart';

import 'api.dart';
import 'session.dart';
import 'screens/home_shell.dart';
import 'screens/lock_gate.dart';
import 'screens/login_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
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

// Yandex Eats/Wolt uslubidagi qorong'i mavzu: qora fon, oq matn, yashil urg'u.
final _darkScheme = ColorScheme.fromSeed(
  seedColor: const Color(0xFF1B873F),
  brightness: Brightness.dark,
);

class ChustApp extends StatelessWidget {
  const ChustApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ChustApp',
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
        cardTheme: const CardThemeData(
          color: Color(0xFF1E1E1E),
          elevation: 0,
        ),
        // Pastki navigatsiya paneli ixchamlashtirildi: Material 3'ning
        // standart balandligi 80dp — bu kichik ekranlarda kontent uchun
        // juda ko'p joy oladi. 58dp + kichikroq ikonka/matn bilan
        // ancha yig'iq, lekin barmoq bilan bosish uchun hali ham qulay
        // (Material'ning 48dp minimal teginish maydonidan katta).
        navigationBarTheme: NavigationBarThemeData(
          height: 58,
          backgroundColor: const Color(0xFF121212),
          surfaceTintColor: Colors.transparent,
          indicatorColor: _darkScheme.primary.withValues(alpha: 0.22),
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
