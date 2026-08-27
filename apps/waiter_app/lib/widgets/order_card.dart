import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum;

import '../format.dart';
import '../models/waiter_order.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import 'common.dart';
import 'serve_action.dart';

/// Bitta buyurtma kartochkasi.
///
/// ┌─ VIZUAL USTUVORLIK ───────────────────────────────────────────────┐
/// "Tayyor" kartochka ATAYLAB boshqacha: qalin yashil chegara, yengil
/// yashil fon va kutish vaqti. Qolganlari — oddiy, xira. Sabab: zalda
/// yugurib yurgan odam ekranga yarim soniya qaraydi va "qaysi biriga
/// hozir borishim kerak" degan savolga javob DARHOL ko'rinishi kerak.
/// Hamma kartochka bir xil ko'rinsa, u har safar o'qib chiqishga majbur
/// bo'ladi.
/// └───────────────────────────────────────────────────────────────────┘
class OrderCard extends StatelessWidget {
  const OrderCard({
    super.key,
    required this.order,
    required this.store,
    this.onTap,
    this.showTable = true,
  });

  final WaiterOrder order;
  final WaiterStore store;
  final VoidCallback? onTap;

  /// Stol tafsiloti ichida stol nomi takrorlanmasin.
  final bool showTable;

  @override
  Widget build(BuildContext context) {
    final ready = order.isReady;
    final wait = order.waitingSinceReady;

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        color: ready ? const Color(0x142E9E4F) : kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(
          color: ready ? kReadyColor : kBorder,
          width: ready ? 1.6 : 1,
        ),
      ),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(kRadiusCard),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(kRadiusCard),
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    if (showTable) ...[
                      // `Flexible` — uzun stol nomi ("Terastadagi katta
                      // stol") telefon ekranida qatordan chiqib
                      // ketmasin: Flutter'ning sariq-qora "overflow"
                      // chizig'i restoran panelida aynan shu sababdan
                      // chiqqan edi.
                      Flexible(
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 10,
                            vertical: 5,
                          ),
                          decoration: BoxDecoration(
                            color: ready
                                ? kReadyColor
                                : kSurfaceRaised,
                            borderRadius: BorderRadius.circular(kRadiusChip),
                          ),
                          child: Text(
                            tableText(order.tableLabel),
                            maxLines: 1,
                            softWrap: false,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontWeight: FontWeight.bold,
                              fontSize: 14,
                              color: ready ? Colors.white : kInk,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                    ],
                    if (order.partySize > 0) ...[
                      const Icon(Icons.people_outline, size: 14, color: kInkFaint),
                      const SizedBox(width: 3),
                      Text(
                        '${order.partySize}',
                        style: const TextStyle(color: kInkFaint, fontSize: 12),
                      ),
                      const SizedBox(width: 8),
                    ],
                    const Spacer(),
                    StatusChip(status: order.status, dense: true),
                  ],
                ),

                // Kutish vaqti — FAQAT tayyor buyurtmada. Boshqa
                // holatda bu raqam ma'nosiz (taom hali oshxonada) va
                // faqat e'tiborni chalg'itardi.
                if (ready && wait != null) ...[
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      Icon(
                        Icons.schedule,
                        size: 14,
                        color: _waitColor(wait),
                      ),
                      const SizedBox(width: 5),
                      Text(
                        waitBadge(wait),
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: _waitColor(wait),
                        ),
                      ),
                    ],
                  ),
                ],

                const SizedBox(height: 10),

                // Mahsulotlar — uzun ro'yxat kartochkani cho'zmasligi
                // uchun 3 tadan keyin qisqartiriladi. To'liq ro'yxat
                // tafsilot ekranida.
                ...order.items.take(3).map(
                      (it) => Padding(
                        padding: const EdgeInsets.only(bottom: 4),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            SizedBox(
                              width: 28,
                              child: Text(
                                '${it.qty}×',
                                style: const TextStyle(
                                  color: kInkFaint,
                                  fontSize: 13,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ),
                            Expanded(
                              child: Text(
                                it.name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: kInkDim,
                                  fontSize: 13,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                if (order.items.length > 3)
                  Padding(
                    padding: const EdgeInsets.only(left: 28, top: 2),
                    child: Text(
                      've yana ${order.items.length - 3} ta',
                      style: const TextStyle(color: kInkGhost, fontSize: 12),
                    ),
                  ),

                const SizedBox(height: 12),
                Row(
                  children: [
                    Text(
                      formatSum(order.totalTiyin),
                      style: const TextStyle(
                        fontWeight: FontWeight.bold,
                        fontSize: 15,
                        color: kInk,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      order.shortNumber,
                      style: const TextStyle(color: kInkGhost, fontSize: 12),
                    ),
                    const Spacer(),
                    if (ready) ServeButton(order: order, store: store),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// Uzoq kutgan taom — ogohlantiruvchi rangga o'tadi.
  ///
  /// 10 daqiqadan keyin taom sovuydi; 20 daqiqadan keyin bu allaqachon
  /// muammo. Rang shu chegaralarda o'zgaradi — raqamni o'qimasdan ham
  /// ko'rinadi.
  Color _waitColor(Duration d) {
    if (d.inMinutes >= 20) return kDangerColor;
    if (d.inMinutes >= 10) return kWaitingColor;
    return kReadyColor;
  }
}
