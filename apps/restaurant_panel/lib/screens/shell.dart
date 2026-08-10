import 'dart:async';

import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import '../pages/coming_soon_page.dart';
import '../pages/dashboard_page.dart';
import '../pages/menu_page.dart';
import '../pages/orders_page.dart';
import '../pages/promotions_page.dart';
import '../sound.dart';
import '../widgets/main_layout.dart';
import 'login_screen.dart';

const _iDashboard = 0;
const _iOrders = 1;
const _iMenu = 2;

class RestaurantShell extends StatefulWidget {
  const RestaurantShell({super.key});

  @override
  State<RestaurantShell> createState() => _RestaurantShellState();
}

class _RestaurantShellState extends State<RestaurantShell> {
  int _index = _iDashboard;
  String _name = '';
  String _address = '';
  String _logoUrl = '';
  bool _open = true;
  String _staffName = '';
  String _staffRole = '';
  DateTime _selectedDate = DateTime.now();
  int _newOrdersCount = 0;
  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    _loadInfo();
    _pollOrders();
    // Buyurtmalar sahifasining o'zi WebSocket orqali jonli yangilanadi;
    // sidebar rozetkasi/qo'ng'iroq belgisi uchun esa shunchaki 20s'da
    // yangilanib tursa yetarli — bu yerda alohida WS ulanish ochish
    // ortiqcha bo'lardi (MainLayout/Sidebar/TopBar barchasi shu BITTA
    // qiymatdan foydalanadi, ilgari har biri o'zi alohida so'rov yuborardi).
    _pollTimer = Timer.periodic(const Duration(seconds: 20), (_) => _pollOrders());
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  Future<void> _loadInfo() async {
    try {
      final results = await Future.wait([api.myRestaurant(), api.me()]);
      if (!mounted) return;
      final r = results[0];
      final u = results[1];
      setState(() {
        _name = r['name'] ?? '';
        _address = r['address'] as String? ?? '';
        _logoUrl = r['logo_url'] as String? ?? '';
        _open = r['open'] == true;
        _staffName = (u['name'] as String?)?.trim() ?? '';
        _staffRole = _roleLabel(u['role'] as String? ?? '');
      });
    } catch (_) {}
  }

  Future<void> _pollOrders() async {
    try {
      final list = await api.orders();
      if (!mounted) return;
      final newCount = list
          .cast<Map<String, dynamic>>()
          .where((o) => o['status'] == 'created')
          .length;
      setState(() => _newOrdersCount = newCount);
    } catch (_) {
      // Ko'makchi hisoblagich — tarmoq xatosi butun panelni to'xtatmasin,
      // keyingi davriy urinishda o'zi tuzaladi.
    }
  }

  Future<void> _toggleOpen(bool v) async {
    try {
      await api.setOpen(v);
      setState(() => _open = v);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(v
              ? 'Restoran OCHIQ — buyurtmalar qabul qilinadi'
              : 'Restoran YOPIQ — yangi buyurtmalar kelmaydi')));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Xato: $e')));
    }
  }

  Future<void> _logout() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('rest_token');
    await prefs.remove('rest_rid');
    api.token = null;
    api.rid = '';
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const RestaurantLoginScreen()),
    );
  }

  void _goTo(int i) => setState(() => _index = i);

  @override
  Widget build(BuildContext context) {
    // Kirish tokeni saqlanib qolgani uchun panel bosishsiz avtomatik ochiladi
    // — brauzer esa gesture'siz ovoz chalishga ruxsat bermaydi. Shu sababli
    // butun panelni tinglovchiga o'raymiz: qayerga bo'lsa ham BIRINCHI
    // bosishda ovoz "qulfdan chiqariladi", shundan keyin yangi buyurtma
    // qo'ng'irog'i har doim eshitiladi.
    return Listener(
      onPointerDown: (_) => RingSound.unlock(),
      child: MainLayout(
        selectedIndex: _index,
        onSelect: _goTo,
        restaurantName: _name,
        restaurantAddress: _address,
        restaurantLogoUrl: _logoUrl,
        restaurantOpen: _open,
        onOpenChanged: _toggleOpen,
        staffName: _staffName,
        staffRole: _staffRole,
        selectedDate: _selectedDate,
        onDateChanged: (d) => setState(() => _selectedDate = d),
        newOrdersCount: _newOrdersCount,
        onLogout: _logout,
        child: _buildPage(),
      ),
    );
  }

  Widget _buildPage() {
    switch (_index) {
      case _iDashboard:
        return DashboardPage(
          referenceDate: _selectedDate,
          onGoToOrders: () => _goTo(_iOrders),
          onGoToMenu: () => _goTo(_iMenu),
        );
      case _iOrders:
        return const OrdersPage();
      case _iMenu:
        return const MenuPage();
      case 3:
        return const PromotionsPage();
      case 4:
        return const ComingSoonPage(
          title: 'Statistika',
          icon: Icons.bar_chart_rounded,
          description:
              'Savdo dinamikasi, eng ko\'p sotilgan taomlar va band soatlar tahlili — tez orada. Asosiy ko\'rsatkichlar hozircha Bosh sahifada.',
        );
      case 5:
        return const ComingSoonPage(
          title: 'Moliya',
          icon: Icons.account_balance_wallet_rounded,
          description:
              'To\'lov tarixi, komissiya tafsiloti, bank rekvizitlari — to\'lov tizimi (Payme/Click) ulanganidan keyin qo\'shiladi.',
        );
      case 6:
        return const ComingSoonPage(
          title: 'Restoran sozlamalari',
          icon: Icons.settings_rounded,
          description:
              'Ish vaqti jadvali, yetkazish sozlamalari — tez orada. Hozircha restoran profilini superadmin tahrirlaydi.',
        );
      case 7:
        return const ComingSoonPage(
          title: 'Xodimlar',
          icon: Icons.groups_rounded,
          description:
              'Bir nechta xodim uchun alohida kirish (rol bilan) — tez orada. Hozircha bitta restoran uchun bitta umumiy login bor.',
        );
      case 8:
        return const ComingSoonPage(
          title: 'Bildirishnomalar',
          icon: Icons.notifications_rounded,
          description: 'Barcha o\'tgan bildirishnomalar tarixi — tez orada.',
        );
      case 9:
        return const ComingSoonPage(
          title: 'Yordam markazi',
          icon: Icons.help_rounded,
          description:
              'Ko\'p so\'raladigan savollar va qo\'llab-quvvatlash bilan bog\'lanish — tez orada.',
        );
      default:
        return const SizedBox.shrink();
    }
  }
}

String _roleLabel(String role) => switch (role) {
      'restaurant' => 'Xodim',
      'admin' => 'Administrator',
      _ => role,
    };
