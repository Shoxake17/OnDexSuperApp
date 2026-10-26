import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Huquqiy hujjat (ommaviy oferta / maxfiylik siyosati) ILOVA ICHIDA.
///
/// Ilgari bu havolalar tizim brauzerida ochilardi — foydalanuvchi
/// ilovadan chiqib ketgandek tuyulardi. Endi WebView orqali, ilova
/// ichida (`ondex_core.LegalSection.onOpen` shu ekranni chaqiradi).
class LegalWebViewScreen extends StatefulWidget {
  const LegalWebViewScreen({super.key, required this.title, required this.url});

  final String title;
  final String url;

  @override
  State<LegalWebViewScreen> createState() => _LegalWebViewScreenState();
}

class _LegalWebViewScreenState extends State<LegalWebViewScreen> {
  late final WebViewController _controller;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(Colors.white)
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageStarted: (_) {
            if (mounted) setState(() => _loading = true);
          },
          onPageFinished: (_) {
            if (mounted) setState(() => _loading = false);
          },
          onWebResourceError: (err) {
            if (!err.isForMainFrame!) return;
            if (mounted) {
              setState(() {
                _loading = false;
                _error = 'Sahifa ochilmadi. Internetni tekshiring.';
              });
            }
          },
        ),
      )
      ..loadRequest(Uri.parse(widget.url));
  }

  Future<void> _openInBrowser() =>
      launchUrl(Uri.parse(widget.url), mode: LaunchMode.externalApplication);

  void _close() {
    if (mounted) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        if (await _controller.canGoBack()) {
          await _controller.goBack();
        } else {
          _close();
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: Text(widget.title),
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
                  if (_loading) const Center(child: CircularProgressIndicator()),
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
          const Icon(Icons.wifi_off, size: 48),
          const SizedBox(height: 16),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: onOpenBrowser,
            icon: const Icon(Icons.open_in_browser),
            label: const Text('Brauzerda ochish'),
          ),
        ],
      ),
    );
  }
}
