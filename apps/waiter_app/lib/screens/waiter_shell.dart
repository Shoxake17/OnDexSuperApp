import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../push.dart';
import '../session.dart';
import '../theme.dart';
import 'login_screen.dart';

/// Affitsiantning asosiy ekrani — stol buyurtmalari jonli ro'yxati.
///
/// ┌─ IKKI QATLAMLI YANGILANISH ───────────────────────────────────────┐
/// 1. WebSocket — darhol (odatiy holat);
/// 2. 20 soniyalik polling — ZAXIRA.
///
/// Ikkinchisi shart: WebSocket uzilib qolsa (tunnel, Wi-Fi almashuvi,
/// telefon uxlashi) affitsiant buni SEZMASDAN eskirgan ro'yxatga qarab
/// turardi va taom sovib qolardi. `WsClient` qayta ulanadi, lekin
/// backoff 2 daqiqagacha o'sadi — o'sha oraliqda polling qoplaydi.
/// └───────────────────────────────────────────────────────────────────┘
class WaiterShell extends StatefulWidget {
  const WaiterShell({super.key});

  @override
  State<WaiterShell> createState() => _WaiterShellState();
}

class _WaiterShellState extends State<WaiterShell> {
  List<Map<String, dynamic>> _orders = [];
  String _restaurantName = '';
  bool _loading = true;
  bool _online = false;
  String? _error;
  // Hozir server bilan gaplashayotgan buyurtmalar — tugma ikki marta
  // bosilmasligi uchun.
  final Set<String> _busy = {};

  WsClient? _ws;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _bootstrap();
  }

  @override
  void dispose() {
    _ws?.dispose();
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _bootstrap() async {
    // 401 kelsa — sessiya tugagan, login ekraniga.
    api.onUnauthorized = _forceLogout;
    try {
      final me = await api.waiterMe();
      _restaurantName = me['restaurant_name'] as String? ?? '';
    } catch (_) {
      // Restoran nomi — bezak, usiz ham ishlayveradi.
    }
    await _load();
    _connectWs();
    _poll = Timer.periodic(const Duration(seconds: 20), (_) => _load());
    // Push ruxsati va tokeni. Ilova ochiq bo'lmaganda xabar aynan shu
    // yo'l bilan yetadi.
    await registerPush();
  }

  Future<void> _load() async {
    try {
      final list = await api.orders();
      if (!mounted) return;
      setState(() {
        _orders = list;
        _loading = false;
        _error = null;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      // 401 ni `onUnauthorized` allaqachon ushlaydi.
      if (e.isUnauthorized) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    }
  }

  void _connectWs() {
    _ws = WsClient(
      ticketProvider: api.wsTicket,
      urlBuilder: (t) => wsUrl(t),
      onStateChange: (c) {
        if (mounted) setState(() => _online = c);
      },
      onEvent: (event) {
        final type = event['type'] as String?;
        // Har qanday buyurtma hodisasi ro'yxatni yangilaydi. Hodisa
        // ichidagi ma'lumotga TAYANMAYMIZ — server javobi yagona
        // haqiqat manbai bo'lib qoladi (aks holda ikki manba
        // ajralib ketardi).
        if (type == 'new_order' || type == 'order_status') {
          _load();
          if (event['status'] == 'ready') playReadySound();
        }
      },
    )..connect();
  }

  Future<void> _markServed(String id) async {
    setState(() => _busy.add(id));
    try {
      await api.markServed(id);
      await _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.message)),
        );
      }
    } finally {
      if (mounted) setState(() => _busy.remove(id));
    }
  }

  Future<void> _forceLogout() async {
    await unregisterPush();
    await api.logout();
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (_) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    // Tayyor buyurtmalar TEPADA: affitsiantning yagona shoshilinch ishi
    // shu. Qolganlari yaratilish tartibida.
    final ready = _orders.where((o) => o['status'] == 'ready').toList();
    final others = _orders.where((o) => o['status'] != 'ready').toList();

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Stol buyurtmalari', style: TextStyle(fontSize: 18)),
            if (_restaurantName.isNotEmpty)
              Text(
                _restaurantName,
                style: const TextStyle(fontSize: 12, color: Colors.white54),
              ),
          ],
        ),
        actions: [
          // Ulanish holati — affitsiant ro'yxat "tirik"ligini bilishi
          // kerak. Busiz uzilgan ulanish jimgina eskirgan ma'lumot
          // ko'rsatib turardi.
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: Icon(
              _online ? Icons.wifi : Icons.wifi_off,
              size: 18,
              color: _online ? kReadyColor : Colors.orangeAccent,
            ),
          ),
          IconButton(
            icon: const Icon(Icons.logout),
            onPressed: _forceLogout,
            tooltip: 'Chiqish',
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _buildBody(ready, others),
      ),
    );
  }

  Widget _buildBody(
    List<Map<String, dynamic>> ready,
    List<Map<String, dynamic>> others,
  ) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null && _orders.isEmpty) {
      return ListView(
        children: [
          const SizedBox(height: 120),
          Center(child: Text(_error!, textAlign: TextAlign.center)),
          const SizedBox(height: 16),
          Center(
            child: FilledButton(onPressed: _load, child: const Text('Qaytadan')),
          ),
        ],
      );
    }
    if (_orders.isEmpty) {
      // `ListView` ATAYLAB (Center emas): `RefreshIndicator` faqat
      // suriladigan vidjet ustida ishlaydi, aks holda bo'sh ekranda
      // qo'lda yangilash umuman mumkin bo'lmasdi.
      return ListView(
        children: const [
          SizedBox(height: 140),
          Icon(Icons.check_circle_outline, size: 56, color: Colors.white24),
          SizedBox(height: 12),
          Center(
            child: Text(
              'Hozircha faol buyurtma yo\'q',
              style: TextStyle(color: Colors.white54),
            ),
          ),
        ],
      );
    }
    return ListView(
      padding: const EdgeInsets.all(12),
      children: [
        for (final o in ready) _OrderCard(
              order: o,
              busy: _busy.contains(o['id']),
              onServed: () => _markServed(o['id'] as String),
            ),
        if (ready.isNotEmpty && others.isNotEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 12),
            child: Text('Tayyorlanmoqda', style: TextStyle(color: Colors.white38)),
          ),
        for (final o in others) _OrderCard(order: o, busy: false),
      ],
    );
  }
}

/// Bitta buyurtma kartochkasi.
class _OrderCard extends StatelessWidget {
  const _OrderCard({required this.order, required this.busy, this.onServed});

  final Map<String, dynamic> order;
  final bool busy;
  final VoidCallback? onServed;

  @override
  Widget build(BuildContext context) {
    final status = order['status'] as String? ?? '';
    final isReady = status == 'ready';
    final table = (order['table_label'] as String?) ?? '—';
    final party = order['party_size'] as int? ?? 0;
    final items = (order['items'] as List?) ?? const [];
    final total = order['total_tiyin'] as int? ?? 0;

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      color: isReady ? kReadyColor.withValues(alpha: 0.18) : null,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(
          color: isReady ? kReadyColor : Colors.white12,
          width: isReady ? 2 : 1,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                  decoration: BoxDecoration(
                    color: isReady ? kReadyColor : Colors.white10,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Text(
                    '$table-stol',
                    style: const TextStyle(
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                if (party > 0)
                  Row(
                    children: [
                      const Icon(Icons.people_outline, size: 16),
                      const SizedBox(width: 4),
                      Text('$party kishi'),
                    ],
                  ),
                const Spacer(),
                Text(
                  _statusText(status),
                  style: TextStyle(
                    color: isReady ? kReadyColor : Colors.white54,
                    fontWeight: isReady ? FontWeight.bold : FontWeight.normal,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            for (final it in items)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Row(
                  children: [
                    Text(
                      '${(it as Map)['qty']}×',
                      style: const TextStyle(color: Colors.white54),
                    ),
                    const SizedBox(width: 8),
                    Expanded(child: Text('${it['name']}')),
                  ],
                ),
              ),
            const Divider(height: 20),
            Row(
              children: [
                Text(
                  formatSum(total),
                  style: const TextStyle(fontWeight: FontWeight.bold),
                ),
                const Spacer(),
                if (isReady)
                  FilledButton.icon(
                    onPressed: busy ? null : onServed,
                    style: FilledButton.styleFrom(backgroundColor: kReadyColor),
                    icon: busy
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.done),
                    label: const Text('Berildi'),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  static String _statusText(String s) => switch (s) {
        'created' => 'Yangi',
        'accepted' => 'Qabul qilindi',
        'preparing' => 'Tayyorlanmoqda',
        'ready' => 'TAYYOR',
        _ => s,
      };
}
