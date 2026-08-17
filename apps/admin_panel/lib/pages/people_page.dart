import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/delete_account_action.dart';

/// Mijozlar va affitsiantlar sahifalari.
///
/// ┌─ NEGA BITTA FAYLDA ───────────────────────────────────────────────┐
/// Ikkala sahifa ham bir xil ishni qiladi: ro'yxatni oladi, qidiruv
/// bilan filtrlaydi, jadval chizadi va akkauntni o'chiradi. Ular
/// alohida yozilganda har bir tuzatishni (masalan o'chirish
/// dialogidagi matn yoki 409 xatosini ko'rsatish) IKKI joyda qilish
/// kerak bo'lardi — loyihada bu xato allaqachon bir necha marta
/// takrorlangan (`ondex_core` paketining paydo bo'lish sababi).
///
/// Farqi atigi ikki narsada: sarlavha va qo'shimcha ustun (affitsiant
/// qaysi restoranga biriktirilgan). Shu ikkisi parametr sifatida
/// beriladi.
/// └───────────────────────────────────────────────────────────────────┘

class CustomersPage extends StatelessWidget {
  const CustomersPage({super.key});

  @override
  Widget build(BuildContext context) => _PeopleView(
        title: 'Mijozlar',
        emptyText: 'Hozircha mijoz yo\'q',
        hint: 'Mijozlar ilovadan yoki Telegram Mini App orqali o\'zi '
            'ro\'yxatdan o\'tadi. "Qurilma" ustuni — oxirgi marta qaysi '
            'dasturdan kirgani.',
        load: api.customers,
      );
}

class WaitersPage extends StatelessWidget {
  const WaitersPage({super.key});

  @override
  Widget build(BuildContext context) => _PeopleView(
        title: 'Affitsiantlar',
        emptyText: 'Hozircha affitsiant yo\'q',
        hint: 'Affitsiant akkauntini restoran o\'z panelidan yaratadi. '
            'U panelga shu telefon raqami orqali (Telegram kodi bilan) '
            'kiradi.',
        showRestaurant: true,
        load: api.waiters,
      );
}

/// Server qaytargan platforma kodini odam o'qiydigan nomga aylantiradi.
///
/// Notanish qiymat YASHIRILMAYDI, o'zi ko'rsatiladi: server yopiq
/// ro'yxatdan tashqarisini `unknown` ga aylantiradi, ya'ni bu yerda
/// begona matn paydo bo'lmaydi, lekin yangi platforma qo'shilib bu
/// ro'yxat yangilanmasa — admin buni darhol ko'radi.
String _platformLabel(String code) => switch (code) {
      'tma' => 'Telegram Mini App',
      'android' => 'Android ilova',
      'ios' => 'iOS ilova',
      'web' => 'Brauzer',
      'windows' => 'Windows panel',
      'macos' => 'macOS panel',
      'linux' => 'Linux panel',
      'unknown' => 'Noma\'lum',
      _ => code,
    };

Color _platformColor(String code) => switch (code) {
      'tma' => const Color(0xFFD6ECFF),
      'android' || 'ios' => const Color(0xFFD0F0D8),
      'web' => const Color(0xFFEDE1FF),
      _ => const Color(0xFFEEEEEE),
    };

String _formatDate(String? iso) {
  if (iso == null || iso.isEmpty) return '—';
  final d = DateTime.tryParse(iso)?.toLocal();
  if (d == null) return '—';
  String two(int v) => v.toString().padLeft(2, '0');
  return '${two(d.day)}.${two(d.month)}.${d.year}';
}

class _PeopleView extends StatefulWidget {
  const _PeopleView({
    required this.title,
    required this.emptyText,
    required this.hint,
    required this.load,
    this.showRestaurant = false,
  });

  final String title;
  final String emptyText;
  final String hint;
  final bool showRestaurant;
  final Future<Map<String, dynamic>> Function() load;

  @override
  State<_PeopleView> createState() => _PeopleViewState();
}

class _PeopleViewState extends State<_PeopleView> {
  List<Map<String, dynamic>> _all = [];
  bool _loading = true;
  String? _error;
  String _query = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final d = await widget.load();
      if (!mounted) return;
      setState(() {
        _all = (d['items'] as List<dynamic>? ?? [])
            .map((e) => Map<String, dynamic>.from(e as Map))
            .toList();
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      // Xato YUTILMAYDI. Avvalgi sahifalarda (`couriers_page`) xato
      // jimgina bo'sh ro'yxatga aylanardi va admin "hech kim yo'q"
      // deb o'ylardi — aslida server javob bermayotgan bo'lardi.
      setState(() {
        _loading = false;
        _error = e.toString();
      });
    }
  }

  /// Qidiruv: ism, telefon va email bo'yicha.
  List<Map<String, dynamic>> get _filtered {
    if (_query.trim().isEmpty) return _all;
    final q = _query.trim().toLowerCase();
    return _all.where((p) {
      final haystack = [
        p['name'], p['first_name'], p['last_name'],
        p['phone'], p['email'], p['restaurant_name'],
      ].whereType<String>().join(' ').toLowerCase();
      return haystack.contains(q);
    }).toList();
  }

  Future<void> _confirmDelete(Map<String, dynamic> person) async {
    final deleted = await confirmDeleteAccount(
      context,
      id: person['id'] as String,
      name: _displayName(person),
      phone: (person['phone'] as String?) ?? '',
    );
    if (deleted && mounted) _load();
  }

  String _displayName(Map<String, dynamic> p) {
    final name = (p['name'] as String?)?.trim() ?? '';
    if (name.isNotEmpty) return name;
    final first = (p['first_name'] as String?)?.trim() ?? '';
    final last = (p['last_name'] as String?)?.trim() ?? '';
    final full = '$first $last'.trim();
    return full.isEmpty ? '(ism kiritilmagan)' : full;
  }

  @override
  Widget build(BuildContext context) {
    final rows = _filtered;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(widget.title,
                  style: Theme.of(context).textTheme.headlineMedium),
              const SizedBox(width: 12),
              Chip(label: Text('Jami: ${_all.length}')),
              const Spacer(),
              SizedBox(
                width: 280,
                child: TextField(
                  decoration: const InputDecoration(
                    isDense: true,
                    prefixIcon: Icon(Icons.search),
                    hintText: 'Ism yoki telefon bo\'yicha qidirish',
                    border: OutlineInputBorder(),
                  ),
                  onChanged: (v) => setState(() => _query = v),
                ),
              ),
              IconButton(onPressed: _load, icon: const Icon(Icons.refresh)),
            ],
          ),
          const SizedBox(height: 8),
          Text(widget.hint, style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 16),
          Expanded(child: _body(rows)),
        ],
      ),
    );
  }

  Widget _body(List<Map<String, dynamic>> rows) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, color: Colors.red, size: 40),
            const SizedBox(height: 8),
            Text('Ro\'yxatni yuklab bo\'lmadi: $_error'),
            const SizedBox(height: 8),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }
    if (rows.isEmpty) {
      return Center(
          child: Text(_query.isEmpty ? widget.emptyText : 'Topilmadi'));
    }
    return SingleChildScrollView(
      child: SizedBox(
        width: double.infinity,
        child: DataTable(
          columns: [
            const DataColumn(label: Text('Ism familiya')),
            const DataColumn(label: Text('Telefon')),
            if (widget.showRestaurant) const DataColumn(label: Text('Restoran')),
            const DataColumn(label: Text('Qurilma / ilova')),
            const DataColumn(label: Text('Qo\'shilgan')),
            const DataColumn(label: Text('Amal')),
          ],
          rows: [
            for (final p in rows)
              DataRow(cells: [
                DataCell(Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(_displayName(p)),
                    if (p['telegram_linked'] == true) ...[
                      const SizedBox(width: 6),
                      const Tooltip(
                        message: 'Telegram bilan bog\'langan',
                        child: Icon(Icons.telegram, size: 16,
                            color: Color(0xFF2AABEE)),
                      ),
                    ],
                  ],
                )),
                DataCell(Text((p['phone'] as String?)?.isNotEmpty == true
                    ? p['phone'] as String
                    : '—')),
                if (widget.showRestaurant)
                  DataCell(Text((p['restaurant_name'] as String?) ?? '—')),
                DataCell(_devices(p['devices'])),
                DataCell(Text(_formatDate(p['created_at'] as String?))),
                DataCell(IconButton(
                  tooltip: 'Akkauntni o\'chirish',
                  icon: const Icon(Icons.delete_outline, color: Colors.red),
                  onPressed: () => _confirmDelete(p),
                )),
              ]),
          ],
        ),
      ),
    );
  }

  /// Qurilma ustuni: har bir platforma uchun bitta chip + versiya.
  ///
  /// Bo'sh bo'lishi MUMKIN va bu normal: foydalanuvchi bu funksiya
  /// qo'shilgandan beri ilovaga umuman kirmagan bo'lsa, yozuv hali
  /// yaratilmagan.
  Widget _devices(dynamic raw) {
    final list = (raw as List<dynamic>? ?? [])
        .map((e) => Map<String, dynamic>.from(e as Map))
        .toList();
    if (list.isEmpty) {
      return const Text('—', style: TextStyle(color: Colors.black45));
    }
    return Wrap(
      spacing: 6,
      runSpacing: 4,
      children: [
        for (final d in list)
          Tooltip(
            message: 'Oxirgi faollik: ${_formatDate(d['last_seen'] as String?)}'
                '\nBirinchi marta: ${_formatDate(d['first_seen'] as String?)}',
            child: Chip(
              visualDensity: VisualDensity.compact,
              backgroundColor: _platformColor(d['platform'] as String? ?? ''),
              label: Text(
                '${_platformLabel(d['platform'] as String? ?? '')}'
                '${(d['app_version'] as String?)?.isNotEmpty == true
                    ? ' · ${d['app_version']}'
                    : ''}',
                style: const TextStyle(fontSize: 12),
              ),
            ),
          ),
      ],
    );
  }
}
