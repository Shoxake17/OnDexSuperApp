import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';

class CouriersPage extends StatefulWidget {
  const CouriersPage({super.key});

  @override
  State<CouriersPage> createState() => _CouriersPageState();
}

class _CouriersPageState extends State<CouriersPage> {
  List<dynamic> _list = [];
  bool _loading = true;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 10), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final l = await api.couriers();
      if (!mounted) return;
      setState(() {
        _list = l;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
    }
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
                                  DataCell(c['approved'] == true
                                      ? TextButton(
                                          onPressed: () =>
                                              _setApproved(c['id'], false),
                                          child: const Text('Bloklash',
                                              style: TextStyle(
                                                  color: Colors.red)))
                                      : FilledButton(
                                          onPressed: () =>
                                              _setApproved(c['id'], true),
                                          child: const Text('Tasdiqlash'))),
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
