import 'dart:typed_data';

import 'package:flutter/services.dart' show rootBundle;
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;

import 'receipt_model.dart';

/// Chekni termal lenta (58/80 mm) uchun PDF ga chizadi.
///
/// ┌─ O'LCHAMLAR ─────────────────────────────────────────────────────────┐
/// 58 mm lentada bosiladigan qism ~48 mm, 80 mm da ~72 mm. Sahifa kengligi
/// lenta kengligi, chetlar 2-3 mm — drayver qolganini o'zi kesadi.
/// Balandlik — mazmunga qarab (`double.infinity` + `pw.Page` emas,
/// `MultiPage` emas): bitta uzun "sahifa", lenta o'rtasida bo'linmaydi.
/// Termal printer kulrangni yomon chiqaradi — hamma narsa QORA, ingichka
/// chiziqlar yo'q (faqat uzuq chiziq).
/// └──────────────────────────────────────────────────────────────────────┘
class ReceiptFonts {
  ReceiptFonts._(this.regular, this.bold);
  final pw.Font regular;
  final pw.Font bold;

  static ReceiptFonts? _cache;

  static Future<ReceiptFonts> load() async {
    final c = _cache;
    if (c != null) return c;
    final r = await rootBundle.load('assets/fonts/Roboto-Regular.ttf');
    final b = await rootBundle.load('assets/fonts/Roboto-Bold.ttf');
    return _cache = ReceiptFonts._(pw.Font.ttf(r), pw.Font.ttf(b));
  }
}

/// Lenta kengligi (mm) -> sahifa formati.
PdfPageFormat receiptFormat(int widthMm) {
  final w = (widthMm == 80 ? 80 : 58) * PdfPageFormat.mm;
  final margin = (widthMm == 80 ? 3 : 2) * PdfPageFormat.mm;
  return PdfPageFormat(w, double.infinity, marginAll: margin);
}

Future<Uint8List> buildReceiptPdf(Receipt r, {required int widthMm, ReceiptFonts? fonts}) async {
  final f = fonts ?? await ReceiptFonts.load();
  final wide = widthMm == 80;
  final base = wide ? 10.0 : 8.5;
  final doc = pw.Document(
    theme: pw.ThemeData.withFont(base: f.regular, bold: f.bold),
    title: 'Chek ${r.orderNumber}',
    creator: 'OnDex restoran paneli',
  );

  pw.Widget dashed() => pw.Container(
        margin: const pw.EdgeInsets.symmetric(vertical: 4),
        decoration: const pw.BoxDecoration(
          border: pw.Border(top: pw.BorderSide(width: 0.8, style: pw.BorderStyle.dashed)),
        ),
      );

  final kitchen = r.kind == ReceiptKind.kitchen;
  final itemStyle = pw.TextStyle(
      fontSize: kitchen ? base + 3 : base, fontWeight: kitchen ? pw.FontWeight.bold : null);

  doc.addPage(pw.Page(
    pageFormat: receiptFormat(widthMm),
    build: (_) => pw.Column(
      crossAxisAlignment: pw.CrossAxisAlignment.stretch,
      children: [
        pw.Text(r.header,
            textAlign: pw.TextAlign.center,
            style: pw.TextStyle(fontSize: base + 4, fontWeight: pw.FontWeight.bold)),
        pw.SizedBox(height: 4),
        pw.Row(mainAxisAlignment: pw.MainAxisAlignment.spaceBetween, children: [
          pw.Text(r.orderNumber, style: pw.TextStyle(fontSize: base + 3, fontWeight: pw.FontWeight.bold)),
          pw.Flexible(
            child: pw.Text(r.badge,
                textAlign: pw.TextAlign.right,
                style: pw.TextStyle(fontSize: base + 3, fontWeight: pw.FontWeight.bold)),
          ),
        ]),
        for (final m in r.meta) pw.Text(m, style: pw.TextStyle(fontSize: base)),
        dashed(),
        for (final it in r.items)
          pw.Padding(
            padding: const pw.EdgeInsets.only(bottom: 2),
            child: pw.Row(crossAxisAlignment: pw.CrossAxisAlignment.start, children: [
              pw.SizedBox(
                width: (kitchen ? 9 : 7) * PdfPageFormat.mm,
                child: pw.Text('${it.qty}x', style: itemStyle),
              ),
              pw.Expanded(child: pw.Text(it.name, style: itemStyle)),
              if (it.amount != null)
                pw.Padding(
                  padding: const pw.EdgeInsets.only(left: 4),
                  child: pw.Text(it.amount!, style: itemStyle),
                ),
            ]),
          ),
        if (r.totals.isNotEmpty) dashed(),
        for (final t in r.totals)
          pw.Row(mainAxisAlignment: pw.MainAxisAlignment.spaceBetween, children: [
            pw.Flexible(
              child: pw.Text(t.label,
                  style: pw.TextStyle(
                      fontSize: t.strong ? base + 2 : base, fontWeight: t.strong ? pw.FontWeight.bold : null)),
            ),
            pw.Text(t.value,
                style: pw.TextStyle(
                    fontSize: t.strong ? base + 2 : base, fontWeight: t.strong ? pw.FontWeight.bold : null)),
          ]),
        dashed(),
        for (final line in r.footer)
          pw.Text(line, textAlign: pw.TextAlign.center, style: pw.TextStyle(fontSize: base - 0.5)),
        // Kesgichsiz printer (XP-58IIL): chek tishli qirrada yirtiladi —
        // oxirgi qator qirradan o'tib chiqsin.
        pw.SizedBox(height: 12 * PdfPageFormat.mm),
      ],
    ),
  ));
  return doc.save();
}
