import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import 'shell.dart';

class AdminLoginScreen extends StatefulWidget {
  const AdminLoginScreen({super.key});

  @override
  State<AdminLoginScreen> createState() => _AdminLoginScreenState();
}

class _AdminLoginScreenState extends State<AdminLoginScreen> {
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
        final role = (d['user'] as Map)['role'];
        if (role != 'admin') {
          api.token = null;
          setState(() => _error = 'Bu panel faqat superadmin uchun');
          return;
        }
        final prefs = await SharedPreferences.getInstance();
        await prefs.setString('admin_token', api.token!);
        if (!mounted) return;
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => const AdminShell()),
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
                const Icon(Icons.admin_panel_settings, size: 56),
                const SizedBox(height: 8),
                Text('ChustApp Superadmin',
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.headlineSmall),
                const SizedBox(height: 24),
                TextField(
                  controller: _phone,
                  enabled: !_codeSent,
                  decoration: const InputDecoration(
                    labelText: 'Admin telefon raqami',
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
