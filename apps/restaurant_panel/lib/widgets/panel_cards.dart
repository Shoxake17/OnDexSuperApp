import 'package:flutter/material.dart';

import '../theme.dart';

/// Hisobot sahifalarining UMUMIY qurilish bloklari.
///
/// ┌─ NEGA BU FAYL ────────────────────────────────────────────────────┐
/// Bu vidjetlar "Statistika" sahifasi ichida yopiq (`_Panel`, `_KpiCard`
/// va h.k.) yozilgan edi. "Moliya" sahifasi xuddi shu maketda: KPI
/// kartochkalari, grafik kartochkasi, jadval, davr tugmasi. Ularni
/// nusxalash `page_header.dart` da tasvirlangan xatoni takrorlardi —
/// ikki sahifa asta-sekin bir-biridan farq qila boshlaydi (bir joyda
/// kartochka chekinishi 14, boshqasida 16 va hokazo).
///
/// Shuning uchun BIR MARTA shu yerda. Sahifa faqat ma'lumot va matn
/// beradi.
/// └───────────────────────────────────────────────────────────────────┘

/// Kartochkalar orasidagi masofa — hamma hisobot sahifasida bir xil.
const kPanelGap = 14.0;

/// Shundan tor bo'lganda juft kartochkalar ustma-ust tushadi.
const kSplitBreakpoint = 1000.0;

/// Oq kartochka — barcha bloklarning asosi.
class PanelCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;

  const PanelCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.fromLTRB(16, 14, 16, 14),
  });

  @override
  Widget build(BuildContext context) => Container(
        padding: padding,
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: child,
      );
}

/// Elementlarni sig'adigan qadar teng ustunlarga taqsimlaydi.
///
/// Kenglik PASTGA yaxlitlanadi: aks holda suzuvchi nuqta xatosi tufayli
/// ustunlar yig'indisi konteynerdan bir necha pikselga oshib, oxirgi
/// kartochka sababsiz keyingi qatorga tushib ketardi.
class ResponsiveWrap extends StatelessWidget {
  final double minItemWidth;
  final double spacing;
  final List<Widget> children;

  const ResponsiveWrap({
    super.key,
    required this.minItemWidth,
    required this.spacing,
    required this.children,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final columns = ((c.maxWidth + spacing) / (minItemWidth + spacing))
          .floor()
          .clamp(1, children.length);
      final width =
          ((c.maxWidth - spacing * (columns - 1)) / columns).floorToDouble();
      return Wrap(
        spacing: spacing,
        runSpacing: spacing,
        children: [
          for (final child in children) SizedBox(width: width, child: child)
        ],
      );
    });
  }
}

/// Qat'iy balandlikdagi ikki kartochka yonma-yon (namunadagi nisbatda);
/// tor ekranda ustma-ust.
class SplitRow extends StatelessWidget {
  final double height;
  final int leftFlex;
  final int rightFlex;
  final Widget left;
  final Widget right;

  const SplitRow({
    super.key,
    required this.height,
    required this.leftFlex,
    required this.rightFlex,
    required this.left,
    required this.right,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      if (c.maxWidth >= kSplitBreakpoint) {
        return SizedBox(
          height: height,
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(flex: leftFlex, child: left),
              const SizedBox(width: kPanelGap),
              Expanded(flex: rightFlex, child: right),
            ],
          ),
        );
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(height: height, child: left),
          const SizedBox(height: kPanelGap),
          SizedBox(height: height, child: right),
        ],
      );
    });
  }
}

class HintIcon extends StatelessWidget {
  final String message;

  const HintIcon({super.key, required this.message});

  @override
  Widget build(BuildContext context) => Tooltip(
        message: message,
        waitDuration: const Duration(milliseconds: 250),
        child: const Icon(Icons.info_outline_rounded,
            size: 15, color: OnDexColors.inkFaint),
      );
}

class CardTitle extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? hint;
  final Widget? trailing;

  const CardTitle({
    super.key,
    required this.icon,
    required this.title,
    this.hint,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    // Sarlavha ICHKI qatorda, butun qolgan joy `Expanded` da: aks holda
    // `Flexible` + `Spacer` bo'sh joyni teng bo'lib, o'ngdagi tugma chetga
    // yetmay qolardi (`top_bar.dart` dagi xuddi shu xato).
    return SizedBox(
      height: 32,
      child: Row(
        children: [
          Expanded(
            child: Row(
              children: [
                Icon(icon, size: 19, color: OnDexColors.primary),
                const SizedBox(width: 8),
                Flexible(
                  child: Text(title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w700,
                          color: OnDexColors.ink)),
                ),
                if (hint != null) ...[
                  const SizedBox(width: 6),
                  HintIcon(message: hint!),
                ],
              ],
            ),
          ),
          if (trailing != null) ...[const SizedBox(width: 8), trailing!],
        ],
      ),
    );
  }
}

class EmptyText extends StatelessWidget {
  final String text;

  const EmptyText(this.text, {super.key});

  @override
  Widget build(BuildContext context) => Center(
        child: Text(text,
            textAlign: TextAlign.center,
            style: const TextStyle(color: OnDexColors.inkDim, fontSize: 13)),
      );
}

/// Qat'iy kenglikdagi jadval katagi. Matn sig'masa KESILMAYDI, biroz
/// kichrayadi — summa va sonlar to'liq o'qilishi shart.
Widget panelCell(double width, Widget child) => SizedBox(
      width: width,
      child: Align(
        alignment: Alignment.centerLeft,
        child: FittedBox(
            fit: BoxFit.scaleDown,
            alignment: Alignment.centerLeft,
            child: child),
      ),
    );

/// Jadval sarlavhasi uchun matn uslubi.
const kTableHeadStyle = TextStyle(
    fontSize: 11.5, color: OnDexColors.inkFaint, fontWeight: FontWeight.w600);

// ─── O'zgarish foizi ────────────────────────────────────────────────────

/// Joriy va oldingi davr solishtiruvi.
///
/// ⚠️ O'zi HISOBLAYDI: "+12%" kabi raqam hech qachon qo'lda yozilmaydi.
/// Solishtirib bo'lmaydigan hollarda (oldingi davr noli, ma'lumot yo'q)
/// foiz o'rniga SABAB yoziladi — "+∞%" yoki soxta o'sish ko'rsatilmaydi.
class Delta {
  final num current;
  final num previous;

  /// Qaysi yo'nalish yaxshi: tushum o'sishi yaxshi, qaytarishlar yoki
  /// tayyorlash vaqti o'sishi — yomon.
  final bool higherIsBetter;
  final bool available;

  /// Solishtirib bo'lmaganda foiz o'rniga yoziladigan sabab.
  final String reason;

  const Delta(this.current, this.previous, {this.higherIsBetter = true})
      : available = true,
        reason = '';

  const Delta.missing(this.reason)
      : current = 0,
        previous = 0,
        higherIsBetter = true,
        available = false;

  static const none = Delta.missing('O\'tgan davrda ma\'lumot yo\'q');
}

class DeltaLine extends StatelessWidget {
  final Delta delta;

  const DeltaLine({super.key, required this.delta});

  static const _rest = TextStyle(fontSize: 11, color: OnDexColors.inkDim);

  @override
  Widget build(BuildContext context) {
    if (!delta.available) {
      return Text(delta.reason,
          maxLines: 1, overflow: TextOverflow.ellipsis, style: _rest);
    }
    if (delta.previous == 0) {
      // Nolga bo'lib foiz chiqarib bo'lmaydi — "+∞%" o'rniga halol matn.
      return Text(
          delta.current == 0
              ? 'O\'tgan davr bilan bir xil'
              : 'O\'tgan davrda ma\'lumot yo\'q',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: _rest);
    }
    final pct = (delta.current - delta.previous) / delta.previous.abs() * 100;
    final rounded = pct.round();
    if (rounded == 0) {
      return const Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.remove_rounded, size: 12, color: OnDexColors.inkDim),
          SizedBox(width: 3),
          Flexible(
            child: Text('0% o\'tgan davrga',
                maxLines: 1, overflow: TextOverflow.ellipsis, style: _rest),
          ),
        ],
      );
    }
    final up = pct > 0;
    final good = up == delta.higherIsBetter;
    final color = good ? OnDexColors.success : OnDexColors.danger;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(up ? Icons.arrow_upward_rounded : Icons.arrow_downward_rounded,
            size: 12, color: color),
        const SizedBox(width: 3),
        Flexible(
          child: Text.rich(
            TextSpan(children: [
              TextSpan(
                text: '${up ? '+' : '-'}${rounded.abs()}%',
                style: TextStyle(color: color, fontWeight: FontWeight.w700),
              ),
              const TextSpan(text: ' o\'tgan davrga'),
            ]),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: _rest,
          ),
        ),
      ],
    );
  }
}

// ─── Kartochkalar ───────────────────────────────────────────────────────

class KpiCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  final Color color;
  final Color background;

  /// `null` — solishtiriladigan davr yo'q (masalan butun tarix); o'rniga
  /// [caption] yoziladi.
  final Delta? delta;
  final String? caption;
  final String? hint;

  /// Qo'shimcha izoh (masalan "+144 000 so'm jarayonda (2 ta)").
  final String? note;

  /// Izoh qatori uchun joy ajratilsinmi — bitta qatordagi kartochkalar
  /// bir xil balandlikda turishi uchun, izohi bo'lmaganlarida ham.
  final bool reserveNote;

  const KpiCard({
    super.key,
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
    required this.background,
    required this.delta,
    this.caption,
    this.hint,
    this.note,
    this.reserveNote = false,
  });

  @override
  Widget build(BuildContext context) {
    final badge = Container(
      width: 38,
      height: 38,
      decoration: BoxDecoration(
          color: background, borderRadius: BorderRadius.circular(10)),
      child: Icon(icon, size: 20, color: color),
    );
    // Ikonka CHAPDA (namunadagi kabi): tepada alohida qator egallamaydi,
    // kartochka balandligi shu sababli deyarli uchdan biriga kamaydi.
    return PanelCard(
      padding: const EdgeInsets.fromLTRB(14, 12, 12, 12),
      child: Row(
        children: [
          if (hint == null)
            badge
          else
            Tooltip(
              message: hint!,
              waitDuration: const Duration(milliseconds: 250),
              child: badge,
            ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 12.5,
                        color: OnDexColors.inkDim,
                        fontWeight: FontWeight.w500)),
                const SizedBox(height: 3),
                // Katta summa tor kartochkaga sig'masa kesilmaydi —
                // kichrayadi ("24 180 000 so'm" to'liq o'qilishi SHART).
                SizedBox(
                  width: double.infinity,
                  child: FittedBox(
                    fit: BoxFit.scaleDown,
                    alignment: Alignment.centerLeft,
                    child: Text(value,
                        maxLines: 1,
                        style: const TextStyle(
                            fontSize: 19,
                            fontWeight: FontWeight.w800,
                            color: OnDexColors.ink)),
                  ),
                ),
                const SizedBox(height: 3),
                if (delta != null)
                  DeltaLine(delta: delta!)
                else
                  Text(caption ?? '',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 11, color: OnDexColors.inkDim)),
                if (reserveNote) ...[
                  const SizedBox(height: 3),
                  SizedBox(
                    height: 14,
                    child: note == null
                        ? null
                        : Text(note!,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                fontSize: 11,
                                color: OnDexColors.amber,
                                fontWeight: FontWeight.w600)),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Davr tanlash tugmasi — ko'rinishi yuqori paneldagi sana tugmasi bilan
/// bir xil.
class RangeButton extends StatelessWidget {
  final String label;
  final VoidCallback onTap;

  const RangeButton({super.key, required this.label, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          border: Border.all(color: OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.calendar_today_rounded,
                size: 15, color: OnDexColors.inkDim),
            const SizedBox(width: 9),
            Flexible(
              child: Text(label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 13,
                      color: OnDexColors.ink,
                      fontWeight: FontWeight.w600)),
            ),
            const SizedBox(width: 6),
            const Icon(Icons.keyboard_arrow_down_rounded,
                size: 17, color: OnDexColors.inkDim),
          ],
        ),
      ),
    );
  }
}

/// Butun sahifa o'rniga ko'rsatiladigan xato (birinchi yuklash yiqildi).
class ErrorPanel extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  final IconData icon;

  /// "Qayta urinish" yonidagi qo'shimcha amal.
  ///
  /// Zarur: ba'zi xatolarni qayta urinish TUZATMAYDI — masalan noto'g'ri
  /// kiritilgan to'lov do'koni raqami. Bunday holatda foydalanuvchiga
  /// sabab tuzatiladigan joyga o'tish yo'li berilishi kerak.
  final Widget? action;

  const ErrorPanel({
    super.key,
    required this.message,
    required this.onRetry,
    this.icon = Icons.bar_chart_rounded,
    this.action,
  });

  @override
  Widget build(BuildContext context) {
    return PanelCard(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 48),
      child: Column(
        children: [
          Icon(icon, size: 40, color: OnDexColors.inkFaint),
          const SizedBox(height: 12),
          Text(message,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 14, color: OnDexColors.ink)),
          const SizedBox(height: 16),
          Wrap(
            spacing: 10,
            runSpacing: 8,
            alignment: WrapAlignment.center,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              FilledButton(
                  onPressed: onRetry, child: const Text('Qayta urinish')),
              if (action != null) action!,
            ],
          ),
        ],
      ),
    );
  }
}

/// Eski ma'lumot ekranda turganda chiqadigan xato satri — sahifa
/// yiqilmaydi, faqat yangilash muvaffaqiyatsiz bo'lgani aytiladi.
class ErrorBanner extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const ErrorBanner({super.key, required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 8, 8, 8),
      decoration: BoxDecoration(
        color: OnDexColors.dangerBg,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline_rounded,
              size: 18, color: OnDexColors.danger),
          const SizedBox(width: 10),
          Expanded(
            child: Text(message,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          TextButton(onPressed: onRetry, child: const Text('Qayta urinish')),
        ],
      ),
    );
  }
}

// ─── Halqa grafik afsonasi ──────────────────────────────────────────────

class LegendEntry {
  final String label;
  final num value;
  final Color color;

  const LegendEntry(this.label, this.value, this.color);
}

/// Afsona satri: rang · nom · qiymat · ulush.
///
/// [valueText] TASHQARIDAN beriladi — bitta joyda "12 ta", boshqasida
/// "1 200 000 so'm" bo'ladi va bu vidjet birlikni bilmasligi kerak.
class LegendLine extends StatelessWidget {
  final LegendEntry entry;
  final num total;
  final String valueText;

  /// Qiymat ustunining kengligi — summa "ta" dan uzunroq.
  final double valueWidth;

  const LegendLine({
    super.key,
    required this.entry,
    required this.total,
    required this.valueText,
    this.valueWidth = 58,
  });

  @override
  Widget build(BuildContext context) {
    const numbers = TextStyle(fontSize: 12.5, color: OnDexColors.inkDim);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Container(
            width: 10,
            height: 10,
            decoration:
                BoxDecoration(color: entry.color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(entry.label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          const SizedBox(width: 8),
          panelCell(valueWidth, Text(valueText, style: numbers)),
          panelCell(
              42,
              Text(total <= 0 ? '0%' : '${(entry.value * 100 / total).round()}%',
                  style: numbers)),
        ],
      ),
    );
  }
}
