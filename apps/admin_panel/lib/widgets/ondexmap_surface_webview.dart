import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:webview_windows/webview_windows.dart';

/// WINDOWS DESKTOP — OnDexMap muharririni WebView2 ichida ko'rsatadi.
///
/// ┌─ NEGA WEBVIEW, NATIV EKRAN EMAS ───────────────────────────────────┐
/// OnDexMap — ALOHIDA loyiha va alohida repo (`F:\OnDexMap`). Uning
/// muharriri o'z serveridan beriladi va o'z bazasiga yozadi. Bu yerda
/// nativ ekran yozilsa:
///   - geometriya chizish mantiqi (poligon, chiziq, snapping) Flutter'da
///     qaytadan yozilishi kerak bo'lardi;
///   - OnDexMap API'si o'zgarganda ikkala loyiha birga o'zgartirilardi.
/// WebView bilan esa ChustApp faqat OYNA bo'lib qoladi — OnDexMap
/// mustaqil rivojlanaveradi va bu fayl o'zgarmaydi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────────┐
/// Manzil `127.0.0.1` — ya'ni muharrir FAQAT shu kompyuterda ishlaydi.
/// OnDexMap'ning admin serveri ataylab shunday bog'langan va internetga
/// chiqarilmaydi.
///
/// `ONDEXMAP_ADMIN_KEY` bu yerga YOZILMAYDI va ChustApp serveriga ham
/// berilmaydi. Kalitni Flutter binariga joylash — uni har bir
/// o'rnatilgan nusxaga tarqatish degani bo'lardi; EXE ochib o'qiladi.
///
/// Uning o'rniga OnDexMap serveri ishga tushganda BIR MARTALIK sessiya
/// tokeni yaratib, uni foydalanuvchi profilidagi faylga yozadi
/// (`%LOCALAPPDATA%\OnDexMap\admin_session.json`). Panel shu faylni
/// o'qib tokenni sahifaga beradi — natijada muharrirda kirish ekrani
/// UMUMAN yo'q: bo'lim ochilishi bilan xarita ko'rinadi.
///
/// Nega token serverdan so'ralmaydi: brauzerdagi zararli sahifa ham
/// `http://127.0.0.1:8091` ga so'rov yubora oladi, lekin lokal FAYLNI
/// o'qiy olmaydi. Ya'ni fayl orqali qo'l berish haqiqiy chegara.
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapSurface extends StatefulWidget {
  final String url;

  const OndexMapSurface({super.key, required this.url});

  @override
  State<OndexMapSurface> createState() => _OndexMapSurfaceState();
}

class _OndexMapSurfaceState extends State<OndexMapSurface> {
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

      // ┌─ NEGA `onLoadError` MAJBURIY ───────────────────────────────┐
      // `loadUrl()` navigatsiyani faqat BOSHLAB beradi — u server
      // javob bermasa ham muvaffaqiyatli qaytadi. Ya'ni bu yerdagi
      // `try/catch` tarmoq xatosini USHLAMAYDI: OnDexMap serveri
      // ishlamayotgan bo'lsa `_error` bo'sh qolib, ekranda WebView2
      // ning bo'sh yuzasi — QOP-QORA to'rtburchak — ko'rinardi.
      // Pastdagi foydali izoh ("go run ./cmd/admin") esa hech qachon
      // chiqmasdi.
      //
      // Xatoni faqat shu oqim aytadi. Tinglash `loadUrl` dan OLDIN
      // boshlanadi — aks holda darhol kelgan xato yo'qolardi.
      //
      // `onLoadError` — BIR MARTALIK oqim (broadcast emas), shuning
      // uchun `listen` faqat shu yerda, `_reload` da esa YO'Q.
      // └────────────────────────────────────────────────────────────┘
      _controller.onLoadError.listen((status) {
        if (mounted) setState(() => _error = _explain(status));
      });

      // ┌─ SESSIYA SAHIFADAN OLDIN JOYLASHTIRILADI ───────────────────┐
      // `addScriptToExecuteOnDocumentCreated` skriptni hujjat
      // YARATILISHIDA — sahifaning o'z skriptlaridan OLDIN bajaradi.
      // Ya'ni `window.__ondexMapAdmin` sahifa uchun boshidanoq mavjud:
      // kutish yo'q, xabar almashish yo'q, tartib muammosi yo'q.
      // Bu `map_surface_webview.dart` dagi naqsh bilan bir xil.
      //
      // Token URL'ga QO'YILMAYDI — u WebView tarixiga va server
      // kirish loglariga tushardi.
      // └────────────────────────────────────────────────────────────┘
      final session = _readSession();
      if (session != null) {
        await _controller.addScriptToExecuteOnDocumentCreated(
          'window.__ondexMapAdmin = ${jsonEncode({'key': session.token})};',
        );
      }

      await _controller.loadUrl(session?.url ?? widget.url);
      if (mounted) setState(() => _ready = true);
    } catch (e) {
      if (mounted) {
        setState(() => _error = 'WebView ochilmadi: $e\n\n'
            'Windows 11 da WebView2 Runtime standart bo\'ladi. '
            'Yo\'q bo\'lsa Microsoft saytidan o\'rnating.');
      }
    }
  }

  /// WebView2 xato kodini sababga aylantiradi.
  ///
  /// Eng ko'p uchraydigan holat birinchi: server ishga tushirilmagan.
  static String _explain(WebErrorStatus status) {
    switch (status) {
      case WebErrorStatus.WebErrorStatusCannotConnect:
      case WebErrorStatus.WebErrorStatusServerUnreachable:
      case WebErrorStatus.WebErrorStatusConnectionReset:
      case WebErrorStatus.WebErrorStatusConnectionAborted:
      case WebErrorStatus.WebErrorStatusDisconnected:
        return 'OnDexMap serveri javob bermadi (ulanib bo\'lmadi).';
      case WebErrorStatus.WebErrorStatusTimeout:
        return 'OnDexMap serveri vaqtida javob bermadi.';
      default:
        return 'Sahifa yuklanmadi: ${status.name}';
    }
  }

  Future<void> _reload() async {
    setState(() {
      _error = null;
      _ready = false;
    });
    try {
      // Sessiya QAYTA o'qiladi: OnDexMap serveri panel ochilgandan
      // keyin ishga tushirilgan (yoki qayta ishga tushirilgan) bo'lsa
      // token boshqa bo'ladi. Eski tokenni qayta yuborish — muharrirda
      // "sessiya qabul qilinmadi" holatida qotib qolish degani.
      // Keyin qo'shilgan skript oxirida bajariladi, ya'ni yangi token
      // eskisini almashtiradi.
      final session = _readSession();
      if (session != null) {
        await _controller.addScriptToExecuteOnDocumentCreated(
          'window.__ondexMapAdmin = ${jsonEncode({'key': session.token})};',
        );
      }
      await _controller.loadUrl(session?.url ?? widget.url);
      if (mounted) setState(() => _ready = true);
    } catch (e) {
      if (mounted) setState(() => _error = 'Yuklab bo\'lmadi: $e');
    }
  }

  /// OnDexMap serveri yozgan lokal sessiyani o'qiydi.
  ///
  /// Fayl bo'lmasa `null` qaytadi — bu XATO EMAS: server hali ishga
  /// tushirilmagan bo'lishi mumkin va u holda navigatsiya xatosi
  /// (`onLoadError`) baribir sababni ko'rsatadi.
  ///
  /// Faylning yo'li OnDexMap tomonidagi `internal/localsession` bilan
  /// SHARTNOMA (`os.UserCacheDir()` Windows'da `%LOCALAPPDATA%` ni
  /// qaytaradi).
  static _Session? _readSession() {
    final base = Platform.environment['LOCALAPPDATA'];
    if (base == null || base.isEmpty) return null;

    final file = File('$base\\OnDexMap\\admin_session.json');
    if (!file.existsSync()) return null;

    try {
      final m = jsonDecode(file.readAsStringSync()) as Map<String, dynamic>;
      final token = (m['token'] as String?)?.trim() ?? '';
      if (token.isEmpty) return null;

      // ┌─ MANZIL TEKSHIRILADI ──────────────────────────────────────┐
      // Manzil FAYLDAN keladi, shuning uchun u ko'r-ko'rona
      // ochilmaydi: faqat loopback va faqat `http`. Aks holda shu
      // faylni yozish imkoniyatiga ega narsa WebView'ni tashqi
      // saytga burib, unga admin tokenini ko'rsatib qo'yardi.
      // └────────────────────────────────────────────────────────────┘
      final raw = (m['url'] as String?) ?? '';
      String? url;
      final uri = Uri.tryParse(raw);
      if (uri != null &&
          uri.scheme == 'http' &&
          (uri.host == '127.0.0.1' || uri.host == 'localhost' || uri.host == '::1')) {
        url = raw;
      }
      return _Session(token: token, url: url);
    } catch (_) {
      // Yarim yozilgan yoki buzilgan fayl — sessiyasiz davom etamiz.
      return null;
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
      return _Problem(message: _error!, url: widget.url, onRetry: _reload);
    }
    if (!_ready) return const Center(child: CircularProgressIndicator());

    return Stack(
      children: [
        Positioned.fill(child: Webview(_controller)),
        // Qayta yuklash tugmasi: OnDexMap serveri panel ochilgandan
        // KEYIN ishga tushirilsa, sahifa "ulanib bo'lmadi" holatida
        // qotib qoladi va uni yangilash yo'li kerak bo'ladi.
        Positioned(
          right: 12,
          top: 12,
          child: Material(
            elevation: 2,
            shape: const CircleBorder(),
            child: IconButton(
              tooltip: 'Qayta yuklash',
              icon: const Icon(Icons.refresh),
              onPressed: _reload,
            ),
          ),
        ),
      ],
    );
  }
}

/// OnDexMap serverining lokal sessiyasi (fayldan o'qilgan).
class _Session {
  final String token;

  /// Serverning o'zi yozgan manzil. `null` — fayldagi manzil
  /// ishonchsiz (loopback emas) yoki yo'q; u holda standart manzil
  /// ishlatiladi.
  final String? url;

  const _Session({required this.token, this.url});
}

/// Xato holati — sabab va nima qilish kerakligi bilan.
class _Problem extends StatelessWidget {
  final String message;
  final String url;
  final VoidCallback onRetry;

  const _Problem({
    required this.message,
    required this.url,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.map_outlined, size: 48),
            const SizedBox(height: 16),
            SelectableText(message, textAlign: TextAlign.center),
            const SizedBox(height: 24),
            const Text(
              'OnDexMap muharriri alohida server sifatida ishlaydi.\n'
              'Uni ishga tushiring:',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 10),
            const SelectableText(
              'cd F:\\OnDexMap\ngo run ./cmd/admin',
              style: TextStyle(fontFamily: 'monospace'),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 20),
            SelectableText(url, style: const TextStyle(fontSize: 12)),
            const SizedBox(height: 20),
            FilledButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh),
              label: const Text('Qayta urinish'),
            ),
          ],
        ),
      ),
    );
  }
}
