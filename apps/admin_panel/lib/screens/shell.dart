import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../pages/banners_page.dart';
import '../pages/couriers_page.dart';
import '../pages/flow_page.dart';
import '../pages/kyc_page.dart';
import '../pages/live_ops_page.dart';
import '../pages/ondexmap_page.dart';
import '../pages/people_page.dart';
import '../pages/qurt_section.dart';
import '../pages/restaurant_module.dart';
import '../pages/settings_page.dart';
import '../pages/stats_page.dart';
import '../pages/stores_page.dart';
import '../support_inbox.dart';
import 'login_screen.dart';

// ┌─ XIZMATLAR BO'YICHA IZOLYATSIYA (Plane 479, egasi 2026-11-19) ─────────────┐
// Avval 12 ta bo'lim bitta ro'yxatda aralash turardi (restoran, do'kon,
// qurt, platforma, xarita). Endi sidebar XIZMATGA bog'liq: pastdagi
// almashtirgichda xizmat tanlanadi va yuqorida FAQAT o'sha xizmatning
// bo'limlari ko'rinadi. Do'konga kirilsa — faqat do'kon, OnDexMap'ga —
// faqat xarita.
//
// Bo'lim (`_Section`) global identifikatorga ega: IndexedStack holatni
// saqlaydi (xizmat almashganda ochiq restoran, davr, xarita yo'qolmaydi) va
// bo'lim faqat birinchi ochilganda quriladi (keraksiz so'rov yo'q).
// └────────────────────────────────────────────────────────────────────────────┘

enum _Section {
  restaurants,
  live,
  stats,
  couriers,
  banners,
  storesMarket,
  storesQurt,
  storesPharmacy,
  ondexmap,
  customers,
  kyc,
  flow,
  settings,
}

typedef _SectionSpec = ({
  _Section id,
  // group — sidebar'da ochiladigan guruh nomi (bo'sh = guruhsiz band).
  String group,
  String key,
  String label,
  String screen,
  IconData icon,
  IconData selectedIcon,
});

typedef _Service = ({
  String id,
  String title,
  String subtitle,
  IconData icon,
  Color accent,
  List<_SectionSpec> sections,
});

const _services = <_Service>[
  (
    id: 'food',
    title: 'Restoran',
    subtitle: 'Restoranlar, buyurtmalar, kuryerlar',
    icon: Icons.restaurant_rounded,
    accent: Color(0xFFF4511E),
    sections: [
      (id: _Section.restaurants, group: '', key: 'nav-restaurants', label: 'Restoranlar', screen: 'Restaurants',
          icon: Icons.storefront_outlined, selectedIcon: Icons.storefront),
      (id: _Section.live, group: '', key: 'nav-live', label: 'Jonli holat', screen: 'LiveOps',
          icon: Icons.monitor_heart_outlined, selectedIcon: Icons.monitor_heart),
      (id: _Section.stats, group: '', key: 'nav-stats', label: 'Statistika', screen: 'Statistics',
          icon: Icons.insights_outlined, selectedIcon: Icons.insights),
      (id: _Section.couriers, group: '', key: 'nav-couriers', label: 'OnDex kuryerlari', screen: 'Couriers',
          icon: Icons.delivery_dining_outlined, selectedIcon: Icons.delivery_dining),
      (id: _Section.banners, group: '', key: 'nav-banners', label: 'Bannerlar', screen: 'Banners',
          icon: Icons.campaign_outlined, selectedIcon: Icons.campaign),
    ],
  ),
  (
    id: 'store',
    title: 'Do\'kon',
    subtitle: 'Market, Qurt, Apteka',
    icon: Icons.shopping_bag_rounded,
    accent: Color(0xFF2563EB),
    sections: [
      // Egasi 2026-11-19: Qurt alohida xizmat EMAS — do'kon turi. "Do'konlar"
      // guruhi ochilganda Market / Qurt / Apteka bandlari chiqadi.
      (id: _Section.storesMarket, group: 'Do\'konlar', key: 'nav-stores-market', label: 'Market', screen: 'StoresMarket',
          icon: Icons.local_grocery_store_outlined, selectedIcon: Icons.local_grocery_store),
      (id: _Section.storesQurt, group: 'Do\'konlar', key: 'nav-stores-qurt', label: 'Qurt', screen: 'StoresQurt',
          icon: Icons.local_shipping_outlined, selectedIcon: Icons.local_shipping),
      (id: _Section.storesPharmacy, group: 'Do\'konlar', key: 'nav-stores-pharmacy', label: 'Apteka', screen: 'StoresPharmacy',
          icon: Icons.local_pharmacy_outlined, selectedIcon: Icons.local_pharmacy),
    ],
  ),
  (
    id: 'map',
    title: 'OnDexMap',
    subtitle: 'Xarita, joylar va takliflar',
    icon: Icons.map_rounded,
    accent: Color(0xFF0891B2),
    sections: [
      (id: _Section.ondexmap, group: '', key: 'nav-ondexmap', label: 'Xarita muharriri', screen: 'OnDexMap',
          icon: Icons.map_outlined, selectedIcon: Icons.map),
    ],
  ),
  (
    id: 'platform',
    title: 'Platforma',
    subtitle: 'Mijozlar, tashkilotlar, yangilanish',
    icon: Icons.hub_rounded,
    accent: Color(0xFF7C3AED),
    sections: [
      (id: _Section.customers, group: '', key: 'nav-customers', label: 'Mijozlar', screen: 'Customers',
          icon: Icons.people_outline, selectedIcon: Icons.people),
      (id: _Section.kyc, group: '', key: 'nav-kyc', label: 'Tashkilotlar (KYC)', screen: 'Kyc',
          icon: Icons.verified_user_outlined, selectedIcon: Icons.verified_user),
      (id: _Section.flow, group: '', key: 'nav-flow', label: 'OnDexFlow', screen: 'Flow',
          icon: Icons.system_update_outlined, selectedIcon: Icons.system_update),
      (id: _Section.settings, group: '', key: 'nav-settings', label: 'Sozlamalar', screen: 'Settings',
          icon: Icons.settings_outlined, selectedIcon: Icons.settings),
    ],
  ),
];

// Sidebar ranglari (ikkala rejimda ham bir xil — panel ish quroli).
const _sideBg = Color(0xFF0F172A);
const _sideFg = Color(0xFFE2E8F0);
const _sideDim = Color(0xFF94A3B8);

class AdminShell extends StatefulWidget {
  const AdminShell({super.key});

  @override
  State<AdminShell> createState() => _AdminShellState();
}

class _AdminShellState extends State<AdminShell> {
  int _service = 0;
  _Section _section = _Section.restaurants;

  /// Har xizmatda oxirgi ochilgan bo'lim — xizmatga qaytganda shu ochiladi.
  final Map<int, _Section> _lastInService = {};

  /// Bo'limlar birinchi ochilgunicha qurilmaydi, keyin holati saqlanadi.
  final Set<_Section> _visited = {_Section.restaurants};

  @override
  void initState() {
    super.initState();
    // Jonli kanal BUTUN panel uchun bitta marta ochiladi (`lib/live.dart`).
    adminLive.start();
    // Restoranlardan kelgan chat xabarlari — "Restoranlar" bandidagi rozetka.
    supportInbox.addListener(_onInbox);
    supportInbox.start();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Analytics.instance.screen(_specOf(_section).screen);
    });
  }

  void _onInbox() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    supportInbox.removeListener(_onInbox);
    adminLive.stop();
    super.dispose();
  }

  static _SectionSpec _specOf(_Section s) =>
      _services.expand((x) => x.sections).firstWhere((x) => x.id == s);

  void _select(_Section s) {
    if (s == _section) return;
    setState(() {
      _section = s;
      _visited.add(s);
      _lastInService[_service] = s;
    });
    Analytics.instance.screen(_specOf(s).screen);
  }

  void _switchService(int i) {
    if (i == _service) return;
    final target = _lastInService[i] ?? _services[i].sections.first.id;
    setState(() => _service = i);
    _select(target);
    setState(() {}); // _select bir xil bo'lim bo'lsa ham xizmat almashsin
  }

  Widget _page(_Section s) => switch (s) {
        _Section.restaurants => const RestaurantsSection(),
        _Section.live => const LiveOpsPage(),
        _Section.stats => const StatsPage(),
        // Restoranga bog'lanmagan (platformaning o'z) kuryerlari.
        _Section.couriers => const CouriersPage(onlyPlatform: true),
        _Section.banners => const BannersPage(),
        _Section.storesMarket => const StoresPage(category: 'store'),
        _Section.storesQurt => const QurtSection(),
        _Section.storesPharmacy => const StoresPage(category: 'pharmacy'),
        // OnDexMap — ALOHIDA loyihaning muharriri. ChustApp bazasiga tegmaydi.
        _Section.ondexmap => const OndexMapPage(),
        _Section.customers => const CustomersPage(),
        // KYC (A6): faqat OnDex ID admin sessiyasida.
        _Section.kyc => const KycPage(),
        _Section.flow => const FlowPage(),
        _Section.settings => const SettingsPage(),
      };

  Future<void> _logout() async {
    // Soket tokendan OLDIN yopiladi: aks holda u chiqib ketgan
    // sessiya uchun qayta ulanishga urinib, har safar 401 olardi.
    await adminLive.stop();
    await supportInbox.stop();
    // Token serverda ham bekor qilinadi (bug.md 93-band); internet bo'lmasa
    // ham mahalliy chiqish bajariladi (87-band).
    try {
      await api.logout();
    } catch (_) {}
    await api.clearSession();
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const AdminLoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final compact = MediaQuery.sizeOf(context).width < 1180;
    return Scaffold(
      body: Row(
        children: [
          _SideBar(
            compact: compact,
            service: _services[_service],
            serviceIndex: _service,
            selected: _section,
            unread: supportInbox.unread,
            onSelect: _select,
            onSwitch: _switchService,
            onLogout: _logout,
          ),
          Expanded(
            child: IndexedStack(
              index: _Section.values.indexOf(_section),
              children: [
                for (final s in _Section.values)
                  _visited.contains(s) ? _page(s) : const SizedBox.shrink(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Chap navigatsiya: brend, tanlangan xizmat bo'limlari, pastda xizmat
/// almashtirgich, chiqish va versiya.
class _SideBar extends StatelessWidget {
  const _SideBar({
    required this.compact,
    required this.service,
    required this.serviceIndex,
    required this.selected,
    required this.unread,
    required this.onSelect,
    required this.onSwitch,
    required this.onLogout,
  });

  final bool compact;
  final _Service service;
  final int serviceIndex;
  final _Section selected;
  final int unread;
  final ValueChanged<_Section> onSelect;
  final ValueChanged<int> onSwitch;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 220),
      curve: Curves.easeOutCubic,
      width: compact ? 84 : 248,
      decoration: const BoxDecoration(
        color: _sideBg,
        border: Border(right: BorderSide(color: Color(0xFF1E293B))),
      ),
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _Brand(compact: compact),
            if (!compact)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 6, 16, 8),
                child: Text(
                  service.title.toUpperCase(),
                  key: const ValueKey('service-title'),
                  style: TextStyle(
                    color: service.accent,
                    fontSize: 11.5,
                    fontWeight: FontWeight.w800,
                    letterSpacing: 1.4,
                  ),
                ),
              ),
            // Bo'limlar. Past ekranda sig'masa shu qism aylantiriladi —
            // almashtirgich va chiqish doim ko'rinadi.
            Expanded(
              child: AnimatedSwitcher(
                duration: const Duration(milliseconds: 220),
                transitionBuilder: (child, a) => FadeTransition(
                  opacity: a,
                  child: SlideTransition(
                    position: Tween(begin: const Offset(0, 0.04), end: Offset.zero).animate(a),
                    child: child,
                  ),
                ),
                child: _SectionList(
                  key: ValueKey('sections-${service.id}'),
                  service: service,
                  compact: compact,
                  selected: selected,
                  unread: unread,
                  onSelect: onSelect,
                ),
              ),
            ),
            _ServiceSwitcher(
              compact: compact,
              current: serviceIndex,
              onSwitch: onSwitch,
            ),
            const SizedBox(height: 6),
            Row(
              mainAxisAlignment: compact ? MainAxisAlignment.center : MainAxisAlignment.spaceBetween,
              children: [
                if (!compact)
                  Padding(
                    padding: const EdgeInsets.only(left: 20),
                    child: Text(
                      appVersionLabel,
                      key: const ValueKey('admin-app-version'),
                      style: const TextStyle(color: _sideDim, fontSize: 11.5),
                    ),
                  ),
                IconButton(
                  tooltip: 'Chiqish',
                  icon: const Icon(Icons.logout_rounded, color: _sideDim),
                  onPressed: onLogout,
                ),
              ],
            ),
            if (compact)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Text(
                  appVersionLabel,
                  key: const ValueKey('admin-app-version'),
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: _sideDim, fontSize: 10),
                ),
              ),
            const SizedBox(height: 6),
          ],
        ),
      ),
    );
  }
}

class _Brand extends StatelessWidget {
  const _Brand({required this.compact});
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final mark = Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFFFF7A45), Color(0xFFF4511E)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(12),
        boxShadow: const [BoxShadow(color: Color(0x55F4511E), blurRadius: 14, offset: Offset(0, 4))],
      ),
      alignment: Alignment.center,
      child: const Text('O', style: TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.w900)),
    );
    return Padding(
      padding: EdgeInsets.fromLTRB(compact ? 0 : 18, 18, 12, 14),
      child: compact
          ? Center(child: mark)
          : Row(children: [
              mark,
              const SizedBox(width: 12),
              const Expanded(
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text('OnDex', style: TextStyle(color: Colors.white, fontSize: 17, fontWeight: FontWeight.w900)),
                  Text('Boshqaruv markazi', style: TextStyle(color: _sideDim, fontSize: 11.5)),
                ]),
              ),
            ]),
    );
  }
}

class _NavItem extends StatelessWidget {
  const _NavItem({
    super.key,
    required this.spec,
    required this.compact,
    required this.accent,
    required this.selected,
    required this.onTap,
    this.badge = 0,
  });

  final _SectionSpec spec;
  final bool compact;
  final Color accent;
  final bool selected;
  final VoidCallback onTap;
  final int badge;

  @override
  Widget build(BuildContext context) {
    final icon = Badge(
      isLabelVisible: badge > 0,
      label: Text('$badge'),
      child: Icon(selected ? spec.selectedIcon : spec.icon, size: 22, color: selected ? Colors.white : _sideDim),
    );
    final body = AnimatedContainer(
      duration: const Duration(milliseconds: 180),
      margin: const EdgeInsets.symmetric(vertical: 3),
      padding: EdgeInsets.symmetric(horizontal: compact ? 0 : 12, vertical: 11),
      decoration: BoxDecoration(
        color: selected ? accent.withValues(alpha: 0.18) : Colors.transparent,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: selected ? accent.withValues(alpha: 0.55) : Colors.transparent),
      ),
      child: compact
          ? Center(child: icon)
          : Row(children: [
              icon,
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  spec.label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: selected ? Colors.white : _sideFg,
                    fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                    fontSize: 14,
                  ),
                ),
              ),
              if (selected)
                Container(width: 6, height: 6, decoration: BoxDecoration(color: accent, shape: BoxShape.circle)),
            ]),
    );
    return Semantics(
      button: true,
      selected: selected,
      label: spec.label,
      child: Tooltip(
        message: compact ? spec.label : '',
        child: Material(
          type: MaterialType.transparency,
          child: InkWell(onTap: onTap, borderRadius: BorderRadius.circular(12), child: body),
        ),
      ),
    );
  }
}

/// Pastdagi xizmat almashtirgich: joriy xizmat kartasi; bosilganda barcha
/// xizmatlar ro'yxati ochiladi.
class _ServiceSwitcher extends StatelessWidget {
  const _ServiceSwitcher({required this.compact, required this.current, required this.onSwitch});

  final bool compact;
  final int current;
  final ValueChanged<int> onSwitch;

  Future<void> _open(BuildContext context) async {
    final box = context.findRenderObject() as RenderBox;
    final pos = box.localToGlobal(Offset.zero);
    final picked = await showMenu<int>(
      context: context,
      color: const Color(0xFF111C33),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      position: RelativeRect.fromLTRB(pos.dx + 8, pos.dy - 330, pos.dx + 300, pos.dy),
      items: [
        for (var i = 0; i < _services.length; i++)
          PopupMenuItem<int>(
            key: ValueKey('service-${_services[i].id}'),
            value: i,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            child: _ServiceTile(service: _services[i], selected: i == current),
          ),
      ],
    );
    if (picked != null) onSwitch(picked);
  }

  @override
  Widget build(BuildContext context) {
    final s = _services[current];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 10),
      child: Material(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          key: const ValueKey('service-switcher'),
          borderRadius: BorderRadius.circular(14),
          onTap: () => _open(context),
          child: Padding(
            padding: EdgeInsets.all(compact ? 10 : 12),
            child: compact
                ? Center(child: _ServiceIcon(service: s))
                : Row(children: [
                    _ServiceIcon(service: s),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        Text(s.title,
                            style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w800, fontSize: 14)),
                        const Text('Xizmatni almashtirish', style: TextStyle(color: _sideDim, fontSize: 11.5)),
                      ]),
                    ),
                    const Icon(Icons.unfold_more_rounded, color: _sideDim, size: 20),
                  ]),
          ),
        ),
      ),
    );
  }
}

class _ServiceIcon extends StatelessWidget {
  const _ServiceIcon({required this.service});
  final _Service service;

  @override
  Widget build(BuildContext context) => Container(
        width: 36,
        height: 36,
        decoration: BoxDecoration(
          color: service.accent.withValues(alpha: 0.18),
          borderRadius: BorderRadius.circular(10),
        ),
        alignment: Alignment.center,
        child: Icon(service.icon, color: service.accent, size: 20),
      );
}

class _ServiceTile extends StatelessWidget {
  const _ServiceTile({required this.service, required this.selected});
  final _Service service;
  final bool selected;

  @override
  Widget build(BuildContext context) => SizedBox(
        width: 260,
        child: Row(children: [
          _ServiceIcon(service: service),
          const SizedBox(width: 12),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
              Text(service.title,
                  style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 14)),
              Text(service.subtitle, style: const TextStyle(color: _sideDim, fontSize: 11.5)),
            ]),
          ),
          if (selected) Icon(Icons.check_rounded, color: service.accent, size: 20),
        ]),
      );
}

/// Xizmat bo'limlari ro'yxati; `group` bo'yicha ochiladigan guruhlar
/// ("Do'konlar" -> Market / Qurt / Apteka). Tanlangan band ichidagi guruh
/// doim ochiq turadi.
class _SectionList extends StatefulWidget {
  const _SectionList({
    super.key,
    required this.service,
    required this.compact,
    required this.selected,
    required this.unread,
    required this.onSelect,
  });

  final _Service service;
  final bool compact;
  final _Section selected;
  final int unread;
  final ValueChanged<_Section> onSelect;

  @override
  State<_SectionList> createState() => _SectionListState();
}

class _SectionListState extends State<_SectionList> {
  final Set<String> _collapsed = {};

  @override
  Widget build(BuildContext context) {
    final children = <Widget>[];
    String? group;
    for (final s in widget.service.sections) {
      final inGroup = s.group.isNotEmpty;
      if (inGroup && s.group != group && !widget.compact) {
        final g = s.group;
        final hasSelected = widget.service.sections.any((x) => x.group == g && x.id == widget.selected);
        final open = hasSelected || !_collapsed.contains(g);
        children.add(_NavGroup(
          key: ValueKey('nav-group-$g'),
          label: g,
          open: open,
          accent: widget.service.accent,
          onTap: hasSelected
              ? null
              : () => setState(() => _collapsed.contains(g) ? _collapsed.remove(g) : _collapsed.add(g)),
        ));
      }
      group = inGroup ? s.group : null;
      final hidden = inGroup &&
          !widget.compact &&
          _collapsed.contains(s.group) &&
          !widget.service.sections.any((x) => x.group == s.group && x.id == widget.selected);
      if (hidden) continue;
      children.add(Padding(
        padding: EdgeInsets.only(left: inGroup && !widget.compact ? 14 : 0),
        child: _NavItem(
          key: ValueKey(s.key),
          spec: s,
          compact: widget.compact,
          accent: widget.service.accent,
          selected: widget.selected == s.id,
          badge: s.id == _Section.restaurants ? widget.unread : 0,
          onTap: () => widget.onSelect(s.id),
        ),
      ));
    }
    return ListView(padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4), children: children);
  }
}

class _NavGroup extends StatelessWidget {
  const _NavGroup({super.key, required this.label, required this.open, required this.accent, this.onTap});

  final String label;
  final bool open;
  final Color accent;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Material(
        type: MaterialType.transparency,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(12),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
            child: Row(children: [
              Icon(Icons.storefront_rounded, size: 22, color: accent),
              const SizedBox(width: 12),
              Expanded(
                child: Text(label,
                    style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 14)),
              ),
              AnimatedRotation(
                turns: open ? 0.5 : 0,
                duration: const Duration(milliseconds: 180),
                child: const Icon(Icons.expand_more_rounded, color: _sideDim, size: 20),
              ),
            ]),
          ),
        ),
      );
}
