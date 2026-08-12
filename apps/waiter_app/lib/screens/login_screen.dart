import 'package:flutter/material.dart';

import '../api.dart';
import '../session.dart';
import '../theme.dart';
import 'waiter_shell.dart';

/// Affitsiant kirishi — telefon raqami + SMS kod.
///
/// ┌─ NEGA RO'YXATDAN O'TISH YO'Q ─────────────────────────────────────┐
/// Affitsiant akkauntini RESTORAN o'z panelidan yaratadi (foydalanuvchi
/// qarori, 2026-08-12). Bu ataylab: aks holda istalgan odam o'zini
/// affitsiant deb ro'yxatdan o'tkazib, restoranning buyurtmalarini
/// ko'ra olardi.
///
/// Shuning uchun bu yerda faqat KIRISH bor. Raqam ro'yxatda bo'lmasa
/// yoki roli `waiter` bo'lmasa — aniq xabar ko'rsatiladi.
/// └───────────────────────────────────────────────────────────────────┘
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

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _requestCode() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final devCode = await api.requestCode(_phone.text.trim());
      if (!mounted) return;
      setState(() {
        _codeSent = true;
        // Dev rejimda server kodni javobda qaytaradi — qo'lda
        // yozishning hojati yo'q.
        if (devCode != null) _code.text = devCode;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final res = await api.verify(_phone.text.trim(), _code.text.trim());
      final user = Map<String, dynamic>.from(res['user'] as Map);
      final role = user['role'] as String? ?? '';
      final entityId = user['entity_id'] as String? ?? '';

      // ┌─ ROL TEKSHIRUVI ─────────────────────────────────────────┐
      // Token muvaffaqiyatli olindi, lekin bu odam affitsiant
      // bo'lmasligi mumkin (masalan oddiy mijoz o'z raqami bilan
      // kirdi). Server baribir `/waiter/*` endpointlariga 403
      // beradi, lekin foydalanuvchi buni tushunarsiz xato sifatida
      // ko'rardi. Shuning uchun sabab SHU YERDA aytiladi.
      //
      // Tokenni SAQLAMAYMIZ ham — aks holda ilova keyingi ochilishda
      // shu yaroqsiz sessiya bilan urinaverardi.
      // └───────────────────────────────────────────────────────────┘
      if (role != 'waiter' || entityId.isEmpty) {
        api.token = null;
        if (mounted) {
          setState(() => _error =
              'Bu raqam affitsiant sifatida ro\'yxatdan o\'tmagan.\n'
              'Restoran ma\'muriyatiga murojaat qiling.');
        }
        return;
      }

      await tokenStore.write(res['token'] as String);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const WaiterShell()),
      );
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Ilova logotipi (`image/affitsiant.png` dan). Ilova
                // belgisi (launcher icon) ham AYNAN shu fayldan
                // generatsiya qilinadi — telefondagi belgi va ekrandagi
                // logotip bir xil ko'rinadi.
                Image.asset(
                  'assets/logo.png',
                  height: 96,
                  // Rasm topilmasa ilova YIQILMASLIGI kerak: kirish
                  // ekrani — foydalanuvchi ko'radigan birinchi narsa.
                  errorBuilder: (_, _, _) =>
                      const Icon(Icons.room_service, size: 64, color: kBrandColor),
                ),
                const SizedBox(height: 16),
                const Text(
                  'OnDex Affitsiant',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 32),
                TextField(
                  controller: _phone,
                  enabled: !_codeSent,
                  keyboardType: TextInputType.phone,
                  decoration: const InputDecoration(
                    labelText: 'Telefon raqam',
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
                if (_error != null) ...[
                  const SizedBox(height: 16),
                  Text(
                    _error!,
                    style: const TextStyle(color: Colors.redAccent),
                    textAlign: TextAlign.center,
                  ),
                ],
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: _busy ? null : (_codeSent ? _verify : _requestCode),
                  style: FilledButton.styleFrom(
                    backgroundColor: kBrandColor,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                  ),
                  child: _busy
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(_codeSent ? 'Kirish' : 'Kod olish'),
                ),
                if (_codeSent)
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () => setState(() {
                              _codeSent = false;
                              _code.clear();
                              _error = null;
                            }),
                    child: const Text('Raqamni o\'zgartirish'),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
