import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/foundation.dart' show Factory, defaultTargetPlatform, kIsWeb;
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:webview_flutter/webview_flutter.dart';
import 'package:webview_flutter_android/webview_flutter_android.dart';

import '../api.dart';

/// Chiqishda (logout) WebView sessiyasini TO'LIQ tozalaydi.
///
/// MUHIM XAVFSIZLIK: Flutter tomondagi `SharedPreferences`dan tokenni
/// o'chirish YETARLI EMAS — WebView'da `chust_session` httpOnly cookie'si
/// alohida saqlanadi va 30 kun amal qiladi. U tozalanmasa, chiqqandan
/// keyin ham WebView orqali o'sha akkauntning buyurtmalari/manzili
/// ochiq qolardi. Cookie'lar WebView jarayoni bo'ylab UMUMIY, shuning
/// uchun bu funksiyani statik `WebViewCookieManager` bajaradi — hech
/// qanday ochiq WebView nusxasi bo'lishi shart emas.
Future<void> clearMiniAppSession() async {
  try {
    await WebViewCookieManager().clearCookies();
  } catch (_) {
    // Cookie omborini tozalab bo'lmasa ham chiqish davom etadi —
    // Flutter tomondagi token baribir o'chiriladi.
  }
}

/// Native ekranlardan (Istaklarim, Buyurtmalarim) Next.js'ning bitta
/// ANIQ sahifasiga ("chuqur havola") o'tish uchun — masalan bir
/// restoran menyusi yoki bitta buyurtma kuzatuvi. Doimiy Home tab
/// WebView'idan FARQLI, bu — yangi, vaqtinchalik WebView (orqaga
/// bosilganda/qurilma tugmasi bilan yopiladi, Navigator o'zi bekor
/// qiladi). `/api/bridge`ning `redirect` maydoni orqali — tokenni
/// tasdiqlaydi va to'g'ridan-to'g'ri shu yo'lga o'tkazadi.
Route<void> miniAppRoute(String path) {
  return MaterialPageRoute(
    builder: (ctx) => Scaffold(
      body: SafeArea(
        child: MiniAppWebView(
          initialPath: path,
          // Sahifadagi "<-" tugmasi bosilganda SHU native ekran yopiladi
          // (foydalanuvchi qayerdan kelgan bo'lsa — Istaklarim yoki
          // Buyurtmalarim — o'sha yerga qaytadi). Home tab'dagi doimiy
          // WebView'da bu callback berilmaydi, shuning uchun u yerda
          // oddiy Next.js navigatsiyasi ishlaydi (lib/nav.ts).
          onClose: () => Navigator.of(ctx).maybePop(),
        ),
      ),
    ),
  );
}

/// Generic konteyner — istalgan mini-app URL'ini WebView'da ochadi. Faqat
/// URL parametr qabul qiladi, oziq-ovqat moduliga qattiq bog'lanmagan —
/// kelajakda yangi mini-app qo'shilganda bu faylni o'zgartirish shart
/// emas, shunchaki yangi URL bilan yangi nusxasi yaratiladi (masalan
/// HomeShell'ga yangi tab sifatida).
///
/// `IndexedStack` ichida ishlatilganda (HomeShell'dagi kabi) tab
/// almashtirilganda ham WebView tirik qoladi — Next.js'ning o'z
/// client-side router'i o'sha vaqtgacha bosilgan sahifani saqlab turadi,
/// qayta yuklanmaydi.
class MiniAppWebView extends StatefulWidget {
  /// Mini-app ichidagi boshlang'ich YO'L (masalan `/` yoki
  /// `/restaurants/abc`) — to'liq URL EMAS.
  ///
  /// XAVFSIZLIK (tuzatilgan zaiflik): avval bu yerga tokeni URL
  /// so'rov qatoriga yozilgan to'liq manzil berilardi
  /// (`/api/bridge?token=eyJ...`). URL so'rov qatori — maxfiy ma'lumot
  /// uchun ENG YOMON joy: u reverse-proxy va Next.js kirish
  /// loglariga yoziladi, WebView tarixida qoladi, sahifadan tashqi
  /// resurs so'ralganda `Referer` sarlavhasida chiqib ketishi mumkin.
  /// Endi token faqat POST TANASIDA yuboriladi (`_load` metodiga
  /// qarang) — u loglanmaydi va tarixda qolmaydi.
  final String initialPath;
  // Next.js o'zining client-side router'i orqali (History API, to'liq
  // sahifa yuklanishisiz) navigatsiya qiladi — shuning uchun tashqi
  // Flutter qobig'i (masalan pastki navigatsiya bar'ini ko'rsatish/
  // yashirish) joriy URL'ni bilishi uchun har bir marta chaqiriladi.
  final void Function(String url)? onUrlChanged;
  /// Berilgan bo'lsa — sahifadagi "orqaga" tugmasi shu funksiyani
  /// chaqiradi (`FlutterNavPop` JS kanali orqali). Faqat chuqur havola
  /// bilan PUSH qilingan ekranlarda beriladi (miniAppRoute'ga qarang).
  final VoidCallback? onClose;
  const MiniAppWebView({
    super.key,
    this.initialPath = '/',
    this.onUrlChanged,
    this.onClose,
  });

  @override
  State<MiniAppWebView> createState() => _MiniAppWebViewState();
}

class _MiniAppWebViewState extends State<MiniAppWebView> {
  late final WebViewController _controller;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    // MUHIM (haqiqiy Android qurilmada topilgan bug): standart
    // WebViewController() Android'da ba'zan "Virtual Display" kompozitsiya
    // rejimidan foydalanadi — WebView vizual to'g'ri chiziladi, lekin
    // teginish (touch) hodisalari veb-sahifaga UMUMAN yetib bormaydi
    // (Flutter/webview_flutter'ning tanilgan muammosi, ayniqsa WebView
    // boshqa vidjetlar — masalan bizning holatda pastki navigatsiya
    // qobig'i — bilan bir ierarxiyada bo'lganda). Android uchun ANIQ
    // "Hybrid Composition" so'raladi — bu WebView'ni haqiqiy Android
    // View sifatida (texture emas) chizadi, teginish to'g'ri ishlaydi.
    late final PlatformWebViewControllerCreationParams params;
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      params = AndroidWebViewControllerCreationParams();
    } else {
      params = const PlatformWebViewControllerCreationParams();
    }
    _controller = WebViewController.fromPlatformCreationParams(params)
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      // Sahifa yuklanguncha ko'rinadigan fon — AYNAN Flutter'ning
      // `scaffoldBackgroundColor`i (main.dart) va Next.js'ning qorong'i
      // fon rangi bilan bir xil. Avval `Colors.white` edi — har yuklanishda
      // qisqa OQ chaqnash chiqardi (haqiqiy qurilma skrinshotida ko'rilgan).
      ..setBackgroundColor(const Color(0xFF121212))
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageFinished: (url) {
            setState(() => _loading = false);
            widget.onUrlChanged?.call(url);
          },
          // Next.js sahifalar orasida (Bosh sahifa -> Menyu -> Savat ->
          // Checkout) to'liq qayta yuklanmasdan, History API orqali
          // o'tadi — bu holatda onPageFinished ISHGA TUSHMAYDI, faqat
          // onUrlChange orqali bilib olish mumkin.
          onUrlChange: (change) {
            if (change.url != null) widget.onUrlChanged?.call(change.url!);
          },
        ),
      )
      // MUHIM: veb-sahifadagi `navigator.geolocation` WebView ichida
      // ISHLAMAYDI — Chromium uni faqat "secure context"da (HTTPS yoki
      // localhost) ruxsat beradi, bizning dev manzilimiz esa oddiy
      // `http://<LAN-IP>:3000`. Shu sabab checkout'da "Joylashuvga ruxsat
      // berilmadi" xatosi chiqardi (aslida ruxsat masalasi emas edi).
      // Yechim: joylashuvni NATIVE tomonda (geolocator, allaqachon
      // ilovada bor va Android ruxsatlari sozlangan) olib, natijani
      // JS orqali sahifaga qaytaramiz. Bu production'da HTTPS bo'lganda
      // ham ishlaydi va aniqroq — brauzer emas, qurilmaning o'z GPS'i.
      ..addJavaScriptChannel(
        'FlutterGeo',
        onMessageReceived: (_) => _handleGeoRequest(),
      );
    // Faqat PUSH qilingan (chuqur havolali) ekranlarda — sahifa shu
    // kanalning bor-yo'qligiga qarab "orqaga" xatti-harakatini tanlaydi
    // (apps/web/lib/nav.ts).
    if (widget.onClose != null) {
      await _controller.addJavaScriptChannel(
        'FlutterNavPop',
        onMessageReceived: (_) => widget.onClose!(),
      );
    }
    // MUHIM (haqiqiy Android qurilmada topilgan ikkinchi bug): Android
    // WebView'ning o'z HTTP diskdagi keshi Flutter/Dart jarayonidan
    // MUSTAQIL — ilovani to'liq yopib qayta ochish ham buni tozalamaydi
    // (kesh tizim komponenti darajasida, ilova ma'lumotlar papkasida
    // saqlanadi). Rivojlanish paytida (Next.js dev-server kodi tez-tez
    // o'zgaradi, fayl nomlari esa hash bilan farqlanmaydi) bu eski
    // JS/CSS abadiy "yopishib qolishi"ga olib kelishi mumkin edi — aynan
    // shu sabab "tugma kichraymadi" degan xato tuyg'u paydo bo'lgan (kod
    // to'g'ri edi, WebView shunchaki eskisini ko'rsatardi). Har bir yangi
    // WebView yaratilganda (ya'ni ilova qayta ochilganda) keshni majburan
    // tozalaymiz — cookie/localStorage (sessiya, savat) buzilmaydi, faqat
    // resurslar keshi tozalanadi.
    await _controller.clearCache();
    await _loadBridge();
  }

  /// Sessiyani WebView'ga o'tkazadi va boshlang'ich sahifani yuklaydi.
  ///
  /// Token URL'da EMAS, POST tanasida yuboriladi (`initialPath` izohiga
  /// qarang). POST javobida server 303 redirect qaytaradi, WebView esa
  /// uni GET sifatida kuzatadi — ya'ni foydalanuvchi ko'radigan va
  /// tarixda qoladigan manzilda token umuman bo'lmaydi.
  ///
  /// POST so'rovlar HTTP keshiga tushmaydi, shuning uchun avvalgi
  /// `_cb` vaqt tamg'asi endi kerak emas (u faqat GET keshiga qarshi
  /// edi) — lekin keyingi sahifalar uchun `clearCache()` saqlanadi.
  Future<void> _loadBridge() async {
    final body = Uri(queryParameters: {
      'token': api.token ?? '',
      'redirect': widget.initialPath,
    }).query;
    await _controller.loadRequest(
      Uri.parse('$webAppUrl/api/bridge'),
      method: LoadRequestMethod.post,
      headers: const {'Content-Type': 'application/x-www-form-urlencoded'},
      body: Uint8List.fromList(utf8.encode(body)),
    );
  }

  /// Sahifadan kelgan joylashuv so'rovi — qurilmaning o'z GPS'i orqali
  /// bajariladi va natija `window.__onFlutterGeo(...)` orqali qaytariladi.
  /// Xato holatlari ANIQ ajratiladi (ruxsat / GPS o'chiq / boshqa) —
  /// foydalanuvchiga "ruxsat berilmadi" deb noto'g'ri aytilmasligi uchun.
  Future<void> _handleGeoRequest() async {
    try {
      // 1-QADAM — ruxsat. `deniedForever` ALOHIDA ajratiladi: bu holatda
      // tizim dialogi umuman ko'rsatilmaydi, shuning uchun foydalanuvchiga
      // "ruxsat berilmadi" deyish foydasiz — nima qilish kerakligini
      // aytamiz.
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.deniedForever) {
        await _replyGeo(
          error: 'Joylashuvga ruxsat yopilgan — Sozlamalar > Ilovalar > '
              'ChustApp > Ruxsatlar dan yoqing',
        );
        return;
      }
      if (perm == LocationPermission.denied) {
        await _replyGeo(error: 'Joylashuvga ruxsat berilmadi');
        return;
      }

      // 2-QADAM — OXIRGI MA'LUM nuqta DARHOL olinadi. Bu deyarli har doim
      // mavjud va bir zumda qaytadi; manzil tanlash uchun aniqligi yetarli.
      // Shu sabab foydalanuvchi hech qachon "kutib qolmaydi".
      Position? pos = await Geolocator.getLastKnownPosition();

      // 3-QADAM — aniqroq, yangi nuqta olishga urinamiz, LEKIN unga
      // bog'lanib qolmaymiz.
      //
      // MUHIM (qurilmada o'lchab topilgan bug): `getCurrentPosition()`da
      // standart aniqlik `best` va vaqt chegarasi YO'Q — bino ichida GPS
      // aniq nuqta topa olmay ABADIY kutardi, JS tomonga javob bormasdi
      // va oxirida CHALG'ITUVCHI "ruxsat berilmadi" xatosi chiqardi.
      // Endi: `medium` aniqlik (tarmoq/fused, ancha tez va manzil uchun
      // yetarli) + ikki qatlamli vaqt chegarasi.
      try {
        pos = await Geolocator.getCurrentPosition(
          locationSettings: const LocationSettings(
            accuracy: LocationAccuracy.medium,
            timeLimit: Duration(seconds: 12),
          ),
        ).timeout(const Duration(seconds: 14));
      } catch (_) {
        // Yangi nuqta olinmadi — 2-qadamdagi oxirgi ma'lum nuqta qoladi.
      }

      if (pos != null) {
        await _replyGeo(lat: pos.latitude, lng: pos.longitude);
        return;
      }

      // 4-QADAM — hech narsa yo'q. Sababni ANIQ aytamiz.
      if (!await Geolocator.isLocationServiceEnabled()) {
        await _replyGeo(
          error: 'Qurilmada joylashuv (GPS) o\'chirilgan — yoqib qayta urining',
        );
      } else {
        await _replyGeo(
          error: 'Joylashuv aniqlanmadi — ochiq joyga chiqib qayta urining',
        );
      }
    } catch (_) {
      await _replyGeo(error: 'Joylashuvni aniqlab bo\'lmadi');
    }
  }

  Future<void> _replyGeo({double? lat, double? lng, String? error}) async {
    final payload = error != null
        ? '{"error":${jsonEncode(error)}}'
        : '{"lat":$lat,"lng":$lng}';
    try {
      await _controller.runJavaScript(
        'window.__onFlutterGeo && window.__onFlutterGeo($payload)',
      );
    } catch (_) {
      // Sahifa almashgan bo'lsa — e'tiborsiz qoldiriladi.
    }
  }

  // Scaffold/NavigationBar bilan bir ierarxiyada bo'lgani uchun gesture
  // arenasida WebView'ning o'zi ustuvor bo'lishi ANIQ so'raladi — aks
  // holda ota vidjetlar teginishni "yutib" ketishi mumkin.
  static final Set<Factory<OneSequenceGestureRecognizer>> _gestureRecognizers = {
    Factory<EagerGestureRecognizer>(() => EagerGestureRecognizer()),
  };

  Widget _buildWebView() {
    final platform = _controller.platform;
    if (platform is AndroidWebViewController) {
      return WebViewWidget.fromPlatformCreationParams(
        params: AndroidWebViewWidgetCreationParams(
          controller: platform,
          displayWithHybridComposition: true,
          gestureRecognizers: _gestureRecognizers,
        ),
      );
    }
    return WebViewWidget(
      controller: _controller,
      gestureRecognizers: _gestureRecognizers,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        _buildWebView(),
        if (_loading) const Center(child: CircularProgressIndicator()),
      ],
    );
  }
}
