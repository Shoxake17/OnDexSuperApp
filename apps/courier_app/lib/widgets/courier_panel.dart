import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

import '../api.dart';
import '../order_view.dart';
import '../theme.dart';

/// Bola widget'ning HAQIQIY o'lchami o'zgarganda xabar beradi.
///
/// Xarita tugmalari va xaritaning pastki chegarasi (`GoogleMap.padding`)
/// panel balandligiga bog'lanadi. Avval panel o'lchami `GlobalKey` +
/// `addPostFrameCallback` bilan 15 joyda qo'lda qayta hisoblanardi, xarita
/// tugmalari esa ekranning qat'iy 14% ida turib, panel ostida qolardi.
class SizeReporter extends SingleChildRenderObjectWidget {
  const SizeReporter({super.key, required this.onSize, super.child});

  final ValueChanged<Size> onSize;

  @override
  RenderSizeReporter createRenderObject(BuildContext context) => RenderSizeReporter(onSize);

  @override
  void updateRenderObject(BuildContext context, RenderSizeReporter renderObject) {
    renderObject.onSize = onSize;
  }
}

class RenderSizeReporter extends RenderProxyBox {
  RenderSizeReporter(this.onSize);

  ValueChanged<Size> onSize;
  Size? _last;

  @override
  void performLayout() {
    super.performLayout();
    final s = size;
    if (s == _last) return;
    _last = s;
    // Layout paytida `setState` qilib bo'lmaydi — keyingi frame'da.
    WidgetsBinding.instance.addPostFrameCallback((_) => onSize(s));
  }
}

/// Panel tepasidagi holat qatori (image/Kuryer.png): chapda power tugmasi,
/// o'rtada "Liniyada" / "Liniyada emas".
///
/// Power rangi: liniyada — brend rangi, aks holda kulrang. Kirish yopiq
/// (restoran "Xodimlar" bo'limida yopgan) bo'lsa tugma bosilmaydi.
class CourierStatusHeader extends StatelessWidget {
  const CourierStatusHeader({
    super.key,
    required this.online,
    required this.enabled,
    required this.busy,
    required this.onPowerTap,
  });

  final bool online;
  final bool enabled;
  final bool busy;
  final VoidCallback onPowerTap;

  static const powerSize = 52.0;

  @override
  Widget build(BuildContext context) {
    final active = online && enabled;
    final color = active ? kBrandColor : kInkFaint;
    final subtitle = !enabled
        ? 'Ilovaga kirish yopiq'
        : (online ? 'Buyurtmalar qabul qilinmoqda' : 'Buyurtma olish uchun liniyaga chiqing');
    return Row(
      children: [
        Semantics(
          button: true,
          enabled: enabled && !busy,
          label: online ? 'Liniyadan chiqish' : 'Liniyaga chiqish',
          child: Material(
            color: color.withValues(alpha: active ? 0.12 : 0.10),
            shape: const CircleBorder(),
            child: InkWell(
              key: const ValueKey('courier-power'),
              customBorder: const CircleBorder(),
              onTap: enabled && !busy ? onPowerTap : null,
              child: SizedBox(
                width: powerSize,
                height: powerSize,
                child: Center(
                  child: busy
                      ? SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(strokeWidth: 2.5, color: color),
                        )
                      : Icon(
                          Icons.power_settings_new_rounded,
                          key: const ValueKey('courier-power-icon'),
                          size: 30,
                          color: color,
                        ),
                ),
              ),
            ),
          ),
        ),
        Expanded(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: 10,
                    height: 10,
                    decoration: BoxDecoration(color: color, shape: BoxShape.circle),
                  ),
                  const SizedBox(width: 8),
                  Flexible(
                    child: Text(
                      online ? 'Liniyada' : 'Liniyada emas',
                      key: const ValueKey('courier-status-title'),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: kInk),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 2),
              Text(
                subtitle,
                textAlign: TextAlign.center,
                maxLines: 2,
                style: const TextStyle(fontSize: 13, color: kInkMuted),
              ),
            ],
          ),
        ),
        // Namunadagi simmetriya: sarlavha ekran o'rtasida tursin.
        const SizedBox(width: powerSize),
      ],
    );
  }
}

/// "Restoran" va "Naqd" kartalari.
class CourierInfoCards extends StatelessWidget {
  const CourierInfoCards({
    super.key,
    required this.restaurantName,
    required this.activeOrder,
    this.restaurantLogoUrl,
  });

  /// Kuryer ishlaydigan restoran (kuryer yozuvidagi `restaurant_id` dan).
  final String? restaurantName;

  /// O'sha restoranning logosi — ikonka o'rnida. Faqat restoranning o'z
  /// kuryerlarida bo'ladi (platforma kuryerida restoran yo'q).
  final String? restaurantLogoUrl;
  final Map<String, dynamic>? activeOrder;

  @override
  Widget build(BuildContext context) {
    final name = restaurantName?.trim() ?? '';
    final cash = cashSummaryFor(activeOrder);
    final cashColor = switch (cash.kind) {
      PaymentKind.collectCash => kWarning,
      PaymentKind.paidOnline => kSuccess,
      PaymentKind.unknown => kDanger,
      null => kInk,
    };
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: _InfoCard(
            key: const ValueKey('courier-card-restaurant'),
            icon: Icons.storefront_rounded,
            label: 'Restoran',
            value: name.isEmpty ? '—' : name,
            logoUrl: restaurantLogoUrl,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _InfoCard(
            key: const ValueKey('courier-card-cash'),
            icon: Icons.account_balance_wallet_rounded,
            label: cash.label,
            value: cash.value,
            valueColor: cashColor,
          ),
        ),
      ],
    );
  }
}

class _InfoCard extends StatelessWidget {
  const _InfoCard({
    super.key,
    required this.icon,
    required this.label,
    required this.value,
    this.valueColor = kInk,
    this.logoUrl,
  });

  final IconData icon;
  final String label;
  final String value;
  final Color valueColor;

  /// Berilsa ikonka o'rniga shu rasm (restoran logosi); yuklanmasa ikonka.
  final String? logoUrl;

  Widget _iconCircle() => Container(
        width: 40,
        height: 40,
        decoration: const BoxDecoration(color: kInk, shape: BoxShape.circle),
        child: Icon(icon, color: kSurface, size: 20),
      );

  Widget _leading() {
    final url = logoUrl?.trim() ?? '';
    if (url.isEmpty) return _iconCircle();
    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        color: kSurface,
        shape: BoxShape.circle,
        border: Border.all(color: kLine),
      ),
      child: ClipOval(
        child: Image.network(
          fullImageUrl(url),
          key: const ValueKey('courier-restaurant-logo'),
          fit: BoxFit.cover,
          // Katta logotip xotirani to'ldirmasin (kuryer auditi, 11-band).
          cacheWidth: 120,
          errorBuilder: (_, _, _) => _iconCircle(),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: kCardBg, borderRadius: BorderRadius.circular(16)),
      child: Row(
        children: [
          _leading(),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12.5, color: kInkMuted)),
                const SizedBox(height: 2),
                Text(value,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 15.5, fontWeight: FontWeight.w700, color: valueColor)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Xarita ustidagi pastki panel.
///
/// ┌─ NEGA DraggableScrollableSheet EMAS ───────────────────────────────┐
/// Avvalgi panel qat'iy ulushlarda (0.14–0.5) ishlardi va har kontent
/// o'zgarishida balandlik QO'LDA o'lchanib `animateTo` qilinardi: panel
/// "sakrardi", oflaynda butunlay yo'qolardi, amal tugmasini tortib panelni
/// surib bo'lmasdi. Endi panel mazmuniga qarab o'zi o'lchanadi (eng ko'pi
/// `maxHeight`), uzun mazmun ichkarida aylanadi, amal tugmasi doim pastda.
/// Tutqichni bosish yoki pastga tortish — yig'ish (faqat holat qatori va
/// tugma qoladi), yuqoriga tortish — kengaytirish.
/// └────────────────────────────────────────────────────────────────────┘
class CourierBottomPanel extends StatelessWidget {
  const CourierBottomPanel({
    super.key,
    required this.header,
    required this.expanded,
    required this.onExpandedChanged,
    required this.maxHeight,
    this.cards,
    this.body,
    this.footer,
  });

  final Widget header;
  final Widget? cards;
  final Widget? body;
  final Widget? footer;
  final bool expanded;
  final ValueChanged<bool> onExpandedChanged;
  final double maxHeight;

  void _onDragEnd(DragEndDetails d) {
    final v = d.primaryVelocity ?? 0;
    if (v > 150) onExpandedChanged(false);
    if (v < -150) onExpandedChanged(true);
  }

  @override
  Widget build(BuildContext context) {
    final hasDetails = cards != null || body != null;
    return ConstrainedBox(
      constraints: BoxConstraints(maxHeight: maxHeight),
      child: Material(
        color: kSurface,
        elevation: 12,
        shadowColor: Colors.black26,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(24)),
        clipBehavior: Clip.antiAlias,
        child: SafeArea(
          top: false,
          child: AnimatedSize(
            duration: const Duration(milliseconds: 200),
            curve: Curves.easeOut,
            alignment: Alignment.bottomCenter,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                GestureDetector(
                  key: const ValueKey('courier-panel-grip'),
                  behavior: HitTestBehavior.opaque,
                  onTap: hasDetails ? () => onExpandedChanged(!expanded) : null,
                  onVerticalDragEnd: hasDetails ? _onDragEnd : null,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(20, 10, 20, 8),
                    child: Center(
                      child: Container(
                        width: 44,
                        height: 5,
                        decoration: BoxDecoration(color: kLine, borderRadius: BorderRadius.circular(3)),
                      ),
                    ),
                  ),
                ),
                GestureDetector(
                  behavior: HitTestBehavior.translucent,
                  onVerticalDragEnd: hasDetails ? _onDragEnd : null,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
                    child: header,
                  ),
                ),
                if (expanded && hasDetails)
                  Flexible(
                    child: SingleChildScrollView(
                      key: const ValueKey('courier-panel-details'),
                      padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          if (cards != null) ...[cards!, const SizedBox(height: 14)],
                          ?body,
                        ],
                      ),
                    ),
                  ),
                if (footer != null)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 4, 16, 16),
                    child: footer,
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
