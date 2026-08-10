import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import 'shell.dart';

/// Restoran paneliga kirish. Ro'yxatdan o'tish YO'Q — akkauntni superadmin
/// yaratadi va telefon raqamni restoranga beradi.
class RestaurantLoginScreen extends StatefulWidget {
  const RestaurantLoginScreen({super.key});

  @override
  State<RestaurantLoginScreen> createState() => _RestaurantLoginScreenState();
}

class _RestaurantLoginScreenState extends State<RestaurantLoginScreen> {
  final _phone = TextEditingController(text: '+998');
  final _code = TextEditingController();
  bool _codeSent = false;
  bool _busy = false;
  String? _error;

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _requestCode() => _run(() async {
        final devCode = await api.requestCode(_phone.text.trim());
        setState(() {
          _codeSent = true;
          if (devCode != null) _code.text = devCode;
        });
      });

  Future<void> _verify() => _run(() async {
        final d = await api.verify(_phone.text.trim(), _code.text.trim());
        final user = d['user'] as Map;
        if (user['role'] != 'restaurant') {
          api.token = null;
          setState(() => _error =
              'Bu raqam restoran akkaunti emas. Akkaunt olish uchun ChustApp administratsiyasiga murojaat qiling.');
          return;
        }
        api.rid = user['entity_id'] as String;
        final prefs = await SharedPreferences.getInstance();
        await prefs.setString('rest_token', api.token!);
        await prefs.setString('rest_rid', api.rid);
        if (!mounted) return;
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => const RestaurantShell()),
        );
      });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Card(
          margin: const EdgeInsets.all(24),
          child: Container(
            width: 400,
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const Icon(Icons.storefront, size: 56),
                const SizedBox(height: 8),
                Text('ChustApp Restoran',
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.headlineSmall),
                const SizedBox(height: 4),
                Text('Buyurtmalar va menyu boshqaruvi',
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.bodySmall),
                const SizedBox(height: 24),
                TextField(
                  controller: _phone,
                  enabled: !_codeSent,
                  decoration: const InputDecoration(
                    labelText: 'Restoran telefon raqami',
                    border: OutlineInputBorder(),
                  ),
                ),
                if (_codeSent) ...[
                  const SizedBox(height: 12),
                  TextField(
                    controller: _code,
                    decoration: const InputDecoration(
                      labelText: 'SMS kod',
                      border: OutlineInputBorder(),
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Text(_error!,
                        style: TextStyle(
                            color: Theme.of(context).colorScheme.error)),
                  ),
                FilledButton(
                  onPressed:
                      _busy ? null : (_codeSent ? _verify : _requestCode),
                  child: Text(_busy
                      ? 'Kutilmoqda...'
                      : (_codeSent ? 'Kirish' : 'Kod olish')),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
