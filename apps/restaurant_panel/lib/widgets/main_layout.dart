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
  final VoidCallback onLogout;

  final Widget child;

  const MainLayout({
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
                  newOrdersCount: newOrdersCount,
                  onBellTap: () => onSelect(1), // Buyurtmalar
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
