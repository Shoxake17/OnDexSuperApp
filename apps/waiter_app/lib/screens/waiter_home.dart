import 'package:flutter/material.dart';

import '../session.dart';
import '../api.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/bottom_bar.dart';
import '../widgets/common.dart';
import 'login_screen.dart';
import 'notifications_screen.dart';
import 'orders_screen.dart';
import 'profile_screen.dart';
import 'tables_screen.dart';

/// Ilovaning asosiy qobig'i — to'rt bo'lim va umumiy holat.
///
/// `WaiterStore` AYNAN shu yerda yaratiladi va shu yerda o'chiriladi:
/// bo'limlar almashganda WebSocket uzilib-ulanmasligi kerak, aks holda
/// har teginishda yangi bilet so'ralib, server ortiqcha yuklanardi.
class WaiterHome extends StatefulWidget {
  const WaiterHome({super.key});

  @override
  State<WaiterHome> createState() => _WaiterHomeState();
}

class _WaiterHomeState extends State<WaiterHome> {
  final _store = WaiterStore();
  int _tab = 0;

  @override
  void initState() {
    super.initState();
    _store.onSessionLost = _goToLogin;
    _store.start();
  }

  @override
  void dispose() {
    _store.dispose();
    super.dispose();
  }

  /// Sessiya tugadi (401 yoki akkaunt o'chirildi) — kirish ekraniga.
  ///
  /// Tokenni tozalash SHART: aks holda ilova keyingi ochilishda o'sha
  /// yaroqsiz sessiya bilan urinaverardi va odam sababni ko'rmasdi.
  Future<void> _goToLogin() async {
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (_) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: _store,
      builder: (context, _) {
        return Scaffold(
          appBar: _buildAppBar(),
          body: SafeArea(
            top: false,
            bottom: false,
            child: IndexedStack(
              index: _tab,
              children: [
                OrdersScreen(store: _store),
                TablesScreen(store: _store),
                NotificationsScreen(store: _store),
                ProfileScreen(store: _store, onLogout: _logout),
              ],
            ),
          ),
          bottomNavigationBar: WaiterBottomBar(
            current: _tab,
            onSelect: (i) {
              setState(() => _tab = i);
              // Bildirishnomalar bo'limi ochilishi = o'qilgan.
              // Alohida "o'qildi" tugmasi ham bor, lekin bo'limni
              // ochib ko'rgandan keyin belgining qizil turishi
              // foydalanuvchini chalg'itardi.
              if (i == 2) _store.markFeedRead();
            },
            readyCount: _store.readyOrders.length,
            unreadCount: _store.unreadCount,
          ),
        );
      },
    );
  }

  PreferredSizeWidget _buildAppBar() {
    final titles = ['Buyurtmalar', 'Stollar', 'Xabarlar', 'Profil'];
    return AppBar(
      titleSpacing: 16,
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            titles[_tab],
            style: const TextStyle(fontSize: 19, fontWeight: FontWeight.bold),
          ),
          if (_store.restaurantName.isNotEmpty)
            Text(
              _store.restaurantName,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 12, color: kInkFaint),
            ),
        ],
      ),
      actions: [
        // Ulanish holati — faqat ma'lumot ko'rsatadigan bo'limlarda.
        // Profilda ro'yxat yo'q, u yerda bu belgi ma'nosiz.
        if (_tab != 3)
          Padding(
            padding: const EdgeInsets.only(right: 12),
            child: Center(child: ConnectionDot(online: _store.online)),
          ),
      ],
    );
  }

  Future<void> _logout() async {
    await _store.logout();
    await _goToLogin();
  }
}
