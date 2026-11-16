import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:webview_windows/webview_windows.dart';

import '../../api.dart';
import '../../theme.dart';

/// Windows: `/courier-map` WebView2 ichida; [data] o'zgarsa sahifaga yuboriladi.
class DeliveryMapView extends StatefulWidget {
  const DeliveryMapView({super.key, required this.data});

  /// `GET /restaurants/{id}/delivery-map` javobi.
  final Map<String, dynamic>? data;

  @override
  State<DeliveryMapView> createState() => _DeliveryMapViewState();
}

class _DeliveryMapViewState extends State<DeliveryMapView> {
  final _controller = WebviewController();
  bool _pageReady = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      await _controller.initialize();
      _controller.webMessage.listen((raw) {
        try {
          final m = raw is String ? jsonDecode(raw) as Map : raw as Map;
          if (m['type'] == 'ready') {
            _pageReady = true;
            _push();
          }
        } catch (_) {}
      });
      await _controller.loadUrl('$baseUrl/courier-map');
      if (mounted) setState(() {});
    } catch (e) {
      if (mounted) {
        setState(() => _error = 'Xarita ochilmadi: $e\n'
            'Windows 11 da WebView2 standart bo\'ladi; yo\'q bo\'lsa Microsoft saytidan o\'rnating.');
      }
    }
  }

  void _push() {
    final d = widget.data;
    if (!_pageReady || d == null) return;
    _controller.postWebMessage(jsonEncode({...d, 'type': 'data'}));
  }

  @override
  void didUpdateWidget(DeliveryMapView old) {
    super.didUpdateWidget(old);
    if (!identical(old.data, widget.data)) _push();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Center(child: Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: OnDexColors.danger)));
    }
    if (!_controller.value.isInitialized) return const Center(child: CircularProgressIndicator());
    return Webview(_controller);
  }
}
