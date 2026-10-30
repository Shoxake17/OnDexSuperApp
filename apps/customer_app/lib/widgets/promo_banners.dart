import 'dart:async';

import 'package:flutter/material.dart';

import 'package:ondex_core/ondex_core.dart' show Repository;

import '../data/catalog_repository.dart';
import 'common.dart' show RemoteImage;

/// Admin panelidan boshqariladigan banner kartasi (`internal/banners`).
///
/// Rasm butun kartani egallaydi; ixtiyoriy matn qatlami (sarlavha, tavsif, tugma)
/// chap tomonda chiziladi. Rasmning o'zida matn bo'lsa admin matnni bo'sh
/// qoldiradi va faqat rasm ko'rinadi.
///
/// ┌─ "ISHLAYDIGANDEK" TUGMA YO'Q ─────────────────────────────────────────┐
/// Tugma FAQAT banner `action` ga ega bo'lsa chiziladi (server ham tugma
/// matnini havolasiz qabul qilmaydi). Bosilganda hech narsa qilmaydigan
/// tugma jimgina buzilgan funksiyadan yomonroq.
/// └───────────────────────────────────────────────────────────────────────┘
class PromoBannerCard extends StatelessWidget {
  const PromoBannerCard({
    super.key,
    required this.banner,
    this.onAction,
    this.radius = 20,
  });

  final Map<String, dynamic> banner;

  /// Banner `action` bilan bosilganda chaqiriladi. `null` — banner bosilmaydi.
  final void Function(String action)? onAction;
  final double radius;

  static const _ink = Color(0xFF111827);
  static const _muted = Color(0xFF6B7280);
  static const _brand = Color(0xFFF4511E);

  /// "OnDex" so'zi brend rangida ("On" to'q, "Dex" to'q sariq) — ilovadagi
  /// logotip yozuvi bilan bir xil.
  static List<InlineSpan> _brandSpans(String text) {
    final spans = <InlineSpan>[];
    var rest = text;
    while (true) {
      final i = rest.indexOf('OnDex');
      if (i < 0) break;
      if (i > 0) spans.add(TextSpan(text: rest.substring(0, i)));
      spans.add(const TextSpan(text: 'On'));
      spans.add(const TextSpan(text: 'Dex', style: TextStyle(color: _brand)));
      rest = rest.substring(i + 5);
    }
    if (rest.isNotEmpty) spans.add(TextSpan(text: rest));
    return spans;
  }

  @override
  Widget build(BuildContext context) {
    final image = (banner['image_url'] ?? '').toString();
    final title = (banner['title'] ?? '').toString();
    final subtitle = (banner['subtitle'] ?? '').toString();
    final button = (banner['button_text'] ?? '').toString();
    final action = (banner['action'] ?? '').toString();
    final tappable = onAction != null && action.isNotEmpty;
    final hasText =
        title.isNotEmpty || subtitle.isNotEmpty || button.isNotEmpty;

    Widget card = ClipRRect(
      borderRadius: BorderRadius.circular(radius),
      child: Stack(
        fit: StackFit.expand,
        children: [
          RemoteImage(
            url: image,
            fit: BoxFit.cover,
            placeholder: Container(color: const Color(0xFFFDF1E7)),
          ),
          if (hasText)
            LayoutBuilder(
              builder:
                  (context, c) => Padding(
                    padding: const EdgeInsets.fromLTRB(18, 14, 0, 14),
                    child: SizedBox(
                      width: c.maxWidth * 0.52,
                      height: c.maxHeight - 28,
                      // Matn qatlami kartaga sig'masa (tor ekran, uzun matn) —
                      // butun qatlam proporsional kichrayadi, overflow bo'lmaydi.
                      child: FittedBox(
                        fit: BoxFit.scaleDown,
                        alignment: Alignment.centerLeft,
                        child: SizedBox(
                          width: c.maxWidth * 0.52,
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              if (title.isNotEmpty)
                                Text.rich(
                                  TextSpan(children: _brandSpans(title)),
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(
                                    fontSize: 24,
                                    height: 1.1,
                                    fontWeight: FontWeight.w800,
                                    color: _ink,
                                  ),
                                ),
                              if (subtitle.isNotEmpty) ...[
                                const SizedBox(height: 6),
                                Text(
                                  subtitle,
                                  maxLines: 3,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(
                                    fontSize: 12.5,
                                    height: 1.3,
                                    color: _muted,
                                    fontWeight: FontWeight.w500,
                                  ),
                                ),
                              ],
                              if (button.isNotEmpty && action.isNotEmpty) ...[
                                const SizedBox(height: 10),
                                Container(
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 14,
                                    vertical: 8,
                                  ),
                                  decoration: BoxDecoration(
                                    color: _brand,
                                    borderRadius: BorderRadius.circular(18),
                                  ),
                                  child: Row(
                                    mainAxisSize: MainAxisSize.min,
                                    children: [
                                      Flexible(
                                        child: Text(
                                          button,
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                          style: const TextStyle(
                                            color: Colors.white,
                                            fontSize: 12.5,
                                            fontWeight: FontWeight.w700,
                                          ),
                                        ),
                                      ),
                                      const SizedBox(width: 5),
                                      const Icon(
                                        Icons.arrow_forward,
                                        size: 14,
                                        color: Colors.white,
                                      ),
                                    ],
                                  ),
                                ),
                              ],
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
            ),
        ],
      ),
    );

    if (tappable) {
      card = InkWell(
        borderRadius: BorderRadius.circular(radius),
        onTap: () => onAction!(action),
        child: card,
      );
    }
    return card;
  }
}

/// Bir joylashuvdagi (`menu` / `store`) bannerlar ro'yxati — serverdan keladi.
///
/// Birorta FAOL banner bo'lmasa vidjet BUTUNLAY yashiriladi (bo'sh joy yoki
/// "yuklanmoqda" aylanmasi "nimadir buzildi" taassurotini bermasin) — yoki
/// [fallback] chiziladi.
///
/// [carousel] — bitta qatorda surib ko'riladigan, 5 soniyada almashadigan
/// (do'kon tepasi); aks holda ketma-ket ustma-ust (bosh sahifa).
class PromoBanners extends StatefulWidget {
  const PromoBanners({
    super.key,
    required this.placement,
    required this.aspectRatio,
    this.onAction,
    this.carousel = false,
    this.fallback,
    this.spacing = 14,
    this.bottomGap = 0,
    this.tick = 0,
    this.repository,
  });

  final String placement;
  final double aspectRatio;
  final void Function(String action)? onAction;
  final bool carousel;

  /// Faol banner yo'q (yoki hali kelmagan/xato) bo'lganda ko'rsatiladi.
  final Widget? fallback;
  final double spacing;

  /// Bannerlar BOR bo'lsagina ularning ostiga qo'shiladigan bo'shliq (bo'sh bo'lsa
  /// vidjet butunlay yo'qoladi va bo'sh joy ham qolmaydi).
  final double bottomGap;

  /// O'zgarsa ro'yxat serverdan majburan yangilanadi (pull-to-refresh).
  final int tick;

  /// Faqat sinov uchun: ombor/kesh o'rniga berilgan manba.
  @visibleForTesting
  final Repository<List<dynamic>>? repository;

  @override
  State<PromoBanners> createState() => _PromoBannersState();
}

class _PromoBannersState extends State<PromoBanners> {
  final _page = PageController();
  Timer? _timer;
  int _count = 0;
  int _index = 0;

  @override
  void initState() {
    super.initState();
    if (widget.carousel) {
      _timer = Timer.periodic(const Duration(seconds: 5), (_) {
        if (!_page.hasClients || _count <= 1) return;
        final next = ((_page.page?.round() ?? 0) + 1) % _count;
        _page.animateToPage(
          next,
          duration: const Duration(milliseconds: 400),
          curve: Curves.easeInOut,
        );
      });
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    _page.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final repo = widget.repository ?? Repos.bannersFor(widget.placement);
    return StreamBuilder(
      key: ValueKey('banners-${widget.placement}-${widget.tick}'),
      stream: repo.observe(force: widget.tick > 0),
      builder: (context, snap) {
        final list =
            (snap.data?.value ?? const <dynamic>[])
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
        _count = list.length;
        if (list.isEmpty) return widget.fallback ?? const SizedBox.shrink();

        if (!widget.carousel) {
          return Column(
            children: [
              for (var i = 0; i < list.length; i++) ...[
                if (i > 0) SizedBox(height: widget.spacing),
                AspectRatio(
                  aspectRatio: widget.aspectRatio,
                  child: PromoBannerCard(
                    banner: list[i],
                    onAction: widget.onAction,
                  ),
                ),
              ],
              if (widget.bottomGap > 0) SizedBox(height: widget.bottomGap),
            ],
          );
        }

        return Column(
          children: [
            AspectRatio(
              aspectRatio: widget.aspectRatio,
              child: PageView.builder(
                controller: _page,
                itemCount: list.length,
                onPageChanged: (i) => setState(() => _index = i),
                itemBuilder:
                    (_, i) => PromoBannerCard(
                      banner: list[i],
                      onAction: widget.onAction,
                      radius: 22,
                    ),
              ),
            ),
            if (list.length > 1) ...[
              const SizedBox(height: 8),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  for (var i = 0; i < list.length; i++)
                    AnimatedContainer(
                      duration: const Duration(milliseconds: 200),
                      margin: const EdgeInsets.symmetric(horizontal: 3),
                      width: i == _index ? 18 : 6,
                      height: 6,
                      decoration: BoxDecoration(
                        color:
                            i == _index
                                ? PromoBannerCard._brand
                                : const Color(0xFFD1D5DB),
                        borderRadius: BorderRadius.circular(3),
                      ),
                    ),
                ],
              ),
            ],
          ],
        );
      },
    );
  }
}
