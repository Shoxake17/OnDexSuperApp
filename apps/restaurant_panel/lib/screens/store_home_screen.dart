import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../main.dart' show isLocalDevApi;
import 'login_screen.dart';

/// Do'kon egasi uchun bosh ekran.
///
/// Do'kon akkaunti OnDex akkaunti — restoran kabi shu ilovaga ham kiradi,
/// lekin kassa/ombor (POS) hozircha VEB Merchant Panelda. U yerda ham
/// AYNAN shu akkaunt bilan kiriladi ("OnDex orqali kirish"), alohida
/// login/parol yo'q. Do'kon uchun ilova bo'limlari keyin qo'shiladi.
class StoreHomeScreen extends StatelessWidget {
  const StoreHomeScreen({super.key, required this.name, required this.phone});

  final String name;
  final String phone;

  // Mahalliy API bo'lsa — mahalliy veb panel (`ondex run`), aks holda ishchi manzil.
  static String get _posUrl =>
      isLocalDevApi() ? 'http://localhost:3001' : 'https://pos.ondex.uz';

  Future<void> _logout(BuildContext context) async {
    await restTokenStore.clear();
    api.token = null;
    if (!context.mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const RestaurantLoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        title: const Text('OnDex Merchant Panel'),
        actions: [
          TextButton.icon(
            onPressed: () => _logout(context),
            icon: const Icon(Icons.logout),
            label: const Text('Chiqish'),
          ),
        ],
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 440),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const Icon(Icons.store_mall_directory_outlined, size: 56),
                const SizedBox(height: 12),
                Text(name.isEmpty ? 'Do\'kon' : name,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.headlineSmall),
                const SizedBox(height: 4),
                Text(phone,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodySmall),
                const SizedBox(height: 24),
                Text(
                  'Do\'kon boshqaruvi (kassa, ombor, sotuvlar) hozircha veb '
                  'Merchant Panelda. U yerga shu akkaunt bilan kirasiz.',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium,
                ),
                const SizedBox(height: 16),
                FilledButton.icon(
                  onPressed: () => launchUrl(Uri.parse(_posUrl),
                      mode: LaunchMode.externalApplication),
                  icon: const Icon(Icons.open_in_new),
                  label: const Text('Veb Merchant Panelni ochish'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
