import 'package:flutter/material.dart';

import '../theme.dart';

/// Merchant xizmati — admin paneldagi "xizmatni almashtirish" bilan bir xil g'oya.
enum MerchantService { restaurant, store }

/// Sidebar pastidagi profil kartasi bosilganda ochiladigan menyu (admin
/// paneldagi kabi): xizmatlar — **Restoran**, **Do'kon** (faoli belgilangan,
/// akkauntga ulanmagani qulf bilan) — va pastda **Chiqish**.
///
/// Akkaunt hozircha bitta xizmatga bog'langan (rol `restaurant` yoki
/// `store`); bir akkauntga ikkalasi ulanganda faqat [available] ga
/// ikkinchisi qo'shiladi.
List<PopupMenuEntry<String>> merchantMenuItems({
  required MerchantService active,
  Set<MerchantService> available = const {},
  String storeLabel = 'Do\'kon',
}) {
  PopupMenuItem<String> service(MerchantService s, IconData icon, String title, String subtitle) {
    final isActive = s == active;
    final enabled = isActive || available.contains(s);
    return PopupMenuItem<String>(
      key: ValueKey('service-${s.name}'),
      value: 'service:${s.name}',
      child: SizedBox(
        width: 240,
        child: Row(children: [
          Container(
            width: 34,
            height: 34,
            decoration: BoxDecoration(
              color: isActive ? OnDexColors.primary : OnDexColors.primaryTint,
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(enabled ? icon : Icons.lock_outline_rounded,
                size: 18, color: isActive ? Colors.white : OnDexColors.primaryPressed),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
              Text(title,
                  style: TextStyle(
                      fontWeight: isActive ? FontWeight.w800 : FontWeight.w600,
                      color: enabled ? OnDexColors.ink : OnDexColors.inkFaint)),
              Text(enabled ? subtitle : 'Ulanmagan',
                  style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
            ]),
          ),
          if (isActive) const Icon(Icons.check_rounded, color: OnDexColors.primary, size: 20),
        ]),
      ),
    );
  }

  return [
    service(MerchantService.restaurant, Icons.restaurant_rounded, 'Restoran', 'Buyurtmalar, menyu, stollar'),
    service(MerchantService.store, Icons.shopping_bag_rounded, storeLabel, 'Kassa, mahsulotlar, qarzlar'),
    const PopupMenuDivider(),
    const PopupMenuItem<String>(
      key: ValueKey('menu-logout'),
      value: 'logout',
      child: Row(children: [
        Icon(Icons.logout_rounded, size: 18, color: OnDexColors.danger),
        SizedBox(width: 10),
        Text('Chiqish', style: TextStyle(color: OnDexColors.danger)),
      ]),
    ),
  ];
}

/// Menyu tanlovini bajaradi: chiqish, xizmatga o'tish yoki ulanmagan xizmat
/// haqida xabar.
void handleMerchantMenu(
  BuildContext context,
  String value, {
  required MerchantService active,
  Set<MerchantService> available = const {},
  required VoidCallback onLogout,
  ValueChanged<MerchantService>? onSwitch,
}) {
  if (value == 'logout') {
    onLogout();
    return;
  }
  final s = MerchantService.values.firstWhere((x) => 'service:${x.name}' == value, orElse: () => active);
  if (s == active) return;
  if (available.contains(s)) {
    onSwitch?.call(s);
    return;
  }
  (ScaffoldMessenger.of(context)..hideCurrentSnackBar()).showSnackBar(SnackBar(
    content: Text('Bu akkauntga ${s == MerchantService.restaurant ? 'restoran' : 'do\'kon'} ulanmagan. '
        'Ulash uchun OnDex administratoriga murojaat qiling.'),
  ));
}
