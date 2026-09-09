import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../widgets/delete_account_action.dart';

class CouriersPage extends StatefulWidget {
  const CouriersPage({super.key});

  @override
  State<CouriersPage> createState() => _CouriersPageState();
}

class _CouriersPageState extends State<CouriersPage> {
  List<dynamic> _list = [];
  bool _loading = true;
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    // Yangi ariza (`courier_registered`) va onlayn holati
    // (`courier_status`) JONLI keladi — tasdiq kutayotgan kuryer
    // ro'yxatda darhol paydo bo'ladi. So'rov sikli zaxira sifatida
    // qoladi (`live.dart`).
    _live = LiveRefresher(
      bus: adminLive,
      onRefresh: _load,
      types: const {'courier_registered', 'courier_status', 'courier_assigned'},
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  /// Kuryer -> uning akkaunti (PostHog dagi "odam"). Izohi
  /// `restaurants_page.dart` dagi `_posthogButton` da.
  Map<String, ({String userId, String phone, String name})> _accounts = {};

  Future<void> _load() async {
    try {
      final l = await api.couriers();
      // Xatosi ro'yxatni yiqitmaydi — faqat PostHog tugmasi uchun.
      final acc = await api.accountsByEntity('courier')
          .catchError((_) => <String, ({String userId, String phone, String name})>{});
      if (!mounted) return;
      setState(() {
        _list = l;
        _accounts = acc;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
    }
  }

  /// Kuryer ilovada nima qilgani — seans yozuvi bilan (ilova Android,
  /// ya'ni bu yerda haqiqiy video kabi qayta ko'rish mumkin).
  ///
  /// Izohi `people_page.dart` dagi `_posthogButton` va
  /// `restaurants_page.dart` dagi o'xshash funksiya bilan bir xil.
  ///
  /// NIMA UCHUN IKKALA HOLATDA HAM ICON KO'RINADI:
  ///   * Yashil/Ko'k icon → PostHog ochilgan va havola mavjud
  ///   * Kulrang icon → nima sababdan ishlamasligini SABABI bilan
  ///     (tooltip + snackbar). Avvalgi versiyada bu yerda
  ///     `SizedBox.shrink()` qaytarilgani sababli icon UMUMAN
  ///     ko'rinmaydi va admin "nima uchun yo'q?" degan savol
  ///     doim paydo bo'lardi.
  Widget _posthogButton(Map<String, dynamic> c) {
    final cId = (c['id']?.toString() ?? '').trim();
    final rec = _accounts[cId];
    final userId = rec?.userId ?? '';
    final url = posthogPersonUrl(userId);
    if (url.isEmpty) {
      String reason;
      if (!posthogEnabled) {
        reason = 'Admin panel PostHog kaliti yo\'q (build config)';
      } else if (posthogProjectId.isEmpty) {
        reason = 'PostHog loyiha raqami yo\'q';
      } else if (rec == null) {
        reason = 'Bu kuryerga kirish akkaunti biriktirilmagan'
            ' (users jadvalida entity_id="$cId" bo\'lgan user yo\'q)';
      } else if (userId.isEmpty) {
        reason = 'Kuryer akkauntining user_id si bo\'sh';
      } else {
        reason = '';
      }
      if (reason.isEmpty) return const SizedBox.shrink();
      return IconButton(
        tooltip: 'PostHog yoqilmagan: $reason',
        icon: Icon(Icons.play_circle_outline, color: Colors.grey.shade400),
        onPressed: () => ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('PostHog: $reason')),
        ),
      );
    }
    return IconButton(
      tooltip: 'Ilovada nima qilgani (PostHog)\n\nEslatma: "Person not found" '
          'chiqsa — bu kuryer hali mobil ilovaga PRODUCTION rejimda '
          'hech qachon kirmagan demak.',
      icon: const Icon(Icons.play_circle_outline, color: Color(0xFF1D4AFF)),
      onPressed: () async {
        final ok = await openLegalUrl(url);
        if (!ok && mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Havola ochilmadi')),
          );
        }
      },
    );
  }

  Future<void> _setApproved(String id, bool approved) async {
    try {
      await api.approveCourier(id, approved);
      _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(approved
              ? 'Kuryer tasdiqlandi — endi ishlashi mumkin'
              : 'Kuryer bloklandi')));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Xato: $e')));
    }
  }

  /// Kuryer akkauntini o'chiradi.
  ///
  /// So'rov AKKAUNT ID'siga (`user_id`) yuboriladi, kuryer yozuvining
  /// ID'siga emas: o'chirish akkauntga tegishli amal va server o'zi
  /// kuryer yozuvini ham ro'yxatlardan olib tashlaydi.
  Future<void> _delete(Map<String, dynamic> c) async {
    final deleted = await confirmDeleteAccount(
      context,
      id: c['user_id'] as String,
      name: (c['name'] as String?)?.isNotEmpty == true
          ? c['name'] as String
          : '(ismsiz kuryer)',
      phone: (c['phone'] as String?) ?? '',
      extraNote: 'Kuryer ro\'yxatidan ham chiqariladi va boshqa '
          'buyurtma taklifi olmaydi.',
    );
    if (!mounted) return;
    if (deleted) _load();
  }

  @override
  Widget build(BuildContext context) {
    final pending =
        _list.where((c) => c['approved'] != true).length;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('Kuryerlar',
                  style: Theme.of(context).textTheme.headlineMedium),
              const SizedBox(width: 12),
              if (pending > 0)
                Chip(
                  label: Text('$pending ta tasdiq kutmoqda'),
                  backgroundColor: Colors.red.shade100,
                ),
              const Spacer(),
              IconButton(onPressed: _load, icon: const Icon(Icons.refresh)),
            ],
          ),
          const SizedBox(height: 8),
          Text(
              'Kuryerlar ilovada o\'zi ro\'yxatdan o\'tadi, lekin siz tasdiqlamaguningizcha ishlay olmaydi.',
              style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 16),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _list.isEmpty
                    ? const Center(child: Text('Hozircha kuryer yo\'q'))
                    : SingleChildScrollView(
                        child: SizedBox(
                          width: double.infinity,
                          child: DataTable(
                            columns: const [
                              DataColumn(label: Text('Ismi')),
                              DataColumn(label: Text('Telefon')),
                              DataColumn(label: Text('Holat')),
                              DataColumn(label: Text('Online')),
                              DataColumn(label: Text('Amal')),
                            ],
                            rows: [
                              for (final c in _list.cast<Map<String, dynamic>>())
                                DataRow(cells: [
                                  DataCell(Text(c['name'] ?? '')),
                                  DataCell(Text(c['phone'] ?? '—')),
                                  DataCell(c['approved'] == true
                                      ? const Chip(
                                          label: Text('Tasdiqlangan'),
                                          backgroundColor:
                                              Color(0xFFD0F0D8))
                                      : const Chip(
                                          label: Text('Kutilmoqda'),
                                          backgroundColor:
                                              Color(0xFFFFE0B2))),
                                  DataCell(Icon(
                                    c['available'] == true
                                        ? Icons.circle
                                        : Icons.circle_outlined,
                                    size: 14,
                                    color: c['available'] == true
                                        ? Colors.green
                                        : Colors.grey,
                                  )),
                                  DataCell(Row(
                                    mainAxisSize: MainAxisSize.min,
                                    children: [
                                      _posthogButton(c),
                                      c['approved'] == true
                                          ? TextButton(
                                              onPressed: () =>
                                                  _setApproved(c['id'], false),
                                              child: const Text('Bloklash',
                                                  style: TextStyle(
                                                      color: Colors.red)))
                                          : FilledButton(
                                              onPressed: () =>
                                                  _setApproved(c['id'], true),
                                              child: const Text('Tasdiqlash')),
                                      const SizedBox(width: 4),
                                      // `user_id` bo'lmasligi mumkin: kuryer
                                      // yozuvi bor, unga bog'langan akkaunt
                                      // esa yo'q (eski demo ma'lumot).
                                      // Bunday holatda tugma o'chirilgan
                                      // holatda qoladi — bosilganda
                                      // tushunarsiz xato berishdan ko'ra
                                      // sababni tooltip'da aytgan ma'qul.
                                      IconButton(
                                        tooltip: c['user_id'] == null
                                            ? 'Bu kuryerga bog\'langan akkaunt yo\'q'
                                            : 'Akkauntni o\'chirish',
                                        icon: Icon(Icons.delete_outline,
                                            color: c['user_id'] == null
                                                ? Colors.grey
                                                : Colors.red),
                                        onPressed: c['user_id'] == null
                                            ? null
                                            : () => _delete(c),
                                      ),
                                    ],
                                  )),
                                ]),
                            ],
                          ),
                        ),
                      ),
          ),
        ],
      ),
    );
  }
}
