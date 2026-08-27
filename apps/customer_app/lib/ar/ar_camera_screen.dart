import 'dart:convert';

import 'package:camera/camera.dart';
import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Taomni ILOVA ICHIDA, kamera tasviri ustida ko'rsatadi.
///
/// ┌─ NEGA TIZIM AR KO'RUVCHISI EMAS ──────────────────────────────────┐
/// Avval Google "Scene Viewer" ochilardi — u ALOHIDA ilova: mijoz
/// OnDex'dan chiqib ketardi, yuqorida brauzer manzili ko'rinardi va
/// qaytish uchun tizim tugmasini bosishga to'g'ri kelardi. Bu super
/// app uchun yaramaydi: mijoz ilovadan chiqib ketmasligi kerak.
///
/// Endi hammasi shu ekranda:
///   * pastda — kamera oynasi (`camera` plagini);
///   * ustida — SHAFFOF fonli WebView, ichida `<model-viewer>`.
///
/// Natijada taom kamera tasviri ustida "turgandek" ko'rinadi va mijoz
/// uni burab, kattalashtirib ko'radi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// HALOL CHEKLOV: bu tizim AR'idagi kabi tekislikni (stol yuzasini)
/// aniqlab, modelni unga MAHKAMLAMAYDI — telefonni burganda model
/// ekranda qolaveradi. Yuza kuzatuvi uchun ARCore sahnasi kerak
/// bo'lardi (og'ir va ko'p qurilmada beqaror). Ko'rish maqsadi —
/// porsiya va ko'rinishni baholash — bu usulda to'liq bajariladi.
class ArCameraScreen extends StatefulWidget {
  const ArCameraScreen({
    super.key,
    required this.modelUrl,
    required this.title,
  });

  final String modelUrl;
  final String title;

  @override
  State<ArCameraScreen> createState() => _ArCameraScreenState();
}

class _ArCameraScreenState extends State<ArCameraScreen>
    with WidgetsBindingObserver {
  CameraController? _camera;
  late final WebViewController _web;
  bool _cameraFailed = false;
  bool _modelReady = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _initWeb();
    _initCamera();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    // Kamera OG'IR resurs: bo'shatilmasa boshqa ekranlarda (QR
    // skaner) kamera ochilmay qoladi.
    _camera?.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Ilova fonga o'tganda kamerani ushlab turish mumkin emas —
    // Android uni tortib oladi va qaytganda qora ekran qolardi.
    final cam = _camera;
    if (cam == null || !cam.value.isInitialized) return;
    if (state == AppLifecycleState.inactive) {
      cam.dispose();
    } else if (state == AppLifecycleState.resumed) {
      _initCamera();
    }
  }

  Future<void> _initCamera() async {
    try {
      final cams = await availableCameras();
      if (cams.isEmpty) {
        if (mounted) setState(() => _cameraFailed = true);
        return;
      }
      final back = cams.firstWhere(
        (c) => c.lensDirection == CameraLensDirection.back,
        orElse: () => cams.first,
      );
      // `medium` — ataylab: taom modeli ustidagi fon uchun yuqori
      // aniqlik shart emas, past rezolyutsiya esa issiqlik va
      // batareyani tejaydi.
      final controller = CameraController(
        back,
        ResolutionPreset.medium,
        enableAudio: false,
      );
      await controller.initialize();
      if (!mounted) {
        await controller.dispose();
        return;
      }
      setState(() {
        _camera = controller;
        _cameraFailed = false;
      });
    } catch (_) {
      if (mounted) setState(() => _cameraFailed = true);
    }
  }

  void _initWeb() {
    _web = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      // SHAFFOF fon — busiz WebView oq to'siq bo'lib, kamera
      // tasvirini butunlay yopib qo'yadi (foydalanuvchi aynan shu
      // xatoni ko'rgan edi).
      ..setBackgroundColor(Colors.transparent)
      ..addJavaScriptChannel(
        'ModelReady',
        onMessageReceived: (_) {
          if (mounted) setState(() => _modelReady = true);
        },
      )
      ..setNavigationDelegate(
        NavigationDelegate(
          // Bu ko'rsatkich, brauzer emas: sahifadan tashqariga
          // hech qanday o'tish bo'lmaydi.
          onNavigationRequest: (r) => r.url.startsWith('about:')
              ? NavigationDecision.navigate
              : NavigationDecision.prevent,
        ),
      )
      ..loadHtmlString(_html(widget.modelUrl),
          baseUrl: _originOf(widget.modelUrl));
  }

  static String _originOf(String url) {
    final u = Uri.tryParse(url);
    if (u == null || !u.hasScheme || u.host.isEmpty) return '';
    return '${u.scheme}://${u.host}/';
  }

  /// Ko'rsatkich sahifasi.
  ///
  /// `camera-orbit` radiusi 170% — model ATAYLAB uzoqroqdan
  /// ko'rsatiladi. Standart holatda `<model-viewer>` modelni butun
  /// ekranga sig'dirib chizadi va taom haddan tashqari katta bo'lib
  /// ketadi. Mijoz baribir barmoq bilan kattalashtira oladi.
  String _html(String url) => '''
<!DOCTYPE html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<script type="module" src="https://unpkg.com/@google/model-viewer@3.5.0/dist/model-viewer.min.js"></script>
<style>
  html,body{margin:0;padding:0;height:100%;background:transparent;overflow:hidden}
  model-viewer{width:100%;height:100%;background-color:transparent;--poster-color:transparent}
</style>
</head>
<body>
<model-viewer id="mv"
  src=${jsonEncode(url)}
  camera-controls
  touch-action="none"
  camera-orbit="0deg 72deg 170%"
  min-camera-orbit="auto auto 60%"
  max-camera-orbit="auto auto 400%"
  shadow-intensity="0.6"
  exposure="1.1"
  interaction-prompt="none">
</model-viewer>
<script>
  document.getElementById('mv').addEventListener('load', () => {
    if (window.ModelReady) window.ModelReady.postMessage('ok');
  });
</script>
</body>
</html>
''';

  @override
  Widget build(BuildContext context) {
    final cam = _camera;

    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        fit: StackFit.expand,
        children: [
          // ── 1-qatlam: kamera ──
          if (cam != null && cam.value.isInitialized)
            FittedBox(
              fit: BoxFit.cover,
              child: SizedBox(
                width: cam.value.previewSize?.height ?? 1,
                height: cam.value.previewSize?.width ?? 1,
                child: CameraPreview(cam),
              ),
            )
          else
            Container(
              color: const Color(0xFF1A1A1A),
              alignment: Alignment.center,
              child: _cameraFailed
                  ? const Padding(
                      padding: EdgeInsets.symmetric(horizontal: 40),
                      child: Text(
                        'Kamera ochilmadi — taomni 3D ko\'rinishda '
                        'aylantirib ko\'rishingiz mumkin',
                        textAlign: TextAlign.center,
                        style: TextStyle(color: Colors.white70, fontSize: 13),
                      ),
                    )
                  : const CircularProgressIndicator(color: Colors.white24),
            ),

          // ── 2-qatlam: 3D model (shaffof fon) ──
          WebViewWidget(controller: _web),

          // ── 3-qatlam: boshqaruv ──
          if (!_modelReady)
            const Center(
              child: SizedBox(
                width: 34,
                height: 34,
                child: CircularProgressIndicator(
                    strokeWidth: 2.5, color: Colors.white),
              ),
            ),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                children: [
                  Row(
                    children: [
                      _RoundButton(
                        icon: Icons.close_rounded,
                        onTap: () => Navigator.of(context).pop(),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 14, vertical: 8),
                          decoration: BoxDecoration(
                            color: Colors.black.withValues(alpha: 0.45),
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: Text(
                            widget.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                color: Colors.white,
                                fontSize: 14,
                                fontWeight: FontWeight.w600),
                          ),
                        ),
                      ),
                    ],
                  ),
                  const Spacer(),
                  if (_modelReady)
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 16, vertical: 10),
                      decoration: BoxDecoration(
                        color: Colors.black.withValues(alpha: 0.45),
                        borderRadius: BorderRadius.circular(20),
                      ),
                    ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RoundButton extends StatelessWidget {
  const _RoundButton({required this.icon, required this.onTap});

  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.black.withValues(alpha: 0.45),
      shape: const CircleBorder(),
      child: InkWell(
        onTap: onTap,
        customBorder: const CircleBorder(),
        child: Padding(
          padding: const EdgeInsets.all(9),
          child: Icon(icon, color: Colors.white, size: 22),
        ),
      ),
    );
  }
}
