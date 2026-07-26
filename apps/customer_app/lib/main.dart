import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';
import 'screens/login_screen.dart';
import 'screens/restaurants_screen.dart';

void main() {
  runApp(const ChustApp());
}

class ChustApp extends StatelessWidget {
  const ChustApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ChustApp',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF1B873F)),
        useMaterial3: true,
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
    final prefs = await SharedPreferences.getInstance();
    final token = prefs.getString('token');
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
    return _loggedIn ? const RestaurantsScreen() : const LoginScreen();
  }
}
