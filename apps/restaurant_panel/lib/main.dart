import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';
import 'screens/login_screen.dart';
import 'screens/shell.dart';
import 'theme.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Mahsulot tahlili. Kalit (ONDEX_POSTHOG_KEY) berilmagan bo'lsa
  // hech narsa yuborilmaydi va hech qanday kechikish qo'shilmaydi.
  //
  // Xatolik ilovani TO'XTATMAYDI: tahlil hech qachon ishga tushishga
  // to'sqinlik qilmasligi kerak.
  await Analytics.bootstrap();

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
    // Token — shifrlangan ombordan (bug.md 2-band). `rest_rid` sir
    // emas (oddiy restoran ID'si) va `SharedPreferences` da qoladi —
    // lekin u endi FAQAT zaxira: haqiqiy manba `me()` javobi.
    final token = await restTokenStore.read();
    if (token == null || token.isEmpty) {
      if (mounted) setState(() => _loading = false);
      return;
    }
    api.token = token;
    // ┌─ TUZATILGAN NOSOZLIK (bug.md 63-band) ────────────────────────┐
    // Panel AVVAL tokenni ham, `rid` ni ham tekshirmasdan ichkariga
    // kiritardi (sabab admin panelidagi bir xil izohda).
    //
    // `rid` endi `me()` javobidagi `entity_id` dan olinadi — YAGONA
    // HAQIQAT MANBAI. Avval u `SharedPreferences` dan kelardi va
    // Windows'da o'sha fayl tahrirlanishi mumkin edi: xavfsizlik
    // xavfi yo'q (server `claims.EntityID` ni tekshiradi va 403
    // beradi), lekin panel tushunarsiz xatolar bilan to'lardi.
    // └───────────────────────────────────────────────────────────────┘
    try {
      final user = await api.me();
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';
      if (role == 'restaurant' && entityId.isNotEmpty) {
        api.rid = entityId;
        final prefs = await SharedPreferences.getInstance();
        await prefs.setString('rest_rid', entityId);
        _loggedIn = true;
      } else {
        await restTokenStore.clear();
        api.token = null;
      }
    } on ApiException catch (e) {
      // Faqat 401 da chiqaramiz — tarmoq xatosida emas (60-band):
      // restoran zaif tarmoqda ham panelni ocha olishi kerak.
      if (e.isUnauthorized) {
        await restTokenStore.clear();
        api.token = null;
        _loggedIn = false;
      } else {
        _loggedIn = await _resumeFromCache();
      }
    } catch (_) {
      _loggedIn = await _resumeFromCache();
    }
    if (!mounted) return;
    setState(() => _loading = false);
  }

  /// Server javob bermaganda — keshdagi `rid` bilan davom etish.
  /// `rid` bo'lmasa ichkariga kirishning ma'nosi yo'q: panelning
  /// deyarli har bir so'rovi unga tayanadi.
  Future<bool> _resumeFromCache() async {
    final prefs = await SharedPreferences.getInstance();
    final rid = prefs.getString('rest_rid') ?? '';
    if (rid.isEmpty) return false;
    api.rid = rid;
    return true;
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return _loggedIn ? const RestaurantShell() : const RestaurantLoginScreen();
  }
}
