import 'package:flutter/material.dart';

import '../api.dart';
import '../session.dart';
import 'courier_shell.dart';
import 'register_screen.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _phone = TextEditingController(text: '+998');
  final _code = TextEditingController();
  bool _codeSent = false;
  bool _busy = false;
  String? _error;

  Future<void> _requestCode() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final devCode = await api.requestCode(_phone.text.trim());
      setState(() {
        _codeSent = true;
        // Dev rejimda server kodni qaytaradi — qo'lda yozmaslik uchun to'ldirib qo'yamiz.
        if (devCode != null) _code.text = devCode;
      });
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await api.verify(_phone.text.trim(), _code.text.trim());
      await tokenStore.write(api.token!);
      final user = await api.me();
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(
          builder: (_) => (role == 'courier' && entityId.isNotEmpty)
              ? CourierShell(courierId: entityId)
              : const RegisterScreen(),
        ),
      );
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Icon(Icons.delivery_dining, size: 64),
              const SizedBox(height: 8),
              Text('ChustApp Kuryer',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.headlineMedium),
              const SizedBox(height: 32),
              TextField(
                controller: _phone,
                enabled: !_codeSent,
                keyboardType: TextInputType.phone,
                decoration: const InputDecoration(
                  labelText: 'Telefon raqam',
                  hintText: '+998901234567',
                  border: OutlineInputBorder(),
                ),
              ),
              if (_codeSent) ...[
                const SizedBox(height: 16),
                TextField(
                  controller: _code,
                  keyboardType: TextInputType.number,
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
                onPressed: _busy ? null : (_codeSent ? _verify : _requestCode),
                child: Text(_busy
                    ? 'Kutilmoqda...'
                    : (_codeSent ? 'Kirish' : 'Kod olish')),
              ),
              if (_codeSent)
                TextButton(
                  onPressed: _busy
                      ? null
                      : () => setState(() {
                            _codeSent = false;
                            _code.clear();
                          }),
                  child: const Text('Raqamni o\'zgartirish'),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
