import 'package:flutter/material.dart';

import '../theme.dart';

/// OnDex pastki navigatsiyasi — to'rt bo'lim.
///
/// ┌─ NEGA QO'LDA QURILGAN ────────────────────────────────────────────┐
/// Material'ning `NavigationBar`i badge'ni faqat o'z `NavigationDestination`
/// ichida qo'llaydi va uning balandligi/rangini OnDex uslubiga keltirish
/// uchun baribir deyarli hammasini qayta yozishga to'g'ri kelardi. Mijoz
/// ilovasida ham shu sabab bilan qo'lda qurilgan — ikkala ilova bir xil
/// ko'rinishi uchun naqsh takrorlanadi.
///
/// MARKAZIY QR TUGMASI YO'Q: QR skanerlash — MIJOZNING ishi (stoldagi
/// kodni skanerlab menyuni ochadi). Affitsiant stolni ro'yxatdan
/// tanlaydi, skanerlamaydi.
/// └───────────────────────────────────────────────────────────────────┘
class WaiterBottomBar extends StatelessWidget {
  const WaiterBottomBar({
    super.key,
    required this.current,
    required this.onSelect,
    required this.readyCount,
    required this.unreadCount,
  });

  final int current;
  final ValueChanged<int> onSelect;

  /// Tayyor buyurtmalar soni — "Buyurtmalar" ustidagi belgi.
  final int readyCount;

  /// O'qilmagan bildirishnomalar soni.
  final int unreadCount;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: kSurface,
      elevation: 0,
      child: Container(
        decoration: const BoxDecoration(
          border: Border(top: BorderSide(color: kBorder)),
        ),
        // ┌─ SAFEAREA SHART ──────────────────────────────────────────┐
        // Busiz menyu ekranning eng pastiga chizilardi va Android'ning
        // TIZIM navigatsiya paneli uning ustiga tushardi: uch tugmali
        // navigatsiyada (48dp) yozuvlar tizim tugmalari ostida qolardi.
        // Jest navigatsiyali telefonda past chiziq ~24dp bo'lgani uchun
        // bu deyarli sezilmaydi — xato AYNAN uch tugmali qurilmada
        // ko'rinadi.
        // └───────────────────────────────────────────────────────────┘
        child: SafeArea(
          top: false,
          child: SizedBox(
            height: 62,
            child: Row(
              children: [
                _NavItem(
                  icon: Icons.receipt_long_outlined,
                  activeIcon: Icons.receipt_long,
                  label: 'Buyurtmalar',
                  selected: current == 0,
                  badge: readyCount,
                  badgeColor: kReadyColor,
                  onTap: () => onSelect(0),
                ),
                _NavItem(
                  icon: Icons.table_restaurant_outlined,
                  activeIcon: Icons.table_restaurant,
                  label: 'Stollar',
                  selected: current == 1,
                  onTap: () => onSelect(1),
                ),
                _NavItem(
                  icon: Icons.notifications_none,
                  activeIcon: Icons.notifications,
                  label: 'Xabarlar',
                  selected: current == 2,
                  badge: unreadCount,
                  badgeColor: kBrandColor,
                  onTap: () => onSelect(2),
                ),
                _NavItem(
                  icon: Icons.person_outline,
                  activeIcon: Icons.person,
                  label: 'Profil',
                  selected: current == 3,
                  onTap: () => onSelect(3),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _NavItem extends StatelessWidget {
  const _NavItem({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.selected,
    required this.onTap,
    this.badge = 0,
    this.badgeColor = kBrandColor,
  });

  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool selected;
  final VoidCallback onTap;
  final int badge;
  final Color badgeColor;

  @override
  Widget build(BuildContext context) {
    final color = selected ? kBrandColor : kInkGhost;
    return Expanded(
      child: InkResponse(
        onTap: onTap,
        radius: 42,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Badge ikonka ustida — `Stack` bilan, chunki uni qatorga
            // qo'shsak yozuv markazdan siljib ketardi.
            Stack(
              clipBehavior: Clip.none,
              children: [
                Icon(selected ? activeIcon : icon, size: 22, color: color),
                if (badge > 0)
                  Positioned(
                    right: -7,
                    top: -4,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 5,
                        vertical: 1,
                      ),
                      constraints: const BoxConstraints(minWidth: 16),
                      decoration: BoxDecoration(
                        color: badgeColor,
                        borderRadius: BorderRadius.circular(10),
                        border: Border.all(color: kSurface, width: 1.5),
                      ),
                      child: Text(
                        badge > 9 ? '9+' : '$badge',
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 10,
                          fontWeight: FontWeight.bold,
                          height: 1.3,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              label,
              style: TextStyle(
                fontSize: 10.5,
                color: color,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
