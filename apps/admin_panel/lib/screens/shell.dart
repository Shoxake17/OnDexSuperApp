import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../pages/books_page.dart';
import '../pages/couriers_page.dart';
import '../pages/dashboard_page.dart';
import '../pages/ondexmap_page.dart';
import '../pages/orders_page.dart';
import '../pages/people_page.dart';
import '../pages/restaurants_page.dart';
import '../pages/settings_page.dart';
import '../pages/support_chat_page.dart';
import '../support_inbox.dart';
import 'login_screen.dart';

/// Asosiy tuzilma: chapda navigatsiya, o'ngda sahifa.
class AdminShell extends StatefulWidget {
  const AdminShell({super.key});

  @override
  State<AdminShell> createState() => _AdminShellState();
}

class _AdminShellState extends State<AdminShell> {
  int _index = 0;

  static const _pageNames = [
    'Dashboard',
    'Orders',
    'Restaurants',
    'Couriers',
    'Customers',
    'Waiters',
    'Books',
    'OnDexMap',
    'Chat',
    'Settings',
  ];

  @override
  void initState() {
    super.initState();
    // Jonli kanal BUTUN panel uchun bitta marta ochiladi (sahifalar
    // unga obuna bo'ladi — `lib/live.dart`). Ekran darajasida ochilsa
    // har bo'limga o'tganda yangi soket va yangi bilet kerak bo'lardi.
    adminLive.start();
    // Restoranlardan kelgan chat xabarlari — navigatsiyadagi rozetka.
    supportInbox.addListener(_onInbox);
    supportInbox.start();
    // Ilova ochilganda birinchi sahifani PostHog'ga yozamiz.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Analytics.instance.screen(_pageNames[0]);
    });
  }

  void _onInbox() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    supportInbox.removeListener(_onInbox);
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
    BooksPage(),
    // OnDexMap — ALOHIDA loyihaning muharriri (oyna sifatida).
    // ChustApp bazasiga ham, API'siga ham tegmaydi.
    OndexMapPage(),
    // Restoranlar bilan yozishma (restoran panelidagi "Chat markazi").
    SupportChatPage(),
    // Aloqa ma'lumotlari — barcha restoran panellarida ko'rinadi.
    SettingsPage(),
  ];

  static const _baseDestinations = [
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
    NavigationRailDestination(
        icon: Icon(Icons.menu_book_outlined),
        selectedIcon: Icon(Icons.menu_book),
        label: Text('Kutubxona')),
    NavigationRailDestination(
        icon: Icon(Icons.map_outlined),
        selectedIcon: Icon(Icons.map),
        label: Text('OnDexMap')),
  ];

  Future<void> _logout() async {
    // Soket tokendan OLDIN yopiladi: aks holda u chiqib ketgan
    // sessiya uchun qayta ulanishga urinib, har safar 401 olardi.
    await adminLive.stop();
    await supportInbox.stop();
    // ┌─ TUZATILGAN NOSOZLIK (bug.md 93-band) ────────────────────────┐
    // "Chiqish" AVVAL faqat mahalliy nusxani o'chirardi — token esa
    // serverda 30 KUN yaroqli qolaverardi. Uchala mobil ilova
    // (`customer`, `courier`, `waiter`) `api.logout()` ni chaqiradi,
    // panellar — yo'q edi.
    //
    // Aynan panellarda bu eng yomon: admin tokeni butun platformaga
    // kirish beradi, va agar u bir marta olingan bo'lsa (fayldan yoki
    // WebView'dan) uni to'xtatishning BOSHQA yo'li yo'q — bekor
    // qilish faqat `/auth/logout` orqali bo'ladi.
    //
    // `try/catch` SHART: internet yo'q bo'lsa ham foydalanuvchi
    // mahalliy sessiyadan chiqishi kerak (bug.md 87-band).
    // └───────────────────────────────────────────────────────────────┘
    try {
      await api.logout();
    } catch (_) {
      // Server bilan bog'lanib bo'lmadi — mahalliy chiqish baribir
      // bajariladi (pastda).
    }
    await adminTokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const AdminLoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final unread = supportInbox.unread;
    return Scaffold(
      body: Row(
        children: [
          // 10 ta bo'lim past ekranda (1366x768 noutbuk) sig'masligi mumkin —
          // navigatsiya o'zi aylantiriladi, sahifa esa kesilmaydi.
          LayoutBuilder(
            builder: (context, box) => SingleChildScrollView(
              child: ConstrainedBox(
                constraints: BoxConstraints(minHeight: box.maxHeight),
                child: IntrinsicHeight(
                  child: NavigationRail(
                    selectedIndex: _index,
                    onDestinationSelected: (i) {
                      setState(() => _index = i);
                      Analytics.instance.screen(_pageNames[i]);
                    },
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
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              // ┌─ HUQUQIY HAVOLALAR OLIB TASHLANDI ──────────┐
                              // Oferta va maxfiylik siyosati MIJOZGA kerak —
                              // u xizmatdan foydalanish shartlarini qabul
                              // qiladi. Superadmin esa platformaning egasi:
                              // u hujjatlarni o'zi yozadi va panelda ularga
                              // havola bosishning ma'nosi yo'q edi.
                              //
                              // Mijoz ilovasidagi havolalar TEGILMADI.
                              // └──────────────────────────────────────────────┘
                              IconButton(
                                tooltip: 'Chiqish',
                                icon: const Icon(Icons.logout),
                                onPressed: _logout,
                              ),
                              // Qaysi build ishlayotgani — har reliz +1
                              // (`scripts/version.ps1`).
                              const SizedBox(height: 6),
                              Text(
                                appVersionLabel,
                                key: const ValueKey('admin-app-version'),
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                    // Tartib `_pages` bilan AYNAN bir xil bo'lishi shart —
                    // ikkalasi uchun bitta `_index` ishlatiladi.
                    destinations: [
                      ..._baseDestinations,
                      NavigationRailDestination(
                          icon: Badge(
                            isLabelVisible: unread > 0,
                            label: Text('$unread'),
                            child: const Icon(Icons.forum_outlined),
                          ),
                          selectedIcon: Badge(
                            isLabelVisible: unread > 0,
                            label: Text('$unread'),
                            child: const Icon(Icons.forum),
                          ),
                          label: const Text('Chat')),
                      const NavigationRailDestination(
                          icon: Icon(Icons.settings_outlined),
                          selectedIcon: Icon(Icons.settings),
                          label: Text('Sozlamalar')),
                    ],
                  ),
                ),
              ),
            ),
          ),
          const VerticalDivider(width: 1),
          Expanded(child: _pages[_index]),
        ],
      ),
    );
  }
}
