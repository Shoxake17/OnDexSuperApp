import 'dart:async';

import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import '../api.dart';

/// Stol QR kodini skanerlash ekrani — Telegram Mini App'dagi
/// `showScanQrPopup` bilan bir xil tajriba: to'liq ekran kamera,
/// o'rtada ramka va qisqa ko'rsatma.
///
/// ┌─ NEGA ILOVADA O'Z SKANERI KERAK ──────────────────────────────────┐
/// Mini App'da kamerani Telegram beradi. Mijoz ilovasida esa bunday
/// imkon yo'q edi va QR tugmasi faqat "telefoningizning kamerasi bilan
/// skanerlang" degan tushuntirishni ko'rsatardi — ya'ni ilovada stol
/// oqimi amalda ISHLAMASDI: foydalanuvchi ilovadan chiqib, tizim
/// kamerasini ochib, Telegram orqali qaytib kirishi kerak edi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ NIMA QAYTARADI ──────────────────────────────────────────────────┐
/// Ekran QR kodni o'qiydi, tokenni ajratadi va uni SERVERDA yechadi
/// (`GET /tables/resolve`). Muvaffaqiyatli bo'lsa `TableScanResult`
/// qaytaradi; chaqiruvchi (`home_shell.dart`) uni mini-app sahifasiga
/// uzatadi.
///
/// Tokenni bu yerda yechish ATAYLAB: xato holatlari (yaroqsiz kod,
/// o'chirilgan stol, tarmoq yo'q) NATIVE ekranda, tushunarli matn
/// bilan ko'rsatiladi. Agar token shunchaki sahifaga uzatilganda,
/// foydalanuvchi WebView ichida tushunarsiz holatga tushardi.
/// └───────────────────────────────────────────────────────────────────┘
class TableScanResult {
  final String token;
  final String restaurantId;
  final String tableLabel;
  final String restaurantName;

  const TableScanResult({
    required this.token,
    required this.restaurantId,
    required this.tableLabel,
    required this.restaurantName,
  });
}

/// Skanerlangan matndan stol tokenini ajratadi.
///
/// Qoidalar `apps/web/lib/open-table.ts` dagi `extractTableToken` bilan
/// AYNAN bir xil bo'lishi shart — bitta QR kod ikkala ilovada ham
/// birdek o'qilishi kerak:
///   * `https://t.me/<bot>/<short>?startapp=<token>` — asosiy shakl;
///   * `?start=<token>` — bot havolasining eski shakli;
///   * xom token — qo'lda chop etilgan kodlar uchun.
///
/// `null` — bu bizning QR kodimiz emas (begona sayt, vizitka va h.k.).
String? extractTableToken(String raw) {
  final s = raw.trim();
  if (s.isEmpty) return null;

  final uri = Uri.tryParse(s);
  if (uri != null && uri.hasScheme) {
    final q = uri.queryParameters['startapp'] ?? uri.queryParameters['start'];
    if (q != null && q.trim().isNotEmpty) return q.trim();
    // Havola, lekin token yo'q — begona QR.
    return null;
  }

  // Token — 32 baytlik tasodifiy sirning URL-xavfsiz ko'rinishi.
  // Qat'iy shablon: har qanday matnni serverga yuborib ko'rish
  // tokenlarni birma-bir sinashga o'xshab qolardi.
  return RegExp(r'^[A-Za-z0-9_-]{16,128}$').hasMatch(s) ? s : null;
}

class QrScanScreen extends StatefulWidget {
  const QrScanScreen({super.key});

  @override
  State<QrScanScreen> createState() => _QrScanScreenState();
}

class _QrScanScreenState extends State<QrScanScreen> {
  final _controller = MobileScannerController(
    // Faqat QR — chiziqli shtrix-kodlar (mahsulot kodlari) bu yerda
    // keraksiz va ular tasodifan o'qilib, "yaroqsiz kod" xatosini
    // chiqarardi.
    formats: const [BarcodeFormat.qrCode],
    detectionSpeed: DetectionSpeed.normal,
  );

  /// Kod topilgach ikkinchi marta ishlov berilmasin: kamera sekundiga
  /// o'nlab kadr beradi va bitta QR ketma-ket o'nlab marta topiladi.
  bool _handling = false;
  String? _error;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _onDetect(BarcodeCapture capture) async {
    if (_handling) return;

    for (final b in capture.barcodes) {
      final token = extractTableToken(b.rawValue ?? '');
      if (token == null) continue; // begona QR — skaner ochiq qoladi

      _handling = true;
      await _controller.stop();
      if (!mounted) return;
      setState(() => _error = null);

      try {
        final t = await api.resolveTable(token);
        if (!mounted) return;
        Navigator.of(context).pop(TableScanResult(
          token: token,
          restaurantId: t['restaurant_id'] as String? ?? '',
          tableLabel: t['table_label'] as String? ?? '',
          restaurantName: t['restaurant_name'] as String? ?? '',
        ));
      } on ApiException catch (e) {
        if (!mounted) return;
        // Server sababni aniq aytadi ("stol o'chirilgan", "topilmadi") —
        // uni o'zgartirmasdan ko'rsatamiz, chunki mijoz nima qilishini
        // aynan shu matn hal qiladi (ofitsiantga murojaat qilish).
        setState(() => _error = e.isUnauthorized
            ? 'Sessiya tugagan — ilovaga qaytadan kiring'
            : e.message);
        _handling = false;
        await _controller.start();
      }
      return;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        fit: StackFit.expand,
        children: [
          MobileScanner(
            controller: _controller,
            onDetect: _onDetect,
            // Kamera ochilmadi (ruxsat berilmadi, qurilmada kamera
            // yo'q, boshqa ilova band qilgan) — foydalanuvchi qora
            // ekranga qarab qolmasligi kerak.
            errorBuilder: (context, error) => _CameraError(error: error),
          ),
          const _ScanFrame(),
          _TopBar(controller: _controller),
          if (_error != null) _ErrorBanner(text: _error!),
        ],
      ),
    );
  }
}

/// Ekran tepasidagi yopish va chiroq tugmalari.
class _TopBar extends StatelessWidget {
  final MobileScannerController controller;
  const _TopBar({required this.controller});

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Row(
          children: [
            IconButton(
              tooltip: 'Yopish',
              icon: const Icon(Icons.close, color: Colors.white, size: 28),
              onPressed: () => Navigator.of(context).pop(),
            ),
            const Spacer(),
            // Chiroq — restoranda yorug'lik kam bo'lishi mumkin.
            // Qo'llab-quvvatlanmagan qurilmada tugma ishlamaydi, lekin
            // xato ham bermaydi.
            ValueListenableBuilder<MobileScannerState>(
              valueListenable: controller,
              builder: (context, state, _) {
                final on = state.torchState == TorchState.on;
                return IconButton(
                  tooltip: on ? 'Chiroqni o\'chirish' : 'Chiroqni yoqish',
                  icon: Icon(on ? Icons.flash_on : Icons.flash_off,
                      color: Colors.white, size: 26),
                  onPressed: () => controller.toggleTorch(),
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

/// O'rtadagi ramka va ko'rsatma — Telegram skaneridagi bilan bir xil
/// ma'noda: foydalanuvchi kodni qayerga tutishini ko'rsatadi.
class _ScanFrame extends StatelessWidget {
  const _ScanFrame();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 240,
            height: 240,
            decoration: BoxDecoration(
              border: Border.all(color: Colors.white, width: 3),
              borderRadius: BorderRadius.circular(20),
            ),
          ),
          const SizedBox(height: 20),
          const Text(
            'Stoldagi QR kodni kameraga tuting',
            textAlign: TextAlign.center,
            style: TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  final String text;
  const _ErrorBanner({required this.text});

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.bottomCenter,
      child: SafeArea(
        child: Container(
          margin: const EdgeInsets.all(16),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: Colors.red.shade700,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Text(text,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.white)),
        ),
      ),
    );
  }
}

/// Kamera ochilmagandagi ekran.
///
/// Ruxsat rad etilgan holat ALOHIDA ajratiladi: bunda tizim dialogi
/// boshqa ko'rsatilmaydi va foydalanuvchiga "qayta urinib ko'ring"
/// deyishning foydasi yo'q — nima qilish kerakligi aytiladi.
class _CameraError extends StatelessWidget {
  final MobileScannerException error;
  const _CameraError({required this.error});

  String get _message {
    switch (error.errorCode) {
      case MobileScannerErrorCode.permissionDenied:
        return 'Kameraga ruxsat berilmagan.\n\nSozlamalar > Ilovalar > '
            'OnDex > Ruxsatlar > Kamera dan yoqing.';
      case MobileScannerErrorCode.unsupported:
        return 'Bu qurilmada kamera skaneri qo\'llab-quvvatlanmaydi.';
      default:
        return 'Kamerani ochib bo\'lmadi. Boshqa ilova kamerani band '
            'qilgan bo\'lishi mumkin — uni yopib qayta urining.';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Colors.black,
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.no_photography_outlined,
              color: Colors.white70, size: 48),
          const SizedBox(height: 16),
          Text(
            _message,
            textAlign: TextAlign.center,
            style: const TextStyle(color: Colors.white, height: 1.4),
          ),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Yopish'),
          ),
        ],
      ),
    );
  }
}
