import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show rootBundle;
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:pointer_interceptor/pointer_interceptor.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../api.dart';
import '../theme.dart';
import '../session.dart';
import '../widgets/maps_loader.dart';
import 'login_screen.dart';

/// Kuryerning asosiy ekrani: TO'LIQ XARITA (o'z joylashuvi, restoran/mijoz
/// nuqtasi) + pastda suzuvchi panel (onlayn holat, kelgan takliflar, joriy
/// buyurtma) + pastki menyu (Xarita/Profil) — Yandex Go/Uber Driver
/// uslubida. Real-time — WebSocket orqali ("offer", "offer_cancelled",
/// "order_status"), uzilib qolsa avtomatik qayta ulanadi.
///
/// BILINGAN CHEKLOV: joriy band buyurtma faqat shu SESSIYa xotirasida
/// saqlanadi — agar kuryer sahifani browser'da qayta yuklasa (F5), "joriy
/// buyurtma" ko'rinishi yo'qoladi (backend'da "mening joriy buyurtmam"
/// so'rovi hali yo'q). Onlayn/oflayn holatning o'zi esa serverda saqlanadi.
class CourierShell extends StatefulWidget {
  final String courierId;
  const CourierShell({super.key, required this.courierId});

  @override
  State<CourierShell> createState() => _CourierShellState();
}

class _CourierShellState extends State<CourierShell>
    with SingleTickerProviderStateMixin, WidgetsBindingObserver {
  static const _chustCenter = LatLng(41.0030, 71.2360);

  bool _loading = true;
  bool? _approved;
  bool _online = false;
  String _name = '';
  int _tabIndex = 0;
  // Onlayn bo'lganda yuqori-chap burchakda aylanib turadigan "qidiryapman"
  // animatsiyasi (Yandex Eats uslubida).
  late final AnimationController _searchSpinController;

  // ---- Xarita ----
  GoogleMapController? _mapController;
  bool _mapsReady = false;
  bool _mapsFailed = false;
  LatLng? _myPosition;
  MapType _mapType = MapType.normal;
  LatLng _mapCenter = _chustCenter;
  double _currentZoom = 15;
  bool _locatingMe = false;
  // `CameraUpdate.zoomBy()` google_maps_flutter_web'da JORIY holatga nisbatan
  // hisoblaydi — tez-tez bosilganda markaz asta-sekin siljib ketadi. Shuning
  // uchun +/- tugmalari har doim BIZ kuzatib turgan `_mapCenter`/`_currentZoom`
  // asosida ANIQ target bilan yangi kamera pozitsiyasi beradi (address_screen.dart
  // bilan bir xil yechim). Bu hisoblagich dastur o'zi animateCamera chaqirganda
  // (zoom, "joriy joylashuvim") `onCameraMove`ning oraliq qiymatlari markazni
  // buzib qo'yishining oldini oladi.
  int _programmaticMoves = 0;

  WebSocketChannel? _channel;
  Timer? _locationTimer;
  Timer? _offerCountdown;

  Map<String, dynamic>?
  _offer; // {order_id, seconds_left, total_seconds, restaurant_name, restaurant_address, eta_minutes}
  bool _respondingOffer = false;

  Map<String, dynamic>? _activeOrder;
  Map<String, dynamic>? _activeRestaurant;
  String? _deliveryAddress;
  bool _loadingActiveOrder = false;
  bool _transitioning = false;
  // Kuryer restoranga JISMONAN yetib borganini o'zi bildiradi ("Yetib
  // keldim" tugmasi) — bu MAHALLIY (faqat shu qurilmada) holat, backend'ga
  // hech qanday so'rov yubormaydi (taom hali "ready" bo'lmasa ham
  // ko'rinishi/bosilishi kerak — foydalanuvchi so'rovi bo'yicha
  // TUZATILDI: avval noto'g'ri ravishda faqat taom "ready" bo'lgandagina
  // ko'rinardi). Yangi buyurtma qabul qilinganda `false`ga qaytadi.
  bool _arrivedAtRestaurant = false;
  // Joriy maqsadgacha (picked_up'gacha — restoran, undan keyin — mijoz)
  // ORQA SANOQ (countdown, soniyada) — Yandex Pro uslubida "Joriy
  // buyurtma" kartochkasi tepasida MM:SS ko'rinishida ko'rsatiladi.
  // `_updateRoute()` HAQIQIY yo'l vaqtini (Google Directions) olib,
  // ustiga XAVFSIZLIK BUFERI (har km uchun 2 daqiqa — restoranga kirish,
  // to'xtash, kutilmagan kechikish uchun) qo'shib qayta hisoblaydi;
  // `_countdownTicker` esa har soniyada 1 kamaytirib, silliq (tekis)
  // orqa sanoq taassurotini beradi. Ikkalasi ham foydalanuvchi so'rovi
  // bo'yicha: kuryer KECHIKMASLIGI uchun aniq va doimiy ko'rinishi kerak.
  int? _countdownSecondsLeft;
  Timer? _countdownTicker;

  // ---- Marshrut chizig'i (kuryerdan restoran/mijozgacha) ----
  String _vehicleType = 'moped'; // /couriers/{id} javobidan o'qiladi
  Set<Polyline> _polylines = {};
  int _routeRequestSeq =
      0; // eskirgan (kechikib kelgan) so'rov natijasini rad etish uchun

  // ---- Yandex Navi uslubidagi yo'nalish o'qi (kuryer belgisi) ----
  BitmapDescriptor? _navIcon;
  double _navBearing = 0; // gradus, shimoldan soat yo'nalishida

  // ---- Restoran belgisi: HAQIQIY logotip (yoki assets/box.png) — aniq
  // joylashuv nuqtasida kichik nuqta, uning tepasida logotip (Yandex
  // uslubi) ----
  static const _restaurantLogoSize = 36.0; // 2x kichikroq (avvalgi 72dan)
  static const _restaurantLogoCorner = 10.0;
  static const _restaurantDotDiameter = 14.0;
  static const _restaurantIconGap = 4.0; // logotip va nuqta orasidagi bo'shliq
  static const _restaurantIconWidth = _restaurantLogoSize;
  static const _restaurantIconHeight =
      _restaurantLogoSize + _restaurantIconGap + _restaurantDotDiameter;
  // Marker `anchor`i — aniq GPS nuqtasi rasmning QAYSI pikselida ekanini
  // bildiradi (0,0=yuqori-chap, 1,1=pastki-o'ng). Bizda bu — pastdagi
  // nuqtaning markazi (logotip emas!), shu bilan nuqta ANIQ bino ustida
  // turadi, logotip esa undan yuqorida "suzib" ko'rinadi.
  static const _restaurantIconAnchorY =
      (_restaurantLogoSize +
          _restaurantIconGap +
          _restaurantDotDiameter / 2) /
      _restaurantIconHeight;

  BitmapDescriptor? _restaurantIcon;
  final Map<String, BitmapDescriptor> _restaurantIconCache = {};

  // ---- Suzuvchi panel: statik emas, ICHIDAGI kartochka qancha joy olsa
  // o'shancha balandlikda (dinamik) ----
  final _sheetController = DraggableScrollableController();
  final _panelContentKey = GlobalKey();
  // Qotirilgan (fixed) pastki tugma qatorining HAQIQIY balandligini
  // o'lchash uchun — `_scheduleSheetResize()` panel o'lchamini
  // hisoblaganda shu balandlikni ham qo'shishi kerak (aks holda tugma
  // sheet chegarasidan tashqarida qolib ketishi mumkin).
  final _footerKey = GlobalKey();

  // ---- Yangi taklif kelganda jiringlaydigan ovoz — qabul/rad
  // etilguncha (yoki muddati tugaguncha) davom etadi ----
  final _offerPlayer = AudioPlayer();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _searchSpinController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 2),
    );
    _startCountdownTicker();
    _init();
    _loadMaps();
    _refreshMyLocation(sendToServer: false);
    // `initState()`da `context`ning ota-bobo (MediaQuery va h.k.)larga
    // bog'liqligi hali to'liq tayyor bo'lmasligi mumkin — shu sabab
    // ikonka yuklanishi BIRINCHI FRAME chizilib bo'lgach ishga tushiriladi
    // (`addPostFrameCallback`). Xatolik chiqsa endi `.catchError` orqali
    // logga chiqadi — avval xato jimgina yutilib, `_navIcon` abadiy `null`
    // qolar va marker doim eski ko'k nuqta bo'lib qolardi.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _buildNavArrowIcon()
          .then((icon) {
            debugPrint('[navi] icon yuklandi: $icon');
            if (mounted) setState(() => _navIcon = icon);
          })
          .catchError((Object e, StackTrace st) {
            debugPrint('[navi] icon yuklashda XATO: $e\n$st');
          });
    });
  }

  /// Yandex Navi uslubidagi yo'nalish o'qi belgisi — `assets/navi.png`
  /// (shaffof fondagi asl rasm, o'zim chizmayman) xarita belgisi
  /// (Marker) sifatida ishlatish uchun bitta marta yuklanadi va
  /// keshlanadi (har safar marker qayta chizilganda qayta yuklanmaydi).
  Future<BitmapDescriptor> _buildNavArrowIcon() async {
    return BitmapDescriptor.asset(
      createLocalImageConfiguration(context),
      'assets/navi.png',
      width: 56,
      height: 56,
    );
  }

  /// Restoran xaritadagi belgisi — Yandex uslubida: ANIQ GPS nuqtasida
  /// kichik nuqta (dot), uning tepasida HAQIQIY restoran logotipi
  /// (`logo_url`, tarmoqdan yuklanadi) yoki, logotip bo'lmasa/yuklanmasa,
  /// `assets/box.png` — burchaklari yumaloqlangan to'rtburchak shaklida.
  /// Ikkalasi (nuqta + logotip) BITTA rasmga chizilib, marker `anchor`i
  /// nuqtaning markaziga to'g'irlanadi — shu bilan nuqta bino ustida ANIQ
  /// turadi, logotip esa undan yuqorida suzib turgandek ko'rinadi. Natija
  /// `restaurantId` bo'yicha keshlanadi.
  Future<BitmapDescriptor> _buildRestaurantIcon(
    String? restaurantId,
    String? logoUrl,
  ) async {
    final cacheKey = restaurantId ?? logoUrl ?? 'box';
    final cached = _restaurantIconCache[cacheKey];
    if (cached != null) return cached;

    Uint8List? bytes;
    if (logoUrl != null && logoUrl.isNotEmpty) {
      try {
        final resp = await http.get(Uri.parse(fullImageUrl(logoUrl)));
        if (resp.statusCode == 200) bytes = resp.bodyBytes;
      } catch (_) {
        // Tarmoq xatosi — pastda assets/box.png fallback'iga tushamiz.
      }
    }
    bytes ??= (await rootBundle.load('assets/box.png')).buffer.asUint8List();

    final codec = await ui.instantiateImageCodec(bytes);
    final frame = await codec.getNextFrame();
    final image = frame.image;
    // Markazdan kvadrat kesib olamiz (BoxFit.cover kabi) — logotip
    // proporsiyasi buzilmasligi (cho'zilib ketmasligi) uchun.
    final side = math.min(image.width, image.height).toDouble();
    final srcRect = Rect.fromLTWH(
      (image.width - side) / 2,
      (image.height - side) / 2,
      side,
      side,
    );
    const logoRect = Rect.fromLTWH(
      0,
      0,
      _restaurantLogoSize,
      _restaurantLogoSize,
    );

    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    canvas.save();
    canvas.clipRRect(
      RRect.fromRectAndRadius(
        logoRect,
        const Radius.circular(_restaurantLogoCorner),
      ),
    );
    canvas.drawImageRect(image, srcRect, logoRect, Paint());
    canvas.restore();

    // Aniq joylashuv nuqtasi — logotip tagida kichik nuqta.
    const dotCenter = Offset(
      _restaurantIconWidth / 2,
      _restaurantLogoSize + _restaurantIconGap + _restaurantDotDiameter / 2,
    );
    canvas.drawCircle(
      dotCenter,
      _restaurantDotDiameter / 2,
      Paint()..color = Colors.white,
    );
    canvas.drawCircle(
      dotCenter,
      _restaurantDotDiameter / 2,
      Paint()
        ..color = const Color(0xFFFF9800)
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2,
    );

    final picture = recorder.endRecording();
    final rendered = await picture.toImage(
      _restaurantIconWidth.toInt(),
      _restaurantIconHeight.toInt(),
    );
    final byteData = await rendered.toByteData(
      format: ui.ImageByteFormat.png,
    );
    final icon = BitmapDescriptor.bytes(byteData!.buffer.asUint8List());
    _restaurantIconCache[cacheKey] = icon;
    return icon;
  }

  /// Ikki koordinata orasidagi kompas yo'nalishi (gradus, 0=shimol,
  /// soat yo'nalishida) — standart "great-circle bearing" formulasi.
  double _bearingBetween(LatLng from, LatLng to) {
    final lat1 = from.latitude * math.pi / 180;
    final lat2 = to.latitude * math.pi / 180;
    final dLng = (to.longitude - from.longitude) * math.pi / 180;
    final y = math.sin(dLng) * math.cos(lat2);
    final x =
        math.cos(lat1) * math.sin(lat2) -
        math.sin(lat1) * math.cos(lat2) * math.cos(dLng);
    final bearing = math.atan2(y, x) * 180 / math.pi;
    return (bearing + 360) % 360;
  }

  Future<void> _init() async {
    try {
      final c = await api.getCourier(widget.courierId);
      if (!mounted) return;
      setState(() {
        _approved = c['approved'] == true;
        _online = c['available'] == true;
        _name = c['name'] as String? ?? '';
        _vehicleType = c['vehicle_type'] as String? ?? 'moped';
      });
      if (_online) {
        _startLocationTimer();
        _searchSpinController.repeat();
      }
    } catch (_) {
      // Jim — pastdagi UI xatoni qayta urinish bilan ko'rsatadi.
    }
    _connectWs();
    if (mounted) setState(() => _loading = false);
  }

  Future<void> _loadMaps() async {
    try {
      await ensureGoogleMapsLoaded();
      if (mounted) setState(() => _mapsReady = true);
    } catch (_) {
      if (mounted) setState(() => _mapsFailed = true);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _searchSpinController.dispose();
    _locationTimer?.cancel();
    _offerCountdown?.cancel();
    _countdownTicker?.cancel();
    _channel?.sink.close();
    _sheetController.dispose();
    _offerPlayer.dispose();
    super.dispose();
  }

  /// HAQIQIY qurilmada aniqlangan holat (2026-07-30): ilova fonga chiqib
  /// qolganda (masalan kuryer boshqa ilovaga o'tsa) Android WebSocket
  /// ulanishini to'xtatib qo'yishi mumkin — shu oraliqda buyurtma holati
  /// o'zgarsa (masalan admin bekor qilsa), `order_status` eventi HECH
  /// KIMGA yetib bormaydi (`Hub.Send` navbatga qo'ymaydi, jimgina
  /// o'tkazib yuboradi) va WS qayta ulangach ham o'tkazib yuborilgan
  /// eventni QAYTA olib bo'lmaydi. Shuning uchun har safar oldinga
  /// chiqqanda joriy buyurtmani serverdan QAYTA so'raymiz — WebSocket
  /// holatiga bog'liq bo'lmagan zaxira himoya.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) return;
    final id = _activeOrder?['id'] as String?;
    if (id != null) unawaited(_loadActiveOrder(id));
  }

  /// Suzuvchi panelni ICHIDAGI kartochka (taklif/joriy buyurtma/kutish)
  /// haqiqiy balandligiga moslab qayta o'lchaydi — statik `initialChildSize`
  /// EMAS, har bir holat (zakaz kelishi, buyurtma o'zgarishi va h.k.) o'z
  /// mazmuniga mos balandlikda ko'tariladi. Kartochka almashgan HAR safar
  /// (frame chizilib bo'lgach) chaqirilishi kerak — shu sababdan alohida
  /// chaqiruvchi joylarda emas, shu yordamchi orqali.
  void _scheduleSheetResize() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final renderBox =
          _panelContentKey.currentContext?.findRenderObject() as RenderBox?;
      if (renderBox == null || !renderBox.hasSize) return;
      // Panel padding (fromLTRB 20,10,20,12/24) — o'lchangan Column shu
      // padding'lar ICHIDA, shuning uchun ularni tashqi hisobga qo'shamiz.
      // Pastki qotirilgan (fixed) tugma qatori ENDI scroll qilinuvchi
      // Column'ning TASHQARISIDA (alohida qatlam) — shuning uchun uning
      // HAQIQIY balandligi ham (bor bo'lsa) qo'shiladi, aks holda tugma
      // sheet chegarasidan tashqarida (yashirin) qolib ketardi.
      final footerBox =
          _footerKey.currentContext?.findRenderObject() as RenderBox?;
      final footerHeight = (footerBox != null && footerBox.hasSize)
          ? footerBox.size.height
          : 0.0;
      const verticalPadding = 10.0 + 12.0;
      final screenHeight = MediaQuery.of(context).size.height;
      var fraction =
          (renderBox.size.height + verticalPadding + footerHeight) /
              screenHeight;
      // Foydalanuvchi so'rovi: panel HECH QACHON ekranning yarmidan
      // (0.5) yuqoriga ko'tarilmasin — kontent qancha katta bo'lmasin
      // (scroll o'zi hal qiladi).
      fraction = fraction.clamp(0.14, 0.5);
      if (_sheetController.isAttached) {
        _sheetController.animateTo(
          fraction,
          duration: const Duration(milliseconds: 260),
          curve: Curves.easeOut,
        );
      }
    });
  }

  /// Suzuvchi panelning QOTIRILGAN (fixed) pastki tugmasi — holatga qarab
  /// (taklif/faol buyurtma/yo'q) mos amal tugmasini qaytaradi. `null`
  /// bo'lsa (masalan hali hech narsa kutilmayapti) footer umuman
  /// ko'rsatilmaydi.
  Widget? _buildFooterAction() {
    if (_offer != null) {
      return _AcceptCountdownButton(
        secondsLeft: (_offer!['seconds_left'] as num? ?? 0).toInt(),
        totalSeconds: (_offer!['total_seconds'] as num? ?? 1).toInt(),
        busy: _respondingOffer,
        onAccept: _acceptOffer,
      );
    }
    if (_loadingActiveOrder || _activeOrder == null) return null;
    final status = _activeOrder!['status'] as String? ?? 'ready';
    final pickedUp = status == 'picked_up';
    final ready = status == 'ready';

    if (pickedUp) {
      return _SlideToConfirm(
        label: 'Mijozga yetkazdim',
        busy: _transitioning,
        onConfirm: _markDelivered,
      );
    }

    // TO'RT ANIQ BOSQICH (foydalanuvchi so'rovi bo'yicha tuzatildi,
    // 2026-07-30 — avval "Yetib keldim" xato ravishda faqat taom
    // "ready" bo'lgandagina ko'rinardi):
    //  1) Kuryer HALI restoranga yetib bormagan — "Yetib keldim" DOIM
    //     ko'rinadi, taom holatidan qat'i nazar (mahalliy, backend
    //     so'rovisiz).
    //  2) Yetib bordi, lekin taom hali tayyor emas — passiv "tayyorlanmoqda" indikatori.
    //  3) Yetib bordi VA taom tayyor — haqiqiy "Buyurtma olindi" (backend picked_up).
    if (!_arrivedAtRestaurant) {
      return _SlideToConfirm(
        label: 'Yetib keldim',
        busy: false,
        onConfirm: _markArrivedAtRestaurant,
      );
    }
    if (!ready) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(vertical: 14),
        decoration: BoxDecoration(
          color: Colors.grey.withValues(alpha: 0.08),
          borderRadius: BorderRadius.circular(28),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: Colors.grey.shade500,
              ),
            ),
            const SizedBox(width: 10),
            Text(
              'Restoran taomni tayyorlamoqda...',
              style: TextStyle(color: Colors.grey.shade500),
            ),
          ],
        ),
      );
    }
    return _SlideToConfirm(
      label: 'Buyurtma olindi',
      busy: _transitioning,
      onConfirm: _confirmPickedUp,
    );
  }

  // ---------- WebSocket ----------

  Future<void> _connectWs() async {
    if (api.token == null) return;
    final String ticket;
    try {
      ticket = await api.wsTicket();
    } catch (_) {
      _scheduleReconnect();
      return;
    }
    if (!mounted) return;
    _channel = WebSocketChannel.connect(Uri.parse(wsUrl(ticket)));
    _channel!.stream.listen(
      (msg) {
        final e = jsonDecode(msg as String) as Map<String, dynamic>;
        switch (e['type']) {
          // Akkaunt superadmin tomonidan o'chirildi.
          //
          // Kuryer ilovasi soatlab ochiq turadi va fon so'rovlari
          // siyrak — usiz o'chirilgan kuryer ekranda "ishlayotgan"
          // bo'lib qolaverardi (xarita, buyurtma taklifi kutish).
          // Server tokenni allaqachon bekor qilgan, bu esa shu
          // holatni DARHOL ko'rsatadi.
          case 'account_deleted':
            _forceLogout();
          case 'offer':
            _handleOfferReceived(e);
          case 'offer_cancelled':
            if (_offer != null && _offer!['order_id'] == e['order_id']) {
              _dismissOffer();
            }
          case 'order_status':
            if (_activeOrder != null && _activeOrder!['id'] == e['order_id']) {
              final status = e['status'] as String?;
              if (status == 'delivered' ||
                  status == 'cancelled' ||
                  status == 'rejected') {
                if (mounted) {
                  setState(() {
                    _activeOrder = null;
                    _arrivedAtRestaurant = false;
                    _polylines = {};
                    _restaurantIcon = null;
                  });
                  _scheduleSheetResize();
                }
              } else if (mounted) {
                setState(
                  () => _activeOrder = {..._activeOrder!, 'status': status},
                );
                _scheduleSheetResize();
                // Holat o'zgardi (masalan picked_up) — maqsad nuqta
                // o'zgargan bo'lishi mumkin (restoran -> mijoz manzili).
                unawaited(_updateRoute());
              }
            }
        }
      },
      // Boshqa ilovalardagi bilan bir xil: uzilsa 2 soniyadan keyin
      // avtomatik qayta ulanadi — real-time takliflar bu tuzatishsiz
      // faqat bir marta ishlab, keyin jim qolib ketardi.
      onError: (_) => _scheduleReconnect(),
      onDone: _scheduleReconnect,
      cancelOnError: true,
    );
  }

  void _scheduleReconnect() {
    Future.delayed(const Duration(seconds: 2), () {
      if (!mounted) return;
      _connectWs();
    });
  }

  void _handleOfferReceived(Map<String, dynamic> e) {
    if (!mounted) return;
    // `as num` + `toInt()` — `as int` EMAS. JSON'da 20 va 20.0 ikkalasi
    // ham to'g'ri son, lekin Dart'da `20.0 as int` istisno tashlaydi.
    // Bu kod WebSocket tinglovchisi ichida ishlaydi: istisno tashlansa
    // butun soket ishlovchisi o'lib qolardi va kuryer boshqa HECH
    // QANDAY taklif/holat yangilanishini olmasdi (jimgina buzilish).
    final seconds = (e['expires_in_sec'] as num? ?? 20).toInt();
    final orderId = e['order_id'] as String;
    setState(() {
      _offer = {
        'order_id': orderId,
        'seconds_left': seconds,
        'total_seconds': seconds,
        'restaurant_name': e['restaurant_name'] ?? '',
        'restaurant_address': e['restaurant_address'] ?? '',
        'eta_minutes': null,
      };
    });
    _scheduleSheetResize();
    unawaited(_playOfferSound());
    unawaited(_loadOfferEta(orderId, e));
    _offerCountdown?.cancel();
    _offerCountdown = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted || _offer == null) {
        t.cancel();
        return;
      }
      final left = (_offer!['seconds_left'] as num? ?? 0).toInt() - 1;
      if (left <= 0) {
        t.cancel();
        _dismissOffer();
        return;
      }
      setState(() => _offer = {..._offer!, 'seconds_left': left});
    });
  }

  /// Taklif kelgan zahoti kuryerning HOZIRGI joylashuvidan restorangacha
  /// HAQIQIY yo'l bo'yicha (Google Directions, backend orqali) necha
  /// daqiqada yetib borishini hisoblaydi — taklif kartochkasida
  /// ko'rsatish uchun. `orderId` bilan tekshiriladi: javob kelguncha
  /// taklif allaqachon yopilgan yoki YANGI taklif kelib ulgurgan bo'lsa,
  /// eskirgan natija e'tiborga olinmaydi.
  Future<void> _loadOfferEta(String orderId, Map<String, dynamic> e) async {
    final lat = (e['restaurant_lat'] as num?)?.toDouble();
    final lng = (e['restaurant_lng'] as num?)?.toDouble();
    if (_myPosition == null || lat == null || lng == null) return;
    final mode = switch (_vehicleType) {
      'foot' => 'walking',
      'bike' => 'bicycling',
      _ => 'driving',
    };
    final result = await api.route(
      _myPosition!.latitude,
      _myPosition!.longitude,
      lat,
      lng,
      mode: mode,
    );
    if (!mounted || _offer == null || _offer!['order_id'] != orderId) return;
    if (result.durationSeconds == null) return;
    setState(() {
      _offer = {
        ..._offer!,
        'eta_minutes': (result.durationSeconds! / 60).round(),
      };
    });
  }

  void _dismissOffer() {
    _offerCountdown?.cancel();
    if (mounted) setState(() => _offer = null);
    _scheduleSheetResize();
    unawaited(_offerPlayer.stop());
  }

  /// Yangi taklif kelganda jiringlaydigan ovoz — kuryer QABUL yoki RAD
  /// etguncha (yoki taklif muddati tugab avtomatik yopilguncha) TAKRORLANIB
  /// (loop) chalinadi. Yopilishi — yagona joy: [_dismissOffer] (qabul
  /// qilinganda ham, rad etilganda/vaqti tugaganda ham shu chaqiriladi).
  Future<void> _playOfferSound() async {
    try {
      await _offerPlayer.stop();
      await _offerPlayer.setReleaseMode(ReleaseMode.loop);
      await _offerPlayer.play(AssetSource('sound/kuryersound.mp3'));
    } catch (_) {
      // Ovoz ijro etilmasa ham (masalan qurilma ovozsiz rejimda) taklif
      // o'zi ishlashda davom etadi — bloklovchi xato shart emas.
    }
  }

  // ---------- Onlayn/oflayn ----------

  Future<void> _toggleOnline(bool value) async {
    setState(() => _online = value); // optimistik — xato bo'lsa qaytariladi
    if (value) _scheduleSheetResize(); // panel endi ko'rinadi (faqat onlaynda)
    _applySpinForOnlineState(value);
    try {
      await api.setAvailable(widget.courierId, value);
      if (value) {
        _startLocationTimer();
      } else {
        _locationTimer?.cancel();
      }
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _online = !value);
      _applySpinForOnlineState(!value);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.message)));
    } catch (_) {
      if (!mounted) return;
      setState(() => _online = !value);
      _applySpinForOnlineState(!value);
    }
  }

  void _applySpinForOnlineState(bool online) {
    if (online) {
      _searchSpinController.repeat();
    } else {
      _searchSpinController.stop();
    }
  }

  void _startLocationTimer() {
    _locationTimer?.cancel();
    _refreshMyLocation(sendToServer: true);
    _locationTimer = Timer.periodic(
      const Duration(seconds: 25),
      (_) => _refreshMyLocation(sendToServer: true),
    );
  }

  /// Joriy koordinatani oladi — xaritada "Siz" markerini yangilaydi va,
  /// so'ralsa, serverga yuboradi (dispatch shu qiymatga qarab eng yaqin
  /// kuryerni tanlaydi). Ruxsat berilmagan/xato bo'lsa jim o'tkaziladi.
  Future<void> _refreshMyLocation({required bool sendToServer}) async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied ||
          perm == LocationPermission.deniedForever) {
        return;
      }
      final pos = await Geolocator.getCurrentPosition();
      final latLng = LatLng(pos.latitude, pos.longitude);
      if (mounted) setState(() => _myPosition = latLng);
      if (sendToServer) {
        await api.updateLocation(widget.courierId, pos.latitude, pos.longitude);
      }
      unawaited(_updateRoute());
    } catch (_) {
      // Internet yo'q yoki joylashuv aniqlanmadi — keyingi urinishda davom etadi.
    }
  }

  /// Kuryerdan HOZIRGI maqsadgacha (picked_up'gacha — restoran, undan
  /// keyin — mijoz manzili) HAQIQIY yo'l chizig'ini (Google Directions,
  /// backend orqali) oladi va xaritada chizadi. Avval bu funksiya UMUMAN
  /// yo'q edi — xarita faqat markerlarni ko'rsatardi, ular orasida hech
  /// qanday chiziq chizilmasdi.
  ///
  /// `_routeRequestSeq` — tarmoq so'rovi ketma-ket tez-tez chaqirilganda
  /// (masalan har 25s joylashuv yangilanganda) ESKI so'rov natijasi
  /// KEYINGI (yangiroq) so'rovdan keyin qaytib, uni bosib qo'ymasligi
  /// uchun (xuddi customer_app'dagi geokodlash bilan bir xil naqsh).
  Future<void> _updateRoute() async {
    final mySeq = ++_routeRequestSeq;
    if (_myPosition == null || _activeOrder == null) {
      if (_polylines.isNotEmpty && mounted) {
        setState(() {
          _polylines = {};
          _countdownSecondsLeft = null;
        });
      }
      return;
    }
    final pickedUp = _activeOrder!['status'] == 'picked_up';
    final destLat = pickedUp
        ? (_activeOrder!['delivery_lat'] as num?)?.toDouble() ?? 0
        : (_activeRestaurant?['lat'] as num?)?.toDouble() ?? 0;
    final destLng = pickedUp
        ? (_activeOrder!['delivery_lng'] as num?)?.toDouble() ?? 0
        : (_activeRestaurant?['lng'] as num?)?.toDouble() ?? 0;
    if (destLat == 0 && destLng == 0) {
      if (_polylines.isNotEmpty && mounted) {
        setState(() {
          _polylines = {};
          _countdownSecondsLeft = null;
        });
      }
      return;
    }
    final mode = switch (_vehicleType) {
      'foot' => 'walking',
      'bike' => 'bicycling',
      _ => 'driving',
    };
    final result = await api.route(
      _myPosition!.latitude,
      _myPosition!.longitude,
      destLat,
      destLng,
      mode: mode,
    );
    if (!mounted || mySeq != _routeRequestSeq) return; // eskirgan javob
    setState(() {
      _polylines = result.points.isEmpty
          ? {}
          : {
              Polyline(
                polylineId: const PolylineId('route'),
                points: result.points.map((p) => LatLng(p.lat, p.lng)).toList(),
                color: const Color(0xFFFF9800),
                width: 5,
              ),
            };
      // Bufer: har km uchun +2 daqiqa (foydalanuvchi so'rovi) — masalan
      // 3 km ~15 daqiqalik HAQIQIY yo'l bo'lsa, +6 daqiqa qo'shilib
      // ~21 daqiqadan orqa sanoq boshlanadi ("taxminan 20 daqiqa" —
      // real yo'l sharoitiga (svetofor, kutilmagan holat) zaxira).
      if (result.durationSeconds != null) {
        final distanceKm = (result.distanceMeters ?? 0) / 1000.0;
        final bufferSeconds = (distanceKm * 2 * 60).round();
        _countdownSecondsLeft = result.durationSeconds! + bufferSeconds;
      } else {
        _countdownSecondsLeft = null;
      }
    });
  }

  /// Orqa sanoqni HAR SONIYADA 1 kamaytiradi — `_updateRoute()` yangi
  /// (aniqroq) qiymat bilan qayta sinxronlashtirguncha silliq tik-tik
  /// hisoblab turadi (real navigatsiya ilovalaridagi kabi).
  void _startCountdownTicker() {
    _countdownTicker?.cancel();
    _countdownTicker = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      if (_countdownSecondsLeft == null || _countdownSecondsLeft! <= 0) return;
      setState(() => _countdownSecondsLeft = _countdownSecondsLeft! - 1);
    });
  }

  void _focusCamera(LatLng target) {
    unawaited(_animateTo(target, 16));
  }

  Future<void> _animateTo(LatLng target, double zoom) async {
    _programmaticMoves++;
    try {
      await _mapController?.animateCamera(
        CameraUpdate.newCameraPosition(
          CameraPosition(target: target, zoom: zoom),
        ),
      );
    } finally {
      _programmaticMoves--;
      _mapCenter = target;
      _currentZoom = zoom;
    }
  }

  void _zoom(double delta) {
    final newZoom = (_currentZoom + delta).clamp(2.0, 20.0);
    _animateTo(_mapCenter, newZoom);
  }

  /// "Joriy joylashuvim" tugmasi bosilganda chaqiriladi. `_refreshMyLocation`
  /// (fon rejimidagi, jim) dan FARQLI o'laroq, bu yerda har bir muvaffaqiyatsiz
  /// holat uchun ANIQ SnackBar ko'rsatiladi — foydalanuvchi tugmani bosganda
  /// "hech nima bo'lmadi" degan taassurot qolmasligi kerak: GPS o'chirilgan
  /// bo'lsa "Yoqish" tugmasi bilan to'g'ridan-to'g'ri tizim sozlamalariga,
  /// ruxsat butunlay rad etilgan bo'lsa "Sozlamalar" tugmasi bilan ilova
  /// ruxsatlariga yo'naltiriladi.
  Future<void> _goToMyLocation() async {
    setState(() => _locatingMe = true);
    try {
      final serviceEnabled = await Geolocator.isLocationServiceEnabled();
      if (!serviceEnabled) {
        _locationSnack(
          'Joylashuv xizmati (GPS) o\'chirilgan',
          actionLabel: 'Yoqish',
          onAction: Geolocator.openLocationSettings,
        );
        return;
      }
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.deniedForever) {
        _locationSnack(
          'Joylashuvga ruxsat berilmagan — sozlamalardan yoqing',
          actionLabel: 'Sozlamalar',
          onAction: Geolocator.openAppSettings,
        );
        return;
      }
      if (perm == LocationPermission.denied) {
        _locationSnack('Joylashuvga ruxsat berilmadi');
        return;
      }
      final pos = await Geolocator.getCurrentPosition();
      final latLng = LatLng(pos.latitude, pos.longitude);
      if (mounted) setState(() => _myPosition = latLng);
      await _animateTo(latLng, 16);
    } catch (_) {
      _locationSnack('Joylashuvni aniqlab bo\'lmadi');
    } finally {
      if (mounted) setState(() => _locatingMe = false);
    }
  }

  void _locationSnack(
    String message, {
    String? actionLabel,
    Future<bool> Function()? onAction,
  }) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        action: (actionLabel != null && onAction != null)
            ? SnackBarAction(label: actionLabel, onPressed: onAction)
            : null,
      ),
    );
  }

  // ---------- Taklifga javob ----------

  Future<void> _acceptOffer() async {
    final orderId = _offer!['order_id'] as String;
    setState(() => _respondingOffer = true);
    try {
      await api.respond(widget.courierId, orderId, true);
      _dismissOffer();
      // Yangi buyurtma — hali qayerga ham yetib bormagan, avvalgi
      // buyurtmadan qolgan "yetib keldim" belgisi TOZALANISHI kerak.
      _arrivedAtRestaurant = false;
      await _loadActiveOrder(orderId);
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.message)));
      _dismissOffer();
    } finally {
      if (mounted) setState(() => _respondingOffer = false);
    }
  }

  /// Taklif qabul qilingach, buyurtma serverga BIRIKTIRILISHI (AssignCourier)
  /// bir oz vaqt (fon jarayonida) oladi — shuning uchun bir necha marta
  /// qisqa orada qayta urinib ko'ramiz.
  Future<void> _loadActiveOrder(String orderId) async {
    setState(() => _loadingActiveOrder = true);
    Map<String, dynamic>? order;
    for (var i = 0; i < 6; i++) {
      try {
        order = await api.getOrder(orderId);
        break;
      } catch (_) {
        await Future.delayed(const Duration(milliseconds: 400));
      }
    }
    if (!mounted) return;
    if (order == null) {
      setState(() => _loadingActiveOrder = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Buyurtma yuklanmadi — "Yangilash" tugmasini bosing'),
        ),
      );
      return;
    }
    setState(() => _activeOrder = order);
    _deliveryAddress = null;
    _activeRestaurant = null;
    _restaurantIcon = null;
    unawaited(_enrichActiveOrder(order));
    setState(() => _loadingActiveOrder = false);
    _scheduleSheetResize();
  }

  Future<void> _enrichActiveOrder(Map<String, dynamic> order) async {
    final restaurantId = order['restaurant_id'] as String?;
    if (restaurantId != null && restaurantId.isNotEmpty) {
      try {
        final rest = await api.restaurant(restaurantId);
        if (mounted) setState(() => _activeRestaurant = rest);
        _scheduleSheetResize();
        final rLat = (rest['lat'] as num?)?.toDouble() ?? 0;
        final rLng = (rest['lng'] as num?)?.toDouble() ?? 0;
        if (rLat != 0 || rLng != 0) _focusCamera(LatLng(rLat, rLng));
        _buildRestaurantIcon(restaurantId, rest['logo_url'] as String?)
            .then((icon) {
              if (mounted) setState(() => _restaurantIcon = icon);
            })
            .catchError((Object e, StackTrace st) {
              debugPrint('[restaurant-icon] yuklashda XATO: $e\n$st');
            });
      } catch (_) {}
    }
    final lat = (order['delivery_lat'] as num?)?.toDouble() ?? 0;
    final lng = (order['delivery_lng'] as num?)?.toDouble() ?? 0;
    if (lat != 0 || lng != 0) {
      try {
        final addr = await api.reverseGeocode(lat, lng);
        if (mounted) setState(() => _deliveryAddress = addr);
        _scheduleSheetResize();
      } catch (_) {}
      _focusCamera(LatLng(lat, lng));
    }
    // Restoran/mijoz manzili endi ma'lum — kuryerdan shu nuqtagacha
    // marshrut chizig'ini chizamiz.
    unawaited(_updateRoute());
  }

  /// Kuryer restoranda buyurtma raqamining OXIRGI 4 xonasini xodimga
  /// og'zaki aytib, taomni qo'lga oladi — ilova ichida hech qanday
  /// "Yetib keldim" — kuryer restoranga JISMONAN yetib borganini
  /// bildiradi. MAHALLIY (faqat shu qurilmada) holat — backend'ga hech
  /// qanday so'rov YO'Q, chunki bunday oraliq bosqich backend'da umuman
  /// kuzatilmaydi (faqat ready/picked_up bor). Taom hali tayyor
  /// bo'lmasa ham bosish mumkin (foydalanuvchi so'rovi bo'yicha
  /// tuzatildi) — shundan keyingina taom holatiga (tayyorlanmoqda/
  /// tayyor) mos ekran ko'rsatiladi.
  Future<bool> _markArrivedAtRestaurant() async {
    setState(() => _arrivedAtRestaurant = true);
    _scheduleSheetResize();
    return true;
  }

  /// tasdiqlash kodi kerak emas, restoran ham tugma bosmaydi (Yandex Eats
  /// uslubi). Shu tugma bosilgach, server TO'LIQ buyurtmani (taomlar,
  /// mijoz manzili — bulargacha yashiringan edi) qaytaradi.
  Future<bool> _confirmPickedUp() async {
    final orderId = _activeOrder!['id'] as String;
    setState(() => _transitioning = true);
    try {
      final updated = await api.transition(orderId, 'picked_up');
      if (!mounted) return true;
      setState(() => _activeOrder = updated);
      _scheduleSheetResize();
      unawaited(_enrichActiveOrder(updated));
      return true;
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.message)));
      }
      return false;
    } catch (_) {
      return false;
    } finally {
      if (mounted) setState(() => _transitioning = false);
    }
  }

  Future<bool> _markDelivered() async {
    final orderId = _activeOrder!['id'] as String;
    setState(() => _transitioning = true);
    try {
      await api.transition(orderId, 'delivered');
      if (mounted) {
        setState(() {
          _activeOrder = null;
          _arrivedAtRestaurant = false;
        });
      }
      _scheduleSheetResize();
      return true;
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.message)));
      }
      return false;
    } catch (_) {
      return false;
    } finally {
      if (mounted) setState(() => _transitioning = false);
    }
  }

  /// Akkaunt server tomonda o'chirilgan/bekor qilingan — chiqamiz.
  ///
  /// `_logout` dan farqi: bu yerda `api.logout()` CHAQIRILMAYDI. Token
  /// allaqachon yaroqsiz, ya'ni o'sha so'rov 401 bilan qaytardi va
  /// foydalanuvchi kirish ekraniga o'tishdan oldin bekorga kutardi.
  Future<void> _forceLogout() async {
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
      content: Text('Akkaunt o\'chirildi'),
      backgroundColor: Colors.red,
    ));
    Navigator.of(
      context,
    ).pushReplacement(MaterialPageRoute(builder: (_) => const LoginScreen()));
  }

  Future<void> _logout() async {
    // Server tomonda ham bekor qilinadi (token o'chirilishidan OLDIN —
    // so'rov aynan shu token bilan yuboriladi).
    await api.logout();
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(
      context,
    ).pushReplacement(MaterialPageRoute(builder: (_) => const LoginScreen()));
  }

  Set<Marker> _buildMarkers() {
    final markers = <Marker>{};

    // Hozirgi maqsad (picked_up'gacha — restoran, undan keyin — mijoz
    // manzili) — "men" belgisining Yandex Navi uslubidagi o'qi AYNAN shu
    // nuqtaga qarab buriladi (foydalanuvchi so'ragan "B nuqtaga qarab
    // xarakatlanish").
    LatLng? destination;
    if (_activeOrder != null) {
      final pickedUp = _activeOrder!['status'] == 'picked_up';
      final lat = pickedUp
          ? (_activeOrder!['delivery_lat'] as num?)?.toDouble() ?? 0
          : (_activeRestaurant?['lat'] as num?)?.toDouble() ?? 0;
      final lng = pickedUp
          ? (_activeOrder!['delivery_lng'] as num?)?.toDouble() ?? 0
          : (_activeRestaurant?['lng'] as num?)?.toDouble() ?? 0;
      if (lat != 0 || lng != 0) destination = LatLng(lat, lng);
    }

    if (_myPosition != null) {
      if (destination != null) {
        _navBearing = _bearingBetween(_myPosition!, destination);
      }
      markers.add(
        Marker(
          markerId: const MarkerId('me'),
          position: _myPosition!,
          icon:
              _navIcon ??
              BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueAzure),
          // Markazdan buriladi (pin uchidan emas) — xuddi navigatsiya
          // ilovalaridagi "joriy joylashuv o'qi" kabi. `flat: true` —
          // xarita burilsa/qiyalansa ham yer yuzasiga nisbatan to'g'ri
          // yo'nalishda qoladi (kamera ekraniga emas).
          anchor: const Offset(0.5, 0.5),
          rotation: _navBearing,
          flat: true,
          infoWindow: const InfoWindow(title: 'Siz'),
        ),
      );
    }
    if (_activeOrder != null) {
      final pickedUp = _activeOrder!['status'] == 'picked_up';
      if (!pickedUp) {
        final lat = (_activeRestaurant?['lat'] as num?)?.toDouble() ?? 0;
        final lng = (_activeRestaurant?['lng'] as num?)?.toDouble() ?? 0;
        if (lat != 0 || lng != 0) {
          markers.add(
            Marker(
              markerId: const MarkerId('restaurant'),
              position: LatLng(lat, lng),
              icon:
                  _restaurantIcon ??
                  BitmapDescriptor.defaultMarkerWithHue(
                    BitmapDescriptor.hueOrange,
                  ),
              // `_restaurantIcon`da GPS nuqtasi rasmning pastidagi kichik
              // nuqta markazida (logotip emas!) — shu sabab anchor ham shu
              // nuqtaga to'g'irlanadi. Hali yuklanmagan bo'lsa (standart
              // pin), pinning odatdagi pastki uchi ishlatiladi.
              anchor: _restaurantIcon != null
                  ? const Offset(0.5, _restaurantIconAnchorY)
                  : const Offset(0.5, 1.0),
              infoWindow: InfoWindow(
                title: _activeRestaurant?['name'] as String? ?? 'Restoran',
              ),
            ),
          );
        }
      } else {
        final lat = (_activeOrder!['delivery_lat'] as num?)?.toDouble() ?? 0;
        final lng = (_activeOrder!['delivery_lng'] as num?)?.toDouble() ?? 0;
        if (lat != 0 || lng != 0) {
          markers.add(
            Marker(
              markerId: const MarkerId('delivery'),
              position: LatLng(lat, lng),
              icon: BitmapDescriptor.defaultMarkerWithHue(
                BitmapDescriptor.hueGreen,
              ),
              infoWindow: const InfoWindow(title: 'Mijoz manzili'),
            ),
          );
        }
      }
    }
    return markers;
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return Scaffold(
      // Xarita tabida AppBar YO'Q — nom o'rniga xaritaning o'zida suzuvchi
      // onlayn/oflayn tugmasi turadi (pastga qarang, _buildMapTab). Profil
      // tabida esa odatdagidek AppBar bilan ism ko'rsatiladi.
      appBar: _tabIndex == 1
          ? AppBar(title: Text(_name.isEmpty ? 'Kuryer' : _name))
          : null,
      body: Stack(
        children: [
          // 1-QATLAM — XARITA: butunlay MUSTAQIL, doimiy qatlam, tab/panel
          // bilan bir Stack'ni BO'LISHMAYDI. Shu ajratish tufayli pastdagi
          // DraggableScrollableSheet qanchalik tortilsa/animatsiya qilinsa
          // ham, bu qatlamning render daraxti umuman qayta o'lchanmaydi —
          // xarita 1mm ham qimirlamaydi (avvalgi versiyada xarita va panel
          // BITTA Stack ichida "qo'shni" edi — panel animatsiyasi umumiy
          // qatlamni qayta chizishga majburlab, xaritaning veb platforma-view
          // (iframe) pozitsiyasi bilan "sakrab" ketishiga sabab bo'lgan edi).
          if (_approved == true)
            Positioned.fill(child: RepaintBoundary(child: _buildMapLayer())),
          // 2-QATLAM — TABLAR: xaritaning USTIDA suzadi, lekin undan
          // to'liq mustaqil filial (branch) sifatida.
          IndexedStack(
            index: _tabIndex,
            children: [
              _approved != true
                  ? _PendingApproval(onRefresh: _init)
                  : _buildMapOverlay(),
              _ProfileTab(name: _name, approved: _approved, onLogout: _logout),
            ],
          ),
        ],
      ),
      // Foydalanuvchi so'rovi (2026-07-30): kuryer ONLAYN bo'lganda pastki
      // Xarita/Profil menyusi YASHIRILADI — ish vaqtida e'tiborni
      // chalg'itmasin, butun ekran xarita+suzuvchi panelga tegishli
      // bo'lsin. OFLAYNda esa qaytadan ko'rinadi (Profilga o'tish uchun).
      bottomNavigationBar: _online
          ? null
          : NavigationBar(
              selectedIndex: _tabIndex,
              onDestinationSelected: (i) => setState(() => _tabIndex = i),
              destinations: const [
                NavigationDestination(
                  icon: Icon(Icons.map_outlined),
                  selectedIcon: Icon(Icons.map),
                  label: 'Xarita',
                ),
                NavigationDestination(
                  icon: Icon(Icons.person_outline),
                  selectedIcon: Icon(Icons.person),
                  label: 'Profil',
                ),
              ],
            ),
    );
  }

  /// FAQAT xaritaning o'zi — hech qanday panel/tugma bilan bir Stack'da
  /// EMAS. `build()`da mustaqil qatlam sifatida qo'yiladi va `_tabIndex`
  /// yoki panel holatidan butunlay mustaqil qoladi.
  Widget _buildMapLayer() {
    if (_mapsFailed) return const Center(child: Text('Xarita yuklanmadi'));
    if (!_mapsReady) return const Center(child: CircularProgressIndicator());
    return GoogleMap(
      initialCameraPosition: CameraPosition(
        target: _myPosition ?? _chustCenter,
        zoom: _currentZoom,
      ),
      mapType: _mapType,
      onMapCreated: (c) => _mapController = c,
      onCameraMove: (pos) {
        if (_programmaticMoves > 0) return;
        _mapCenter = pos.target;
        _currentZoom = pos.zoom;
      },
      myLocationEnabled: false,
      myLocationButtonEnabled: false,
      zoomControlsEnabled: false,
      mapToolbarEnabled: false,
      compassEnabled: false,
      markers: _buildMarkers(),
      polylines: _polylines,
    );
  }

  /// Xarita USTIDA suzadigan qism: onlayn/oflayn, 3D/2D, zoom, "joriy
  /// joylashuvim" va pastdagi suzuvchi panel — xaritaning o'zi bu yerda
  /// UMUMAN yo'q (u alohida, mustaqil qatlamda — `_buildMapLayer`).
  Widget _buildMapOverlay() {
    final topInset = MediaQuery.of(context).padding.top + 12;
    return Stack(
      children: [
        // MUHIM: har bir suzuvchi element `PointerInterceptor` bilan
        // o'ralgan. Google Maps veb'da xom HTML platform-view sifatida
        // chiziladi — Flutter gesture arenasidan TASHQARIDA — shuning uchun
        // bu himoyasiz qoldirilsa, shu tugma/panel USTIDAGI sichqoncha/
        // barmoq harakati "sizib o'tib" pastdagi xaritaning O'ZINI ham
        // surib/aylantirib yuboradi (aynan "panel tortilsa, xarita ham
        // ko'tarilib-tushadi" muammosining haqiqiy sababi shu edi).
        //
        // Onlayn/oflayn — image/online.png va image/offline.png dagi kabi
        // XARITANING YUQORI-O'RTASIDA. ENDI dropdown EMAS — oddiy bosish:
        // bosilsa darhol qarama-qarshi holatga o'tadi (bir bosish =
        // oflayn, yana bir bosish = onlayn), Yandex Eats'dagi kabi.
        Positioned(
          left: 0,
          right: 0,
          top: topInset,
          child: Center(
            child: PointerInterceptor(
              child: _StatusPill(
                online: _online,
                onTap: () => _toggleOnline(!_online),
              ),
            ),
          ),
        ),
        // Onlayn bo'lganda yuqori-CHAP burchakda "yaqin atrofda buyurtma
        // qidirilmoqda" animatsiyasi — aylanib turadigan dumaloq belgi
        // (Yandex Eats uslubida). Oflaynda butunlay ko'rinmaydi.
        if (_online)
          Positioned(
            left: 16,
            top: topInset,
            child: PointerInterceptor(
              child: RotationTransition(
                turns: _searchSpinController,
                child: Material(
                  color: Theme.of(context).colorScheme.surface,
                  shape: const CircleBorder(),
                  elevation: 3,
                  child: const Tooltip(
                    message: 'Yaqin atrofda buyurtma qidirilmoqda...',
                    child: Padding(
                      padding: EdgeInsets.all(10),
                      child: Icon(Icons.radar, size: 22),
                    ),
                  ),
                ),
              ),
            ),
          ),
        // 3D/2D — yuqori o'ng burchakda, yolg'iz.
        Positioned(
          right: 12,
          top: topInset,
          child: PointerInterceptor(
            child: _RoundButton(
              tooltip: _mapType == MapType.normal
                  ? 'Sputnik (3D) ko\'rinish'
                  : 'Oddiy xarita (2D)',
              icon: _mapType == MapType.normal
                  ? Icons.threed_rotation
                  : Icons.map,
              onTap: () => setState(() {
                _mapType = _mapType == MapType.normal
                    ? MapType.hybrid
                    : MapType.normal;
              }),
            ),
          ),
        ),
        // Joriy joylashuv + zoom — pastki o'ng burchakda, suzuvchi panelning
        // yig'ilgan (minChildSize) holatidan yuqorida qoladi.
        Positioned(
          right: 12,
          bottom: MediaQuery.of(context).size.height * 0.14 + 16,
          width: 48,
          child: PointerInterceptor(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              mainAxisSize: MainAxisSize.min,
              children: [
                _RoundButton(
                  tooltip: 'Joriy joylashuvim',
                  icon: _locatingMe ? Icons.hourglass_top : Icons.my_location,
                  onTap: _locatingMe ? null : _goToMyLocation,
                ),
                const SizedBox(height: 14),
                _ZoomBlock(
                  onZoomIn: () => _zoom(1),
                  onZoomOut: () => _zoom(-1),
                ),
              ],
            ),
          ),
        ),
        // Suzuvchi panel — Yandex Go/Uber uslubida, tortib katta-kichik
        // qilish mumkin: pastda qisqa xulosa, yuqoriga tortilsa to'liq
        // tafsilot (buyurtma tarkibi va h.k.) ko'rinadi. FAQAT onlaynda —
        // oflaynda butunlay yashiringan (foydalanuvchi talabi).
        if (_online)
          DraggableScrollableSheet(
            controller: _sheetController,
            // Statik boshlang'ich qiymat — ilk frame chizilgach
            // `_scheduleSheetResize()` darhol haqiqiy mazmun balandligiga
            // moslab qayta o'lchaydi (pastga qarang).
            initialChildSize: 0.36,
            minChildSize: 0.14,
            // Foydalanuvchi so'rovi: eng balandi ekranning YARMIGACHA —
            // undan yuqoriga ko'tarilib, xaritadagi kartani yopib
            // qo'ymasin (qo'lda tortib ham, avtomatik o'lchanganda ham).
            maxChildSize: 0.5,
            builder: (context, scrollController) {
              final footer = _buildFooterAction();
              // Butun panel `PointerInterceptor` bilan o'ralgan — tortish
              // "handle"idan boshlab ichidagi ro'yxatgacha, hech bir joyda
              // sichqoncha bosilishi pastdagi xaritaga o'tib ketmaydi.
              return PointerInterceptor(
                child: Container(
                  decoration: BoxDecoration(
                    color: Theme.of(context).scaffoldBackgroundColor,
                    borderRadius: const BorderRadius.vertical(
                      top: Radius.circular(24),
                    ),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.3),
                        blurRadius: 16,
                        offset: const Offset(0, -4),
                      ),
                    ],
                  ),
                  // MUHIM (foydalanuvchi so'rovi, 2026-07-30): "Qabul
                  // qilish"/"Yetib keldim"/"Mijozga yetkazdim" tugmasi
                  // ENDI QOTIRILGAN (fixed) — ekranning eng pastida
                  // doim ko'rinadi, qolgan barcha ma'lumot (countdown,
                  // manzil, buyurtma tafsiloti, yordam bloklari) esa
                  // YUQORIDAGI qismda mustaqil SCROLL qilinadi. Avval
                  // hammasi BITTA scroll ichida edi — kontent
                  // ko'payishi bilan (yangi dizayn) tugma pastga
                  // "yashirinib" ketishi mumkin edi.
                  child: Column(
                    children: [
                      Expanded(
                        child: SingleChildScrollView(
                          controller: scrollController,
                          padding: EdgeInsets.fromLTRB(
                            20,
                            10,
                            20,
                            footer != null ? 12 : 24,
                          ),
                          // `key` — `_scheduleSheetResize()` shu
                          // Column'ning HAQIQIY (barcha bolalari
                          // yig'indisi) balandligini o'lchab, panelni
                          // statik EMAS, aynan shu mazmunga mos
                          // balandlikka ko'taradi/tushiradi.
                          child: Column(
                            key: _panelContentKey,
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              Center(
                                child: Container(
                                  width: 40,
                                  height: 4,
                                  margin: const EdgeInsets.only(bottom: 16),
                                  decoration: BoxDecoration(
                                    color: Colors.grey.shade600,
                                    borderRadius: BorderRadius.circular(4),
                                  ),
                                ),
                              ),
                              if (_offer != null)
                                _OfferCard(offer: _offer!)
                              else if (_loadingActiveOrder)
                                const Center(
                                  child: CircularProgressIndicator(),
                                )
                              else if (_activeOrder != null)
                                _ActiveOrderCard(
                                  order: _activeOrder!,
                                  restaurant: _activeRestaurant,
                                  deliveryAddress: _deliveryAddress,
                                  countdownSecondsLeft: _countdownSecondsLeft,
                                  arrivedAtRestaurant: _arrivedAtRestaurant,
                                )
                              else
                                const _WaitingCard(),
                            ],
                          ),
                        ),
                      ),
                      // MUHIM (real qurilmada topilgan bug, 2026-07-30):
                      // qurilmaning O'ZINING tizim navigatsiya paneli
                      // (orqaga/uy tugmalari) fixed footer'ni QISMAN
                      // yopib qo'yardi — `SafeArea` shu pastki
                      // "xavfsiz bo'lmagan" zonani avtomatik hisobga
                      // oladi (gesture-navigatsiya ham, 3-tugmali
                      // navigatsiya ham to'g'ri ishlaydi).
                      if (footer != null)
                        SafeArea(
                          top: false,
                          child: Container(
                            key: _footerKey,
                            padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
                            child: footer,
                          ),
                        ),
                    ],
                  ),
                ),
              );
            },
          ),
      ],
    );
  }
}

class _ProfileTab extends StatelessWidget {
  final String name;
  final bool? approved;
  final VoidCallback onLogout;
  const _ProfileTab({
    required this.name,
    required this.approved,
    required this.onLogout,
  });

  @override
  Widget build(BuildContext context) {
    // MUHIM: xarita endi Scaffold body'da mustaqil, DOIMIY qatlam sifatida
    // shu tabning ORQASIDA ham chizilib turadi — shuning uchun bu yerda
    // qattiq fon rang shart, aks holda Profil tabida xarita orqadan "ko'rinib"
    // qolar edi.
    return Container(
      color: Theme.of(context).scaffoldBackgroundColor,
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 28,
                  backgroundColor: Theme.of(
                    context,
                  ).colorScheme.surfaceContainerHighest,
                  child: const Icon(Icons.person, size: 32),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        name.isEmpty ? 'Kuryer' : name,
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        approved == true
                            ? 'Tasdiqlangan kuryer'
                            : 'Tasdiq kutilmoqda',
                        style: TextStyle(
                          color: approved == true
                              ? Colors.green
                              : Colors.orange,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),
            const Divider(height: 1),
            ListTile(
              leading: const Icon(Icons.logout, color: Colors.red),
              title: const Text('Chiqish', style: TextStyle(color: Colors.red)),
              onTap: onLogout,
            ),
          ],
        ),
      ),
    );
  }
}

class _PendingApproval extends StatelessWidget {
  final Future<void> Function() onRefresh;
  const _PendingApproval({required this.onRefresh});

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          const SizedBox(height: 80),
          Icon(
            Icons.hourglass_top,
            size: 64,
            color: Theme.of(context).colorScheme.primary,
          ),
          const SizedBox(height: 20),
          Text(
            'Arizangiz ko\'rib chiqilmoqda',
            textAlign: TextAlign.center,
            style: Theme.of(
              context,
            ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            'Superadmin tasdiqlagach, onlayn bo\'lib buyurtmalar qabul qila olasiz. Tasdiqlanganini tekshirish uchun pastga torting.',
            textAlign: TextAlign.center,
            style: TextStyle(color: Colors.grey.shade400),
          ),
        ],
      ),
    );
  }
}

/// Xaritaning yuqori-o'rtasida suzuvchi onlayn/oflayn tugmasi (AppBar'dagi
/// kuryer nomi o'rniga — xarita tabida AppBar umuman ko'rsatilmaydi).
/// ENDI oddiy TOGGLE: bosilsa darhol qarama-qarshi holatga o'tadi (bir
/// bosish = oflayn, yana bir bosish = onlayn) — dropdown/tanlov YO'Q,
/// Yandex Eats kuryer ilovasidagi kabi.
class _StatusPill extends StatelessWidget {
  static const _onlineColor = Color(0xFF1FAE4A);

  final bool online;
  final VoidCallback onTap;
  const _StatusPill({required this.online, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final fg = online ? Colors.white : Theme.of(context).colorScheme.onSurface;
    return Material(
      color: online ? _onlineColor : Theme.of(context).colorScheme.surface,
      borderRadius: BorderRadius.circular(28),
      elevation: 4,
      child: InkWell(
        borderRadius: BorderRadius.circular(28),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 10,
                height: 10,
                decoration: BoxDecoration(
                  color: online ? Colors.white : Colors.grey.shade500,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 8),
              Text(
                online ? 'Onlayn' : 'Offline',
                style: TextStyle(
                  color: fg,
                  fontWeight: FontWeight.bold,
                  fontSize: 15,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _RoundButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback? onTap;
  final String? tooltip;
  const _RoundButton({required this.icon, required this.onTap, this.tooltip});

  @override
  Widget build(BuildContext context) {
    final btn = Material(
      color: Theme.of(context).colorScheme.surface,
      shape: const CircleBorder(),
      elevation: 3,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(10),
          child: Icon(icon, size: 22),
        ),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}

/// Zoom bloki — faqat kattalashtirish/kichiklashtirish, boshqa ikon yo'q.
class _ZoomBlock extends StatelessWidget {
  final VoidCallback onZoomIn;
  final VoidCallback onZoomOut;
  const _ZoomBlock({required this.onZoomIn, required this.onZoomOut});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 44,
      child: Material(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(22),
        elevation: 3,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            InkWell(
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(22),
              ),
              onTap: onZoomIn,
              child: const Padding(
                padding: EdgeInsets.symmetric(vertical: 16, horizontal: 10),
                child: Icon(Icons.add, size: 20),
              ),
            ),
            Container(width: 22, height: 1, color: Colors.grey.shade400),
            InkWell(
              borderRadius: const BorderRadius.vertical(
                bottom: Radius.circular(22),
              ),
              onTap: onZoomOut,
              child: const Padding(
                padding: EdgeInsets.symmetric(vertical: 16, horizontal: 10),
                child: Icon(Icons.remove, size: 20),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Suzuvchi panel faqat ONLAYN holatda ko'rinadi (oflaynda butunlay
/// yashiringan) — shuning uchun bu kartochka doim "kutilmoqda" holatini
/// ko'rsatadi, oflayn variantga ehtiyoj yo'q.
class _WaitingCard extends StatelessWidget {
  const _WaitingCard();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 40),
      child: Column(
        children: [
          const Icon(
            Icons.notifications_active_outlined,
            size: 48,
            color: Colors.grey,
          ),
          const SizedBox(height: 12),
          Text(
            'Yangi buyurtma kutilmoqda...',
            textAlign: TextAlign.center,
            style: TextStyle(color: Colors.grey.shade400, fontSize: 15),
          ),
        ],
      ),
    );
  }
}

/// Yangi buyurtma taklifi kartochkasi — endi o'rtada suzuvchi modal EMAS,
/// suzuvchi panelning (DraggableScrollableSheet) ICHIDA ko'rsatiladi
/// (image/zakaz.png namunasidagi kabi): restoran nomi, undan pastda
/// kuryerning HOZIRGI joylashuvidan restorangacha necha daqiqa yo'l, o'ng
/// tomonda OnDex belgisi va pastda "Qayerdan" (restoran manzili). Alohida
/// "Rad etish" tugmasi YO'Q — vaqt tugasa backend (`offerTTL`) o'zi
/// avtomatik rad etadi, bosilmasa ham.
class _OfferCard extends StatelessWidget {
  final Map<String, dynamic> offer;
  const _OfferCard({required this.offer});

  @override
  Widget build(BuildContext context) {
    final name = (offer['restaurant_name'] as String?)?.trim();
    final address = (offer['restaurant_address'] as String?)?.trim();
    final etaMinutes = (offer['eta_minutes'] as num?)?.toInt();

    // MUHIM (foydalanuvchi so'rovi, 2026-07-30): avval alohida fon+padding
    // bilan alohida "blok" edi — endi panelning o'zida to'g'ridan-to'g'ri
    // (ikki qatlam padding olib tashlandi, to'liq kenglik ishlatiladi).
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name?.isNotEmpty == true ? name! : 'Yangi buyurtma',
                    style: const TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 4),
                  // ANIQ: bu VAQT (daqiqa), MASOFA (km) emas — kuryer
                  // restoranga qachon yetib borishini bilib, KECHIKMASLIGI
                  // uchun (foydalanuvchi so'rovi bo'yicha aniqlashtirildi,
                  // avval "Masofa hisoblanmoqda..." deb chalkashtirilgan edi).
                  Text(
                    etaMinutes != null ? '$etaMinutes daqiqa' : 'Hisoblanmoqda...',
                    style: TextStyle(color: Colors.grey.shade400),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: Image.asset(
                'assets/ondex.png',
                width: 48,
                height: 48,
                fit: BoxFit.cover,
              ),
            ),
          ],
        ),
        if (address?.isNotEmpty == true) ...[
          const SizedBox(height: 12),
          Text(
            'Qayerdan',
            style: TextStyle(fontSize: 11, color: Colors.grey.shade500),
          ),
          const SizedBox(height: 2),
          Text(address!, style: const TextStyle(fontWeight: FontWeight.w600)),
        ],
      ],
    );
  }
}

/// "Qabul qilish" tugmasi — image/zakaz.png namunasidagi sariq "Принять"ning
/// STRUKTURASI bilan bir xil, lekin rangi BREND RANGI (`kBrandColor`,
/// #F64E03 — foydalanuvchi so'rovi bo'yicha, boshqa barcha
/// tugmalar/slayderlar bilan bir xil): tugma FONI soniyalar o'tishi bilan
/// chapdan o'ngga "eriydi" (drain) — vizual taymer. Vaqt tugaguncha
/// bosilmasa, dispatcher backend darajasida (`offerTTL`) avtomatik rad
/// etadi — `_offerCountdown` aynan shu payt `_dismissOffer()`ni chaqiradi.
class _AcceptCountdownButton extends StatelessWidget {
  static const _orange = kBrandColor;

  final int secondsLeft;
  final int totalSeconds;
  final bool busy;
  final VoidCallback onAccept;
  const _AcceptCountdownButton({
    required this.secondsLeft,
    required this.totalSeconds,
    required this.busy,
    required this.onAccept,
  });

  @override
  Widget build(BuildContext context) {
    final fraction = totalSeconds <= 0
        ? 0.0
        : (secondsLeft / totalSeconds).clamp(0.0, 1.0);
    return ClipRRect(
      borderRadius: BorderRadius.circular(12),
      child: SizedBox(
        height: 52,
        child: Stack(
          fit: StackFit.expand,
          children: [
            Container(color: _orange.withValues(alpha: 0.25)),
            AnimatedFractionallySizedBox(
              duration: const Duration(seconds: 1),
              curve: Curves.linear,
              alignment: Alignment.centerLeft,
              widthFactor: fraction,
              child: Container(color: _orange),
            ),
            Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: busy ? null : onAccept,
                child: Center(
                  child: busy
                      ? const SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : const Text(
                          'Qabul qilish',
                          style: TextStyle(
                            color: Colors.white,
                            fontWeight: FontWeight.bold,
                            fontSize: 16,
                          ),
                        ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Buyurtma raqamining ("DDMMYY-0000123" kabi) oxirgi 4 xonasi — kuryer
/// restoranda shu raqamni og'zaki aytadi.
String _lastFourDigits(String orderNumber) => orderNumber.length <= 4
    ? orderNumber
    : orderNumber.substring(orderNumber.length - 4);

/// Soniyani "MM:SS" ko'rinishiga o'giradi — orqa sanoq uchun (image/bu.png
/// namunasidagi "12:55" uslubida).
String _formatCountdown(int totalSeconds) {
  final s = totalSeconds < 0 ? 0 : totalSeconds;
  final minutes = s ~/ 60;
  final seconds = s % 60;
  return '${minutes.toString().padLeft(2, '0')}:${seconds.toString().padLeft(2, '0')}';
}

Future<void> _callPhone(String phone) async {
  try {
    await launchUrl(Uri(scheme: 'tel', path: phone));
  } catch (_) {
    // Qurilmada qo'ng'iroq imkoniyati bo'lmasa (masalan veb) — jimgina
    // o'tkazib yuboriladi, bloklovchi xato ko'rsatilmaydi.
  }
}

/// Zaxira navigatsiya: ilova ichidagi xarita ishlamay qolsa yoki kuryer
/// o'zi o'rgangan xarita ilovasini (Google Maps, Yandex Navi va h.k.)
/// ishlatmoqchi bo'lsa — qurilmaning STANDART xarita ilovasini ochadi,
/// manzil o'sha yerda avtomatik chiziladi. Universal havola (`google.com/
/// maps/dir`) — Android/iOS/veb barchasida ishlaydi, alohida platforma
/// tekshiruvi shart emas.
Future<void> _openInDeviceMaps(double lat, double lng) async {
  final uri = Uri.parse(
    'https://www.google.com/maps/dir/?api=1&destination=$lat,$lng',
  );
  try {
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  } catch (_) {
    // Xarita ilovasi topilmasa ham — bloklovchi xato ko'rsatilmaydi.
  }
}

void _showHelpDialog(BuildContext context, String title, String message) {
  showDialog<void>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(title),
      content: Text(message),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(),
          child: const Text('Tushunarli'),
        ),
      ],
    ),
  );
}

/// Joriy buyurtma kartochkasi — Yandex Pro uslubidagi dizayn
/// (`image/bu.png` namunasiga mos, foydalanuvchi so'rovi bo'yicha,
/// 2026-07-30): tepada OnDex logotipi, undan pastda joriy maqsadgacha
/// (picked_up'gacha — restoran, undan keyin — mijoz) ORQA SANOQ (MM:SS),
/// bosqich ko'rsatkichi (restoran/mijoz ikonkalari), yuboruvchi/qabul
/// qiluvchi nomi + qo'ng'iroq tugmasi, manzil, buyurtma raqami+narxi,
/// yordam bloklari va eng pastda amal tugmasi:
/// - Hali "picked_up" bo'lmagan ("ready"da): "Yetib keldim" — taomlar
///   ro'yxati va mijoz manzili ATAYLAB YASHIRILGAN (backend ham buni
///   qat'iy ta'minlaydi), shuning uchun bu bosqichda faqat restoran
///   ma'lumoti ko'rinadi.
/// - "Yetib keldim" bosilgach ("picked_up"): "Mijozga yetkazdim" — endi
///   to'liq tarkib (taomlar) ham ko'rinadi.
/// Buyurtmadagi `delivery_address` — mijoz xarita ekranida kiritgan
/// tafsilotlar SURATI (backend buni buyurtma yaratilganda nusxalab
/// saqlaydi). Bo'sh maydonlar tashlab yuboriladi.
String _addressDetails(Map<String, dynamic> order) {
  final a = order['delivery_address'];
  if (a is! Map) return '';
  final parts = <String>[];
  void add(String label, String key) {
    final v = (a[key] as String?)?.trim();
    if (v != null && v.isNotEmpty) parts.add('$label $v');
  }

  add('Podyezd', 'entrance');
  add('Qavat', 'floor');
  add('Kvartira', 'apartment');
  add('Domofon', 'intercom');
  return parts.join(' · ');
}

String _addressComment(Map<String, dynamic> order) {
  final a = order['delivery_address'];
  if (a is! Map) return '';
  return (a['comment'] as String?)?.trim() ?? '';
}

class _ActiveOrderCard extends StatelessWidget {
  final Map<String, dynamic> order;
  final Map<String, dynamic>? restaurant;
  final String? deliveryAddress;
  // Joriy maqsadgacha (hali picked_up bo'lmasa — restoran, keyin — mijoz)
  // orqa sanoq (soniyada, HAQIQIY yo'l vaqti + xavfsizlik buferi) — kuryer
  // KECHIKMASLIGI uchun butun yo'l davomida ko'rinib turadi. `null` bo'lsa
  // hali hisoblanmoqda (yoki joylashuv noma'lum).
  final int? countdownSecondsLeft;
  // Kuryer restoranga JISMONAN yetib borganini bildirgan-bildirmagani
  // (mahalliy holat, "Yetib keldim" tugmasi) — tepadagi orqa sanoq
  // blokini to'g'ri xabar bilan almashtirish uchun kerak (yetib
  // borgandan keyin "yuboruvchining oldiga boring" degan matn
  // ma'nosizga aylanadi).
  final bool arrivedAtRestaurant;

  const _ActiveOrderCard({
    required this.order,
    required this.restaurant,
    required this.deliveryAddress,
    required this.countdownSecondsLeft,
    required this.arrivedAtRestaurant,
  });

  @override
  Widget build(BuildContext context) {
    final status = order['status'] as String? ?? 'ready';
    final pickedUp = status == 'picked_up';
    final ready = status == 'ready';
    final items = (order['items'] as List?) ?? [];
    final total = (order['total_tiyin'] as num? ?? 0).toInt();
    final orderNumber = order['order_number']?.toString() ?? '—';
    final restaurantName = restaurant?['name'] as String? ?? 'Restoran';
    final restaurantAddress = restaurant?['address'] as String? ?? '';
    // Restoran/mijoz telefon raqami — backend faqat shu buyurtma orqali,
    // FAQAT kuryer/adminga beradi (GET /orders/{id}, `restaurant_phone`/
    // `customer_phone`). Mijoz raqami ham xuddi manzil/taomlar kabi FAQAT
    // picked_up'dan keyin ochiladi (redaksiya bilan bir xil falsafa —
    // kuryer restoranga bormasdan mijoz bilan bog'lanmasin).
    final restaurantPhone = order['restaurant_phone'] as String?;
    final customerPhone = order['customer_phone'] as String?;

    final destinationLabel = pickedUp ? 'Qabul qiluvchi' : 'Yuboruvchi';
    final destinationName = pickedUp ? 'Mijoz' : restaurantName;
    final destinationAddress = pickedUp
        ? (deliveryAddress ?? 'Manzil aniqlanmoqda...')
        : restaurantAddress;
    final callPhone = pickedUp ? customerPhone : restaurantPhone;
    // "Xarita ishlamay qolsa / o'zi o'rgangan xaritani ishlatmoqchi
    // bo'lsa" — zaxira sifatida qurilmaning o'z xarita ilovasiga
    // yo'naltiruvchi tugma (foydalanuvchi so'rovi, 2026-07-30).
    final destLat = pickedUp
        ? (order['delivery_lat'] as num?)?.toDouble()
        : (restaurant?['lat'] as num?)?.toDouble();
    final destLng = pickedUp
        ? (order['delivery_lng'] as num?)?.toDouble()
        : (restaurant?['lng'] as num?)?.toDouble();
    final hasMapFallback =
        destLat != null && destLng != null && (destLat != 0 || destLng != 0);

    // MUHIM (foydalanuvchi so'rovi, 2026-07-30): bu widget avval alohida
    // `Card` (o'z foni+padding'i bilan) edi — DraggableScrollableSheet
    // O'ZI ham allaqachon padding+fon beradi, natijada IKKI QATLAM
    // padding qo'shilib (sheet 20px + card 18px), matn/tugmalar
    // kerakidan ko'ra torroq bo'lib qolar edi. Endi bu widget
    // panelning o'z konteynerida TO'G'RIDAN-TO'G'RI joylashadi — alohida
    // fon/blok YO'Q, sheet'ning to'liq kengligidan foydalanadi.
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Center(
          child: Image.asset('assets/ondex.png', height: 26),
        ),
        const SizedBox(height: 16),
        Builder(
          builder: (context) {
            // Restoran bosqichida (!pickedUp) kuryer ALLAQACHON yetib
            // borgan bo'lsa, "yuboruvchining oldiga boring" degan
            // countdown matni endi ma'nosiz — o'rniga holat xabari
            // ko'rsatiladi (foydalanuvchi so'rovi, 2026-07-30).
            final showCountdown = pickedUp || !arrivedAtRestaurant;
            final IconData icon = showCountdown
                ? Icons.timer_outlined
                : (ready ? Icons.check_circle_outline : Icons.storefront);
            final String primaryText = showCountdown
                ? (countdownSecondsLeft != null
                    ? _formatCountdown(countdownSecondsLeft!)
                    : '--:--')
                : 'Siz restorandasiz';
            final String secondaryText = showCountdown
                ? (pickedUp
                    ? 'Shu vaqt ichida qabul qiluvchining oldiga boring'
                    : 'Shu vaqt ichida yuboruvchining oldiga boring')
                : (ready
                    ? 'Taom tayyor — xodimdan buyurtmani so\'rang'
                    : 'Taom hali tayyorlanmoqda, biroz kuting...');
            return Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: const BoxDecoration(
                    color: kBrandColor,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(icon, color: Colors.white, size: 22),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        primaryText,
                        style: const TextStyle(
                          fontSize: 22,
                          fontWeight: FontWeight.bold,
                          letterSpacing: 0.5,
                        ),
                      ),
                      Text(
                        secondaryText,
                        style: TextStyle(
                            color: Colors.grey.shade400, fontSize: 12),
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
        const SizedBox(height: 16),
        Divider(color: Colors.grey.withValues(alpha: 0.2)),
        const SizedBox(height: 14),
        Center(
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _StepIcon(
                active: !pickedUp,
                child: _RestaurantLogoAvatar(
                  logoUrl: restaurant?['logo_url'] as String?,
                ),
              ),
              Icon(Icons.chevron_right, color: Colors.grey.shade600),
              _StepIcon(
                active: pickedUp,
                child: const Icon(Icons.person, color: Colors.white, size: 20),
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),
        Divider(color: Colors.grey.withValues(alpha: 0.2)),
        const SizedBox(height: 14),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    destinationLabel,
                    style: TextStyle(fontSize: 11, color: Colors.grey.shade500),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    destinationName,
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 16,
                    ),
                  ),
                ],
              ),
            ),
            if (callPhone != null && callPhone.isNotEmpty)
              _CircleIconButton(
                icon: Icons.call,
                onTap: () => _callPhone(callPhone),
              ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Manzil',
                    style:
                        TextStyle(fontSize: 11, color: Colors.grey.shade500),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    destinationAddress,
                    style: const TextStyle(fontWeight: FontWeight.w600),
                  ),
                  // Mijoz kiritgan tafsilotlar (podyezd/qavat/kvartira/
                  // domofon) — FAQAT mijozga borish bosqichida. Avval bu
                  // ma'lumot buyurtmaga umuman qo'shilmasdi va kuryer
                  // ko'p qavatli uyda qaysi kvartiraga borishni bilmasdi.
                  if (pickedUp && _addressDetails(order).isNotEmpty) ...[
                    const SizedBox(height: 3),
                    Text(
                      _addressDetails(order),
                      style: TextStyle(
                          fontSize: 13, color: Colors.grey.shade400),
                    ),
                  ],
                  if (pickedUp && _addressComment(order).isNotEmpty) ...[
                    const SizedBox(height: 3),
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(Icons.chat_bubble_outline,
                            size: 14, color: Colors.grey.shade400),
                        const SizedBox(width: 5),
                        Expanded(
                          child: Text(
                            _addressComment(order),
                            style: TextStyle(
                                fontSize: 13, color: Colors.grey.shade400),
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
            // Zaxira: agar ilova ichidagi xarita ishlamasa yoki kuryer
            // o'zi o'rgangan xarita ilovasini afzal ko'rsa — qurilmaning
            // o'z xarita ilovasiga (Google Maps va h.k.) yo'naltiradi,
            // manzil o'sha yerda chiziladi.
            if (hasMapFallback)
              _CircleIconButton(
                icon: Icons.directions,
                onTap: () => _openInDeviceMaps(destLat, destLng),
              ),
          ],
        ),
        const SizedBox(height: 16),
        Divider(color: Colors.grey.withValues(alpha: 0.2)),
        const SizedBox(height: 14),
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  orderNumber,
                  style: const TextStyle(
                    fontWeight: FontWeight.bold,
                    fontSize: 16,
                  ),
                ),
                if (!pickedUp)
                  Text(
                    'Xodimga oxirgi 4 xonani ayting: '
                    '${_lastFourDigits(orderNumber)}',
                    style:
                        TextStyle(fontSize: 12, color: Colors.grey.shade500),
                  )
                else
                  Text(
                    '${items.length} ta mahsulot',
                    style:
                        TextStyle(fontSize: 12, color: Colors.grey.shade500),
                  ),
              ],
            ),
            Text(
              formatSum(total),
              style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
            ),
          ],
        ),
        if (pickedUp) ...[
          const SizedBox(height: 10),
          for (final raw in items)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Row(
                children: [
                  Text(
                    '${(raw as Map)['qty']}x ',
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
                  Expanded(child: Text(raw['name'] as String? ?? '')),
                ],
              ),
            ),
        ],
        const SizedBox(height: 16),
        Divider(color: Colors.grey.withValues(alpha: 0.2)),
        _SupportRow(
          title: 'Qo\'llab-quvvatlash',
          subtitle: 'Agar buyurtma bilan muammo bo\'lsa',
          onTap: () => _showHelpDialog(
            context,
            'Qo\'llab-quvvatlash',
            'Agar buyurtma bilan bog\'liq muammo yuzaga kelsa, '
                '$destinationLabel bilan to\'g\'ridan-to\'g\'ri bog\'laning.',
          ),
        ),
        Divider(color: Colors.grey.withValues(alpha: 0.1), height: 1),
        _SupportRow(
          title: 'Nima qilay?',
          subtitle: null,
          onTap: () => _showHelpDialog(
            context,
            'Nima qilay?',
            pickedUp
                ? 'Mijoz manziliga yetib borib, buyurtmani topshiring, '
                    'so\'ng "Mijozga yetkazdim" tugmasini bosing.'
                : (arrivedAtRestaurant
                    ? 'Taom tayyor bo\'lishini kuting, so\'ng xodimga '
                        'buyurtma raqamining oxirgi 4 xonasini ayting va '
                        '"Buyurtma olindi" tugmasini bosing.'
                    : 'Restoranga yetib borgach "Yetib keldim" tugmasini '
                        'bosing, so\'ng taom tayyor bo\'lishini kuting.'),
          ),
        ),
      ],
    );
  }
}

/// Bosqich ko'rsatkichidagi bitta dumaloq belgi (restoran/mijoz) — joriy
/// bosqich brend rangida yoritilgan, kelgusi bosqich xira (grey).
class _StepIcon extends StatelessWidget {
  final Widget child;
  final bool active;
  const _StepIcon({required this.child, required this.active});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 40,
      height: 40,
      margin: const EdgeInsets.symmetric(horizontal: 4),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: active ? kBrandColor : Colors.grey.withValues(alpha: 0.25),
      ),
      child: ClipOval(child: child),
    );
  }
}

/// Dumaloq ikon-tugma — masalan restoranga qo'ng'iroq qilish.
class _CircleIconButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;
  const _CircleIconButton({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: kBrandColor.withValues(alpha: 0.15),
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(10),
          child: Icon(icon, color: kBrandColor, size: 20),
        ),
      ),
    );
  }
}

/// "Qo'llab-quvvatlash" / "Nima qilay?" qatori — bosilganda oddiy
/// ma'lumot oynasi ochiladi (haqiqiy qo'llab-quvvatlash tizimi hali
/// yo'q, shuning uchun soxta telefon raqami/chat ko'rsatilmaydi).
class _SupportRow extends StatelessWidget {
  final String title;
  final String? subtitle;
  final VoidCallback onTap;
  const _SupportRow({
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: const TextStyle(fontWeight: FontWeight.w600),
                  ),
                  if (subtitle != null) ...[
                    const SizedBox(height: 2),
                    Text(
                      subtitle!,
                      style: TextStyle(
                          fontSize: 12, color: Colors.grey.shade500),
                    ),
                  ],
                ],
              ),
            ),
            Icon(Icons.help_outline, color: Colors.grey.shade500, size: 22),
          ],
        ),
      ),
    );
  }
}

/// "Chapdan o'ngga sudrab" tasdiqlash — Yandex/Uber uslubidagi tasodifiy
/// bosishning oldini oluvchi harakat (foydalanuvchi so'rovi bo'yicha
/// oddiy tugma o'rniga). Tugma bosilishi shart emas — dumaloq tutqichni
/// yo'lakning oxirigacha (`_threshold`) surish kerak, aks holda tutqich
/// avtomatik boshiga qaytadi.
class _SlideToConfirm extends StatefulWidget {
  final String label;
  final bool busy;
  final Future<bool> Function() onConfirm;

  const _SlideToConfirm({
    required this.label,
    required this.busy,
    required this.onConfirm,
  });

  @override
  State<_SlideToConfirm> createState() => _SlideToConfirmState();
}

class _SlideToConfirmState extends State<_SlideToConfirm> {
  static const _thumbSize = 48.0;
  static const _threshold = 0.85;

  double _fraction = 0; // 0..1 — tutqichning yo'lak bo'ylab joriy holati
  bool _dragging = false;
  bool _submitting = false;

  double _maxDrag(double trackWidth) => trackWidth - _thumbSize - 8;

  Future<void> _finishDrag(double trackWidth) async {
    setState(() => _dragging = false);
    if (_fraction < _threshold) {
      setState(() => _fraction = 0);
      return;
    }
    setState(() => _submitting = true);
    final ok = await widget.onConfirm();
    if (!mounted) return;
    // `ok == false` — server rad etdi (masalan tarmoq xatosi), tutqich
    // boshiga qaytadi va foydalanuvchi qayta urinishi mumkin. `ok == true`
    // bo'lsa ota-widget `_activeOrder`ni almashtiradi va bu widget umuman
    // qayta chizilmaydi (picked_up ekraniga o'tiladi) — holatni tozalash
    // shart emas.
    if (!ok) {
      setState(() {
        _submitting = false;
        _fraction = 0;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final busy = widget.busy || _submitting;
    final color = Theme.of(context).colorScheme.primary;
    return LayoutBuilder(
      builder: (context, constraints) {
        final maxDrag = _maxDrag(constraints.maxWidth);
        return Container(
          height: 56,
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(28),
          ),
          child: Stack(
            alignment: Alignment.centerLeft,
            children: [
              Center(
                child: Opacity(
                  opacity: (1 - _fraction * 1.6).clamp(0.0, 1.0),
                  child: Text(
                    '${widget.label}   »',
                    style: TextStyle(
                      color: color,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ),
              AnimatedPositioned(
                duration: _dragging
                    ? Duration.zero
                    : const Duration(milliseconds: 220),
                curve: Curves.easeOut,
                left: 4 + _fraction * maxDrag,
                top: 4,
                child: GestureDetector(
                  onHorizontalDragStart: busy
                      ? null
                      : (_) => setState(() => _dragging = true),
                  onHorizontalDragUpdate: busy
                      ? null
                      : (details) {
                          setState(() {
                            _fraction = (_fraction + details.delta.dx / maxDrag)
                                .clamp(0.0, 1.0);
                          });
                        },
                  onHorizontalDragEnd: busy
                      ? null
                      : (_) => _finishDrag(constraints.maxWidth),
                  child: Container(
                    width: _thumbSize,
                    height: _thumbSize,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: color,
                    ),
                    child: Center(
                      child: busy
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: Colors.white,
                              ),
                            )
                          : const Icon(
                              Icons.arrow_forward,
                              color: Colors.white,
                            ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// Restoran logotipi — HAQIQIY `logo_url` (tarmoqdan) yoki, bo'lmasa/
/// yuklanmasa, `assets/box.png`. "Joriy buyurtma" kartochkasidagi "Olib
/// ketish" qatorida umumiy `Icons.storefront` o'rniga ishlatiladi (xaritadagi
/// restoran belgisi bilan bir xil g'oya — [_buildRestaurantIcon]).
class _RestaurantLogoAvatar extends StatelessWidget {
  final String? logoUrl;
  const _RestaurantLogoAvatar({required this.logoUrl});

  @override
  Widget build(BuildContext context) {
    final url = logoUrl;
    return ClipRRect(
      borderRadius: BorderRadius.circular(6),
      child: SizedBox(
        width: 28,
        height: 28,
        child: (url != null && url.isNotEmpty)
            ? Image.network(
                fullImageUrl(url),
                fit: BoxFit.cover,
                errorBuilder: (_, _, _) =>
                    Image.asset('assets/box.png', fit: BoxFit.cover),
              )
            : Image.asset('assets/box.png', fit: BoxFit.cover),
      ),
    );
  }
}
