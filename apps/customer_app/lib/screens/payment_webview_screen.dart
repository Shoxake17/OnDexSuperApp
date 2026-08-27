import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:webview_flutter/webview_flutter.dart';

import 'catalog_screen.dart' show kBrand;

/// Octo to'lov sahifasini ILOVA ICHIDA ochadi.
///
/// ┌─ NEGA WEBVIEW, NEGA NATIVE FORMA EMAS ────────────────────────────┐
/// Karta raqami BIZNING kodimizdan o'tmasligi kerak. Agar uni ilovadagi
/// formadan olsak, PCI DSS sertifikati majburiy bo'ladi. Shuning uchun
/// karta faqat Octo sahifasida kiritiladi — biz uni shunchaki ko'rsatib
/// turamiz. Octo'da mobil SDK yo'q (hujjatlarda umuman tilga olinmagan),
/// ya'ni boshqa yo'l ham yo'q.
///
/// Mijoz uchun farqi sezilmaydi: brauzer ochilmaydi, ilovadan chiqmaydi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ BU EKRAN TO'LOV HOLATINI ANIQLAMAYDI ────────────────────────────┐
/// WebView "muvaffaqiyatli" sahifasini ko'rsatgani hech narsani
/// isbotlamaydi — uni soxtalashtirish oson. Haqiqiy holatni FAQAT
/// provayderning callback'i belgilaydi (`payments.Service.HandleCallback`).
/// Bu ekran yopilgach chaqiruvchi serverdan holatni so'raydi.
/// └───────────────────────────────────────────────────────────────────┘
class PaymentWebViewScreen extends StatefulWidget {
  const PaymentWebViewScreen({
    super.key,
    required this.payUrl,
    required this.returnUrl,
  });

  /// Octo bergan to'lov sahifasi havolasi.
  final String payUrl;

  /// To'lov tugagach provayder qaytaradigan manzil. Shu manzilga
  /// o'tilishi — "mijoz jarayonni tugatdi" degan BELGI, "to'ladi"
  /// degani emas.
  final String returnUrl;

  @override
  State<PaymentWebViewScreen> createState() => _PaymentWebViewScreenState();
}

class _PaymentWebViewScreenState extends State<PaymentWebViewScreen> {
  late final WebViewController _controller;
  bool _loading = true;
  bool _finished = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      // To'lov sahifalari JavaScript'siz ishlamaydi.
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(Colors.white)
      ..setNavigationDelegate(
        NavigationDelegate(
          onProgress: (_) {},
          onPageStarted: (_) {
            if (mounted) setState(() => _loading = true);
          },
          onPageFinished: (_) {
            if (mounted) setState(() => _loading = false);
          },
          onNavigationRequest: _onNavigation,
          onWebResourceError: (err) {
            // Faqat ASOSIY sahifa xatosi ko'rsatiladi. Reklama/analitika
            // resurslarining yiqilishi to'lovga xalaqit bermaydi va
            // mijozni bezovta qilmasligi kerak.
            if (!err.isForMainFrame!) return;
            if (mounted) {
              setState(() {
                _loading = false;
                _error = 'Sahifa ochilmadi. Internetni tekshiring yoki '
                    'brauzerda ochib ko\'ring.';
              });
            }
          },
        ),
      )
      ..loadRequest(Uri.parse(widget.payUrl));
  }

  NavigationDecision _onNavigation(NavigationRequest req) {
    // 1) Qaytish manziliga o'tildi — jarayon tugadi, oynani yopamiz.
    //    `startsWith` ishlatiladi, chunki provayder manzilga o'z
    //    parametrlarini qo'shib yuborishi mumkin.
    if (widget.returnUrl.isNotEmpty && req.url.startsWith(widget.returnUrl)) {
      _finish();
      return NavigationDecision.prevent;
    }

    // 2) http(s) bo'lmagan sxema: bank ilovasiga havola (masalan
    //    `uzcard://`, `intent://`). WebView bularni ocholmaydi —
    //    tizimga uzatamiz, aks holda mijoz oq ekranda qolib ketadi.
    final uri = Uri.tryParse(req.url);
    if (uri != null && uri.scheme != 'http' && uri.scheme != 'https') {
      launchUrl(uri, mode: LaunchMode.externalApplication).catchError((_) {
        return false;
      });
      return NavigationDecision.prevent;
    }

    return NavigationDecision.navigate;
  }

  void _finish() {
    if (_finished) return;
    _finished = true;
    if (mounted) Navigator.of(context).pop(true);
  }

  /// Zaxira yo'l: ba'zi banklarning 3-D Secure sahifasi WebView'da
  /// ochilmasligi mumkin. Bunday holatda mijoz tuzoqda qolmasligi uchun
  /// tashqi brauzerda ochish imkoni qoldirilgan.
  Future<void> _openInBrowser() async {
    await launchUrl(Uri.parse(widget.payUrl),
        mode: LaunchMode.externalApplication);
    if (mounted) Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      // Orqaga tugmasi: avval WebView tarixi bo'yicha ortga qaytamiz
      // (masalan SMS kod sahifasidan karta sahifasiga), ekran faqat
      // tarix tugagach yopiladi.
      canPop: false,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        if (await _controller.canGoBack()) {
          await _controller.goBack();
        } else {
          _finish();
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('To\'lov'),
          leading: IconButton(
            icon: const Icon(Icons.close),
            tooltip: 'Bekor qilish',
            onPressed: _finish,
          ),
          actions: [
            IconButton(
              icon: const Icon(Icons.open_in_browser),
              tooltip: 'Brauzerda ochish',
              onPressed: _openInBrowser,
            ),
          ],
          bottom: _loading
              ? const PreferredSize(
                  preferredSize: Size.fromHeight(2),
                  child: LinearProgressIndicator(minHeight: 2),
                )
              : null,
        ),
        body: _error != null
            ? _ErrorView(message: _error!, onOpenBrowser: _openInBrowser)
            : Stack(
                children: [
                  WebViewWidget(controller: _controller),
                  if (_loading)
                    const Center(child: CircularProgressIndicator()),
                ],
              ),
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onOpenBrowser});

  final String message;
  final VoidCallback onOpenBrowser;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.wifi_off, size: 48, color: Colors.black38),
          const SizedBox(height: 16),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 24),
          FilledButton.icon(
            style: FilledButton.styleFrom(backgroundColor: kBrand),
            onPressed: onOpenBrowser,
            icon: const Icon(Icons.open_in_browser),
            label: const Text('Brauzerda ochish'),
          ),
        ],
      ),
    );
  }
}
