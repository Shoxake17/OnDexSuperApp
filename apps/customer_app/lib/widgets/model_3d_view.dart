import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Taomning 3D modelini (GLB) ko'rsatuvchi vidjet.
///
/// ┌─ NEGA WEBVIEW ────────────────────────────────────────────────────┐
/// Flutter'da GLB ni chizadigan tayyor, barqaror native yechim yo'q.
/// Amaldagi standart — Google'ning `<model-viewer>` veb-komponenti:
/// u glTF/GLB ni to'g'ri o'qiydi, kamera boshqaruvi va yoritishni o'zi
/// beradi.
///
/// `webview_flutter` ILOVADA ALLAQACHON BOR (karta to'lovi ekrani uni
/// ishlatadi) — ya'ni yangi og'ir bog'liqlik QO'SHILMAYDI.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Vidjet OG'IR: 3D chizish xotira talab qiladi. Shuning uchun u faqat
/// foydalanuvchi 3D rejimini YOQQANDA quriladi (`menu_screen.dart` da
/// almashtirgich bor) va ekran yopilganda `dispose` bo'ladi.
class Model3DView extends StatefulWidget {
  const Model3DView({
    super.key,
    required this.modelUrl,
    this.backgroundColor = const Color(0xFFF5F5F5),
  });

  /// R2 dagi GLB havolasi (`product.model_3d_url`).
  final String modelUrl;
  final Color backgroundColor;

  @override
  State<Model3DView> createState() => _Model3DViewState();
}

class _Model3DViewState extends State<Model3DView> {
  late final WebViewController _controller;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(widget.backgroundColor)
      ..setNavigationDelegate(
        NavigationDelegate(
          // Sahifa ichida hech qanday HAVOLA bosilmasligi kerak: bu
          // ko'rsatkich, brauzer emas. Model'dan tashqari har qanday
          // navigatsiya bloklanadi.
          onNavigationRequest: (req) => req.url.startsWith('about:')
              ? NavigationDecision.navigate
              : NavigationDecision.prevent,
          onWebResourceError: (_) {
            if (mounted) setState(() => _failed = true);
          },
        ),
      )
      // ┌─ `baseUrl` MODEL DOMENIGA QO'YILADI — SHART ────────────────┐
      // `<model-viewer>` GLB ni `fetch` orqali oladi. Agar sahifa
      // origin'i modeldan boshqa bo'lsa, brauzer javobni CORS
      // sarlavhasisiz BLOKLAYDI va model hech qachon ko'rinmaydi.
      //
      // R2 bucket'i CORS sarlavhasini yubormaydi (uni qo'yish uchun
      // token'da bucket sozlamalari huquqi yo'q — ataylab, eng kam
      // huquq prinsipi). Yechim: sahifaning O'ZINI model turgan
      // domendan "kelgan" qilib ko'rsatamiz — shunda so'rov
      // BIR XIL ORIGIN'ga tushadi va CORS umuman qo'llanmaydi.
      //
      // Havola noto'g'ri bo'lsa `_originOf` bo'sh qaytaradi va oddiy
      // yuklash ishlaydi (model ko'rinmasa xato ekrani chiqadi).
      // └─────────────────────────────────────────────────────────────┘
      ..loadHtmlString(_html(widget.modelUrl),
          baseUrl: _originOf(widget.modelUrl));
  }

  /// Havoladan origin ajratadi: "https://cdn.example.com/a/b.glb" →
  /// "https://cdn.example.com/". Noto'g'ri havolada bo'sh satr.
  static String _originOf(String url) {
    final u = Uri.tryParse(url);
    if (u == null || !u.hasScheme || u.host.isEmpty) return '';
    return '${u.scheme}://${u.host}/';
  }

  /// Ko'rsatkich sahifasi.
  ///
  /// Havola JSON orqali qo'yiladi (`jsonEncode`) — bu qo'shtirnoq va
  /// maxsus belgilarni to'g'ri qochiradi. Oddiy satr qo'shilsa, havola
  /// ichidagi tirnoq HTML ni buzib, skript kiritish uchun teshik
  /// ochilardi.
  String _html(String url) => '''
<!DOCTYPE html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<script type="module" src="https://unpkg.com/@google/model-viewer@3.5.0/dist/model-viewer.min.js"></script>
<style>
  html,body{margin:0;padding:0;height:100%;background:transparent;overflow:hidden}
  model-viewer{width:100%;height:100%;--poster-color:transparent}
</style>
</head>
<body>
<model-viewer
  src=${jsonEncode(url)}
  camera-controls
  touch-action="pan-y"
  auto-rotate
  auto-rotate-delay="800"
  shadow-intensity="1"
  exposure="1"
  interaction-prompt="none">
</model-viewer>
</body>
</html>
''';

  @override
  Widget build(BuildContext context) {
    if (_failed) {
      return Container(
        color: widget.backgroundColor,
        alignment: Alignment.center,
        child: const Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.view_in_ar_outlined, size: 40, color: Color(0xFFBDBDBD)),
            SizedBox(height: 8),
            Text('3D ko\'rinish yuklanmadi',
                style: TextStyle(fontSize: 12.5, color: Color(0xFF9E9E9E))),
          ],
        ),
      );
    }
    return WebViewWidget(controller: _controller);
  }
}
