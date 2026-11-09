import 'package:flutter/material.dart';

import '../api.dart';

/// Qurilmalar (Plane 137): akkauntga kirilgan barcha qurilmalar. Begona
/// qurilmani shu yerdan chiqarish mumkin — o'sha qurilma darhol chiqib
/// ketadi. Ma'lumot faqat `Text` orqali ko'rsatiladi.
class DevicesScreen extends StatefulWidget {
  const DevicesScreen({super.key});

  @override
  State<DevicesScreen> createState() => _DevicesScreenState();
}

class _DevicesScreenState extends State<DevicesScreen> {
  List<Map<String, dynamic>>? _items;
  String? _error;
  String? _busyId;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final list = await api.mySessions();
      if (mounted) setState(() => _items = list);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = 'Serverga ulanib bo\'lmadi');
    }
  }

  Future<void> _revoke(Map<String, dynamic> s) async {
    final ok = await showDialog<bool>(
      context: context,
      builder:
          (ctx) => AlertDialog(
            title: const Text('Qurilmani chiqarish'),
            content: Text('"${_title(s)}" qurilmasi akkauntdan chiqariladi.'),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('Bekor'),
              ),
              FilledButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('Chiqarish'),
              ),
            ],
          ),
    );
    if (ok != true) return;
    setState(() => _busyId = s['id'] as String?);
    try {
      await api.revokeSession(s['id'] as String);
      await _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.message)));
      }
    } finally {
      if (mounted) setState(() => _busyId = null);
    }
  }

  static String _title(Map<String, dynamic> s) {
    final dev = (s['device_name'] as String?)?.trim() ?? '';
    if (dev.isNotEmpty) return dev;
    return s['client_id'] == 'ondex-customer' ? 'OnDex ilovasi' : 'Qurilma';
  }

  static String _when(Object? iso) {
    final t = DateTime.tryParse('$iso')?.toLocal();
    if (t == null) return '';
    String p(int n) => n.toString().padLeft(2, '0');
    return '${p(t.day)}.${p(t.month)}.${t.year} ${p(t.hour)}:${p(t.minute)}';
  }

  @override
  Widget build(BuildContext context) {
    final items = _items;
    return Scaffold(
      appBar: AppBar(title: const Text('Qurilmalar')),
      body: RefreshIndicator(
        onRefresh: _load,
        child:
            _error != null
                ? ListView(
                  children: [
                    Padding(
                      padding: const EdgeInsets.all(24),
                      child: Text(_error!, textAlign: TextAlign.center),
                    ),
                  ],
                )
                : items == null
                ? const Center(child: CircularProgressIndicator())
                : ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    const Text(
                      'Akkauntingizga kirilgan qurilmalar. Tanimagan '
                      'qurilmangizni chiqaring va parolni almashtiring.',
                    ),
                    const SizedBox(height: 12),
                    for (final s in items)
                      Card(
                        child: ListTile(
                          leading: const Icon(Icons.smartphone),
                          title: Text(_title(s)),
                          subtitle: Text(
                            '${s['current'] == true ? 'Shu qurilma · ' : ''}'
                            'oxirgi faollik: ${_when(s['last_used_at'])}',
                          ),
                          trailing:
                              s['current'] == true
                                  ? null
                                  : _busyId == s['id']
                                  ? const SizedBox(
                                    width: 20,
                                    height: 20,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                  : TextButton(
                                    onPressed: () => _revoke(s),
                                    child: const Text('Chiqarish'),
                                  ),
                        ),
                      ),
                  ],
                ),
      ),
    );
  }
}
