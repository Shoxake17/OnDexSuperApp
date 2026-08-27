import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum;

import '../format.dart';
import '../models/waiter_order.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/serve_action.dart';

/// Bitta buyurtma tafsiloti — TO'LIQ tarkib va yagona harakat.
///
/// ┌─ FAQAT O'QISH UCHUN (bitta istisno bilan) ────────────────────────┐
/// Bu ekranda "Qabul qilish", "Rad etish", "Tayyorlandi" kabi tugmalar
/// YO'Q va bo'lmaydi. Sabab — holat mashinasi
/// (`internal/orders/statemachine.go`): `created → accepted` va
/// `preparing → ready` o'tishlarini FAQAT restoran/oshxona qiladi
/// (`ActorRestaurant`). Affitsiantga bu tugmalarni berish ikki rolni
/// aralashtirib yuborardi va server baribir 403 qaytarardi — ya'ni
/// tugma ishlamaydigan bo'lardi.
///
/// Affitsiantning yagona o'tishi: `ready → served` (`ActorWaiter`).
/// Shuning uchun bitta tugma bor va u faqat `ready` holatida chiqadi.
/// └───────────────────────────────────────────────────────────────────┘
class OrderDetailScreen extends StatelessWidget {
  const OrderDetailScreen({
    super.key,
    required this.orderId,
    required this.store,
  });

  final String orderId;
  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        // Buyurtma jonli ro'yxatdan olinadi — WS orqali holat
        // o'zgarsa (masalan oshxona "tayyor" desa) ekran O'ZI
        // yangilanadi va tugma paydo bo'ladi. Nusxa saqlansa,
        // affitsiant eskirgan holatga qarab turardi.
        final order = store.orderById(orderId);

        return Scaffold(
          appBar: AppBar(
            title: Text(
              order == null ? 'Buyurtma' : tableText(order.tableLabel),
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
          ),
          body: order == null ? _gone(context) : _content(context, order),
        );
      },
    );
  }

  /// Buyurtma faol ro'yxatdan chiqib ketgan: kimdir (boshqa affitsiant
  /// yoki restoranning o'zi) uni allaqachon yakunlagan.
  ///
  /// Bu holat HAQIQATAN sodir bo'ladi: kichik oshxonalarda restoran ham
  /// `served` qo'ya oladi (`statemachine.go` — `ActorRestaurant`).
  /// Ekranni bo'sh qoldirish o'rniga sabab aytiladi.
  Widget _gone(BuildContext context) => ListView(
        children: [
          const EmptyState(
            icon: Icons.task_alt,
            title: 'Bu buyurtma yakunlangan',
            subtitle: 'Uni siz yoki restoran allaqachon yopgan bo\'lishi '
                'mumkin. Faol ro\'yxatda endi ko\'rinmaydi.',
          ),
          Center(
            child: OutlinedButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Ro\'yxatga qaytish'),
            ),
          ),
        ],
      );

  Widget _content(BuildContext context, WaiterOrder order) {
    final wait = order.waitingSinceReady;

    return Column(
      children: [
        Expanded(
          child: ListView(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
            children: [
              // ── Sarlavha bloki ──
              Row(
                children: [
                  StatusChip(status: order.status),
                  const SizedBox(width: 8),
                  Text(
                    order.shortNumber,
                    style: const TextStyle(color: kInkFaint, fontSize: 13),
                  ),
                  const Spacer(),
                  if (order.partySize > 0)
                    Row(
                      children: [
                        const Icon(Icons.people_outline,
                            size: 15, color: kInkFaint),
                        const SizedBox(width: 4),
                        Text(
                          '${order.partySize} kishi',
                          style: const TextStyle(
                              color: kInkFaint, fontSize: 13),
                        ),
                      ],
                    ),
                ],
              ),

              if (wait != null) ...[
                const SizedBox(height: 14),
                Container(
                  padding: const EdgeInsets.symmetric(
                      horizontal: 14, vertical: 12),
                  decoration: BoxDecoration(
                    color: kReadyColor.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(kRadiusCard),
                    border: Border.all(
                        color: kReadyColor.withValues(alpha: 0.4)),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.schedule, size: 18, color: kReadyColor),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          waitSentence(wait),
                          style: const TextStyle(
                            color: kReadyColor,
                            fontWeight: FontWeight.w600,
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],

              const SizedBox(height: 20),
              const Text(
                'TARKIBI',
                style: TextStyle(
                  color: kInkFaint,
                  fontSize: 11,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 0.8,
                ),
              ),
              const SizedBox(height: 10),

              // ── Mahsulotlar ──
              Container(
                decoration: BoxDecoration(
                  color: kSurface,
                  borderRadius: BorderRadius.circular(kRadiusCard),
                  border: Border.all(color: kBorder),
                ),
                child: Column(
                  children: [
                    for (var i = 0; i < order.items.length; i++) ...[
                      if (i > 0) const Divider(height: 1),
                      Padding(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 14, vertical: 12),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            SizedBox(
                              width: 34,
                              child: Text(
                                '${order.items[i].qty}×',
                                style: const TextStyle(
                                  color: kBrandColor,
                                  fontWeight: FontWeight.bold,
                                  fontSize: 14,
                                ),
                              ),
                            ),
                            Expanded(
                              child: Text(
                                order.items[i].name,
                                style: const TextStyle(
                                    color: kInk, fontSize: 14, height: 1.3),
                              ),
                            ),
                            const SizedBox(width: 10),
                            Text(
                              formatSum(order.items[i].lineTotalTiyin),
                              style: const TextStyle(
                                  color: kInkDim, fontSize: 13),
                            ),
                          ],
                        ),
                      ),
                    ],
                    const Divider(height: 1),
                    Padding(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 14, vertical: 14),
                      child: Row(
                        children: [
                          const Text(
                            'Jami',
                            style: TextStyle(
                              color: kInk,
                              fontWeight: FontWeight.bold,
                              fontSize: 15,
                            ),
                          ),
                          const Spacer(),
                          Text(
                            formatSum(order.totalTiyin),
                            style: const TextStyle(
                              color: kBrandColor,
                              fontWeight: FontWeight.bold,
                              fontSize: 16,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),

              const SizedBox(height: 16),
              if (order.createdAt != null)
                Row(
                  children: [
                    const Icon(Icons.access_time, size: 14, color: kInkGhost),
                    const SizedBox(width: 6),
                    Text(
                      'Buyurtma vaqti: ${formatClock(order.createdAt!)}',
                      style: const TextStyle(color: kInkGhost, fontSize: 12),
                    ),
                  ],
                ),
            ],
          ),
        ),

        // ── Yagona harakat ──
        //
        // Tugma pastda, doimiy joyda: affitsiant ro'yxatni pastgacha
        // surib chiqishga majbur bo'lmasin.
        if (order.isReady)
          Container(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
            decoration: const BoxDecoration(
              color: kSurface,
              border: Border(top: BorderSide(color: kBorder)),
            ),
            child: SafeArea(
              top: false,
              child: ServeButton(order: order, store: store, expanded: true),
            ),
          )
        else
          Container(
            width: double.infinity,
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
            decoration: const BoxDecoration(
              color: kSurface,
              border: Border(top: BorderSide(color: kBorder)),
            ),
            child: SafeArea(
              top: false,
              child: Text(
                // Nima kutilayotgani AYTILADI. Aks holda affitsiant
                // "nega tugma yo'q" deb o'ylardi.
                order.isPreparing
                    ? 'Oshxona tayyorlamoqda — tayyor bo\'lganda ovoz chalinadi'
                    : 'Oshxona buyurtmani qabul qilishini kutmoqdamiz',
                textAlign: TextAlign.center,
                style: const TextStyle(color: kInkFaint, fontSize: 13),
              ),
            ),
          ),
      ],
    );
  }
}
