import 'package:flutter/material.dart';

import '../theme.dart';
import 'order_card_header.dart' show isDineInOrder;

/// Serverdagi `orders.DispatchNotFound` qiymati.
const kDispatchNotFound = 'not_found';

/// Yetkazish buyurtmasiga kuryer TOPILMADIMI — restoran qaror qilishi
/// kerak (bekor qilish yoki qayta qidirish).
///
/// Qaror serverda (`internal/orders/dispatch_state.go`): taom tayyor
/// bo'lgach 10 daqiqada kuryer topilmasa holat `not_found` bo'ladi.
/// Panel vaqtni O'ZI hisoblamaydi — kompyuter soati noto'g'ri bo'lsa
/// tugmalar erta yoki kech chiqib qolardi.
bool isCourierNotFound(Map<String, dynamic> order) =>
    !isDineInOrder(order) &&
    (order['courier_id'] as String? ?? '').isEmpty &&
    order['dispatch_state'] == kDispatchNotFound;

/// Avtomatik bekor qilinishigacha qolgan vaqt (`dispatch_deadline`).
/// Muddat o'tgan bo'lsa `Duration.zero`, noma'lum bo'lsa `null`.
Duration? autoCancelLeft(Map<String, dynamic> order, DateTime now) {
  final raw = order['dispatch_deadline'];
  final deadline = raw is String ? DateTime.tryParse(raw) : null;
  if (deadline == null) return null;
  final left = deadline.difference(now);
  return left.isNegative ? Duration.zero : left;
}

/// "24 daqiqadan keyin avtomatik bekor qilinadi" — daqiqa YUQORIGA
/// yuvarlanadi (29:10 qolganda "30 daqiqa", "29" emas).
String autoCancelText(Duration? left) {
  if (left == null) return 'Javob bo\'lmasa buyurtma avtomatik bekor qilinadi';
  if (left < const Duration(minutes: 1)) {
    return 'Hozir avtomatik bekor qilinadi';
  }
  final minutes = left.inMinutes + (left.inSeconds % 60 > 0 ? 1 : 0);
  return '$minutes daqiqadan keyin avtomatik bekor qilinadi';
}

/// "Tayyor" kartochkasidagi kuryer holati bloki.
///
/// To'rt holat: stol buyurtmasi (affitsiant), kuryer biriktirilgan,
/// kuryer qidirilmoqda va kuryer topilmadi ("Bekor qilish" /
/// "Kuryer qidirish" tugmalari bilan).
class CourierStatusBox extends StatelessWidget {
  final Map<String, dynamic> order;
  final VoidCallback? onCancel;
  final VoidCallback? onRetry;

  /// Amal serverga yuborilmoqda — tugmalar vaqtincha o'chiq, ikki marta
  /// bosib ikkita so'rov yuborilmaydi.
  final bool busy;

  /// Hozirgi vaqt (testlarda almashtiriladi).
  final DateTime? now;

  const CourierStatusBox({
    super.key,
    required this.order,
    this.onCancel,
    this.onRetry,
    this.busy = false,
    this.now,
  });

  @override
  Widget build(BuildContext context) {
    final courierId = order['courier_id'] as String? ?? '';
    final courierName = order['courier_name'] as String? ?? '';

    // Stol buyurtmasiga kuryer UMUMAN tegishli emas — taomni affitsiant
    // zalga olib boradi (`isDineInOrder` izohi).
    if (isDineInOrder(order)) {
      return const _StatusLine(
        background: OnDexColors.primaryTint,
        leading: Icon(Icons.room_service_rounded,
            size: 15, color: OnDexColors.primary),
        text: 'Affitsiant zalga olib boradi',
        color: OnDexColors.primary,
        bold: true,
      );
    }
    if (courierId.isNotEmpty) {
      return _StatusLine(
        background: OnDexColors.successBg,
        leading: const Icon(Icons.pedal_bike_rounded,
            size: 15, color: OnDexColors.success),
        text: courierName.isEmpty
            ? 'Kuryer biriktirildi, kutilmoqda'
            : '$courierName kelmoqda',
        color: OnDexColors.success,
        bold: true,
      );
    }
    if (isCourierNotFound(order)) {
      return _NotFound(
        leftText: autoCancelText(autoCancelLeft(order, now ?? DateTime.now())),
        onCancel: busy ? null : onCancel,
        onRetry: busy ? null : onRetry,
      );
    }
    return const _StatusLine(
      background: OnDexColors.pageBg,
      leading: SizedBox(
        width: 13,
        height: 13,
        child: CircularProgressIndicator(
            strokeWidth: 2, color: OnDexColors.inkFaint),
      ),
      text: 'Kuryer qidirilmoqda...',
      color: OnDexColors.inkDim,
    );
  }
}

class _StatusLine extends StatelessWidget {
  final Color background;
  final Widget leading;
  final String text;
  final Color color;
  final bool bold;

  const _StatusLine({
    required this.background,
    required this.leading,
    required this.text,
    required this.color,
    this.bold = false,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 12),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          leading,
          const SizedBox(width: 8),
          // Tor Kanban ustunida matn kartochkadan chiqib ketmasin.
          Flexible(
            child: Text(text,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                    fontSize: 12.5,
                    color: color,
                    fontWeight: bold ? FontWeight.w700 : FontWeight.w400)),
          ),
        ],
      ),
    );
  }
}

class _NotFound extends StatelessWidget {
  final String leftText;
  final VoidCallback? onCancel;
  final VoidCallback? onRetry;

  const _NotFound({
    required this.leftText,
    required this.onCancel,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey('courier-not-found'),
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: OnDexColors.dangerBg,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Row(
            children: [
              Icon(Icons.person_search_rounded,
                  size: 16, color: OnDexColors.danger),
              SizedBox(width: 8),
              Flexible(
                child: Text('Kuryer topilmadi',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                        fontSize: 13.5,
                        fontWeight: FontWeight.w800,
                        color: OnDexColors.danger)),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(leftText,
              key: const ValueKey('courier-auto-cancel'),
              style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
          const SizedBox(height: 4),
          // Taklif faqat restoranning O'Z yetkazib beruvchilariga ketadi
          // (OnDex kuryerlari hozircha to'xtatilgan) — "topilmadi" deganda
          // restoran nimani tekshirishi kerakligini bilishi kerak.
          const Text(
            'Taklif faqat restoraningizning onlayn yetkazib beruvchilariga boradi — '
            '"OnDexGO" ilovasida onlayn ekanini tekshiring.',
            key: ValueKey('courier-own-hint'),
            style: TextStyle(fontSize: 11.5, color: OnDexColors.inkDim),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  key: const ValueKey('courier-cancel'),
                  onPressed: onCancel,
                  style: OutlinedButton.styleFrom(
                    foregroundColor: OnDexColors.danger,
                    side: const BorderSide(color: OnDexColors.danger),
                    padding:
                        const EdgeInsets.symmetric(vertical: 11, horizontal: 6),
                  ),
                  child: const Text('Bekor qilish',
                      maxLines: 1, overflow: TextOverflow.ellipsis),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: FilledButton(
                  key: const ValueKey('courier-retry'),
                  onPressed: onRetry,
                  style: FilledButton.styleFrom(
                    padding:
                        const EdgeInsets.symmetric(vertical: 11, horizontal: 6),
                  ),
                  child: const Text('Kuryer qidirish',
                      maxLines: 1, overflow: TextOverflow.ellipsis),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
