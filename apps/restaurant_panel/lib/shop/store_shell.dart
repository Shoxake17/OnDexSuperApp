import 'package:flutter/material.dart';

import '../api.dart';
import '../screens/login_screen.dart';
import '../theme.dart';
import '../widgets/service_switcher.dart';
import 'shop_api.dart';
import 'store_pages.dart';

/// Do'kon / Qurt / Dorixona paneli — ShoxPos (OnDex POS) menyusi bilan 1:1,
/// lekin OnDex Merchant ko'rinishida (to'q sariq, binafsha emas).
///
/// Bo'limlar ro'yxati kategoriya bo'yicha ShoxPos `Sidebar.tsx` +
/// `storeAccess.ts` dan olingan; ruxsatni baribir SERVER tekshiradi
/// (`/merchant/shop/*` 403).
class StoreShell extends StatefulWidget {
  const StoreShell({super.key, required this.name, required this.phone});

  final String name;
  final String phone;

  @override
  State<StoreShell> createState() => _StoreShellState();
}

typedef _Entry = ({StorePage page, IconData icon, String label});

/// ShoxPos `regularMenuItems` (oddiy do'kon). Buyurtmalar/Logistika faqat
/// onlayn buyurtma moduli bor kategoriyada (hozir qurt).
const List<_Entry> _storeMenu = [
  (page: StorePage.dashboard, icon: Icons.home_rounded, label: 'Boshqaruv paneli'),
  (page: StorePage.products, icon: Icons.inventory_2_rounded, label: 'Mahsulotlar'),
  (page: StorePage.scanner, icon: Icons.point_of_sale_rounded, label: 'Savdo terminali'),
  (page: StorePage.sales, icon: Icons.payments_rounded, label: 'Sotuvlar'),
  (page: StorePage.incoming, icon: Icons.archive_rounded, label: 'Kirim bazasi'),
  (page: StorePage.reports, icon: Icons.bar_chart_rounded, label: 'Savdo statistikasi'),
  (page: StorePage.debts, icon: Icons.credit_card_rounded, label: 'Qarzlar'),
  (page: StorePage.settings, icon: Icons.settings_rounded, label: 'Sozlamalar'),
];

/// ShoxPos QURT ro'yxati: kassa va boshqaruv paneli YO'Q.
const List<_Entry> _qurtMenu = [
  (page: StorePage.products, icon: Icons.inventory_2_rounded, label: 'Mahsulotlar'),
  (page: StorePage.orders, icon: Icons.assignment_rounded, label: 'Buyurtmalar'),
  (page: StorePage.logistics, icon: Icons.local_shipping_rounded, label: 'Logistika'),
  (page: StorePage.sales, icon: Icons.payments_rounded, label: 'Sotuvlar'),
  (page: StorePage.incoming, icon: Icons.archive_rounded, label: 'Kirim bazasi'),
  (page: StorePage.reports, icon: Icons.bar_chart_rounded, label: 'Savdo statistikasi'),
  (page: StorePage.debts, icon: Icons.credit_card_rounded, label: 'Qarzlar'),
  (page: StorePage.settings, icon: Icons.settings_rounded, label: 'Sozlamalar'),
];

/// ShoxPos `pharmacyMenuItems`. (ShoxPos'da ikki band "Mahsulotlar" va ikki
/// band "Savdo statistikasi" deb nomlangan edi — bu yerda farqlandi.)
const List<_Entry> _pharmMenu = [
  (page: StorePage.pharmDashboard, icon: Icons.home_rounded, label: 'Boshqaruv paneli'),
  (page: StorePage.pharmAdd, icon: Icons.add_circle_rounded, label: 'Dori qo\'shish'),
  (page: StorePage.pharmBase, icon: Icons.medication_rounded, label: 'Dorilar bazasi'),
  (page: StorePage.pharmScanner, icon: Icons.point_of_sale_rounded, label: 'Savdo terminali'),
  (page: StorePage.pharmSales, icon: Icons.history_rounded, label: 'Sotuvlar'),
  (page: StorePage.pharmReports, icon: Icons.bar_chart_rounded, label: 'Savdo statistikasi'),
  (page: StorePage.pharmIncoming, icon: Icons.trending_up_rounded, label: 'Kirim bazasi'),
  (page: StorePage.pharmInfo, icon: Icons.search_rounded, label: 'Dori ma\'lumoti'),
  (page: StorePage.pharmDebts, icon: Icons.credit_card_rounded, label: 'Qarzlar'),
  (page: StorePage.settings, icon: Icons.settings_rounded, label: 'Sozlamalar'),
];

class _StoreShellState extends State<StoreShell> {
  String _category = '';
  Object? _error;
  StorePage? _page;
  String? _arg;

  List<_Entry> get _menu => switch (_category) {
        'qurt' => _qurtMenu,
        'pharmacy' => _pharmMenu,
        _ => _storeMenu,
      };

  String get _storeLabel => switch (_category) {
        'qurt' => 'Qurt',
        'pharmacy' => 'Dorixona',
        _ => 'Do\'kon',
      };

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final me = await api.shopMe();
      if (!mounted) return;
      setState(() {
        _category = me['category'] as String? ?? 'store';
        _page = _menu.first.page;
      });
    } catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  Future<void> _logout() async {
    try {
      await api.logout();
    } catch (_) {}
    await api.clearSession();
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const RestaurantLoginScreen()),
    );
  }

  void _go(StorePage p, {String? arg}) => setState(() {
        _page = p;
        _arg = arg;
      });

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Scaffold(
        body: Center(
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            const Icon(Icons.cloud_off_rounded, size: 48, color: OnDexColors.inkDim),
            const SizedBox(height: 12),
            Text(_error is ApiException ? (_error as ApiException).message : 'Server bilan aloqa yo\'q',
                textAlign: TextAlign.center),
            const SizedBox(height: 12),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
            TextButton(onPressed: _logout, child: const Text('Chiqish')),
          ]),
        ),
      );
    }
    final page = _page;
    if (page == null) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    // Menyuda yo'q sahifa (masalan Mahsulot qo'shish) — sarlavha o'zidan.
    final entry = page == StorePage.addProduct
        ? (page: page, icon: Icons.add, label: _arg == null ? 'Mahsulot qo\'shish' : 'Mahsulotni tahrirlash')
        : _menu.firstWhere((e) => e.page == page, orElse: () => _menu.first);
    final menuPage = page == StorePage.addProduct ? StorePage.products : page;
    final ctx = StoreContext(category: _category, name: widget.name, phone: widget.phone, go: _go, arg: _arg);
    return LayoutBuilder(builder: (context, c) {
      final narrow = c.maxWidth < 900;
      final sidebar = _StoreSidebar(
        menu: _menu,
        selected: menuPage,
        storeLabel: _storeLabel,
        name: widget.name,
        phone: widget.phone,
        onSelect: (p) {
          _go(p);
          if (narrow) Navigator.of(context).maybePop();
        },
        onLogout: _logout,
      );
      return Scaffold(
        drawer: narrow ? Drawer(width: 260, child: sidebar) : null,
        appBar: narrow ? AppBar(title: Text(entry.label)) : null,
        body: Row(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (!narrow) sidebar,
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (!narrow) _TopBar(title: entry.label, name: widget.name),
                  Expanded(
                    child: KeyedSubtree(key: ValueKey('$page/$_arg'), child: buildStorePage(page, ctx)),
                  ),
                ],
              ),
            ),
          ],
        ),
      );
    });
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.title, required this.name});
  final String title;
  final String name;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 64,
      padding: const EdgeInsets.symmetric(horizontal: 24),
      decoration: const BoxDecoration(
        color: OnDexColors.cardBg,
        border: Border(bottom: BorderSide(color: OnDexColors.cardBorder)),
      ),
      child: Row(children: [
        Text(title, style: const TextStyle(fontSize: 19, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        const Spacer(),
        const Icon(Icons.storefront_rounded, size: 18, color: OnDexColors.inkDim),
        const SizedBox(width: 6),
        Text(name, style: const TextStyle(color: OnDexColors.inkDim, fontWeight: FontWeight.w600)),
      ]),
    );
  }
}

class _StoreSidebar extends StatelessWidget {
  const _StoreSidebar({
    required this.menu,
    required this.selected,
    required this.storeLabel,
    required this.name,
    required this.phone,
    required this.onSelect,
    required this.onLogout,
  });

  final List<_Entry> menu;
  final StorePage selected;
  final String storeLabel;
  final String name;
  final String phone;
  final ValueChanged<StorePage> onSelect;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 248,
      color: OnDexColors.sidebarBg,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 22, 20, 20),
            child: Row(children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(10),
                child: Image.asset('assets/ondex.png',
                    width: 38,
                    height: 38,
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) => Container(
                        width: 38,
                        height: 38,
                        color: OnDexColors.primary,
                        child: const Icon(Icons.shopping_bag_rounded, color: Colors.white, size: 20))),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
                  const Text('Ondex',
                      style: TextStyle(
                          color: OnDexColors.sidebarText, fontWeight: FontWeight.w800, fontSize: 17, height: 1.1)),
                  Text('$storeLabel paneli',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(color: OnDexColors.sidebarTextDim, fontSize: 11.5, height: 1.3)),
                ]),
              ),
            ]),
          ),
          const Divider(height: 1, color: Color(0x22FFFFFF)),
          const SizedBox(height: 12),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              children: [
                for (final e in menu)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Material(
                      color: e.page == selected ? OnDexColors.primary : Colors.transparent,
                      borderRadius: BorderRadius.circular(10),
                      child: InkWell(
                        key: ValueKey('store-nav-${e.page.name}'),
                        borderRadius: BorderRadius.circular(10),
                        onTap: () => onSelect(e.page),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                          child: Row(children: [
                            Icon(e.icon,
                                size: 20, color: e.page == selected ? Colors.white : OnDexColors.sidebarTextDim),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Text(e.label,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    color: e.page == selected ? Colors.white : OnDexColors.sidebarTextDim,
                                    fontWeight: e.page == selected ? FontWeight.w700 : FontWeight.w500,
                                    fontSize: 13.5,
                                  )),
                            ),
                          ]),
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
            child: PopupMenuButton<String>(
              key: const ValueKey('account-menu'),
              tooltip: 'Xizmat va chiqish',
              offset: const Offset(0, -8),
              // Admin paneldagi kabi: xizmatni almashtirish (Restoran | Do'kon) va Chiqish.
              onSelected: (v) => handleMerchantMenu(context, v, active: MerchantService.store, onLogout: onLogout),
              itemBuilder: (_) => merchantMenuItems(active: MerchantService.store, storeLabel: storeLabel),
              child: Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                    color: OnDexColors.sidebarBgActive, borderRadius: BorderRadius.circular(12)),
                child: Row(children: [
                  Container(
                    width: 36,
                    height: 36,
                    decoration:
                        BoxDecoration(color: OnDexColors.primary, borderRadius: BorderRadius.circular(9)),
                    child: const Icon(Icons.storefront, color: Colors.white, size: 18),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
                      Text(name.isEmpty ? storeLabel : name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              color: OnDexColors.sidebarText, fontWeight: FontWeight.w700, fontSize: 13)),
                      if (phone.isNotEmpty)
                        Text(phone,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(color: OnDexColors.sidebarTextDim, fontSize: 11)),
                    ]),
                  ),
                  const Icon(Icons.unfold_more_rounded, size: 16, color: OnDexColors.sidebarTextDim),
                ]),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
