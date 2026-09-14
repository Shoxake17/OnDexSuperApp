import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../theme.dart';

/// Grafiklardagi bitta nuqta.
class ChartPoint {
  /// X o'qi ostidagi qisqa yorliq (masalan "24-iyun").
  final String axisLabel;
  final double value;

  /// Sichqoncha nuqta ustiga kelganda chiqadigan quti: sarlavha (sana)
  /// va qiymat matni.
  final String tooltipTitle;
  final String tooltipValue;

  const ChartPoint({
    required this.axisLabel,
    required this.value,
    required this.tooltipTitle,
    required this.tooltipValue,
  });
}

/// Silliq chiziqli grafik: gradient maydon, nuqtalar va hover tooltip.
///
/// Tashqi kutubxonasiz, `CustomPainter` bilan chiziladi (loyihada grafik
/// kutubxonasi yo'q va bitta grafik uchun yangi bog'liqlik qo'shilmaydi).
/// Rang, o'q yorlig'i va butun sonli o'q parametr sifatida beriladi —
/// shu sabab tushum ham, buyurtmalar soni ham bitta vidjet bilan chiziladi.
class OnDexLineChart extends StatefulWidget {
  final List<ChartPoint> points;
  final Color color;
  final String Function(double value) axisLabel;

  /// Y o'qi tepasidagi birlik yozuvi (masalan "so'm"). `null` — yozilmaydi.
  final String? unitLabel;

  /// O'q qadamlari butun son bo'lsin — buyurtmalar soni "2.5" bo'lmaydi.
  final bool integerAxis;

  /// Barcha qiymatlar nol bo'lganda ko'rsatiladigan matn.
  final String emptyText;

  const OnDexLineChart({
    super.key,
    required this.points,
    required this.color,
    required this.axisLabel,
    required this.emptyText,
    this.unitLabel,
    this.integerAxis = false,
  });

  @override
  State<OnDexLineChart> createState() => _OnDexLineChartState();
}

class _OnDexLineChartState extends State<OnDexLineChart> {
  int? _hover;

  @override
  void didUpdateWidget(covariant OnDexLineChart oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Yangi ma'lumotda nuqtalar kamaygan bo'lishi mumkin — eski indeks
    // chegaradan chiqib qolmasin.
    if (_hover != null && _hover! >= widget.points.length) _hover = null;
  }

  @override
  Widget build(BuildContext context) {
    final points = widget.points;
    if (points.isEmpty || points.every((p) => p.value <= 0)) {
      return Center(
        child: Text(widget.emptyText,
            style: const TextStyle(color: OnDexColors.inkDim, fontSize: 13)),
      );
    }
    return LayoutBuilder(builder: (context, constraints) {
      final size = Size(constraints.maxWidth, constraints.maxHeight);
      void update(Offset local) {
        final i = _LineChartPainter.indexAt(local.dx, size.width, points.length);
        if (i != _hover) setState(() => _hover = i);
      }

      return MouseRegion(
        onHover: (e) => update(e.localPosition),
        onExit: (_) {
          if (_hover != null) setState(() => _hover = null);
        },
        // Sensorli ekran uchun: bosish va gorizontal surish ham tooltip
        // ochadi (sichqonchasiz qurilmada `onHover` hech qachon kelmaydi).
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTapDown: (d) => update(d.localPosition),
          onHorizontalDragUpdate: (d) => update(d.localPosition),
          child: CustomPaint(
            size: size,
            painter: _LineChartPainter(
              points: points,
              color: widget.color,
              axisLabel: widget.axisLabel,
              unitLabel: widget.unitLabel,
              integerAxis: widget.integerAxis,
              hover: _hover,
            ),
          ),
        ),
      );
    });
  }
}

class _LineChartPainter extends CustomPainter {
  final List<ChartPoint> points;
  final Color color;
  final String Function(double value) axisLabel;
  final String? unitLabel;
  final bool integerAxis;
  final int? hover;

  _LineChartPainter({
    required this.points,
    required this.color,
    required this.axisLabel,
    required this.unitLabel,
    required this.integerAxis,
    required this.hover,
  });

  static const _left = 46.0;
  static const _right = 8.0;
  static const _bottom = 24.0;
  static const _unitHeight = 18.0;
  static const _gridLines = 4;

  static const _labelStyle =
      TextStyle(fontSize: 10.5, color: OnDexColors.inkFaint);

  /// Kursor x-koordinatasiga ENG YAQIN nuqta. `paint` dagi x formulasi
  /// bilan bir xil — geometriya ikki joyda ajralib ketmasin.
  static int? indexAt(double dx, double width, int n) {
    if (n == 0) return null;
    final chartWidth = width - _left - _right;
    if (chartWidth <= 0) return null;
    if (n == 1) return 0;
    final step = chartWidth / (n - 1);
    final raw = (dx - _left) / step;
    if (raw < -0.5 || raw > n - 0.5) return null;
    return raw.round().clamp(0, n - 1);
  }

  @override
  void paint(Canvas canvas, Size size) {
    final top = unitLabel == null ? 6.0 : _unitHeight + 4;
    final chart =
        Rect.fromLTRB(_left, top, size.width - _right, size.height - _bottom);
    if (chart.width <= 0 || chart.height <= 0) return;

    final maxValue = points.fold<double>(0, (m, p) => math.max(m, p.value));
    var step = _niceStep(maxValue / _gridLines);
    if (integerAxis) step = math.max(1, step.ceilToDouble());
    final topValue = step * _gridLines;

    if (unitLabel != null) _layout(unitLabel!, _labelStyle).paint(canvas, Offset.zero);

    final gridPaint = Paint()
      ..color = OnDexColors.cardBorder
      ..strokeWidth = 1;
    for (var i = 0; i <= _gridLines; i++) {
      final y = chart.bottom - chart.height * i / _gridLines;
      _dashedLine(canvas, Offset(chart.left, y), chart.right, gridPaint);
      final tp = _layout(axisLabel(step * i), _labelStyle);
      tp.paint(canvas, Offset(_left - 8 - tp.width, y - tp.height / 2));
    }

    Offset positionOf(int i) {
      final x = points.length == 1
          ? chart.center.dx
          : chart.left + chart.width * i / (points.length - 1);
      final ratio = (points[i].value / topValue).clamp(0.0, 1.0);
      return Offset(x, chart.bottom - chart.height * ratio);
    }

    final pts = [for (var i = 0; i < points.length; i++) positionOf(i)];

    if (pts.length > 1) {
      final line = _smoothPath(pts, chart.top, chart.bottom);
      final area = Path.from(line)
        ..lineTo(pts.last.dx, chart.bottom)
        ..lineTo(pts.first.dx, chart.bottom)
        ..close();
      canvas.drawPath(
        area,
        Paint()
          ..shader = LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [color.withValues(alpha: 0.24), color.withValues(alpha: 0.0)],
          ).createShader(chart),
      );
      canvas.drawPath(
        line,
        Paint()
          ..color = color
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2.2
          ..strokeJoin = StrokeJoin.round
          ..strokeCap = StrokeCap.round,
      );
    }

    // Juda ko'p nuqtada (masalan bir yillik kunlik grafik) doiralar bir-
    // biriga yopishib, chiziqni qora tasmaga aylantirardi.
    if (pts.length <= 62) {
      final fill = Paint()..color = Colors.white;
      final ring = Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.8;
      for (final p in pts) {
        canvas.drawCircle(p, 3, fill);
        canvas.drawCircle(p, 3, ring);
      }
    }

    // X o'qi: yorliqlar faqat sig'adigan qadar — ustma-ust tushmasligi
    // uchun har biri oldingisidan keyin joy bor-yo'qligi tekshiriladi.
    final maxLabels = math.max(2, (chart.width / 62).floor());
    final every = math.max(1, (points.length / maxLabels).ceil());
    var lastRight = double.negativeInfinity;
    for (var i = 0; i < points.length; i += every) {
      final tp = _layout(points[i].axisLabel, _labelStyle);
      final maxX = math.max(0.0, size.width - tp.width);
      final x = (pts[i].dx - tp.width / 2).clamp(0.0, maxX);
      if (x < lastRight + 6) continue;
      tp.paint(canvas, Offset(x, chart.bottom + 7));
      lastRight = x + tp.width;
    }

    final h = hover;
    if (h == null || h < 0 || h >= pts.length) return;
    final p = pts[h];
    canvas.drawLine(
      Offset(p.dx, chart.top),
      Offset(p.dx, chart.bottom),
      Paint()
        ..color = OnDexColors.inkFaint.withValues(alpha: 0.45)
        ..strokeWidth = 1,
    );
    canvas.drawCircle(p, 6, Paint()..color = Colors.white);
    canvas.drawCircle(
      p,
      6,
      Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2.4,
    );
    canvas.drawCircle(p, 2.6, Paint()..color = color);

    final title = _layout(points[h].tooltipTitle,
        const TextStyle(fontSize: 11, color: OnDexColors.sidebarTextDim));
    final value = _layout(
      points[h].tooltipValue,
      const TextStyle(
          fontSize: 12.5, color: Colors.white, fontWeight: FontWeight.w700),
    );
    const padH = 10.0, padV = 8.0, gap = 3.0;
    final boxW = math.max(title.width, value.width) + padH * 2;
    final boxH = title.height + value.height + gap + padV * 2;
    // Alohida `double` o'zgaruvchi SHART: `clamp` ichida yozilsa
    // `math.max` parametr turi bo'yicha `num` deb chiqariladi va natija
    // ham `num` bo'lib qoladi.
    final double maxLeft = math.max(chart.left, chart.right - boxW);
    final left = (p.dx - boxW / 2).clamp(chart.left, maxLeft);
    var boxTop = p.dy - boxH - 12;
    if (boxTop < 0) boxTop = p.dy + 12;
    canvas.drawRRect(
      RRect.fromRectAndRadius(
          Rect.fromLTWH(left, boxTop, boxW, boxH), const Radius.circular(8)),
      Paint()..color = OnDexColors.sidebarBg,
    );
    title.paint(canvas, Offset(left + padH, boxTop + padV));
    value.paint(canvas, Offset(left + padH, boxTop + padV + title.height + gap));
  }

  static TextPainter _layout(String text, TextStyle style) => TextPainter(
        text: TextSpan(text: text, style: style),
        textDirection: TextDirection.ltr,
        maxLines: 1,
      )..layout();

  static void _dashedLine(Canvas canvas, Offset from, double toX, Paint paint) {
    const dash = 4.0, space = 4.0;
    var x = from.dx;
    while (x < toX) {
      final end = math.min(x + dash, toX);
      canvas.drawLine(Offset(x, from.dy), Offset(end, from.dy), paint);
      x = end + space;
    }
  }

  /// Catmull-Rom egri chizig'i. Boshqaruv nuqtalari grafik chegarasiga
  /// qisiladi: Bezye egri chizig'i har doim o'z boshqaruv nuqtalari
  /// qobig'ida yotadi, ya'ni chiziq hech qachon noldan pastga "sho'ng'imaydi"
  /// (nol kun yonida manfiy qiymat ko'rinardi).
  static Path _smoothPath(List<Offset> pts, double minY, double maxY) {
    final path = Path()..moveTo(pts.first.dx, pts.first.dy);
    for (var i = 0; i < pts.length - 1; i++) {
      final p0 = pts[i == 0 ? 0 : i - 1];
      final p1 = pts[i];
      final p2 = pts[i + 1];
      final p3 = pts[i + 2 < pts.length ? i + 2 : i + 1];
      final c1 = Offset(p1.dx + (p2.dx - p0.dx) / 6,
          (p1.dy + (p2.dy - p0.dy) / 6).clamp(minY, maxY));
      final c2 = Offset(p2.dx - (p3.dx - p1.dx) / 6,
          (p2.dy - (p3.dy - p1.dy) / 6).clamp(minY, maxY));
      path.cubicTo(c1.dx, c1.dy, c2.dx, c2.dy, p2.dx, p2.dy);
    }
    return path;
  }

  @override
  bool shouldRepaint(covariant _LineChartPainter old) =>
      old.points != points ||
      old.hover != hover ||
      old.color != color ||
      old.unitLabel != unitLabel ||
      old.integerAxis != integerAxis;
}

/// O'q uchun "chiroyli" qadam: 1, 2, 5 × 10ⁿ.
double _niceStep(double rough) {
  if (rough <= 0 || rough.isNaN || rough.isInfinite) return 1;
  final magnitude =
      math.pow(10, (math.log(rough) / math.ln10).floor()).toDouble();
  final residual = rough / magnitude;
  final nice = residual > 5
      ? 10
      : residual > 2
          ? 5
          : residual > 1
              ? 2
              : 1;
  return nice * magnitude;
}

/// Donut grafikning bitta bo'lagi.
class DonutSegment {
  final double value;
  final Color color;
  const DonutSegment(this.value, this.color);
}

/// Halqa (donut) grafik. Markazga istalgan vidjet qo'yiladi.
class OnDexDonutChart extends StatelessWidget {
  final List<DonutSegment> segments;
  final double diameter;
  final double thickness;
  final Widget? center;

  const OnDexDonutChart({
    super.key,
    required this.segments,
    this.diameter = 150,
    this.thickness = 26,
    this.center,
  });

  @override
  Widget build(BuildContext context) {
    final hole = diameter - thickness * 2 - 8;
    return SizedBox(
      width: diameter,
      height: diameter,
      child: CustomPaint(
        painter: _DonutPainter(segments, thickness),
        child: center == null
            ? null
            : Center(
                child: SizedBox(
                  width: hole > 0 ? hole : 0,
                  child: FittedBox(fit: BoxFit.scaleDown, child: center),
                ),
              ),
      ),
    );
  }
}

class _DonutPainter extends CustomPainter {
  final List<DonutSegment> segments;
  final double thickness;

  _DonutPainter(this.segments, this.thickness);

  @override
  void paint(Canvas canvas, Size size) {
    final radius = (math.min(size.width, size.height) - thickness) / 2;
    if (radius <= 0) return;
    final rect = Rect.fromCircle(center: size.center(Offset.zero), radius: radius);
    Paint stroke(Color c) => Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = thickness
      ..color = c;

    final visible = segments.where((s) => s.value > 0).toList();
    final total = visible.fold<double>(0, (sum, s) => sum + s.value);
    if (total <= 0) {
      canvas.drawArc(rect, 0, math.pi * 2, false, stroke(OnDexColors.cardBorder));
      return;
    }
    // Bo'laklar orasida ingichka oq tirqish — bitta bo'lak bo'lsa kerak emas.
    final gap = visible.length > 1 ? 0.025 : 0.0;
    var start = -math.pi / 2;
    for (final s in visible) {
      final sweep = math.pi * 2 * s.value / total;
      if (sweep - gap > 0) {
        canvas.drawArc(rect, start + gap / 2, sweep - gap, false, stroke(s.color));
      }
      start += sweep;
    }
  }

  @override
  bool shouldRepaint(covariant _DonutPainter old) =>
      old.segments != segments || old.thickness != thickness;
}
