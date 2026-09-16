import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import '../order_view.dart';
import '../theme.dart';

// Kuryer ekranining kartochkalari (yorug' mavzu, image/Kuryer.png).
// Mantiq `courier_shell.dart` dan AYNAN ko'chirilgan — faqat ranglar
// yorug' fonga moslandi (qorong'i mavzudagi grey.shade400 matni oq fonda
// o'qilmas edi).

/// Xarita ustidagi dumaloq tugma (3D, joriy joylashuv).
class MapRoundButton extends StatelessWidget {
  const MapRoundButton({super.key, required this.icon, required this.onTap, this.tooltip});

  final IconData icon;
  final VoidCallback? onTap;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final btn = Material(
      color: kSurface,
      shape: const CircleBorder(),
      elevation: 3,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(11),
          child: Icon(icon, size: 22, color: kInk),
        ),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}

/// Zoom bloki — faqat kattalashtirish/kichiklashtirish.
class ZoomBlock extends StatelessWidget {
  const ZoomBlock({super.key, required this.onZoomIn, required this.onZoomOut});

  final VoidCallback onZoomIn;
  final VoidCallback onZoomOut;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 44,
      child: Material(
        color: kSurface,
        borderRadius: BorderRadius.circular(22),
        elevation: 3,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            InkWell(
              borderRadius: const BorderRadius.vertical(top: Radius.circular(22)),
              onTap: onZoomIn,
              child: const Padding(
                padding: EdgeInsets.symmetric(vertical: 14, horizontal: 10),
                child: Icon(Icons.add, size: 20, color: kInk),
              ),
            ),
            Container(width: 22, height: 1, color: kLine),
            InkWell(
              borderRadius: const BorderRadius.vertical(bottom: Radius.circular(22)),
              onTap: onZoomOut,
              child: const Padding(
                padding: EdgeInsets.symmetric(vertical: 14, horizontal: 10),
                child: Icon(Icons.remove, size: 20, color: kInk),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Liniyada, buyurtma yo'q.
class WaitingCard extends StatelessWidget {
  const WaitingCard({super.key});

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: 18),
      child: Column(
        children: [
          Icon(Icons.notifications_active_outlined, size: 40, color: kInkFaint),
          SizedBox(height: 10),
          Text(
            'Yangi buyurtma kutilmoqda...',
            textAlign: TextAlign.center,
            style: TextStyle(color: kInkMuted, fontSize: 15),
          ),
        ],
      ),
    );
  }
}

/// Liniyada emas, buyurtma yo'q.
class OfflineHintCard extends StatelessWidget {
  const OfflineHintCard({super.key});

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: 8),
      child: Text(
        'Buyurtma olish uchun chapdagi tugma bilan liniyaga chiqing.',
        textAlign: TextAlign.center,
        style: TextStyle(color: kInkMuted, fontSize: 14),
      ),
    );
  }
}

/// Restoran kirishni yopgan (ta'til, ishdan bo'shatish yoki kirish o'chiq).
class AccessClosedCard extends StatefulWidget {
  const AccessClosedCard({super.key, required this.onRetry});

  final Future<void> Function() onRetry;

  @override
  State<AccessClosedCard> createState() => _AccessClosedCardState();
}

class _AccessClosedCardState extends State<AccessClosedCard> {
  bool _busy = false;

  Future<void> _retry() async {
    setState(() => _busy = true);
    try {
      await widget.onRetry();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Column(
        children: [
          const Icon(Icons.lock_clock_rounded, size: 42, color: kInkFaint),
          const SizedBox(height: 10),
          const Text(
            'Ilovaga kirishingiz yopiq',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: kInk),
          ),
          const SizedBox(height: 6),
          const Text(
            // Kirishni restoran "Xodimlar" bo'limida boshqaradi.
            'Restoraningiz sizga "OnDexGO" ilovasiga kirishni ochgach, '
            'liniyaga chiqib buyurtma qabul qila olasiz.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 13.5, color: kInkMuted),
          ),
          const SizedBox(height: 14),
          OutlinedButton.icon(
            onPressed: _busy ? null : _retry,
            icon: _busy
                ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                : const Icon(Icons.refresh),
            label: const Text('Qayta tekshirish'),
          ),
        ],
      ),
    );
  }
}

/// Yangi buyurtma taklifi. Alohida "Rad etish" tugmasi YO'Q — vaqt tugasa
/// backend (`offerTTL`) o'zi avtomatik rad etadi.
class OfferCard extends StatelessWidget {
  const OfferCard({super.key, required this.offer});

  final Map<String, dynamic> offer;

  @override
  Widget build(BuildContext context) {
    final name = (offer['restaurant_name'] as String?)?.trim();
    final address = (offer['restaurant_address'] as String?)?.trim();
    final etaMinutes = (offer['eta_minutes'] as num?)?.toInt();
    final logoUrl = (offer['restaurant_logo_url'] as String?)?.trim() ?? '';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Yangi buyurtma',
            style: TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: kBrandColor)),
        const SizedBox(height: 4),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name?.isNotEmpty == true ? name! : 'Buyurtma',
                    style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: kInk),
                  ),
                  const SizedBox(height: 4),
                  // VAQT (daqiqa), masofa emas — kuryer restoranga qachon
                  // yetib borishini bilib, kechikmasligi uchun.
                  Text(
                    etaMinutes != null ? 'Restorangacha $etaMinutes daqiqa' : 'Yo\'l vaqti hisoblanmoqda...',
                    style: const TextStyle(color: kInkMuted),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            // Restoran logosi (taklif restoranning o'zidan keladi); bo'lmasa
            // yoki yuklanmasa — OnDex belgisi.
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: SizedBox(
                width: 48,
                height: 48,
                child: logoUrl.isEmpty
                    ? Image.asset('assets/ondex.png', fit: BoxFit.contain)
                    : Image.network(
                        fullImageUrl(logoUrl),
                        key: const ValueKey('offer-restaurant-logo'),
                        fit: BoxFit.cover,
                        cacheWidth: 144,
                        errorBuilder: (_, _, _) => Image.asset('assets/ondex.png', fit: BoxFit.contain),
                      ),
              ),
            ),
          ],
        ),
        if (address?.isNotEmpty == true) ...[
          const SizedBox(height: 12),
          const Text('Qayerdan', style: TextStyle(fontSize: 11.5, color: kInkMuted)),
          const SizedBox(height: 2),
          Text(address!, style: const TextStyle(fontWeight: FontWeight.w600, color: kInk)),
        ],
      ],
    );
  }
}

/// "Qabul qilish" — tugma foni soniyalar o'tishi bilan chapdan o'ngga
/// "eriydi" (vizual taymer).
class AcceptCountdownButton extends StatelessWidget {
  const AcceptCountdownButton({
    super.key,
    required this.secondsLeft,
    required this.totalSeconds,
    required this.busy,
    required this.onAccept,
  });

  final int secondsLeft;
  final int totalSeconds;
  final bool busy;
  final VoidCallback onAccept;

  @override
  Widget build(BuildContext context) {
    final fraction = totalSeconds <= 0 ? 0.0 : (secondsLeft / totalSeconds).clamp(0.0, 1.0);
    return ClipRRect(
      borderRadius: BorderRadius.circular(14),
      child: SizedBox(
        height: 54,
        child: Stack(
          fit: StackFit.expand,
          children: [
            Container(color: kBrandColor.withValues(alpha: 0.25)),
            AnimatedFractionallySizedBox(
              duration: const Duration(seconds: 1),
              curve: Curves.linear,
              alignment: Alignment.centerLeft,
              widthFactor: fraction,
              child: Container(color: kBrandColor),
            ),
            Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: busy ? null : onAccept,
                child: Center(
                  child: busy
                      ? const SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                        )
                      : Text(
                          'Qabul qilish · $secondsLeft',
                          style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 16),
                        ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Buyurtma raqamining oxirgi 4 xonasi — kuryer restoranda og'zaki aytadi.
String _lastFourDigits(String orderNumber) =>
    orderNumber.length <= 4 ? orderNumber : orderNumber.substring(orderNumber.length - 4);

/// Soniya -> "MM:SS" (orqa sanoq).
String _formatCountdown(int totalSeconds) {
  final s = totalSeconds < 0 ? 0 : totalSeconds;
  final minutes = s ~/ 60;
  final seconds = s % 60;
  return '${minutes.toString().padLeft(2, '0')}:${seconds.toString().padLeft(2, '0')}';
}

Future<void> _callPhone(String phone) async {
  try {
    await launchUrl(Uri(scheme: 'tel', path: phone));
  } catch (_) {
    // Qo'ng'iroq imkoniyati bo'lmasa — bloklovchi xato ko'rsatilmaydi.
  }
}

/// Zaxira navigatsiya: qurilmaning standart xarita ilovasi.
Future<void> _openInDeviceMaps(double lat, double lng) async {
  final uri = Uri.parse('https://www.google.com/maps/dir/?api=1&destination=$lat,$lng');
  try {
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  } catch (_) {
    // Xarita ilovasi topilmasa — bloklovchi xato ko'rsatilmaydi.
  }
}

void _showHelpDialog(BuildContext context, String title, String message) {
  showDialog<void>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(title),
      content: Text(message),
      actions: [
        TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Tushunarli')),
      ],
    ),
  );
}

/// Mijoz kiritgan manzil tafsilotlari (podyezd/qavat/kvartira/domofon).
String _addressDetails(Map<String, dynamic> order) {
  final a = order['delivery_address'];
  if (a is! Map) return '';
  final parts = <String>[];
  void add(String label, String key) {
    final v = (a[key] as String?)?.trim();
    if (v != null && v.isNotEmpty) parts.add('$label $v');
  }

  add('Podyezd', 'entrance');
  add('Qavat', 'floor');
  add('Kvartira', 'apartment');
  add('Domofon', 'intercom');
  return parts.join(' · ');
}

String _addressComment(Map<String, dynamic> order) {
  final a = order['delivery_address'];
  if (a is! Map) return '';
  return (a['comment'] as String?)?.trim() ?? '';
}

/// Joriy buyurtma kartochkasi.
///
/// "picked_up" gacha taomlar va mijoz manzili YASHIRIN (backend ham buni
/// qat'iy ta'minlaydi) — bu bosqichda faqat restoran ma'lumoti ko'rinadi.
class ActiveOrderCard extends StatelessWidget {
  const ActiveOrderCard({
    super.key,
    required this.order,
    required this.restaurant,
    required this.deliveryAddress,
    required this.countdown,
    required this.arrivedAtRestaurant,
  });

  final Map<String, dynamic> order;
  final Map<String, dynamic>? restaurant;
  final String? deliveryAddress;

  /// Joriy maqsadgacha orqa sanoq (soniya). `null` — hali hisoblanmoqda.
  /// Tinglanadi: har soniyada faqat vaqt yozuvi qayta chiziladi, karta emas.
  final ValueListenable<int?> countdown;

  /// Kuryer restoranga yetib borganini bildirganmi (mahalliy holat).
  final bool arrivedAtRestaurant;

  @override
  Widget build(BuildContext context) {
    final status = order['status'] as String? ?? 'ready';
    final pickedUp = status == 'picked_up';
    final ready = status == 'ready';
    final items = (order['items'] as List?) ?? [];
    final total = (order['total_tiyin'] as num? ?? 0).toInt();
    final orderNumber = order['order_number']?.toString() ?? '—';
    final restaurantName = restaurant?['name'] as String? ?? 'Restoran';
    final restaurantAddress = restaurant?['address'] as String? ?? '';
    // Telefonlar backend'dan faqat shu buyurtma orqali, faqat kuryer/adminga
    // keladi; mijoz raqami faqat picked_up'dan keyin.
    final restaurantPhone = order['restaurant_phone'] as String?;
    final customerPhone = order['customer_phone'] as String?;

    final destinationLabel = pickedUp ? 'Qabul qiluvchi' : 'Yuboruvchi';
    final destinationName = pickedUp ? 'Mijoz' : restaurantName;
    final destinationAddress = pickedUp ? (deliveryAddress ?? 'Manzil aniqlanmoqda...') : restaurantAddress;
    final callPhone = pickedUp ? customerPhone : restaurantPhone;
    final destLat = pickedUp ? (order['delivery_lat'] as num?)?.toDouble() : (restaurant?['lat'] as num?)?.toDouble();
    final destLng = pickedUp ? (order['delivery_lng'] as num?)?.toDouble() : (restaurant?['lng'] as num?)?.toDouble();
    final hasMapFallback = destLat != null && destLng != null && (destLat != 0 || destLng != 0);

    final showCountdown = pickedUp || !arrivedAtRestaurant;
    final IconData stageIcon =
        showCountdown ? Icons.timer_outlined : (ready ? Icons.check_circle_outline : Icons.storefront);
    const primaryStyle = TextStyle(fontSize: 22, fontWeight: FontWeight.bold, letterSpacing: 0.5, color: kInk);
    final Widget primary = showCountdown
        ? ValueListenableBuilder<int?>(
            valueListenable: countdown,
            builder: (_, left, _) => Text(
              left != null ? _formatCountdown(left) : '--:--',
              key: const ValueKey('courier-countdown'),
              style: primaryStyle,
            ),
          )
        : const Text('Siz restorandasiz', style: primaryStyle);
    final secondaryText = showCountdown
        ? (pickedUp ? 'Shu vaqt ichida qabul qiluvchining oldiga boring' : 'Shu vaqt ichida yuboruvchining oldiga boring')
        : (ready ? 'Taom tayyor — xodimdan buyurtmani so\'rang' : 'Taom hali tayyorlanmoqda, biroz kuting...');

    const muted = TextStyle(fontSize: 12, color: kInkMuted);
    const divider = Divider(color: kLine, height: 1);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: const BoxDecoration(color: kBrandColor, shape: BoxShape.circle),
              child: Icon(stageIcon, color: Colors.white, size: 22),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  primary,
                  Text(secondaryText, style: muted),
                ],
              ),
            ),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                _StepIcon(active: !pickedUp, child: _RestaurantLogoAvatar(logoUrl: restaurant?['logo_url'] as String?)),
                const Icon(Icons.chevron_right, color: kInkFaint, size: 18),
                _StepIcon(active: pickedUp, child: const Icon(Icons.person, color: Colors.white, size: 18)),
              ],
            ),
          ],
        ),
        const SizedBox(height: 14),
        divider,
        const SizedBox(height: 12),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(destinationLabel, style: const TextStyle(fontSize: 11.5, color: kInkMuted)),
                  const SizedBox(height: 2),
                  Text(destinationName,
                      style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 16, color: kInk)),
                ],
              ),
            ),
            if (callPhone != null && callPhone.isNotEmpty)
              _CircleIconButton(icon: Icons.call, onTap: () => unawaited(_callPhone(callPhone))),
          ],
        ),
        const SizedBox(height: 10),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('Manzil', style: TextStyle(fontSize: 11.5, color: kInkMuted)),
                  const SizedBox(height: 2),
                  Text(destinationAddress, style: const TextStyle(fontWeight: FontWeight.w600, color: kInk)),
                  if (pickedUp && _addressDetails(order).isNotEmpty) ...[
                    const SizedBox(height: 3),
                    Text(_addressDetails(order), style: const TextStyle(fontSize: 13, color: kInkMuted)),
                  ],
                  if (pickedUp && _addressComment(order).isNotEmpty) ...[
                    const SizedBox(height: 3),
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Icon(Icons.chat_bubble_outline, size: 14, color: kInkMuted),
                        const SizedBox(width: 5),
                        Expanded(
                          child: Text(_addressComment(order), style: const TextStyle(fontSize: 13, color: kInkMuted)),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
            if (hasMapFallback)
              _CircleIconButton(icon: Icons.directions, onTap: () => unawaited(_openInDeviceMaps(destLat, destLng))),
          ],
        ),
        const SizedBox(height: 12),
        divider,
        const SizedBox(height: 12),
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(orderNumber, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16, color: kInk)),
                  Text(
                    pickedUp
                        ? '${items.length} ta mahsulot'
                        : 'Xodimga oxirgi 4 xonani ayting: ${_lastFourDigits(orderNumber)}',
                    style: muted,
                  ),
                ],
              ),
            ),
            Text(formatSum(total), style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16, color: kInk)),
          ],
        ),
        // To'lov ko'rsatmasi (kuryer ilovasi auditi, 1-band) — mantiq va
        // testlari `lib/order_view.dart`.
        const SizedBox(height: 12),
        _PaymentBanner(instruction: paymentInstructionFor(order)),
        if (pickedUp) ...[
          const SizedBox(height: 10),
          for (final raw in items)
            if (raw is Map)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Row(
                  children: [
                    Text('${raw['qty']}x ', style: const TextStyle(fontWeight: FontWeight.bold, color: kInk)),
                    Expanded(child: Text(raw['name'] as String? ?? '', style: const TextStyle(color: kInk))),
                  ],
                ),
              ),
        ],
        const SizedBox(height: 12),
        divider,
        // Haqiqiy aloqa (kuryer ilovasi auditi, 12-band): kuryer restoranning
        // O'Z xodimi, ya'ni uning qo'llab-quvvatlashi — o'sha restoran.
        _SupportRow(
          title: 'Qo\'llab-quvvatlash',
          subtitle: 'Buyurtma bilan muammo bo\'lsa — restoranga qo\'ng\'iroq',
          onTap: () {
            final phone = (restaurantPhone ?? '').trim();
            showDialog<void>(
              context: context,
              builder: (ctx) => AlertDialog(
                title: const Text('Qo\'llab-quvvatlash'),
                content: Text(phone.isNotEmpty
                    ? 'Mijoz javob bermasa, manzil noto\'g\'ri bo\'lsa yoki taom bilan '
                        'muammo bo\'lsa — $restaurantName bilan bog\'laning.'
                    : 'Restoran raqami topilmadi. Restoran ma\'muriyatiga '
                        'to\'g\'ridan-to\'g\'ri murojaat qiling.'),
                actions: [
                  TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Yopish')),
                  if (phone.isNotEmpty)
                    FilledButton.icon(
                      onPressed: () {
                        Navigator.of(ctx).pop();
                        unawaited(_callPhone(phone));
                      },
                      icon: const Icon(Icons.call),
                      label: const Text('Restoranga qo\'ng\'iroq'),
                    ),
                ],
              ),
            );
          },
        ),
        divider,
        _SupportRow(
          title: 'Nima qilay?',
          subtitle: null,
          onTap: () => _showHelpDialog(
            context,
            'Nima qilay?',
            pickedUp
                ? 'Mijoz manziliga yetib borib, buyurtmani topshiring, so\'ng "Mijozga yetkazdim" tugmasini suring.'
                : (arrivedAtRestaurant
                    ? 'Taom tayyor bo\'lishini kuting, so\'ng xodimga buyurtma raqamining oxirgi 4 xonasini '
                        'ayting va "Buyurtma olindi" tugmasini suring.'
                    : 'Restoranga yetib borgach "Yetib keldim" tugmasini suring, so\'ng taom tayyor '
                        'bo\'lishini kuting.'),
          ),
        ),
      ],
    );
  }
}

class _PaymentBanner extends StatelessWidget {
  const _PaymentBanner({required this.instruction});

  final PaymentInstruction instruction;

  @override
  Widget build(BuildContext context) {
    final color = switch (instruction.kind) {
      PaymentKind.collectCash => kWarning,
      PaymentKind.paidOnline => kSuccess,
      PaymentKind.unknown => kDanger,
    };
    final icon = switch (instruction.kind) {
      PaymentKind.collectCash => Icons.payments_outlined,
      PaymentKind.paidOnline => Icons.verified_outlined,
      PaymentKind.unknown => Icons.help_outline,
    };
    return Container(
      key: const ValueKey('courier-payment'),
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color.withValues(alpha: 0.45)),
      ),
      child: Row(
        children: [
          Icon(icon, color: color),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(instruction.title,
                    style: TextStyle(fontWeight: FontWeight.bold, fontSize: 15, color: color)),
                const SizedBox(height: 2),
                Text(instruction.subtitle, style: const TextStyle(fontSize: 12, color: kInkMuted)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Bosqich belgisi (restoran/mijoz) — joriy bosqich brend rangida.
class _StepIcon extends StatelessWidget {
  const _StepIcon({required this.child, required this.active});

  final Widget child;
  final bool active;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 34,
      height: 34,
      margin: const EdgeInsets.symmetric(horizontal: 2),
      alignment: Alignment.center,
      decoration: BoxDecoration(shape: BoxShape.circle, color: active ? kBrandColor : kLine),
      child: ClipOval(child: child),
    );
  }
}

class _CircleIconButton extends StatelessWidget {
  const _CircleIconButton({required this.icon, required this.onTap});

  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: kBrandColor.withValues(alpha: 0.12),
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(10),
          child: Icon(icon, color: kBrandColor, size: 20),
        ),
      ),
    );
  }
}

class _SupportRow extends StatelessWidget {
  const _SupportRow({required this.title, required this.subtitle, required this.onTap});

  final String title;
  final String? subtitle;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: const TextStyle(fontWeight: FontWeight.w600, color: kInk)),
                  if (subtitle != null) ...[
                    const SizedBox(height: 2),
                    Text(subtitle!, style: const TextStyle(fontSize: 12, color: kInkMuted)),
                  ],
                ],
              ),
            ),
            const Icon(Icons.help_outline, color: kInkFaint, size: 22),
          ],
        ),
      ),
    );
  }
}

/// Restoran logotipi (`logo_url`) yoki `assets/box.png`.
class _RestaurantLogoAvatar extends StatelessWidget {
  const _RestaurantLogoAvatar({required this.logoUrl});

  final String? logoUrl;

  @override
  Widget build(BuildContext context) {
    final url = logoUrl;
    return ClipRRect(
      borderRadius: BorderRadius.circular(6),
      child: SizedBox(
        width: 24,
        height: 24,
        child: (url != null && url.isNotEmpty)
            ? Image.network(
                fullImageUrl(url),
                fit: BoxFit.cover,
                // Katta logotip xotirani to'ldirmasin (kuryer auditi, 11-band).
                cacheWidth: 72,
                errorBuilder: (_, _, _) => Image.asset('assets/box.png', fit: BoxFit.cover),
              )
            : Image.asset('assets/box.png', fit: BoxFit.cover),
      ),
    );
  }
}

/// Profil: ism, restoran, holat, huquqiy hujjatlar, chiqish.
class ProfileTab extends StatelessWidget {
  const ProfileTab({
    super.key,
    required this.name,
    required this.restaurantName,
    required this.approved,
    required this.onLogout,
  });

  final String name;
  final String? restaurantName;
  final bool? approved;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final rest = restaurantName?.trim() ?? '';
    return Container(
      // Xarita shu tabning ORQASIDA ham chizilib turadi — qattiq fon shart.
      color: kSurface,
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Row(
              children: [
                const CircleAvatar(
                  radius: 28,
                  backgroundColor: kCardBg,
                  child: Icon(Icons.person, size: 32, color: kInk),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name.isEmpty ? 'Kuryer' : name,
                          style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold, color: kInk)),
                      const SizedBox(height: 2),
                      if (rest.isNotEmpty)
                        Text('$rest yetkazib beruvchisi', style: const TextStyle(color: kInkMuted)),
                      Text(
                        approved == true ? 'Ilovaga kirish ochiq' : 'Ilovaga kirish yopiq',
                        style: TextStyle(color: approved == true ? kSuccess : kWarning),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),
            const Divider(height: 1, color: kLine),
            // Huquqiy hujjatlar — maxfiylik siyosatiga havola MAJBURIY
            // (bug.md 70-band).
            const LegalSection(),
            const Divider(height: 1, color: kLine),
            ListTile(
              leading: const Icon(Icons.logout, color: kDanger),
              title: const Text('Chiqish', style: TextStyle(color: kDanger)),
              onTap: onLogout,
            ),
            // Nosozlik xabarida birinchi savol — "qaysi build?" (har reliz +1,
            // `scripts/version.ps1`).
            const SizedBox(height: 16),
            Center(
              child: Text(
                'OnDexGO $appVersionLabel',
                key: const ValueKey('courier-app-version'),
                style: const TextStyle(fontSize: 12, color: kInkFaint),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
