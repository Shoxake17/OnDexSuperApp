import 'package:flutter/material.dart';

import 'api.dart';
import 'screens/login_screen.dart';
import 'screens/waiter_home.dart';
import 'session.dart';
import 'theme.dart';

void main() {
  runApp(const WaiterApp());
}

class WaiterApp extends StatelessWidget {
  const WaiterApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'OnDex Affitsiant',
      debugShowCheckedModeBanner: false,
      // Mavzu BITTA joyda (`theme.dart`) — ekranlar rang/radiusni o'zi
      // belgilamaydi. Mijoz va kuryer ilovalari bilan bir xil qorong'i
      // palitra: uchalasi bitta oila ekani ko'rinib turishi kerak.
      theme: buildWaiterTheme(),
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
        _child = const WaiterHome();
      } else {
        await tokenStore.clear();
        api.token = null;
        _child = const LoginScreen();
      }
    } on ApiException catch (e) {
      // ┌─ TUZATILGAN NOSOZLIK (bug.md 60-band) ────────────────────┐
      // Bu yerda AVVAL `catch (_)` turardi — ya'ni HAMMA narsa:
      // 401 ham, tarmoq uzilishi ham, timeout ham. Natijada
      // INTERNET YO'Q paytda ilova ochilsa foydalanuvchi tizimdan
      // chiqarib yuborilardi va qaytadan OTP olishi kerak bo'lardi.
      //
      // Affitsiant ish paytida, zaif tarmoqda ishlaydi — bu unga
      // eng noqulay joyda tegadigan xato edi.
      //
      // Endi tokenni faqat server "bu token yaroqsiz" deganda
      // (`401`) o'chiramiz. Tarmoq xatosida esa ichkariga
      // kiritamiz: keshdagi holat bilan ishlashda davom etadi va
      // birinchi muvaffaqiyatli so'rovda hammasi tiklanadi.
      // └───────────────────────────────────────────────────────────┘
      if (e.isUnauthorized) {
        await tokenStore.clear();
        api.token = null;
        _child = const LoginScreen();
      } else {
        _child = const WaiterHome();
      }
    } catch (_) {
      // Kutilmagan xato — token yaroqsiz deb hisoblamaymiz.
      _child = const WaiterHome();
    }
    if (mounted) setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      );
    }
    return _child;
  }
}
