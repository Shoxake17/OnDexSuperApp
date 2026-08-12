import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:webview_windows/webview_windows.dart';

import '../api.dart';

/// WINDOWS DESKTOP uchun xarita yuzasi — WebView2 ichida Google Maps JS.
///
/// ┌─ NEGA WEBVIEW ─────────────────────────────────────────────────────┐
/// `google_maps_flutter` Windows'ni qo'llab-quvvatlamaydi (paket faqat
/// android/ios/web ni e'lon qiladi) va desktopda
///     "TargetPlatform.windows is not yet supported by the maps plugin"
/// xatosini chizadi. `google_maps_flutter_windows` degan paket
/// MAVJUD EMAS. Shu sabab desktopda xarita brauzer dvigateli ichida,
/// odatdagi Google Maps JS bilan ko'rsatiladi — ya'ni xarita provayderi
/// web va desktopda BIR XIL bo'lib qoladi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ SAHIFA QAYERDAN YUKLANADI ────────────────────────────────────────┐
/// `${apiBaseUrl}/map-picker` — ya'ni BACKENDDAN, `file://` yoki `data:`
/// dan emas. Sabab: Maps kaliti Google Console'da HTTP referrer bo'yicha
/// cheklangan; `file://` da referrer bo'lmaydi va Google
/// `RefererNotAllowedMapError` bilan rad etadi.
///
/// Kalit sahifaga yozilmaydi — sahifa uni `/config/maps` dan, shu yerda
/// berilgan JWT bilan oladi. Token URL'ga QO'YILMAYDI (u WebView tarixi
/// va server loglariga tushardi), `postWebMessage` orqali beriladi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// WebView2 Runtime Windows 11 da standart o'rnatilgan bo'ladi; yo'q
/// bo'lsa `initialize()` xato beradi va ekranda sabab ko'rsatiladi.
class MapSurface extends StatefulWidget {
  final void Function(double lat, double lng) onPick;

  const MapSurface({super.key, required this.onPick});

  @override
  State<MapSurface> createState() => _MapSurfaceState();
}

class _MapSurfaceState extends State<MapSurface> {
  static const _chustLat = 41.0030;
  static const _chustLng = 71.2360;

  final _controller = WebviewController();
  bool _ready = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      await _controller.initialize();

      // Tinglashni yuklashdan OLDIN boshlaymiz: sahifa juda tez
      // ko'tarilsa `boot` xabari biz tayyor bo'lgunimizcha kelib
      // qolishi mumkin edi.
      _controller.webMessage.listen(_onMessage);

      if (api.token == null || api.token!.isEmpty) {
        setState(() => _error = 'Sessiya topilmadi — qaytadan kiring');
        return;
      }

      // ┌─ SOZLAMA SAHIFADAN OLDIN JOYLASHTIRILADI ──────────────────┐
      // Avval sozlama `postWebMessage` bilan, sahifa `boot` yuborgandan
      // KEYIN yuborilardi. Bu ikki tomonlama qo'l berish edi va uning
      // istalgan bo'g'ini uzilsa sahifa jimgina "Xarita yuklanmoqda..."
      // holatida qotib qolardi — aynan shu nosozlik kuzatilgan.
      //
      // `addScriptToExecuteOnDocumentCreated` skriptni hujjat
      // YARATILISHIDA, sahifaning o'z skriptlaridan OLDIN bajaradi.
      // Ya'ni `window.__ondexConfig` sahifa uchun boshidanoq mavjud:
      // kutish yo'q, tartib muammosi yo'q, xabar yo'qolishi yo'q.
      //
      // Token URL'ga QO'YILMAYDI (u WebView tarixi va server loglariga
      // tushardi) — u faqat sahifa xotirasiga boradi.
      // └────────────────────────────────────────────────────────────┘
      await _controller.addScriptToExecuteOnDocumentCreated(
        'window.__ondexConfig = ${jsonEncode({
          'token': api.token,
          'apiBase': baseUrl,
          'lat': _chustLat,
          'lng': _chustLng,
        })};',
      );

      await _controller.loadUrl('$baseUrl/map-picker');
      if (mounted) setState(() => _ready = true);
    } catch (e) {
      if (mounted) {
        setState(() => _error = 'WebView ochilmadi: $e\n\n'
            'Windows 11 da WebView2 Runtime standart bo\'ladi. '
            'Yo\'q bo\'lsa Microsoft saytidan o\'rnating.');
      }
    }
  }

  void _onMessage(dynamic raw) {
    // `webview_windows` xabarni ba'zan dekodlangan Map, ba'zan matn
    // sifatida beradi — ikkalasini ham qabul qilamiz.
    final Map<String, dynamic> m;
    try {
      m = raw is String
          ? jsonDecode(raw) as Map<String, dynamic>
          : Map<String, dynamic>.from(raw as Map);
    } catch (_) {
      return;
    }
    switch (m['type']) {
      case 'boot':
        // ZAXIRA YO'L. Asosiy yo'l — yuqoridagi
        // `addScriptToExecuteOnDocumentCreated`. Sahifa tomonda
        // `startOnce` qo'riqchisi bor, shuning uchun sozlama ikki
        // marta kelsa ham xarita bir marta ishga tushadi.
        _controller.postWebMessage(jsonEncode({
          'token': api.token,
          'apiBase': baseUrl,
          'lat': _chustLat,
          'lng': _chustLng,
        }));
        break;
      case 'pick':
        final lat = (m['lat'] as num?)?.toDouble();
        final lng = (m['lng'] as num?)?.toDouble();
        if (lat != null && lng != null) widget.onPick(lat, lng);
        break;
      case 'error':
        if (mounted) {
          setState(() => _error = 'Xarita xatosi: ${m['message']}');
        }
        break;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(_error!, textAlign: TextAlign.center),
        ),
      );
    }
    if (!_ready) return const Center(child: CircularProgressIndicator());
    return Webview(_controller);
  }
}
