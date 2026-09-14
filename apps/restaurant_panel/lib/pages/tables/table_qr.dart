part of '../tables_page.dart';

const _staticQrHint =
    'QR kod joy yaratilganda BIR MARTA yaratiladi va hech qachon o\'zgarmaydi. '
    'Nomi, turi, zali yoki sig\'imi o\'zgarsa ham chop etilgan QR ishlayveradi. '
    'QR surati begonalar qo\'liga tushsa — joyni vaqtincha yoping.';

/// QR tasviri (PDF varaqasi uchun). Ma'lumot — FAQAT serverdagi abadiy
/// `qr_link`: sana, vaqt yoki boshqa o'zgaruvchan qiymat qo'shilmaydi,
/// shuning uchun ekrandagi va yuklab olingan QR bir xil kodni o'qiydi.
///
/// Oq fon va chetdagi bo'sh joy (quiet zone) ATAYLAB: `QrPainter` faqat
/// modullarni chizadi (fon shaffof), shaffof tasvir esa bosmada
/// skanerlanmay qolardi.
Future<Uint8List> _qrPng(String data, {int size = 1024}) async {
  final painter = QrPainter(
    data: data,
    version: QrVersions.auto,
    gapless: true,
    eyeStyle: const QrEyeStyle(eyeShape: QrEyeShape.square, color: Color(0xFF000000)),
    dataModuleStyle: const QrDataModuleStyle(
        dataModuleShape: QrDataModuleShape.square, color: Color(0xFF000000)),
  );
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  final total = size.toDouble();
  final quiet = total * 0.1;
  canvas.drawRect(Rect.fromLTWH(0, 0, total, total), Paint()..color = const Color(0xFFFFFFFF));
  canvas.save();
  canvas.translate(quiet, quiet);
  painter.paint(canvas, Size.square(total - quiet * 2));
  canvas.restore();
  final image = await recorder.endRecording().toImage(size, size);
  final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
  image.dispose();
  if (bytes == null) throw StateError('QR tasvirini chizib bo\'lmadi');
  return bytes.buffer.asUint8List();
}

String _fileSlug(String s) {
  final slug = s
      .replaceAll(RegExp(r'[^A-Za-z0-9_-]+'), '-')
      .replaceAll(RegExp(r'-{2,}'), '-')
      .replaceAll(RegExp(r'^-+|-+$'), '');
  return slug.isEmpty ? 'joy' : slug;
}

String _qrFileName(DiningTable t, String extension) =>
    'OnDex-QR-${_fileSlug(t.zone)}-${_fileSlug(_fullTitle(t))}.$extension';

/// Bosmaga tayyor QR varaqasi (standart A6).
///
/// Shrift: Roboto (kirillcha restoran nomlari uchun). Internet bo'lmasa
/// varaqa baribir tayyorlanadi — lotindan tashqari belgilar "?" bo'ladi.
Future<Uint8List> _buildQrPdf({
  required DiningTable table,
  required String restaurantName,
  PdfPageFormat format = PdfPageFormat.a6,
}) async {
  final link = table.qrLink;
  if (link == null) throw StateError(_noLinkText);
  final png = await _qrPng(link);

  pw.ThemeData? theme;
  try {
    final fonts = await Future.wait([
      PdfGoogleFonts.robotoRegular(),
      PdfGoogleFonts.robotoBold(),
    ]).timeout(const Duration(seconds: 8));
    theme = pw.ThemeData.withFont(base: fonts[0], bold: fonts[1]);
  } catch (_) {
    theme = null;
  }
  final unicode = theme != null;
  String safe(String s) =>
      unicode ? s : String.fromCharCodes(s.runes.map((r) => r <= 0xFF ? r : 0x3F));

  final doc = pw.Document(title: 'QR — ${safe(table.displayLabel)}', author: 'OnDex');
  doc.addPage(
    pw.Page(
      pageFormat: format,
      margin: const pw.EdgeInsets.all(20),
      theme: theme,
      build: (_) => pw.Center(
        child: pw.Column(
          mainAxisAlignment: pw.MainAxisAlignment.center,
          children: [
            if (restaurantName.trim().isNotEmpty)
              pw.Text(safe(restaurantName.trim()),
                  textAlign: pw.TextAlign.center,
                  style: const pw.TextStyle(fontSize: 12, color: PdfColors.grey700)),
            pw.SizedBox(height: 4),
            pw.Text(safe(_fullTitle(table)).toUpperCase(),
                textAlign: pw.TextAlign.center,
                style: const pw.TextStyle(fontSize: 28, fontWeight: pw.FontWeight.bold)),
            pw.SizedBox(height: 2),
            pw.Text(safe(table.zone),
                textAlign: pw.TextAlign.center,
                style: const pw.TextStyle(fontSize: 12, color: PdfColors.grey700)),
            pw.SizedBox(height: 14),
            pw.Container(
              padding: const pw.EdgeInsets.all(8),
              decoration: pw.BoxDecoration(
                border: pw.Border.all(color: PdfColors.grey300),
                borderRadius: const pw.BorderRadius.all(pw.Radius.circular(8)),
              ),
              child: pw.Image(pw.MemoryImage(png), width: 190, height: 190),
            ),
            pw.SizedBox(height: 14),
            pw.Text('Menyu va buyurtma uchun QR kodni skanerlang',
                textAlign: pw.TextAlign.center, style: const pw.TextStyle(fontSize: 11)),
            pw.SizedBox(height: 6),
            pw.Text('OnDex', style: const pw.TextStyle(fontSize: 8, color: PdfColors.grey600)),
          ],
        ),
      ),
    ),
  );
  return doc.save();
}
