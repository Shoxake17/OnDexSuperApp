import 'package:flutter/material.dart';

import 'api.dart';
import 'screens/login_screen.dart';
import 'screens/shell.dart';

void main() {
  runApp(const AdminApp());
}

class AdminApp extends StatelessWidget {
  const AdminApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ChustApp Admin',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)),
        useMaterial3: true,
      ),
      home: const _Root(),
    );
  }
}

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
    // Shifrlangan ombordan (`ondex_core.TokenStore`) — bug.md 2-band.
    final token = await adminTokenStore.read();
    if (token == null || token.isEmpty) {
      if (mounted) setState(() => _loading = false);
      return;
    }
    api.token = token;
    // ┌─ TUZATILGAN NOSOZLIK (bug.md 63-band) ────────────────────────┐
    // Panel AVVAL tokenni HECH TEKSHIRMASDAN ichkariga kiritardi.
    // Muddati o'tgan yoki bekor qilingan token bilan ham panel
    // ochilardi va har bir so'rovda 401 bilan qulardi — login
    // ekraniga qaytish yo'li esa yo'q edi (`onUnauthorized` ham
    // ulanmagan edi).
    //
    // Admin panelida bu jiddiyroq: 56-bandga ko'ra o'sha token
    // WebView'ga har bir hujjatga kiritiladi, ya'ni yaroqsiz token
    // baribir tarqalardi.
    //
    // Affitsiant va kuryer ilovalaridagi naqsh takrorlandi:
    // `me()` bilan tekshirish + ROL tekshiruvi.
    // └───────────────────────────────────────────────────────────────┘
    try {
      final user = await api.me();
      _loggedIn = (user['role'] as String? ?? '') == 'admin';
      if (!_loggedIn) {
        await adminTokenStore.clear();
        api.token = null;
      }
    } on ApiException catch (e) {
      // Faqat 401 da chiqaramiz — tarmoq xatosida emas (60-band).
      if (e.isUnauthorized) {
        await adminTokenStore.clear();
        api.token = null;
        _loggedIn = false;
      } else {
        _loggedIn = true;
      }
    } catch (_) {
      _loggedIn = true;
    }
    if (!mounted) return;
    setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return _loggedIn ? const AdminShell() : const AdminLoginScreen();
  }
}
