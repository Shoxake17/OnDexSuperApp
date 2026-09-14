import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';

class _NavEntry {
  final IconData icon;
  final String label;
  const _NavEntry(this.icon, this.label);
}

/// Sidebar navigatsiyasi — image/bosh.png namunasiga mos: 12 ta bo'lim,
/// eng yuqorida OnDex/"Restoran paneli" brend belgisi, pastda esa
/// RESTORANNING O'ZINING profil kartochkasi (logo+nomi+manzili), bosilsa
/// chiqish menyusi ochiladi. Faqat 3 ta bo'lim (Bosh sahifa, Buyurtmalar,
/// Menyu) haqiqiy ishlaydigan sahifaga ega — qolganlari hali qurilmagan
/// (ComingSoonPage) bo'lsa ham, navigatsiya ro'yxatining o'zi rasmga 100%
/// mos bo'lishi uchun TO'LIQ ko'rsatiladi.
const List<_NavEntry> _sidebarNavEntries = [
  _NavEntry(Icons.home_rounded, 'Bosh sahifa'),
  _NavEntry(Icons.shopping_bag_rounded, 'Buyurtmalar'),
  _NavEntry(Icons.restaurant_menu_rounded, 'Menyu'),
  _NavEntry(Icons.sell_rounded, 'Aksiyalar'),
  // Stollar (QR kod) — buyurtmalar va menyudan keyin, chunki u ham
  // KUNDALIK ish quroli, "tez orada" bo'limlari emas.
  _NavEntry(Icons.table_restaurant_rounded, 'Stollar (QR)'),
  _NavEntry(Icons.bar_chart_rounded, 'Statistika'),
  _NavEntry(Icons.account_balance_wallet_rounded, 'Moliya'),
  _NavEntry(Icons.settings_rounded, 'Restoran sozlamalari'),
  _NavEntry(Icons.groups_rounded, 'Xodimlar'),
  _NavEntry(Icons.notifications_rounded, 'Bildirishnomalar'),
  _NavEntry(Icons.help_rounded, 'Yordam markazi'),
  // OnDex qo'llab-quvvatlash bilan yozishma. Oxirida — oldingi
  // indekslar surilmaydi (`shell.dart` dagi switch bilan bir xil tartib).
  _NavEntry(Icons.forum_rounded, 'Chat markazi'),
];

class Sidebar extends StatelessWidget {
  final int selectedIndex;
  final ValueChanged<int> onSelect;
  final String restaurantName;
  final String restaurantAddress;
  final String restaurantLogoUrl;
  final int newOrdersCount;
  final int notificationsCount;

  /// O'qilmagan OnDex qo'llab-quvvatlash javoblari ("Chat markazi").
  final int supportCount;
  final VoidCallback onLogout;

  const Sidebar({
    super.key,
    required this.selectedIndex,
    required this.onSelect,
    required this.restaurantName,
    required this.restaurantAddress,
    required this.restaurantLogoUrl,
    required this.newOrdersCount,
    this.notificationsCount = 0,
    this.supportCount = 0,
    required this.onLogout,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 248,
      color: OnDexColors.sidebarBg,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const _BrandHeader(),
          const Divider(height: 1, color: Color(0x22FFFFFF)),
          const SizedBox(height: 12),
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              itemCount: _sidebarNavEntries.length,
              itemBuilder: (context, i) {
                final entry = _sidebarNavEntries[i];
                // Faqat HAQIQIY hisoblangan rozetkalar: "Buyurtmalar"
                // (index 1) — yangi ("created") buyurtmalar,
                // "Bildirishnomalar" (index 9) — o'qilmagan restoran
                // bildirishnomalari (jonli). Boshqa bo'limlarga soxta
                // raqam qo'yilmaydi.
                final count = switch (i) {
                  1 => newOrdersCount,
                  9 => notificationsCount,
                  11 => supportCount,
                  _ => 0,
                };
                final badge = count > 0 ? count : null;
                return _NavItem(
                  icon: entry.icon,
                  label: entry.label,
                  selected: selectedIndex == i,
                  badge: badge,
                  onTap: () => onSelect(i),
                );
              },
            ),
          ),
          _RestaurantCard(
            name: restaurantName,
            address: restaurantAddress,
            logoUrl: restaurantLogoUrl,
            onLogout: onLogout,
          ),
        ],
      ),
    );
  }
}

class _BrandHeader extends StatelessWidget {
  const _BrandHeader();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 22, 20, 20),
      child: Row(
        children: [
          // ┌─ HAQIQIY LOGOTIP, CHIZILGAN BELGI EMAS ──────────────────┐
          // Ilgari bu yerda rangli kvadrat va ichida savat ikonkasi
          // chizilardi — bu OnDex logotipi emas, shunchaki o'xshatma
          // edi.
          //
          // `errorBuilder` SHART: rasm yuklanmasa (asset ro'yxatdan
          // tushib qolsa) butun yon menyu qizil xato kvadrati bilan
          // buzilardi. Bunday holatda eski ko'rinishga qaytamiz.
          // └──────────────────────────────────────────────────────────┘
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: Image.asset(
              'assets/ondex.png',
              width: 38,
              height: 38,
              fit: BoxFit.cover,
              filterQuality: FilterQuality.medium,
              errorBuilder: (_, __, ___) => Container(
                width: 38,
                height: 38,
                decoration: BoxDecoration(
                  color: OnDexColors.primary,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: const Icon(Icons.shopping_bag_rounded,
                    color: Colors.white, size: 20),
              ),
            ),
          ),
          const SizedBox(width: 12),
          const Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('Ondex',
                  style: TextStyle(
                      color: OnDexColors.sidebarText,
                      fontWeight: FontWeight.w800,
                      fontSize: 17,
                      height: 1.1)),
              Text('Restoran paneli',
                  style: TextStyle(
                      color: OnDexColors.sidebarTextDim,
                      fontSize: 11.5,
                      height: 1.3)),
            ],
          ),
        ],
      ),
    );
  }
}

class _NavItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool selected;
  final int? badge;
  final VoidCallback onTap;

  const _NavItem({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
    this.badge,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Material(
        color: selected ? OnDexColors.primary : Colors.transparent,
        borderRadius: BorderRadius.circular(10),
        child: InkWell(
          borderRadius: BorderRadius.circular(10),
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            child: Row(
              children: [
                Icon(icon,
                    size: 20,
                    color:
                        selected ? Colors.white : OnDexColors.sidebarTextDim),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: selected
                          ? Colors.white
                          : OnDexColors.sidebarTextDim,
                      fontWeight:
                          selected ? FontWeight.w700 : FontWeight.w500,
                      fontSize: 13.5,
                    ),
                  ),
                ),
                if (badge != null)
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                    decoration: BoxDecoration(
                      color: selected ? Colors.white : OnDexColors.primary,
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Text(
                      '$badge',
                      style: TextStyle(
                        color: selected ? OnDexColors.primary : Colors.white,
                        fontSize: 11,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Sidebar pastidagi restoran profil kartochkasi — image/bosh.png
/// namunasidagi kabi. Bosilsa kichik menyu ochiladi ("Chiqish") — namunada
/// bu chevron nima ochishi ko'rinmasa ham, ilovada chiqish tugmasi
/// SOMEWHERE bo'lishi shart, shu tabiiy joy tanlandi.
class _RestaurantCard extends StatelessWidget {
  final String name;
  final String address;
  final String logoUrl;
  final VoidCallback onLogout;

  const _RestaurantCard({
    required this.name,
    required this.address,
    required this.logoUrl,
    required this.onLogout,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
      child: PopupMenuButton<String>(
        tooltip: '',
        offset: const Offset(0, -8),
        color: OnDexColors.cardBg,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        onSelected: (v) {
          if (v == 'logout') onLogout();
        },
        itemBuilder: (context) => [
          const PopupMenuItem(
            value: 'logout',
            child: Row(
              children: [
                Icon(Icons.logout_rounded, size: 18, color: OnDexColors.danger),
                SizedBox(width: 10),
                Text('Chiqish', style: TextStyle(color: OnDexColors.danger)),
              ],
            ),
          ),
        ],
        child: Container(
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: OnDexColors.sidebarBgActive,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Row(
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(9),
                child: Container(
                  width: 36,
                  height: 36,
                  color: OnDexColors.primary,
                  child: logoUrl.isEmpty
                      ? const Icon(Icons.storefront,
                          color: Colors.white, size: 18)
                      : Image.network(
                          fullImageUrl(logoUrl),
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => const Icon(
                              Icons.storefront,
                              color: Colors.white,
                              size: 18),
                        ),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(name.isEmpty ? 'Restoran' : name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            color: OnDexColors.sidebarText,
                            fontWeight: FontWeight.w700,
                            fontSize: 13)),
                    if (address.isNotEmpty)
                      Text(address,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              color: OnDexColors.sidebarTextDim,
                              fontSize: 11)),
                  ],
                ),
              ),
              const Icon(Icons.unfold_more_rounded,
                  size: 16, color: OnDexColors.sidebarTextDim),
            ],
          ),
        ),
      ),
    );
  }
}
