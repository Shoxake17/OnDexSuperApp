import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import '../live.dart';
import '../pages/couriers_page.dart';
import '../pages/dashboard_page.dart';
import '../pages/ondexmap_page.dart';
import '../pages/orders_page.dart';
import '../pages/people_page.dart';
import '../pages/restaurants_page.dart';
import 'login_screen.dart';

/// Asosiy tuzilma: chapda navigatsiya, o'ngda sahifa.
class AdminShell extends StatefulWidget {
  const AdminShell({super.key});

  @override
  State<AdminShell> createState() => _AdminShellState();
}

class _AdminShellState extends State<AdminShell> {
  int _index = 0;

  @override
  void initState() {
    super.initState();
    // Jonli kanal BUTUN panel uchun bitta marta ochiladi (sahifalar
    // unga obuna bo'ladi — `lib/live.dart`). Ekran darajasida ochilsa
    // har bo'limga o'tganda yangi soket va yangi bilet kerak bo'lardi.
    adminLive.start();
  }

  @override
  void dispose() {
    // Panel yopilganda soket ham yopiladi.
    adminLive.stop();
    super.dispose();
  }

  // Tartib navigatsiya ro'yxatidagi tartib bilan AYNAN bir xil
  // bo'lishi shart (`_index` ikkalasi uchun bitta).
  static const _pages = [
    DashboardPage(),
    OrdersPage(),
    RestaurantsPage(),
    CouriersPage(),
    CustomersPage(),
    WaitersPage(),
    // OnDexMap — ALOHIDA loyihaning muharriri (oyna sifatida).
    // ChustApp bazasiga ham, API'siga ham tegmaydi.
    OndexMapPage(),
  ];

  Future<void> _logout() async {
    // Soket tokendan OLDIN yopiladi: aks holda u chiqib ketgan
    // sessiya uchun qayta ulanishga urinib, har safar 401 olardi.
    await adminLive.stop();
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('admin_token');
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const AdminLoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: _index,
            onDestinationSelected: (i) => setState(() => _index = i),
            labelType: NavigationRailLabelType.all,
            leading: const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Icon(Icons.admin_panel_settings, size: 32),
            ),
            trailing: Expanded(
              child: Align(
                alignment: Alignment.bottomCenter,
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: IconButton(
                    tooltip: 'Chiqish',
                    icon: const Icon(Icons.logout),
                    onPressed: _logout,
                  ),
                ),
              ),
            ),
            destinations: const [
              NavigationRailDestination(
                  icon: Icon(Icons.dashboard_outlined),
                  selectedIcon: Icon(Icons.dashboard),
                  label: Text('Boshqaruv')),
              NavigationRailDestination(
                  icon: Icon(Icons.receipt_long_outlined),
                  selectedIcon: Icon(Icons.receipt_long),
                  label: Text('Buyurtmalar')),
              NavigationRailDestination(
                  icon: Icon(Icons.storefront_outlined),
                  selectedIcon: Icon(Icons.storefront),
                  label: Text('Restoranlar')),
              NavigationRailDestination(
                  icon: Icon(Icons.delivery_dining_outlined),
                  selectedIcon: Icon(Icons.delivery_dining),
                  label: Text('Kuryerlar')),
              NavigationRailDestination(
                  icon: Icon(Icons.people_outline),
                  selectedIcon: Icon(Icons.people),
                  label: Text('Mijozlar')),
              NavigationRailDestination(
                  icon: Icon(Icons.room_service_outlined),
                  selectedIcon: Icon(Icons.room_service),
                  label: Text('Affitsiantlar')),
              // Tartib `_pages` bilan AYNAN bir xil bo'lishi shart —
              // ikkalasi uchun bitta `_index` ishlatiladi.
              NavigationRailDestination(
                  icon: Icon(Icons.map_outlined),
                  selectedIcon: Icon(Icons.map),
                  label: Text('OnDexMap')),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(child: _pages[_index]),
        ],
      ),
    );
  }
}
