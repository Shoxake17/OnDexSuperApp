import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';
import 'screens/login_screen.dart';
import 'screens/shell.dart';
import 'theme.dart';

void main() {
  runApp(const RestaurantApp());
}

class RestaurantApp extends StatelessWidget {
  const RestaurantApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'OnDex — Restoran paneli',
      debugShowCheckedModeBanner: false,
      theme: buildOnDexTheme(),
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
    final prefs = await SharedPreferences.getInstance();
    final token = prefs.getString('rest_token');
    final rid = prefs.getString('rest_rid');
    if (token != null && token.isNotEmpty && rid != null && rid.isNotEmpty) {
      api.token = token;
      api.rid = rid;
      _loggedIn = true;
    }
    setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return _loggedIn ? const RestaurantShell() : const RestaurantLoginScreen();
  }
}
