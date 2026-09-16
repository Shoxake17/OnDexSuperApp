import 'package:flutter/material.dart';

import '../api.dart';

/// "Akkauntni o'chirish" — profil ekranidan ochiladi.
///
/// Veb nusxasi (https://ondex.uz/delete-account) BILAN BIR XIL backend
/// (`POST /me/delete-account`) va bir xil qoida ishlatadi:
///
///   * MA'LUMOT O'CHIRILMAYDI — faqat kirish yopiladi. O'sha telefon
///     bilan qaytadan tasdiqlansa (SMS/Telegram/Google), akkaunt
///     avtomatik tiklanadi (`internal/users/service.go`).
///   * Akkauntda parol o'rnatilgan bo'lsa VA bu sessiya SMS kod bilan
///     YAQINDA (15 daq) tasdiqlanmagan bo'lsa — joriy parol so'raladi.
///     Bu ekran oldindan buni bilmaydi (server ham `/me` javobida
///     parol bor-yo'qligini oshkor qilmaydi — `User.PasswordHash`
///     `json:"-"`), shuning uchun avval PAROLSIZ urinadi va server
///     "joriy parol noto'g'ri" desagina parol maydonini ko'rsatadi.
///
/// Muvaffaqiyatli o'chirilsa `true` bilan qaytadi — chaqiruvchi
/// (`ProfileScreen`) shundan keyin ODATDAGI chiqish tozalashini
/// (`_doLogout`) bajaradi: server sessiyasi allaqachon bekor qilingan,
/// endi faqat qurilmadagi holat tozalanadi.
class DeleteAccountScreen extends StatefulWidget {
  const DeleteAccountScreen({super.key});

  @override
  State<DeleteAccountScreen> createState() => _DeleteAccountScreenState();
}

class _DeleteAccountScreenState extends State<DeleteAccountScreen> {
  static const _confirmWord = "O'CHIRISH";

  final _confirmCtrl = TextEditingController();
  final _passwordCtrl = TextEditingController();

  bool _needsPassword = false;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _confirmCtrl.dispose();
    _passwordCtrl.dispose();
    super.dispose();
  }

  bool get _ready =>
      _confirmCtrl.text.trim().toUpperCase() == _confirmWord.toUpperCase();

  Future<void> _submit() async {
    if (!_ready || _busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await api.deleteAccount(currentPassword: _passwordCtrl.text);
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        // Server matni AYNAN shu (`users.ErrCurrentPasswordWrong`) —
        // parol maydonini ENDI ko'rsatamiz va qayta urinishni so'raymiz.
        if (e.message.contains('joriy parol')) {
          _needsPassword = true;
          _error = 'Xavfsizlik uchun joriy parolingizni kiriting.';
        } else {
          _error = e.message;
        }
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = 'Tarmoq xatosi. Internet aloqasini tekshiring.';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Akkauntni o\'chirish')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: const Color(0xFFFEF2F2),
                borderRadius: BorderRadius.circular(14),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Icon(Icons.warning_amber_rounded,
                      color: Color(0xFFDC2626), size: 20),
                  const SizedBox(width: 10),
                  Expanded(
                    child: RichText(
                      text: const TextSpan(
                        style: TextStyle(
                            fontSize: 13, height: 1.5, color: Color(0xFFB91C1C)),
                        children: [
                          TextSpan(
                              text: 'Akkaunt o\'chirilgach:\n',
                              style: TextStyle(fontWeight: FontWeight.bold)),
                          TextSpan(
                              text: '• ushbu raqam bilan ilovaga kira olmaysiz;\n'),
                          TextSpan(
                              text: '• buyurtmalar tarixi, sevimlilar va manzil '
                                  'saqlanadi — o\'chirilmaydi;\n'),
                          TextSpan(
                              text: '• o\'sha telefon raqami bilan qaytadan '
                                  'tasdiqlansangiz, akkaunt avtomatik tiklanadi.'),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            const Text.rich(
              TextSpan(
                style: TextStyle(fontSize: 13.5),
                children: [
                  TextSpan(text: 'Tasdiqlash uchun '),
                  TextSpan(
                      text: _confirmWord,
                      style: TextStyle(fontWeight: FontWeight.bold)),
                  TextSpan(text: ' deb yozing'),
                ],
              ),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _confirmCtrl,
              autocorrect: false,
              decoration: const InputDecoration(
                hintText: _confirmWord,
                border: OutlineInputBorder(),
              ),
              onChanged: (_) => setState(() {}),
            ),
            if (_needsPassword) ...[
              const SizedBox(height: 16),
              const Text('Joriy parol', style: TextStyle(fontSize: 13.5)),
              const SizedBox(height: 8),
              TextField(
                controller: _passwordCtrl,
                obscureText: true,
                decoration: const InputDecoration(border: OutlineInputBorder()),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(_error!, style: const TextStyle(color: Color(0xFFDC2626))),
            ],
            const SizedBox(height: 20),
            SizedBox(
              height: 52,
              child: FilledButton.icon(
                onPressed: _ready && !_busy ? _submit : null,
                icon: _busy
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                            strokeWidth: 2, color: Colors.white),
                      )
                    : const Icon(Icons.delete_outline, size: 18),
                label: const Text('Ha, akkauntni o\'chirish'),
                style: FilledButton.styleFrom(
                  backgroundColor: const Color(0xFFDC2626),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(14)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
