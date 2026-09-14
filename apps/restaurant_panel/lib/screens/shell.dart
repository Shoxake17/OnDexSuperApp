import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import '../live.dart';
import '../notification_center.dart';
import '../pages/coming_soon_page.dart';
import '../pages/dashboard_page.dart';
import '../pages/help_page.dart';
import '../pages/menu_page.dart';
import '../pages/notifications_page.dart';
import '../pages/orders_page.dart';
import '../pages/promotions_page.dart';
import '../pages/restaurant_settings_page.dart';
import '../pages/staff_page.dart';
import '../pages/statistics_page.dart';
import '../pages/support_chat_page.dart';
import '../pages/tables_page.dart';
import '../sound.dart';
import '../support_center.dart';
import '../widgets/main_layout.dart';
import 'login_screen.dart';

const _iDashboard = 0;
const _iOrders = 1;
const _iMenu = 2;
const _iStatistics = 5;
const _iSettings = 7;
const _iStaff = 8;
const _iNotifications = 9;
const _iTables = 4;
const _iHelp = 10;
const _iChat = 11;

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
  late final LiveRefresher _live;

  /// Statistika sahifasi va uning yuqori paneldagi davr/Eksport
  /// tugmalarining umumiy holati — shu yerda, ya'ni boshqa sahifaga o'tib
  /// qaytganda tanlangan davr saqlanadi.
  final _stats = StatisticsController();

  /// "Restoran sozlamalari" da saqlanmagan o'zgarish bo'lsa, boshqa
  /// sahifaga o'tishdan oldin so'raladi.
  final _settingsGuard = SettingsLeaveGuard();

  @override
  void initState() {
    super.initState();
    _loadInfo();
    _pollOrders();
    // Jonli kanal BUTUN panel uchun shu yerda ochiladi — buyurtmalar
    // sahifasi ham xuddi shunga obuna bo'ladi (`lib/live.dart`).
    //
    // Avval yon paneldagi hisoblagich 20 soniyalik so'rov sikliga
    // tayanardi: oshxona buyurtmani ko'rgan bo'lsa ham, rozetkadagi
    // raqam eski qiymatda turardi. Endi ikkalasi bir manbadan.
    restaurantLive.start();
    _live = LiveRefresher(
      bus: restaurantLive,
      onRefresh: _pollOrders,
      types: const {'new_order', 'order_status'},
    )..start();
    // Bildirishnomalar markazi — o'sha jonli kanal ustida (qo'ng'iroq,
    // yon menyu rozetkasi va "Bildirishnomalar" sahifasi).
    notificationCenter.addListener(_onAlerts);
    notificationCenter.start();
    // OnDex qo'llab-quvvatlash chati: yon menyu rozetkasi va Yordam
    // markazidagi "yangi javob" belgisi.
    supportCenter.addListener(_onAlerts);
    supportCenter.start();
  }

  void _onAlerts() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    notificationCenter.removeListener(_onAlerts);
    supportCenter.removeListener(_onAlerts);
    _live.dispose();
    restaurantLive.stop();
    _stats.dispose();
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
      final orders = list.cast<Map<String, dynamic>>().toList();
      final newCount = orders.where((o) => o['status'] == 'created').length;
      setState(() => _newOrdersCount = newCount);
      // Statistika sahifasidagi "So'nggi buyurtmalar" jadvali shu
      // ro'yxatdan oziqlanadi — alohida so'rov yubormaydi.
      _stats.setRecentOrders(orders);
      // Qo'ng'iroq AYNAN shu yerdan boshqariladi — qobiq panel ochiq
      // turgan BUTUN vaqt davomida tirik, ya'ni foydalanuvchi qaysi
      // sahifada bo'lishidan qat'i nazar ovoz eshitiladi
      // (`sound.dart` dagi `setPending` izohiga qarang).
      await RingSound.setPending(newCount > 0);
    } catch (_) {
      // Ko'makchi hisoblagich — tarmoq xatosi butun panelni to'xtatmasin,
      // keyingi davriy urinishda o'zi tuzaladi.
    }
  }

  Future<void> _toggleOpen(bool v) async {
    try {
      await api.setOpen(v);
      setState(() => _open = v);
      // Ochish/yopish — restoranning eng muhim amali: mijoz buyurtma
      // bera oladimi yo'qmi shunga bog'liq. "Nega buyurtma kelmadi"
      // degan savolga javob ko'pincha shu yerda bo'ladi.
      Analytics.instance.capture('restoran_holati', {'ochiq': v});
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
    // Soket tokendan OLDIN yopiladi: aks holda u chiqib ketgan
    // sessiya uchun qayta ulanishga urinardi (har safar 401).
    await restaurantLive.stop();
    await notificationCenter.stop();
    await supportCenter.stop();
    // Serverdagi sessiyani ham bekor qilamiz (bug.md 93-band) —
    // avval token 30 kun yaroqli qolib ketardi. `try/catch` SHART:
    // internet yo'q bo'lsa ham mahalliy chiqish bajarilishi kerak
    // (bug.md 87-band).
    try {
      await api.logout();
    } catch (_) {}
    await restTokenStore.clear();
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('rest_rid');
    api.token = null;
    api.rid = '';
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const RestaurantLoginScreen()),
    );
  }

  /// ┌─ PANELDA NIMA QILINGANI YOZIB BORILADI ────────────────────────┐
  /// Superadmin "bu restoran panelda nima qildi" degan savolga javob
  /// olishi kerak. Panel Windows ilovasi bo'lgani uchun SEANS YOZUVI
  /// mumkin emas (`posthog_flutter` Windows'ni qo'llamaydi) — shuning
  /// uchun HODISALAR yoziladi.
  ///
  /// Bo'sh nom yuborilmaydi: PostHog'da nomsiz ekran "(unknown)"
  /// bo'lib chiqib, ro'yxatni o'qib bo'lmas holga keltirardi.
  /// └────────────────────────────────────────────────────────────────┘
  static const _pageNames = <int, String>{
    _iDashboard: 'Boshqaruv',
    _iOrders: 'Buyurtmalar',
    _iMenu: 'Menyu',
    3: 'Aksiyalar',
    4: 'Stollar',
    _iStatistics: 'Statistika',
    _iSettings: 'Sozlamalar',
    8: 'Xodimlar',
    9: 'Bildirishnomalar',
    _iHelp: 'Yordam',
    _iChat: 'Chat',
  };

  /// Buyurtmalar → "Tarix": to'liq tarix Statistika ichidagi "Barcha
  /// buyurtmalar" sahifasida (davr filtri bilan, sahifalab). Qaytish
  /// tugmasi yana Buyurtmalar sahifasiga olib boradi.
  void _openOrderHistory() {
    _goTo(_iStatistics);
    _stats.openHistory(backLabel: 'Buyurtmalar', onBack: () => _goTo(_iOrders));
  }

  Future<void> _goTo(int i) async {
    if (_index == _iSettings && i != _iSettings && _settingsGuard.hasUnsavedChanges) {
      if (!await confirmDiscardSettings(context) || !mounted) return;
    }
    // Yon menyudan "Statistika" bosilsa — "Barcha buyurtmalar" ichida
    // bo'lsa ham asosiy statistikaga qaytadi.
    if (i == _iStatistics) _stats.closeHistory();
    setState(() => _index = i);
    final name = _pageNames[i];
    if (name != null) Analytics.instance.screen('restoran/$name');
  }

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
        notificationsCount: notificationCenter.unread,
        supportCount: supportCenter.unread,
        onBellTap: () => _goTo(_iNotifications),
        onLogout: _logout,
        // Faqat Statistika sahifasida: bir kunlik sana o'rniga davr
        // tanlagich + Eksport. Qolgan sahifalar odatiy holida.
        topBarActions: _index == _iStatistics
            ? StatisticsToolbar(controller: _stats, restaurantName: _name)
            : null,
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
        return OrdersPage(onOpenHistory: _openOrderHistory);
      case _iMenu:
        return const MenuPage();
      case 3:
        return const PromotionsPage();
      // DIQQAT: "Stollar (QR)" sidebar'da 4-o'ringa qo'shildi, shuning
      // uchun undan keyingi HAMMA indeks bittaga surildi. Sidebar
      // ro'yxati (`widgets/sidebar.dart`) va bu switch BIR XIL
      // tartibda bo'lishi SHART — aks holda foydalanuvchi "Moliya"
      // bosib "Statistika" ni ochib qo'yardi.
      case _iTables:
        return const TablesPage();
      case _iStatistics:
        return StatisticsPage(controller: _stats);
      case 6:
        return const ComingSoonPage(
          title: 'Moliya',
          icon: Icons.account_balance_wallet_rounded,
          description:
              'To\'lov tarixi, komissiya tafsiloti, bank rekvizitlari — to\'lov tizimi (Payme/Click) ulanganidan keyin qo\'shiladi.',
        );
      case _iSettings:
        return RestaurantSettingsPage(
          guard: _settingsGuard,
          open: _open,
          onOpenChanged: _toggleOpen,
          // Logo o'zgarsa yuqori panel va yon menyuda ham darhol yangilansin.
          onSaved: _loadInfo,
        );
      case _iStaff:
        return const StaffPage();
      case _iNotifications:
        return NotificationsPage(
          onOpenOrders: () => _goTo(_iOrders),
          onOpenStaff: () => _goTo(_iStaff),
          onOpenStatistics: () => _goTo(_iStatistics),
          onOpenSettings: () => _goTo(_iSettings),
        );
      case _iHelp:
        // Savol-javob, admin kiritgan aloqa ma'lumotlari va "Onlayn
        // yordam" (Chat markazi) — `pages/help_page.dart`.
        return HelpPage(
          supportUnread: supportCenter.unread,
          onOpenChat: () => _goTo(_iChat),
          onOpenOrders: () => _goTo(_iOrders),
          onOpenMenu: () => _goTo(_iMenu),
          onOpenTables: () => _goTo(_iTables),
          onOpenStatistics: () => _goTo(_iStatistics),
          onOpenSettings: () => _goTo(_iSettings),
          onOpenStaff: () => _goTo(_iStaff),
          onOpenUpdates: () => _goTo(_iNotifications),
        );
      case _iChat:
        return const SupportChatPage();
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
