import 'package:flutter/material.dart';

import 'api.dart';
import 'screens/login_screen.dart';
import 'screens/waiter_shell.dart';
import 'session.dart';
import 'theme.dart';

void main() {
  runApp(const WaiterApp());
}

// Mijoz va kuryer ilovalari bilan bir xil qorong'i palitra.
// `.copyWith(primary: kBrandColor)` — Material3'ning `fromSeed` tonal
// palitrasi seed rangni ANIQ o'zi sifatida saqlamaydi, shuning uchun
// brend rangi majburan qayta yoziladi.
final _darkScheme = ColorScheme.fromSeed(
  seedColor: kBrandColor,
  brightness: Brightness.dark,
).copyWith(primary: kBrandColor);

class WaiterApp extends StatelessWidget {
  const WaiterApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'OnDex Affitsiant',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: _darkScheme,
        scaffoldBackgroundColor: kBackground,
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: kBackground,
          elevation: 0,
          scrolledUnderElevation: 0,
        ),
      ),
      home: const _Root(),
    );
  }
}

/// Saqlangan token bo'lsa, roli tekshiriladi.
///
/// Rol tekshiruvi SHART: xodim ishdan bo'shatilganda server uning
/// rolini `customer` ga qaytaradi va sessiyasini bekor qiladi
/// (`routes_tables.go`). Bu holatda ilova login ekraniga qaytishi
/// kerak — aks holda u bo'sh ro'yxat ko'rsatib turardi va sabab
/// ko'rinmasdi.
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
      if (role == 'waiter' && entityId.isNotEmpty) {
        _child = const WaiterShell();
      } else {
        await tokenStore.clear();
        api.token = null;
        _child = const LoginScreen();
      }
    } catch (_) {
      // Token eskirgan/bekor qilingan — qaytadan login.
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
