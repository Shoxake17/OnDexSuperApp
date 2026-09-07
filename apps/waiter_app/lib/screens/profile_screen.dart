import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show LegalSection;

import '../state/waiter_store.dart';
import '../theme.dart';
import 'history_screen.dart';

/// Profil va sozlamalar.
///
/// ┌─ NIMA YO'Q VA NEGA ───────────────────────────────────────────────┐
///  * "Ish vaqti" — backendda smena-jadval tizimi yo'q. Ko'rsatilsa,
///    u yo restoranning ish vaqti (affitsiant profiliga aloqasi yo'q),
///    yo bo'lmagan funksiya haqidagi va'da bo'lardi.
///  * "Til" — butun OnDex oilasida i18n infratuzilmasi yo'q, faqat
///    o'zbekcha. Yolg'iz shu ilovada til tanlash ishlamaydigan tugma
///    bo'lardi.
///  * "Hisobotlar" — "Mening tarixim" bilan bir xil narsa. Bitta narsa
///    ikki nom bilan atalsa, foydalanuvchi ikkalasini ham ochib
///    ko'rishga majbur bo'ladi.
///  * Umumiy kunlik savdo — restoran EGASINING ma'lumoti
///    (`restaurant_panel`), affitsiantniki emas. Bu yerda faqat
///    affitsiantning O'ZI yetkazgan buyurtmalari ko'rinadi.
/// └───────────────────────────────────────────────────────────────────┘
class ProfileScreen extends StatelessWidget {
  const ProfileScreen({
    super.key,
    required this.store,
    required this.onLogout,
  });

  final WaiterStore store;
  final Future<void> Function() onLogout;

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 24),
      children: [
        _header(),
        const SizedBox(height: 16),

        _Section(
          children: [
            _Tile(
              icon: Icons.history,
              title: 'Mening tarixim',
              subtitle: store.todayHistory.isEmpty
                  ? 'Bugun hali buyurtma yetkazilmadi'
                  : 'Bugun ${store.todayHistory.length} ta buyurtma',
              onTap: () => Navigator.of(context).push(
                MaterialPageRoute(builder: (_) => HistoryScreen(store: store)),
              ),
            ),
          ],
        ),

        const SizedBox(height: 12),
        _Section(
          children: [
            SwitchListTile(
              value: store.soundEnabled,
              onChanged: store.setSoundEnabled,
              activeThumbColor: kBrandColor,
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: 14, vertical: 2),
              secondary: const Icon(Icons.volume_up_outlined,
                  color: kInkFaint, size: 20),
              title: const Text(
                'Bildirishnoma ovozi',
                style: TextStyle(color: kInk, fontSize: 14.5),
              ),
              subtitle: Text(
                store.soundEnabled
                    ? 'Taom tayyor bo\'lganda signal chalinadi'
                    : 'Ovoz o\'chirilgan — faqat ekrandagi belgi',
                style: const TextStyle(color: kInkGhost, fontSize: 12),
              ),
            ),
            const Divider(height: 1, indent: 14, endIndent: 14),
            _Tile(
              icon: Icons.refresh,
              title: 'Ro\'yxatni yangilash',
              subtitle: 'Serverdan qayta yuklab olish',
              onTap: () async {
                final messenger = ScaffoldMessenger.of(context);
                await store.refresh();
                messenger.showSnackBar(
                  const SnackBar(
                    content: Text('Ro\'yxat yangilandi'),
                    duration: Duration(seconds: 2),
                  ),
                );
              },
            ),
          ],
        ),

        // Huquqiy hujjatlar — affitsiant ilovasi ham do'konga chiqadi
        // (bug.md 70-band). Manzil `ondex_core.LegalLinks` da, ya'ni
        // beshta ilovada bir xil.
        const SizedBox(height: 12),
        const _Section(children: [LegalSection(showDivider: true)]),

        const SizedBox(height: 12),
        _Section(
          children: [
            _Tile(
              icon: Icons.logout,
              title: 'Chiqish',
              color: kDangerColor,
              onTap: () => _confirmLogout(context),
            ),
          ],
        ),

        const SizedBox(height: 20),
        const Center(
          child: Text(
            'OnDex Affitsiant · 1.0.0',
            style: TextStyle(color: kInkGhost, fontSize: 11.5),
          ),
        ),
      ],
    );
  }

  Widget _header() {
    final name = store.userName.isEmpty ? 'Affitsiant' : store.userName;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(color: kBorder),
      ),
      child: Row(
        children: [
          Container(
            width: 52,
            height: 52,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: kBrandColor.withValues(alpha: 0.15),
            ),
            child: const Icon(Icons.person, color: kBrandColor, size: 26),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: kInk,
                    fontSize: 17,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 2),
                if (store.userPhone.isNotEmpty)
                  Text(
                    store.userPhone,
                    style: const TextStyle(color: kInkFaint, fontSize: 13),
                  ),
                if (store.restaurantName.isNotEmpty) ...[
                  const SizedBox(height: 4),
                  Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: kSurfaceRaised,
                      borderRadius: BorderRadius.circular(kRadiusChip),
                    ),
                    child: Text(
                      store.restaurantName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(color: kInkDim, fontSize: 12),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// Chiqishdan oldin tasdiqlash.
  ///
  /// Chiqish — arzon amal emas: qayta kirish uchun Telegram OTP kerak
  /// bo'ladi va zalda ish qizigan paytda bu bir necha daqiqa vaqt oladi.
  Future<void> _confirmLogout(BuildContext context) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: kSurface,
        title: const Text('Chiqish', style: TextStyle(color: kInk)),
        content: const Text(
          'Qayta kirish uchun Telegram orqali kod olishingiz kerak bo\'ladi.',
          style: TextStyle(color: kInkFaint),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            style: TextButton.styleFrom(foregroundColor: kInkFaint),
            child: const Text('Bekor qilish'),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            style: TextButton.styleFrom(foregroundColor: kDangerColor),
            child: const Text('Chiqish'),
          ),
        ],
      ),
    );
    if (ok == true) await onLogout();
  }
}

class _Section extends StatelessWidget {
  const _Section({required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(color: kBorder),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(children: children),
    );
  }
}

class _Tile extends StatelessWidget {
  const _Tile({
    required this.icon,
    required this.title,
    required this.onTap,
    this.subtitle,
    this.color,
  });

  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback onTap;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      onTap: onTap,
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      leading: Icon(icon, color: color ?? kInkFaint, size: 20),
      title: Text(
        title,
        style: TextStyle(color: color ?? kInk, fontSize: 14.5),
      ),
      subtitle: subtitle == null
          ? null
          : Text(
              subtitle!,
              style: const TextStyle(color: kInkGhost, fontSize: 12),
            ),
      trailing: color == null
          ? const Icon(Icons.chevron_right, color: kInkGhost, size: 20)
          : null,
    );
  }
}
