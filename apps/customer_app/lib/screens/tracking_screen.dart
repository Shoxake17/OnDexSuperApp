import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../widgets/order_status.dart';
import 'catalog_screen.dart' show kBrand;

/// Buyurtma kuzatuvi — NATIVE (mobil oqimdagi OXIRGI WebView shu edi).
///
/// ┌─ JONLI YANGILANISH ───────────────────────────────────────────────┐
/// Holat WebSocket orqali keladi (`lib/live.dart` — butun ilova uchun
/// bitta kanal). Zaxira so'rov sikli `LiveRefresher` ichida: soket
/// ulangan bo'lsa siyrak, uzilgan bo'lsa tez-tez.
///
/// Qayta ulanish mantig'i BU YERDA YOZILMAGAN — u `ondex_core` da,
/// bitta joyda (eksponensial backoff + jitter bilan).
/// └───────────────────────────────────────────────────────────────────┘
class TrackingScreen extends StatefulWidget {
  final String orderId;

  const TrackingScreen({super.key, required this.orderId});

  @override
  State<TrackingScreen> createState() => _TrackingScreenState();
}

class _TrackingScreenState extends State<TrackingScreen> {
  Map<String, dynamic>? _order;
  bool _loading = true;
  String? _error;
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    customerLive.start();
    _live = LiveRefresher(
      bus: customerLive,
      onRefresh: _load,
      // Faqat SHU buyurtmaga tegishli hodisalar.
      types: const {'order_status', 'courier_assigned', 'dispatch_failed'},
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final o = await api.getOrder(widget.orderId);
      if (!mounted) return;
      setState(() {
        _order = o;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        // Buyurtma ALLAQACHON ko'rsatilgan bo'lsa uni o'chirmaymiz —
        // tarmoq uzilgani uchun mijoz holatini yo'qotmasligi kerak.
        if (_order == null) {
          _error = e is ApiException ? e.message : 'Buyurtmani ochib bo\'lmadi';
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final o = _order;

    return Scaffold(
      appBar: AppBar(
        title: Text(o == null
            ? 'Buyurtma'
            : '№ ${(o['order_number'] as String?) ?? ''}'),
      ),
      body: _loading && o == null
          ? const Center(child: CircularProgressIndicator())
          : _error != null && o == null
              ? _ErrorView(message: _error!, onRetry: _load)
              : RefreshIndicator(
                  color: kBrand,
                  onRefresh: _load,
                  child: ListView(
                    padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
                    children: [
                      _StatusHeader(order: o!),
                      const SizedBox(height: 22),
                      _Timeline(order: o),
                      const SizedBox(height: 22),
                      _ItemsCard(order: o),
                      const SizedBox(height: 14),
                      _WhereCard(order: o),
                    ],
                  ),
                ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════

class _StatusHeader extends StatelessWidget {
  final Map<String, dynamic> order;
  const _StatusHeader({required this.order});

  @override
  Widget build(BuildContext context) {
    final status = (order['status'] as String?) ?? '';
    final (label, icon, color) = statusStyleOf(status);

    return Row(
      children: [
        Container(
          width: 56,
          height: 56,
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.15),
            shape: BoxShape.circle,
          ),
          child: Icon(icon, color: color, size: 28),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label,
                  style: const TextStyle(
                      fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 2),
              Text(
                _hint(status),
                style: const TextStyle(fontSize: 13, color: Color(0xFF757575)),
              ),
            ],
          ),
        ),
      ],
    );
  }

  /// Har holat uchun mijoz NIMA KUTISHI kerakligi.
  ///
  /// Holat nomining o'zi yetarli emas: "Tayyor" — mijoz uchun nima
  /// degani? Shuning uchun har biriga kutish izohi qo'shiladi.
  static String _hint(String status) => switch (status) {
        'created' => 'Restoran buyurtmani ko\'rishini kutmoqdamiz',
        'accepted' => 'Restoran qabul qildi, tayyorlash boshlanadi',
        'preparing' => 'Taomingiz tayyorlanmoqda',
        'ready' => 'Tayyor — kuryer olib ketishini kutmoqda',
        'picked_up' => 'Kuryer yo\'lda',
        'delivered' => 'Yoqimli ishtaha!',
        'served' => 'Yoqimli ishtaha!',
        'rejected' => 'Restoran buyurtmani qabul qila olmadi',
        'cancelled' => 'Buyurtma bekor qilindi',
        _ => '',
      };
}

/// Bosqichlar chizig'i — buyurtma tarixidan quriladi.
class _Timeline extends StatelessWidget {
  final Map<String, dynamic> order;
  const _Timeline({required this.order});

  @override
  Widget build(BuildContext context) {
    final status = (order['status'] as String?) ?? '';
    final isDineIn = (order['type'] as String?) == 'dine_in';

    // Bekor qilingan/rad etilgan buyurtmada bosqichlar ma'nosiz.
    if (status == 'rejected' || status == 'cancelled') {
      return const SizedBox.shrink();
    }

    // Stolda "yo'lda" bosqichi YO'Q — afitsiant stolga olib keladi.
    final stages = isDineIn
        ? ['Qabul qilindi', 'Tayyorlanmoqda', 'Stolga berildi']
        : stageLabels;
    final reached = _reachedIndex(status, isDineIn);

    return Column(
      children: [
        for (var i = 0; i < stages.length; i++)
          _Step(
            label: stages[i],
            done: i <= reached,
            isLast: i == stages.length - 1,
            at: _timeOf(i, isDineIn),
          ),
      ],
    );
  }

  /// Joriy holat qaysi bosqichga to'g'ri kelishi.
  static int _reachedIndex(String status, bool dineIn) {
    if (dineIn) {
      return switch (status) {
        'accepted' => 0,
        'preparing' => 1,
        'ready' => 1,
        'served' => 2,
        _ => -1,
      };
    }
    return switch (status) {
      'accepted' => 0,
      'preparing' => 1,
      'ready' => 1,
      'picked_up' => 2,
      'delivered' => 3,
      _ => -1,
    };
  }

  /// Bosqich vaqti — buyurtma tarixidan.
  String? _timeOf(int stage, bool dineIn) {
    final history = (order['history'] as List?) ?? const [];
    final target = dineIn
        ? ['accepted', 'preparing', 'served'][stage.clamp(0, 2)]
        : ['accepted', 'preparing', 'picked_up', 'delivered'][stage.clamp(0, 3)];

    for (final h in history) {
      if (h is Map && h['to'] == target) {
        final at = DateTime.tryParse((h['at'] as String?) ?? '');
        if (at == null) return null;
        final l = at.toLocal();
        return '${l.hour.toString().padLeft(2, '0')}:'
            '${l.minute.toString().padLeft(2, '0')}';
      }
    }
    return null;
  }
}

class _Step extends StatelessWidget {
  final String label;
  final bool done;
  final bool isLast;
  final String? at;

  const _Step({
    required this.label,
    required this.done,
    required this.isLast,
    required this.at,
  });

  @override
  Widget build(BuildContext context) {
    final color = done ? kBrand : const Color(0xFFD5D5D5);
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Column(
            children: [
              Container(
                width: 18,
                height: 18,
                decoration: BoxDecoration(
                  color: done ? kBrand : Colors.white,
                  shape: BoxShape.circle,
                  border: Border.all(color: color, width: 2),
                ),
                child: done
                    ? const Icon(Icons.check, size: 11, color: Colors.white)
                    : null,
              ),
              if (!isLast)
                Expanded(
                  child: Container(width: 2, color: color),
                ),
            ],
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : 18),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      label,
                      style: TextStyle(
                        fontWeight: done ? FontWeight.w600 : FontWeight.normal,
                        color: done ? null : const Color(0xFF9E9E9E),
                      ),
                    ),
                  ),
                  if (at != null)
                    Text(at!,
                        style: const TextStyle(
                            fontSize: 12.5, color: Color(0xFF9E9E9E))),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ItemsCard extends StatelessWidget {
  final Map<String, dynamic> order;
  const _ItemsCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final items = (order['items'] as List?) ?? const [];
    final total = (order['total_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (order['discount_tiyin'] as num?)?.toInt() ?? 0;
    final subtotal = (order['subtotal_tiyin'] as num?)?.toInt() ?? 0;

    return _Panel(
      title: 'Buyurtma',
      child: Column(
        children: [
          for (final it in items)
            if (it is Map)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  children: [
                    Text('${it['qty']}×',
                        style: const TextStyle(
                            color: Color(0xFF757575),
                            fontWeight: FontWeight.w600)),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text((it['name'] as String?) ?? '',
                          maxLines: 2, overflow: TextOverflow.ellipsis),
                    ),
                    Text(formatSum(
                        ((it['price_tiyin'] as num?)?.toInt() ?? 0) *
                            ((it['qty'] as num?)?.toInt() ?? 1))),
                  ],
                ),
              ),
          const Divider(height: 20),
          if (discount > 0) ...[
            _Line(label: 'Taomlar', value: formatSum(subtotal)),
            _Line(
              label: 'Chegirma',
              value: '− ${formatSum(discount)}',
              color: const Color(0xFF16A34A),
            ),
            const SizedBox(height: 6),
          ],
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text('Jami',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              Text(formatSum(total),
                  style: const TextStyle(
                      fontWeight: FontWeight.bold, fontSize: 17)),
            ],
          ),
        ],
      ),
    );
  }
}

class _Line extends StatelessWidget {
  final String label;
  final String value;
  final Color? color;
  const _Line({required this.label, required this.value, this.color});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(color: Color(0xFF757575))),
          Text(value, style: TextStyle(color: color)),
        ],
      ),
    );
  }
}

/// Qayerga — stol yoki yetkazib berish manzili.
class _WhereCard extends StatelessWidget {
  final Map<String, dynamic> order;
  const _WhereCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final isDineIn = (order['type'] as String?) == 'dine_in';
    final table = (order['table_label'] as String?) ?? '';
    final addr = order['delivery_address'];

    if (isDineIn) {
      return _Panel(
        title: 'Qayerda',
        child: Row(
          children: [
            const Icon(Icons.qr_code_2, size: 18, color: kBrand),
            const SizedBox(width: 8),
            Text(table.isEmpty ? 'Stolda' : '$table-stol'),
          ],
        ),
      );
    }

    final text = (addr is Map ? (addr['text'] as String?) : null) ?? '';
    return _Panel(
      title: 'Yetkazib berish',
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.place_outlined, size: 18, color: kBrand),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text.isEmpty ? 'Xaritada tanlangan manzil' : text),
          ),
        ],
      ),
    );
  }
}

class _Panel extends StatelessWidget {
  final String title;
  final Widget child;
  const _Panel({required this.title, required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFE5E5E5)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title,
              style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: Color(0xFF757575))),
          const SizedBox(height: 10),
          child,
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorView({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 40, color: Color(0xFF9E9E9E)),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('Qayta urinish')),
          ],
        ),
      ),
    );
  }
}
