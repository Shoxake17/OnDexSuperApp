import 'package:flutter/material.dart';

import '../theme.dart';
import 'sidebar.dart';
import 'top_bar.dart';

/// Umumiy sahifa qobig'i — Sidebar (chapda) + TopBar (yuqorida) BARCHA
/// sahifalarda (Bosh sahifa, Buyurtmalar, Menyu, va h.k.) bir xil,
/// alohida-alohida qayta yozilmaydi. Har bir sahifa faqat o'zining
/// [child] kontentini beradi, atrofdagi "qobiq" shu yerda bir marta
/// belgilanadi (RestaurantShell shu widget'ni IndexedStack o'rniga har
/// safar bitta sahifa uchun quradi — shell.dart'ga qarang).
class MainLayout extends StatelessWidget {
  final int selectedIndex;
  final ValueChanged<int> onSelect;

  final String restaurantName;
  final String restaurantAddress;
  final String restaurantLogoUrl;
  final bool restaurantOpen;
  final ValueChanged<bool> onOpenChanged;

  final String staffName;
  final String staffRole;

  final DateTime selectedDate;
  final ValueChanged<DateTime> onDateChanged;

  final int newOrdersCount;

  /// O'qilmagan restoran bildirishnomalari (qo'ng'iroq va yon menyu).
  final int notificationsCount;

  /// O'qilmagan qo'llab-quvvatlash javoblari (yon menyudagi "Chat markazi").
  final int supportCount;
  final VoidCallback onBellTap;
  final VoidCallback onLogout;

  /// Yuqori paneldagi sahifaga xos amallar (`TopBar.actions` ga qarang).
  final Widget? topBarActions;

  final Widget child;

  const MainLayout({
    this.topBarActions,
    super.key,
    required this.selectedIndex,
    required this.onSelect,
    required this.restaurantName,
    required this.restaurantAddress,
    required this.restaurantLogoUrl,
    required this.restaurantOpen,
    required this.onOpenChanged,
    required this.staffName,
    required this.staffRole,
    required this.selectedDate,
    required this.onDateChanged,
    required this.newOrdersCount,
    required this.notificationsCount,
    this.supportCount = 0,
    required this.onBellTap,
    required this.onLogout,
    required this.child,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: OnDexColors.pageBg,
      body: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Sidebar(
            selectedIndex: selectedIndex,
            onSelect: onSelect,
            restaurantName: restaurantName,
            restaurantAddress: restaurantAddress,
            restaurantLogoUrl: restaurantLogoUrl,
            newOrdersCount: newOrdersCount,
            notificationsCount: notificationsCount,
            supportCount: supportCount,
            onLogout: onLogout,
          ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TopBar(
                  restaurantName: restaurantName,
                  restaurantAddress: restaurantAddress,
                  restaurantLogoUrl: restaurantLogoUrl,
                  open: restaurantOpen,
                  onOpenChanged: onOpenChanged,
                  staffName: staffName,
                  staffRole: staffRole,
                  selectedDate: selectedDate,
                  onDateChanged: onDateChanged,
                  notificationsCount: notificationsCount,
                  onBellTap: onBellTap,
                  actions: topBarActions,
                ),
                Expanded(child: child),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
