import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../pages/couriers_page.dart';
import '../pages/ondexmap_page.dart';
import '../pages/people_page.dart';
import '../pages/restaurant_module.dart';
import '../pages/settings_page.dart';
import '../pages/stats_page.dart';
import '../support_inbox.dart';
import 'login_screen.dart';

/// Asosiy tuzilma: chapda navigatsiya (Sidebar), o'ngda sahifa.
///
/// Sidebar (yuqoridan pastga):
///   * **Restoran** — restoranlar ro'yxati; restoran tanlansa uning ALOHIDA
///     moduli ochiladi (buyurtmalar, kuryerlar, affitsiantlar, kutubxona, chat);
///   * **Statistika** — barcha restoranlar bo'yicha umumiy ko'rsatkichlar;
///   * **Mijozlar**;
///   * **OnDex kuryerlari** — restoranga bog'lanmagan platforma kuryerlari;
///   * **Sozlamalar** — qo'llab-quvvatlash aloqa ma'lumotlari;
///   * (eng pastda) **OnDexMap** — alohida loyihaning muharriri va takliflari.
class AdminShell extends StatefulWidget {
  const AdminShell({super.key});

  @override
  State<AdminShell> createState() => _AdminShellState();
}

/// Sidebar bandi. Tartib `_AdminShellState._page` bilan AYNAN bir xil
/// (ikkalasi uchun bitta indeks).
typedef _NavSpec = ({
  String key,
  String label,
  String screen,
  IconData icon,
  IconData selectedIcon,
});

const _navTop = <_NavSpec>[
  (
    key: 'nav-restaurants',
    label: 'Restoran',
    screen: 'Restaurants',
    icon: Icons.storefront_outlined,
    selectedIcon: Icons.storefront,
  ),
  (
    key: 'nav-stats',
    label: 'Statistika',
    screen: 'Statistics',
    icon: Icons.insights_outlined,
    selectedIcon: Icons.insights,
  ),
  (
    key: 'nav-customers',
    label: 'Mijozlar',
    screen: 'Customers',
    icon: Icons.people_outline,
    selectedIcon: Icons.people,
  ),
  (
    key: 'nav-couriers',
    label: 'OnDex kuryerlari',
    screen: 'Couriers',
    icon: Icons.delivery_dining_outlined,
    selectedIcon: Icons.delivery_dining,
  ),
  (
    key: 'nav-settings',
    label: 'Sozlamalar',
    screen: 'Settings',
    icon: Icons.settings_outlined,
    selectedIcon: Icons.settings,
  ),
];

const _navBottom = <_NavSpec>[
  (
    key: 'nav-ondexmap',
    label: 'OnDexMap',
    screen: 'OnDexMap',
    icon: Icons.map_outlined,
    selectedIcon: Icons.map,
  ),
];

const _navAll = [..._navTop, ..._navBottom];

class _AdminShellState extends State<AdminShell> {
  int _index = 0;

  /// Bo'limlar birinchi ochilgunicha qurilmaydi (OnDexMap xaritasi og'ir,
  /// qolganlari ortiqcha so'rov yubormasin), keyin esa saqlanib qoladi:
  /// boshqa bo'limga o'tib qaytganda holat (tanlangan restoran, davr,
  /// muharrir) yo'qolmaydi.
  final Set<int> _visited = {0};
  @override
  void initState() {
    super.initState();
    // Jonli kanal BUTUN panel uchun bitta marta ochiladi (sahifalar
    // unga obuna bo'ladi — `lib/live.dart`). Ekran darajasida ochilsa
    // har bo'limga o'tganda yangi soket va yangi bilet kerak bo'lardi.
    adminLive.start();
    // Restoranlardan kelgan chat xabarlari — "Restoran" bo'limidagi rozetka.
    supportInbox.addListener(_onInbox);
    supportInbox.start();
    // Ilova ochilganda birinchi sahifani PostHog'ga yozamiz.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Analytics.instance.screen(_navAll[0].screen);
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

  void _select(int i) {
    if (i == _index) return;
    setState(() {
      _index = i;
      _visited.add(i);
    });
    Analytics.instance.screen(_navAll[i].screen);
  }

  Widget _page(int i) => switch (i) {
        0 => const RestaurantsSection(),
        1 => const StatsPage(),
        2 => const CustomersPage(),
        // Restoranga bog'lanmagan (platformaning o'z) kuryerlari.
        3 => const CouriersPage(onlyPlatform: true),
        4 => const SettingsPage(),
        // OnDexMap — ALOHIDA loyihaning muharriri (oyna sifatida).
        // ChustApp bazasiga ham, API'siga ham tegmaydi.
        _ => const OndexMapPage(),
      };
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
          _SideBar(
            selected: _index,
            unread: unread,
            onSelect: _select,
            onLogout: _logout,
          ),
          const VerticalDivider(width: 1),
          Expanded(
            // IndexedStack: barcha ochilgan bo'limlarning holati saqlanadi;
            // ochilmaganlari qurilmaydi (`_visited`).
            child: IndexedStack(
              index: _index,
              children: [
                for (var i = 0; i < _navAll.length; i++)
                  _visited.contains(i) ? _page(i) : const SizedBox.shrink(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Chap navigatsiya: yuqorida asosiy bo'limlar, pastda "OnDexMap", eng
/// pastda chiqish tugmasi va build versiyasi.
class _SideBar extends StatelessWidget {
  const _SideBar({
    required this.selected,
    required this.unread,
    required this.onSelect,
    required this.onLogout,
  });

  final int selected;
  final int unread;
  final ValueChanged<int> onSelect;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SizedBox(
      width: 88,
      child: SafeArea(
        child: Column(
          children: [
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Icon(Icons.admin_panel_settings, size: 32),
            ),
            // Asosiy bo'limlar. Past ekranda (1366x768 noutbuk) sig'masa
            // shu qism aylantiriladi — chiqish tugmasi va OnDexMap doim ko'rinadi.
            Expanded(
              child: SingleChildScrollView(
                child: Column(
                  children: [
                    for (var i = 0; i < _navTop.length; i++)
                      _NavItem(
                        key: ValueKey(_navTop[i].key),
                        spec: _navTop[i],
                        selected: selected == i,
                        // Restoranlardan kelgan o'qilmagan chat xabarlari.
                        badge: i == 0 ? unread : 0,
                        onTap: () => onSelect(i),
                      ),
                  ],
                ),
              ),
            ),
            for (var j = 0; j < _navBottom.length; j++)
              _NavItem(
                key: ValueKey(_navBottom[j].key),
                spec: _navBottom[j],
                selected: selected == _navTop.length + j,
                onTap: () => onSelect(_navTop.length + j),
              ),            const SizedBox(height: 12),
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
              onPressed: onLogout,
            ),
            // Qaysi build ishlayotgani — har reliz +1
            // (`scripts/version.ps1`).
            const SizedBox(height: 6),
            Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: Text(
                appVersionLabel,
                key: const ValueKey('admin-app-version'),
                style: theme.textTheme.labelSmall,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _NavItem extends StatelessWidget {
  const _NavItem({
    super.key,
    required this.spec,
    required this.selected,
    required this.onTap,
    this.badge = 0,
  });

  final _NavSpec spec;
  final bool selected;
  final VoidCallback onTap;
  final int badge;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final iconWidget = Icon(selected ? spec.selectedIcon : spec.icon,
        color: selected ? scheme.onSecondaryContainer : scheme.onSurfaceVariant);
    return Semantics(
      button: true,
      selected: selected,
      label: spec.label,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(16),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 6),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 56,
                height: 32,
                decoration: BoxDecoration(
                  color: selected ? scheme.secondaryContainer : null,
                  borderRadius: BorderRadius.circular(16),
                ),
                alignment: Alignment.center,
                child: Badge(
                  isLabelVisible: badge > 0,
                  label: Text('$badge'),
                  child: iconWidget,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                spec.label,
                maxLines: 2,
                textAlign: TextAlign.center,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.labelMedium?.copyWith(
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                      color: selected ? scheme.onSurface : scheme.onSurfaceVariant,
                    ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
