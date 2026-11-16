import 'dart:async';
import 'dart:convert';
import 'dart:js_interop';
import 'dart:ui_web' as ui_web;

import 'package:flutter/material.dart';
import 'package:web/web.dart' as web;

import '../../api.dart';

/// Web: `/courier-map` iframe ichida (bir xil manba — merchant.ondex.uz/r/api),
/// ma'lumot `postMessage` bilan, faqat o'z manbasiga.
class DeliveryMapView extends StatefulWidget {
  const DeliveryMapView({super.key, required this.data});

  final Map<String, dynamic>? data;

  @override
  State<DeliveryMapView> createState() => _DeliveryMapViewState();
}

int _seq = 0;

class _DeliveryMapViewState extends State<DeliveryMapView> {
  late final String _viewType = 'ondex-courier-map-${_seq++}';
  late final web.HTMLIFrameElement _frame;
  StreamSubscription<web.MessageEvent>? _sub;
  bool _pageReady = false;

  @override
  void initState() {
    super.initState();
    _frame = web.HTMLIFrameElement()
      ..src = '$baseUrl/courier-map'
      ..title = 'Yetkazish xaritasi';
    _frame.style
      ..border = '0'
      ..width = '100%'
      ..height = '100%';
    ui_web.platformViewRegistry.registerViewFactory(_viewType, (int _) => _frame);
    _sub = web.window.onMessage.listen((e) {
      if (e.origin != web.window.location.origin || e.source != _frame.contentWindow) return;
      try {
        final m = jsonDecode((e.data as JSString).toDart) as Map;
        if (m['type'] == 'ready') {
          _pageReady = true;
          _push();
        }
      } catch (_) {}
    });
  }

  void _push() {
    final d = widget.data;
    if (!_pageReady || d == null) return;
    _frame.contentWindow?.postMessage(jsonEncode({...d, 'type': 'data'}).toJS, web.window.location.origin.toJS);
  }

  @override
  void didUpdateWidget(DeliveryMapView old) {
    super.didUpdateWidget(old);
    if (!identical(old.data, widget.data)) _push();
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => HtmlElementView(viewType: _viewType);
}
