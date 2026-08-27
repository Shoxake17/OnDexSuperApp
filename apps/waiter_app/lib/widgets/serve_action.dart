import 'package:flutter/material.dart';

import '../format.dart';
import '../models/waiter_order.dart';
import '../state/waiter_store.dart';
import '../theme.dart';

/// "Yetkazdim" amali — tasdiqlash bilan.
///
/// ┌─ NEGA TASDIQLASH ─────────────────────────────────────────────────┐
/// `served` — TERMINAL holat: bosilgandan keyin buyurtmani qaytarishning
/// iloji yo'q (`statemachine.go` — terminal holatdan hech qayerga
/// o'tilmaydi). Affitsiant telefonni bir qo'lda, laganni ikkinchi qo'lda
/// ushlab yuradi — tasodifiy tegish oson. Bir marta ortiqcha tegish
/// buyurtmani ro'yxatdan yo'q qiladi va u haqiqatan yetkazilganmi,
/// yo'qmi — bilib bo'lmay qoladi.
///
/// Shuning uchun bitta qadam qo'shildi. Lekin ALOHIDA EKRAN emas, pastdan
/// chiqadigan varaq: band vaqtda ortiqcha ekran ochilib-yopilishi
/// sekinlashtiradi.
/// └───────────────────────────────────────────────────────────────────┘
Future<void> confirmServe(
  BuildContext context,
  WaiterOrder order,
  WaiterStore store,
) async {
  final messenger = ScaffoldMessenger.of(context);

  final ok = await showModalBottomSheet<bool>(
    context: context,
    backgroundColor: kSurface,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
    ),
    builder: (ctx) => SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 36,
              height: 4,
              decoration: BoxDecoration(
                color: kBorder,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: 22),
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: kReadyColor.withValues(alpha: 0.16),
              ),
              child: const Icon(Icons.room_service, color: kReadyColor, size: 28),
            ),
            const SizedBox(height: 16),
            Text(
              '${tableText(order.tableLabel)}ga yetkazdingizmi?',
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: kInk,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              '${order.shortNumber} · ${order.itemCount} ta mahsulot\n'
              'Bu amalni qaytarib bo\'lmaydi.',
              textAlign: TextAlign.center,
              style: const TextStyle(color: kInkFaint, height: 1.5, fontSize: 13),
            ),
            const SizedBox(height: 24),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: () => Navigator.of(ctx).pop(true),
                style: FilledButton.styleFrom(backgroundColor: kReadyColor),
                child: const Text('Ha, yetkazdim'),
              ),
            ),
            const SizedBox(height: 8),
            SizedBox(
              width: double.infinity,
              child: TextButton(
                onPressed: () => Navigator.of(ctx).pop(false),
                style: TextButton.styleFrom(foregroundColor: kInkFaint),
                child: const Text('Bekor qilish'),
              ),
            ),
          ],
        ),
      ),
    ),
  );

  if (ok != true) return;

  final err = await store.markServed(order);

  // Natija — qisqa toast. ALOHIDA "yakunlandi" ekrani ATAYLAB yo'q:
  // u yana bitta tegishni talab qiladi va band vaqtda faqat
  // sekinlashtiradi.
  messenger.showSnackBar(
    SnackBar(
      content: Row(
        children: [
          Icon(
            err == null ? Icons.check_circle : Icons.error_outline,
            color: err == null ? kReadyColor : kDangerColor,
            size: 20,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              err ?? '${tableText(order.tableLabel)} — yetkazildi',
            ),
          ),
        ],
      ),
      duration: Duration(seconds: err == null ? 2 : 4),
    ),
  );
}

/// "Yetkazdim" tugmasi — yuklanish holatini o'zi ko'rsatadi.
class ServeButton extends StatelessWidget {
  const ServeButton({
    super.key,
    required this.order,
    required this.store,
    this.expanded = false,
  });

  final WaiterOrder order;
  final WaiterStore store;

  /// Tafsilot ekranida tugma butun kenglikni egallaydi.
  final bool expanded;

  @override
  Widget build(BuildContext context) {
    final busy = store.isBusy(order.id);
    final button = FilledButton.icon(
      onPressed: busy ? null : () => confirmServe(context, order, store),
      style: FilledButton.styleFrom(
        backgroundColor: kReadyColor,
        padding: EdgeInsets.symmetric(
          horizontal: expanded ? 20 : 16,
          vertical: expanded ? 16 : 12,
        ),
      ),
      icon: busy
          ? const SizedBox(
              height: 16,
              width: 16,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: Colors.white,
              ),
            )
          : const Icon(Icons.done, size: 18),
      label: const Text('Yetkazdim'),
    );
    return expanded ? SizedBox(width: double.infinity, child: button) : button;
  }
}
