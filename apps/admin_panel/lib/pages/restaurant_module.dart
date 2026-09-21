import 'package:flutter/material.dart';

import '../api.dart';
import '../support_inbox.dart';
import 'books_page.dart';
import 'couriers_page.dart';
import 'orders_page.dart';
import 'people_page.dart';
import 'restaurants_page.dart';
import 'support_chat_page.dart';

/// "Restoran" bo'limi: ro'yxat (cover rasmli kartalar) yoki tanlangan
/// restoranning ALOHIDA moduli.
///
/// Tanlangan restoran shu yerda saqlanadi — foydalanuvchi boshqa bo'limga
/// (Statistika, OnDexMap, ...) o'tib qaytsa ham o'sha restoran ochiq qoladi
/// (`AdminShell` bu vidjetni o'chirmaydi).
class RestaurantsSection extends StatefulWidget {
  const RestaurantsSection({super.key});

  @override
  State<RestaurantsSection> createState() => _RestaurantsSectionState();
}

class _RestaurantsSectionState extends State<RestaurantsSection> {
  Map<String, dynamic>? _open;

  @override
  Widget build(BuildContext context) {
    final r = _open;
    if (r == null) {
      return RestaurantsPage(onOpen: (r) => setState(() => _open = r));
    }
    return RestaurantModule(
      // Boshqa restoran ochilsa hamma ichki holat (yuklangan ro'yxatlar,
      // chat) YANGIDAN boshlanadi — eskisidan meros qolmaydi.
      key: ValueKey('restaurant-module-${r['id']}'),
      restaurant: r,
      onBack: () => setState(() => _open = null),
    );
  }
}

/// Bitta restoranning moduli: Buyurtmalar, Kuryerlar, Affitsiantlar,
/// Kutubxona, Chat. Har bo'lim FAQAT shu restoran ma'lumotini ko'rsatadi
/// (umumiy ro'yxat emas).
///
/// Bo'limlar birinchi ochilganda yuklanadi va keyin saqlanib qoladi:
/// avvaldan beshta sahifa birdan so'rov yubormaydi, tab almashganda esa
/// yozilayotgan chat matni yoki ro'yxat holati yo'qolmaydi.
class RestaurantModule extends StatefulWidget {
  const RestaurantModule({
    super.key,
    required this.restaurant,
    required this.onBack,
  });

  /// Restoranning to'liq yozuvi (`GET /restaurants`).
  final Map<String, dynamic> restaurant;
  final VoidCallback onBack;

  @override
  State<RestaurantModule> createState() => _RestaurantModuleState();
}

class _RestaurantModuleState extends State<RestaurantModule>
    with SingleTickerProviderStateMixin {
  static const _tabs = <({String name, IconData icon, String screen})>[
    (name: 'Buyurtmalar', icon: Icons.receipt_long_outlined, screen: 'Orders'),
    (
      name: 'Kuryerlar',
      icon: Icons.delivery_dining_outlined,
      screen: 'Couriers'
    ),
    (
      name: 'Affitsiantlar',
      icon: Icons.room_service_outlined,
      screen: 'Waiters'
    ),
    (name: 'Kutubxona', icon: Icons.menu_book_outlined, screen: 'Books'),
    (name: 'Chat', icon: Icons.forum_outlined, screen: 'Chat'),
  ];

  static const _chatTab = 4;

  late final TabController _controller;
  final Set<int> _visited = {0};
  int _chatUnread = 0;

  String get _id => '${widget.restaurant['id']}';

  @override
  void initState() {
    super.initState();
    _controller = TabController(length: _tabs.length, vsync: this)
      ..addListener(_onTab);
    supportInbox.addListener(_loadUnread);
    _loadUnread();
    Analytics.instance.screen('Restaurant Orders');
  }

  @override
  void dispose() {
    supportInbox.removeListener(_loadUnread);
    _controller
      ..removeListener(_onTab)
      ..dispose();
    super.dispose();
  }

  void _onTab() {
    // `index` tab bosilgan zahoti o'zgaradi (animatsiya tugashini
    // kutmaymiz): bo'lim darhol ko'rinadi.
    final i = _controller.index;
    if (_visited.add(i)) {
      Analytics.instance.screen('Restaurant ${_tabs[i].screen}');
    }
    setState(() {});
  }

  Future<void> _loadUnread() async {
    try {
      final map = await fetchUnreadByRestaurant();
      final n = map[_id] ?? 0;
      if (mounted && n != _chatUnread) setState(() => _chatUnread = n);
    } catch (_) {
      // Rozetka — qo'shimcha ma'lumot; modulni to'xtatmaydi.
    }
  }

  Widget _page(int i) => switch (i) {
        0 => OrdersPage(restaurantId: _id),
        1 => CouriersPage(restaurantId: _id),
        2 => WaitersPage(restaurantId: _id),
        3 => BooksPage(restaurantId: _id),
        _ => SupportChatPage(restaurant: widget.restaurant),
      };

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final r = widget.restaurant;
    final name = '${r['name'] ?? ''}';
    final address = '${r['address'] ?? ''}';
    final logo = '${r['logo_url'] ?? ''}';
    final isOpen = (r['open_now'] ?? r['open']) == true;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 24, 0),
          child: Row(
            children: [
              IconButton(
                key: const ValueKey('restaurant-module-back'),
                tooltip: 'Restoranlar ro\'yxatiga qaytish',
                onPressed: widget.onBack,
                icon: const Icon(Icons.arrow_back),
              ),
              const SizedBox(width: 4),
              ClipOval(
                child: SizedBox(
                  width: 44,
                  height: 44,
                  child: logo.isEmpty
                      ? Container(
                          color: theme.colorScheme.primaryContainer,
                          alignment: Alignment.center,
                          child: Text(
                            name.trim().isEmpty
                                ? '?'
                                : name.trim().characters.first.toUpperCase(),
                            style: TextStyle(
                                fontSize: 18,
                                fontWeight: FontWeight.w700,
                                color: theme.colorScheme.onPrimaryContainer),
                          ),
                        )
                      : Image.network(
                          imageUrl(logo),
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => Container(
                            color: theme.colorScheme.primaryContainer,
                            child: const Icon(Icons.storefront),
                          ),
                        ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(name,
                        key: const ValueKey('restaurant-module-title'),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.titleLarge
                            ?.copyWith(fontWeight: FontWeight.w700)),
                    if (address.isNotEmpty)
                      Text(address,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.bodySmall),
                  ],
                ),
              ),
              Chip(
                visualDensity: VisualDensity.compact,
                label: Text(isOpen ? 'Ochiq' : 'Yopiq'),
                backgroundColor:
                    isOpen ? const Color(0xFFD0F0D8) : const Color(0xFFEEEEEE),
              ),
            ],
          ),
        ),
        Align(
          alignment: Alignment.centerLeft,
          child: TabBar(
            key: const ValueKey('restaurant-module-tabs'),
            controller: _controller,
            isScrollable: true,
            tabAlignment: TabAlignment.start,
            tabs: [
              for (var i = 0; i < _tabs.length; i++)
                Tab(
                  key: ValueKey('restaurant-tab-${_tabs[i].screen}'),
                  icon: i == _chatTab
                      ? Badge(
                          isLabelVisible: _chatUnread > 0,
                          label: Text('$_chatUnread'),
                          child: Icon(_tabs[i].icon),
                        )
                      : Icon(_tabs[i].icon),
                  text: _tabs[i].name,
                ),
            ],
          ),
        ),
        const Divider(height: 1),
        Expanded(
          child: IndexedStack(
            index: _controller.index,
            children: [
              for (var i = 0; i < _tabs.length; i++)
                _visited.contains(i) ? _page(i) : const SizedBox.shrink(),
            ],
          ),
        ),
      ],
    );
  }
}
