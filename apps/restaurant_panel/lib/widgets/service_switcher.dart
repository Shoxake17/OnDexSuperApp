import 'package:flutter/material.dart';

import '../theme.dart';

/// Merchant xizmati — admin paneldagi "xizmat bloklari" bilan bir xil g'oya.
enum MerchantService { restaurant, store }

/// Sidebar tepasidagi xizmat bloklari: **Restoran | Do'kon**.
///
/// Akkaunt hozircha bitta xizmatga bog'langan (rol: `restaurant` yoki
/// `store`), shuning uchun ikkinchisi qulf bilan ko'rinadi va bosilganda
/// nima qilish kerakligini aytadi. Bir akkauntga ikkala xizmat ulanganda
/// faqat [available] ga ikkinchisi qo'shiladi — vidjet o'zgarmaydi.
class ServiceSwitcher extends StatelessWidget {
  const ServiceSwitcher({
    super.key,
    required this.active,
    this.available = const {},
    this.onSwitch,
    this.storeLabel = 'Do\'kon',
  });

  final MerchantService active;

  /// Shu akkauntda ochiq xizmatlar ([active] doim ochiq).
  final Set<MerchantService> available;
  final ValueChanged<MerchantService>? onSwitch;

  /// Do'kon bloki nomi (Do'kon / Qurt / Dorixona).
  final String storeLabel;

  @override
  Widget build(BuildContext context) {
    Widget block(MerchantService s, IconData icon, String label) {
      final isActive = s == active;
      final enabled = isActive || available.contains(s);
      return Expanded(
        child: Tooltip(
          message: enabled
              ? label
              : 'Bu akkauntga ${s == MerchantService.restaurant ? 'restoran' : 'do\'kon'} ulanmagan — '
                  'ulash uchun OnDex administratoriga murojaat qiling',
          child: Material(
            color: isActive ? OnDexColors.primary : OnDexColors.sidebarBgActive,
            borderRadius: BorderRadius.circular(10),
            child: InkWell(
              key: ValueKey('service-${s.name}'),
              borderRadius: BorderRadius.circular(10),
              onTap: isActive
                  ? null
                  : enabled
                      ? () => onSwitch?.call(s)
                      : () => ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                            content: Text('Bu akkauntga ${s == MerchantService.restaurant ? 'restoran' : 'do\'kon'} '
                                'ulanmagan. Ulash uchun OnDex administratoriga murojaat qiling.'),
                          )),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 8),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(enabled ? icon : Icons.lock_outline_rounded,
                        size: 17,
                        color: isActive
                            ? Colors.white
                            : enabled
                                ? OnDexColors.sidebarText
                                : OnDexColors.sidebarTextDim.withValues(alpha: 0.6)),
                    const SizedBox(width: 6),
                    Flexible(
                      child: Text(
                        label,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 12.5,
                          fontWeight: isActive ? FontWeight.w800 : FontWeight.w600,
                          color: isActive
                              ? Colors.white
                              : enabled
                                  ? OnDexColors.sidebarText
                                  : OnDexColors.sidebarTextDim.withValues(alpha: 0.6),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 0),
      child: Row(
        children: [
          block(MerchantService.restaurant, Icons.restaurant_rounded, 'Restoran'),
          const SizedBox(width: 8),
          block(MerchantService.store, Icons.shopping_bag_rounded, storeLabel),
        ],
      ),
    );
  }
}
