import 'package:flutter/material.dart';

import '../api.dart';
import '../session.dart';
import 'courier_shell.dart';
import 'login_screen.dart';

/// Login qilgan (lekin hali kuryer bo'lmagan) foydalanuvchi shu ekranda
/// kuryerlikka ariza beradi. Ariza yuborilgach superadmin panelidan
/// tasdiqlanishi kerak (`CourierShell` o'zi "kutilmoqda" holatini
/// ko'rsatadi — bu yerda faqat ariza shaklining o'zi).
class RegisterScreen extends StatefulWidget {
  const RegisterScreen({super.key});

  @override
  State<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends State<RegisterScreen> {
  final _name = TextEditingController();
  bool _busy = false;
  String? _error;
  // Dispatch matching engine kuryerning ETA'sini shu turga qarab hisoblaydi
  // (Google Distance Matrix'ga piyoda/velosiped/mashina rejimi bilan
  // murojaat qilinadi) — shuning uchun ro'yxatdan o'tishda MAJBURIY.
  String _vehicleType = 'moped';

  Future<void> _submit() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Ismingizni kiriting');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final d = await api.registerCourier(name, _vehicleType);
      final newToken = d['token'] as String;
      final courierId = (d['courier'] as Map)['id'] as String;
      api.token = newToken;
      await tokenStore.write(newToken);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => CourierShell(courierId: courierId)),
      );
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _logout() async {
    await api.logout();
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(
      context,
    ).pushReplacement(MaterialPageRoute(builder: (_) => const LoginScreen()));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Kuryer bo\'lish'),
        actions: [
          IconButton(
            onPressed: _busy ? null : _logout,
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Icon(Icons.delivery_dining, size: 56),
              const SizedBox(height: 12),
              Text(
                'ChustApp kuryerlar jamoasiga qo\'shiling',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              Text(
                'Ariza yuborgach, superadmin tasdiqlashini kutasiz. Tasdiqlangach onlayn bo\'lib buyurtmalar qabul qila olasiz.',
                textAlign: TextAlign.center,
                style: TextStyle(color: Colors.grey.shade400),
              ),
              const SizedBox(height: 32),
              TextField(
                controller: _name,
                decoration: const InputDecoration(
                  labelText: 'To\'liq ismingiz',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 20),
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  'Transport turingiz',
                  style: Theme.of(context).textTheme.labelLarge,
                ),
              ),
              const SizedBox(height: 8),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(
                    value: 'foot',
                    icon: Icon(Icons.directions_walk),
                    label: Text('Piyoda'),
                  ),
                  ButtonSegment(
                    value: 'bike',
                    icon: Icon(Icons.pedal_bike),
                    label: Text('Velosiped'),
                  ),
                  ButtonSegment(
                    value: 'moped',
                    icon: Icon(Icons.moped),
                    label: Text('Moped'),
                  ),
                  ButtonSegment(
                    value: 'car',
                    icon: Icon(Icons.directions_car),
                    label: Text('Mashina'),
                  ),
                ],
                selected: {_vehicleType},
                onSelectionChanged: _busy
                    ? null
                    : (s) => setState(() => _vehicleType = s.first),
              ),
              const SizedBox(height: 16),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Text(
                    _error!,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ),
              FilledButton(
                onPressed: _busy ? null : _submit,
                child: Text(_busy ? 'Yuborilmoqda...' : 'Ariza yuborish'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
