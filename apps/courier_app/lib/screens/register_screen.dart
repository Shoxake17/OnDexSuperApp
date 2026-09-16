import 'package:flutter/material.dart';

import '../api.dart';
import '../session.dart';
import 'courier_shell.dart';
import 'login_screen.dart';

/// Kuryer roli bo'lmagan akkaunt bilan kirilganda ko'rsatiladi.
///
/// ┌─ OnDex KURYERLARI HOZIRCHA YOPIQ (2026-09-15) ────────────────────┐
/// Avval bu yerda "kuryerlikka ariza" formasi turardi. Endi yetkazib
/// beruvchi akkauntini RESTORAN beradi ("Xodimlar" bo'limi), server esa
/// `POST /couriers/register` ni 410 bilan yopgan. Forma qolsa kuryer
/// ariza yuborib, hech qachon kelmaydigan tasdiqni kutib o'tirardi.
/// └───────────────────────────────────────────────────────────────────┘
class RegisterScreen extends StatefulWidget {
  const RegisterScreen({super.key});

  @override
  State<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends State<RegisterScreen> {
  bool _busy = false;
  String? _message;

  /// Restoran kirishni ochgan bo'lsa, rol endi `courier` — to'g'ridan-
  /// to'g'ri asosiy ekranga o'tiladi.
  Future<void> _recheck() async {
    setState(() {
      _busy = true;
      _message = null;
    });
    try {
      final user = await api.me();
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';
      if (!mounted) return;
      if (role == 'courier' && entityId.isNotEmpty) {
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => CourierShell(courierId: entityId)),
        );
        return;
      }
      setState(() => _message = 'Hali ruxsat berilmagan. Restoraningiz kirishni ochgach qayta tekshiring.');
    } on ApiException catch (e) {
      if (mounted) setState(() => _message = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _logout() async {
    await api.logout();
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final muted = TextStyle(color: Colors.grey.shade700);
    return Scaffold(
      appBar: AppBar(
        title: const Text('OnDexGO'),
        actions: [
          IconButton(
            tooltip: 'Chiqish',
            onPressed: _busy ? null : _logout,
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            const Icon(Icons.storefront_rounded, size: 56),
            const SizedBox(height: 12),
            Text(
              'Kuryer akkauntini restoraningiz beradi',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 12),
            Text(
              'Bu raqam hali yetkazib beruvchi sifatida ro\'yxatdan o\'tmagan.',
              textAlign: TextAlign.center,
              style: muted,
            ),
            const SizedBox(height: 20),
            for (final (i, step) in const [
              'Ishlaydigan restoraningiz OnDex panelida "Xodimlar" bo\'limiga kiradi.',
              'Sizni shu telefon raqami bilan "Yetkazib beruvchi" qilib qo\'shadi.',
              '"OnDexGO ilovasiga kirish" ni yoqadi.',
              'Shundan keyin pastdagi "Qayta tekshirish" tugmasini bosing.',
            ].indexed)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    CircleAvatar(radius: 12, child: Text('${i + 1}', style: const TextStyle(fontSize: 12))),
                    const SizedBox(width: 10),
                    Expanded(child: Text(step)),
                  ],
                ),
              ),
            const SizedBox(height: 12),
            if (_message != null)
              Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: Text(
                  _message!,
                  textAlign: TextAlign.center,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ),
            FilledButton.icon(
              onPressed: _busy ? null : _recheck,
              icon: _busy
                  ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.refresh),
              label: const Text('Qayta tekshirish'),
            ),
          ],
        ),
      ),
    );
  }
}
