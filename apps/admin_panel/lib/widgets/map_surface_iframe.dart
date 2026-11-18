import 'dart:async';
import 'dart:convert';
import 'dart:js_interop';
import 'dart:ui_web' as ui_web;

import 'package:flutter/material.dart';
import 'package:web/web.dart' as web;

import '../api.dart';

/// WEB (admin.ondex.uz, Plane 479) uchun xarita yuzasi — backend beradigan
/// `/map-picker` sahifasi IFRAME ichida (Windows WebView2 varianti bilan bir
/// xil sahifa va protokol).
///
/// Sahifa BFF orqali O'SHA origin'dan keladi (`/a/api/map-picker`), shuning
/// uchun Maps kaliti ilova HTML'iga yozilmaydi: sahifa uni `/config/maps` dan
/// oladi, so'rov esa HttpOnly cookie bilan o'tadi (token brauzerda YO'Q —
/// sozlamadagi "bff" faqat belgi). Xabarlar faqat o'z manbasiga
/// (`postMessage(..., origin)`) va faqat shu iframe'dan qabul qilinadi.
class MapSurface extends StatefulWidget {
  final void Function(double lat, double lng) onPick;

  const MapSurface({super.key, required this.onPick});

  @override
  State<MapSurface> createState() => _MapSurfaceState();
}

int _seq = 0;

class _MapSurfaceState extends State<MapSurface> {
  static const _chustLat = 41.0030;
  static const _chustLng = 71.2360;

  late final String _viewType = 'ondex-map-picker-${_seq++}';
  late final web.HTMLIFrameElement _frame;
  StreamSubscription<web.MessageEvent>? _sub;
  String? _error;

  @override
  void initState() {
    super.initState();
    _frame = web.HTMLIFrameElement()
      ..src = '$baseUrl/map-picker'
      ..title = 'Xaritadan joy tanlash';
    _frame.style
      ..border = '0'
      ..width = '100%'
      ..height = '100%';
    ui_web.platformViewRegistry.registerViewFactory(_viewType, (int _) => _frame);
    _sub = web.window.onMessage.listen(_onMessage);
  }

  void _onMessage(web.MessageEvent e) {
    if (e.origin != web.window.location.origin || e.source != _frame.contentWindow) return;
    final Map<String, dynamic> m;
    try {
      m = Map<String, dynamic>.from(jsonDecode((e.data as JSString).toDart) as Map);
    } catch (_) {
      return;
    }
    switch (m['type']) {
      case 'boot':
        _frame.contentWindow?.postMessage(
          jsonEncode({'token': api.token ?? 'bff', 'apiBase': baseUrl, 'lat': _chustLat, 'lng': _chustLng}).toJS,
          web.window.location.origin.toJS,
        );
      case 'pick':
        final lat = (m['lat'] as num?)?.toDouble();
        final lng = (m['lng'] as num?)?.toDouble();
        if (lat != null && lng != null) widget.onPick(lat, lng);
      case 'error':
        if (mounted) setState(() => _error = '${m['message'] ?? 'Xarita yuklanmadi'}');
    }
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Stack(children: [
        HtmlElementView(viewType: _viewType),
        if (_error != null)
          Positioned(
            left: 12,
            right: 12,
            bottom: 12,
            child: Material(
              color: Colors.red.shade50,
              borderRadius: BorderRadius.circular(10),
              child: Padding(
                padding: const EdgeInsets.all(10),
                child: Text(_error!, style: TextStyle(color: Colors.red.shade800)),
              ),
            ),
          ),
      ]);
}
