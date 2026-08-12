import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';

/// Affitsiantlar — restoran o'z xodimlarini o'zi boshqaradi.
///
/// ┌─ NEGA RESTORAN, ADMIN EMAS ───────────────────────────────────────┐
/// Foydalanuvchi qarori (2026-08-12): har bir yangi xodim uchun
/// superadminga murojaat qilish ish jarayonini sekinlashtiradi.
/// Restoran o'z zalini o'zi biladi.
///
/// Xavfsizlik chegarasi saqlanadi: restoran FAQAT o'z restoraniga
/// affitsiant qo'sha oladi (`EntityID` tokendan olinadi, so'rov
/// tanasidan emas) va allaqachon mavjud raqamga TEGA OLMAYDI —
/// aks holda begona akkauntni o'z xodimiga aylantirib yuborardi.
/// └───────────────────────────────────────────────────────────────────┘
class StaffPage extends StatefulWidget {
  const StaffPage({super.key});

  @override
  State<StaffPage> createState() => _StaffPageState();
}

class _StaffPageState extends State<StaffPage> {
  List<Map<String, dynamic>> _waiters = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await api.waiters();
      if (!mounted) return;
      setState(() {
        _waiters = list.cast<Map<String, dynamic>>();
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '$e';
      });
    }
  }

  Future<void> _add() async {
    final phoneCtrl = TextEditingController(text: '+998');
    final nameCtrl = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Affitsiant qo\'shish'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: nameCtrl,
              autofocus: true,
              decoration: const InputDecoration(labelText: 'Ism'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: phoneCtrl,
              keyboardType: TextInputType.phone,
              decoration: const InputDecoration(
                labelText: 'Telefon raqam',
                hintText: '+998901234567',
              ),
            ),
            const SizedBox(height: 12),
            const Text(
              'Xodim shu raqam bilan "OnDex Affitsiant" ilovasiga '
              'SMS kod orqali kiradi. Parol kerak emas.',
              style: TextStyle(color: Colors.grey, fontSize: 12),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Bekor qilish'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Qo\'shish'),
          ),
        ],
      ),
    );
    final phone = phoneCtrl.text.trim();
    final name = nameCtrl.text.trim();
    phoneCtrl.dispose();
    nameCtrl.dispose();
    if (ok != true || phone.isEmpty || name.isEmpty) return;

    try {
      await api.addWaiter(phone, name);
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  Future<void> _remove(Map<String, dynamic> w) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Ishdan bo\'shatish'),
        content: Text(
          '${w['name']} affitsiantlar ro\'yxatidan chiqariladi va '
          'ilovaga kirish darhol yopiladi.\n\n'
          'Uning shaxsiy akkaunti O\'CHIRILMAYDI — u oddiy mijoz '
          'sifatida qolaveradi.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Bekor qilish'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Bo\'shatish'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await api.removeWaiter(w['id'] as String);
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Text(
              'Affitsiantlar',
              style: TextStyle(fontSize: 22, fontWeight: FontWeight.bold),
            ),
            const Spacer(),
            FilledButton.icon(
              onPressed: _add,
              icon: const Icon(Icons.person_add),
              label: const Text('Affitsiant qo\'shish'),
            ),
          ],
        ),
        const SizedBox(height: 6),
        const Text(
          'Affitsiant "OnDex Affitsiant" ilovasida stol buyurtmalarini '
          'ko\'radi va tayyor bo\'lganda xabar oladi.',
          style: TextStyle(color: Colors.grey),
        ),
        const SizedBox(height: 16),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Text(_error!, style: const TextStyle(color: Colors.red)),
          ),
        Expanded(
          child: _waiters.isEmpty
              ? const Center(
                  child: Text(
                    'Hali affitsiant qo\'shilmagan',
                    style: TextStyle(color: Colors.grey),
                  ),
                )
              : ListView.separated(
                  itemCount: _waiters.length,
                  separatorBuilder: (_, __) => const Divider(height: 1),
                  itemBuilder: (_, i) {
                    final w = _waiters[i];
                    return ListTile(
                      leading: const CircleAvatar(
                        child: Icon(Icons.room_service),
                      ),
                      title: Text(w['name'] as String? ?? '—'),
                      subtitle: Text(w['phone'] as String? ?? ''),
                      trailing: IconButton(
                        icon: const Icon(Icons.person_remove),
                        color: OnDexColors.danger,
                        tooltip: 'Ishdan bo\'shatish',
                        onPressed: () => _remove(w),
                      ),
                    );
                  },
                ),
        ),
      ],
    );
  }
}
