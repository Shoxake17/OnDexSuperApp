import 'dart:async';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show MethodChannel, rootBundle;
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:pointer_interceptor/pointer_interceptor.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import '../location_reporter.dart';
import '../offer_alert.dart';
import '../offer_recovery.dart';
import '../push.dart';
import '../route_throttle.dart';
import '../session.dart';
import '../theme.dart';
import '../voice_navigation.dart';
import '../widgets/courier_panel.dart';
import '../widgets/order_cards.dart';
import '../widgets/slide_to_confirm.dart';
import 'login_screen.dart';

/// Kuryerning asosiy ekrani (image/Kuryer.png uslubi, 2026-09-15): TO'LIQ
/// XARITA + pastda panel (power tugmasi va "Liniyada" holati, "Restoran"
/// va "Naqd" kartalari, taklif yoki joriy buyurtma, amal tugmasi) +
/// oflaynda pastki menyu (Xarita/Profil). Real-time — WebSocket orqali
/// ("offer", "offer_cancelled", "order_status").
///
/// Joriy buyurtma ilova qayta ochilganda va oldinga chiqqanda serverdan
/// TIKLANADI (`GET /couriers/{id}/active-order`).
///
/// Kuryer — restoranning O'Z yetkazib beruvchisi: takliflar faqat shu
/// restorandan keladi, kirishni restoran "Xodimlar" bo'limida boshqaradi.
class CourierShell extends StatefulWidget {
  final String courierId;
  const CourierShell({super.key, required this.courierId});

  @override
  State<CourierShell> createState() => _CourierShellState();
}

class _CourierShellState extends State<CourierShell> with WidgetsBindingObserver {
  static const _chustCenter = LatLng(41.0030, 71.2360);

  bool _loading = true;
  bool? _approved;
  bool _online = false;
  // Power tugmasi so'rovi ketayotganda ikkinchi bosish e'tiborsiz.
  bool _togglingOnline = false;
  String _name = '';
  // Kuryer ishlaydigan restoran nomi ("Restoran" kartasi, profil).
  String? _homeRestaurantName;
  String? _homeRestaurantLogo;
  int _tabIndex = 0;

  // ---- Panel ----
  bool _panelExpanded = true;
  // Panelning HAQIQIY balandligi (`SizeReporter`): xarita tugmalari va
  // xaritaning pastki chegarasi shunga bog'lanadi.
  double _panelHeight = 0;

  // ---- Xarita ----
  GoogleMapController? _mapController;
  bool _mapsReady = false;
  bool _mapsFailed = false;
  LatLng? _myPosition;
  MapType _mapType = MapType.normal;
  LatLng _mapCenter = _chustCenter;
  double _currentZoom = 15;
  bool _locatingMe = false;
  // +/- tugmalari har doim BIZ kuzatib turgan `_mapCenter`/`_currentZoom`
  // asosida aniq target beradi; dastur o'zi animateCamera chaqirganda
  // `onCameraMove` ning oraliq qiymatlari markazni buzmasin.
  int _programmaticMoves = 0;

  // Jonli hodisalar — `ondex_core.WsClient` (kuryer ilovasi auditi, 7-band).
  WsClient? _ws;
  // Onlayn paytda joylashuv oqimi (foreground service, fonda ham ishlaydi).
  LocationReporter? _reporter;
  // Kuryer holatini yuklab bo'lmadi (tarmoq/server).
  String? _loadError;
  Timer? _offerCountdown;

  Map<String, dynamic>? _offer; // {order_id, seconds_left, total_seconds, restaurant_name, restaurant_address, eta_minutes}
  bool _respondingOffer = false;
  // Ekrandagi taklif qachon ko'rsatilgan va tiklash so'rovi ketyaptimi
  // (`_recoverPendingOffer`, `offer_recovery.dart`).
  DateTime? _offerShownAt;
  bool _recoveringOffer = false;
  // Ilova ekranda (resumed) — taklif ovozi ilova ichida chalinadi; fonda
  // bo'lsa takrorlanuvchi bildirishnoma (`offer_alert.dart`).
  bool _inForeground = true;

  // "Orqaga" — ilovani yopmasdan fonga o'tkazish (`MainActivity.kt`).
  static const _appChannel = MethodChannel('com.ondex.courier/app');

  Map<String, dynamic>? _activeOrder;
  Map<String, dynamic>? _activeRestaurant;
  String? _deliveryAddress;
  // `_deliveryAddress` qaysi koordinataga tegishli — qayta geokodlamaslik uchun.
  String? _deliveryAddressKey;
  bool _loadingActiveOrder = false;
  bool _transitioning = false;
  // "Yetib keldim" — MAHALLIY holat, backend'ga so'rov yubormaydi.
  bool _arrivedAtRestaurant = false;
  // Joriy maqsadgacha orqa sanoq (soniya): `_updateRoute()` haqiqiy yo'l
  // vaqti + bufer bilan sinxronlaydi, `_countdownTicker` har soniyada kamaytiradi.
  //
  // `ValueNotifier` (optimizatsiya, 2026-09-15): avval har soniyada
  // `setState` butun ekranni — xarita vidjeti, belgilar, panel — qayta
  // qurardi. Endi faqat vaqt yozuvi (`ActiveOrderCard`) yangilanadi.
  final _countdown = ValueNotifier<int?>(null);
  Timer? _countdownTicker;

  // ---- Marshrut chizig'i ----
  String _vehicleType = 'moped';
  Set<Polyline> _polylines = {};
  List<LatLng> _routePoints = const [];
  int _routeRequestSeq = 0; // eskirgan so'rov natijasini rad etish uchun
  // Google Directions qachon qayta so'raladi (`route_throttle.dart`).
  final _routeThrottle = RouteThrottle();

  // ---- Ovozli yo'l ko'rsatish (Kore ovozi, `voice_navigation.dart`) ----
  final _voiceNav = VoiceNavigator();
  final _voicePlayer = AudioPlayer();
  // Har yangi ibora oldingisini to'xtatadi (bo'laklar orasida tekshiriladi).
  int _cueSeq = 0;
  bool _voiceEnabled = true;
  static const _voicePrefKey = 'courier_nav_voice_enabled';
  // Restoran nomi bilan "yetib keldingiz" (serverda yasalgan WAV) va u
  // qaysi buyurtma uchun olingani.
  Uint8List? _arrivalClip;
  String? _arrivalClipOrderId;

  // ---- Yo'nalish o'qi (kuryer belgisi) ----
  BitmapDescriptor? _navIcon;
  double _navBearing = 0;

  // ---- Restoran belgisi: logotip + aniq nuqta ----
  static const _restaurantLogoSize = 36.0;
  static const _restaurantLogoCorner = 10.0;
  static const _restaurantDotDiameter = 14.0;
  static const _restaurantIconGap = 4.0;
  static const _restaurantIconWidth = _restaurantLogoSize;
  static const _restaurantIconHeight = _restaurantLogoSize + _restaurantIconGap + _restaurantDotDiameter;
  // Marker anchor'i — pastdagi nuqtaning markazi (logotip emas).
  static const _restaurantIconAnchorY =
      (_restaurantLogoSize + _restaurantIconGap + _restaurantDotDiameter / 2) / _restaurantIconHeight;

  BitmapDescriptor? _restaurantIcon;
  final Map<String, BitmapDescriptor> _restaurantIconCache = {};

  // Yangi taklif ovozi — qabul/rad etilguncha (yoki muddati tugaguncha).
  final _offerPlayer = AudioPlayer();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _startCountdownTicker();
    // WebSocket BIR MARTA ulanadi (`_init` qayta chaqirilganda soket ochmaydi).
    _connectWs();
    _init();
    _loadMaps();
    _refreshMyLocation();
    unawaited(_loadVoicePreference());
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _buildNavArrowIcon().then((icon) {
        if (mounted) setState(() => _navIcon = icon);
      }).catchError((Object e, StackTrace st) {
        debugPrint('[navi] icon yuklashda XATO: $e\n$st');
      });
    });
  }

  Future<BitmapDescriptor> _buildNavArrowIcon() async {
    return BitmapDescriptor.asset(
      createLocalImageConfiguration(context),
      'assets/navi.png',
      width: 56,
      height: 56,
    );
  }

  /// Restoran xarita belgisi: logotip (yoki `assets/box.png`) + nuqta,
  /// `restaurantId` bo'yicha keshlanadi.
  Future<BitmapDescriptor> _buildRestaurantIcon(String? restaurantId, String? logoUrl) async {
    final cacheKey = restaurantId ?? logoUrl ?? 'box';
    final cached = _restaurantIconCache[cacheKey];
    if (cached != null) return cached;

    // Muddat, hajm va o'lcham chegarasi (kuryer ilovasi auditi, 11-band).
    const maxLogoBytes = 2 * 1024 * 1024;
    const decodeWidth = 108;
    Uint8List? bytes;
    if (logoUrl != null && logoUrl.isNotEmpty) {
      try {
        final resp = await http.get(Uri.parse(fullImageUrl(logoUrl))).timeout(const Duration(seconds: 10));
        if (resp.statusCode == 200 && resp.bodyBytes.length <= maxLogoBytes) {
          bytes = resp.bodyBytes;
        }
      } catch (_) {
        // Tarmoq xatosi yoki muddat — assets/box.png ga tushamiz.
      }
    }
    final fallback = (await rootBundle.load('assets/box.png')).buffer.asUint8List();
    ui.Codec codec;
    try {
      codec = await ui.instantiateImageCodec(bytes ?? fallback, targetWidth: decodeWidth);
    } catch (_) {
      codec = await ui.instantiateImageCodec(fallback, targetWidth: decodeWidth);
    }
    final frame = await codec.getNextFrame();
    final image = frame.image;
    final side = math.min(image.width, image.height).toDouble();
    final srcRect = Rect.fromLTWH((image.width - side) / 2, (image.height - side) / 2, side, side);
    const logoRect = Rect.fromLTWH(0, 0, _restaurantLogoSize, _restaurantLogoSize);

    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    canvas.save();
    canvas.clipRRect(RRect.fromRectAndRadius(logoRect, const Radius.circular(_restaurantLogoCorner)));
    canvas.drawImageRect(image, srcRect, logoRect, Paint());
    canvas.restore();

    const dotCenter = Offset(
      _restaurantIconWidth / 2,
      _restaurantLogoSize + _restaurantIconGap + _restaurantDotDiameter / 2,
    );
    canvas.drawCircle(dotCenter, _restaurantDotDiameter / 2, Paint()..color = Colors.white);
    canvas.drawCircle(
      dotCenter,
      _restaurantDotDiameter / 2,
      Paint()
        ..color = kBrandColor
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2,
    );

    final picture = recorder.endRecording();
    final rendered = await picture.toImage(_restaurantIconWidth.toInt(), _restaurantIconHeight.toInt());
    final byteData = await rendered.toByteData(format: ui.ImageByteFormat.png);
    final icon = BitmapDescriptor.bytes(byteData!.buffer.asUint8List());
    _restaurantIconCache[cacheKey] = icon;
    return icon;
  }

  /// Ikki koordinata orasidagi kompas yo'nalishi (gradus).
  double _bearingBetween(LatLng from, LatLng to) {
    final lat1 = from.latitude * math.pi / 180;
    final lat2 = to.latitude * math.pi / 180;
    final dLng = (to.longitude - from.longitude) * math.pi / 180;
    final y = math.sin(dLng) * math.cos(lat2);
    final x = math.cos(lat1) * math.sin(lat2) - math.sin(lat1) * math.cos(lat2) * math.cos(dLng);
    final bearing = math.atan2(y, x) * 180 / math.pi;
    return (bearing + 360) % 360;
  }

  /// Kuryer holatini serverdan yuklaydi va joriy buyurtmani tiklaydi.
  /// WebSocket bu yerda OCHILMAYDI (u `initState` da bir marta ulanadi).
  Future<void> _init() async {
    // Push — ENG AVVAL va server javobiga bog'lanmasdan: ilova tarmoqsiz
    // ochilsa ham tinglovchilar ulanadi, token esa oldinga chiqqanda qayta
    // yoziladi (`didChangeAppLifecycleState`).
    unawaited(registerCourierPush(onOfferHint: () => unawaited(_recoverPendingOffer())));
    try {
      final c = await api.getCourier(widget.courierId);
      if (!mounted) return;
      setState(() {
        _loadError = null;
        _approved = c['approved'] == true;
        _online = c['available'] == true;
        _name = c['name'] as String? ?? '';
        _vehicleType = c['vehicle_type'] as String? ?? 'moped';
      });
      _applyOnlineSideEffects(_online);
      final restaurantId = (c['restaurant_id'] as String? ?? '').trim();
      if (restaurantId.isNotEmpty) unawaited(_loadHomeRestaurant(restaurantId));
    } on ApiException catch (e) {
      // 401 bo'lsa `api.onUnauthorized` o'zi login ekraniga olib o'tadi.
      if (mounted && !e.isUnauthorized) setState(() => _loadError = e.message);
    } catch (_) {
      if (mounted) setState(() => _loadError = 'Serverga ulanib bo\'lmadi — internetni tekshiring');
    }
    if (mounted) setState(() => _loading = false);
    if (_loadError == null) {
      await _restoreActiveOrder();
      // Ilova yopiq paytda kelgan (hali ochiq) taklif.
      unawaited(_recoverPendingOffer());
    }
  }

  /// "Restoran" kartasi uchun nom (ochiq `GET /restaurants/{id}`). Xato jim:
  /// karta "—" ko'rsatadi, ish to'xtamaydi.
  Future<void> _loadHomeRestaurant(String restaurantId) async {
    try {
      final r = await api.restaurant(restaurantId);
      final name = (r['name'] as String? ?? '').trim();
      final logo = (r['logo_url'] as String? ?? '').trim();
      if (!mounted) return;
      setState(() {
        if (name.isNotEmpty) _homeRestaurantName = name;
        _homeRestaurantLogo = logo.isEmpty ? null : logo;
      });
    } catch (_) {}
  }

  /// Serverdagi yakunlanmagan buyurtmani tiklaydi.
  Future<void> _restoreActiveOrder() async {
    if (_activeOrder != null || _loadingActiveOrder) return;
    try {
      final id = await api.activeOrderId(widget.courierId);
      if (id != null && mounted && _activeOrder == null && !_loadingActiveOrder) {
        _arrivedAtRestaurant = false;
        await _loadActiveOrder(id);
      }
    } catch (_) {}
  }

  /// Server holatini qayta o'qiydi (oldinga chiqqanda va kutilmagan taklifda).
  Future<void> _syncCourierState() async {
    try {
      final c = await api.getCourier(widget.courierId);
      if (!mounted) return;
      final online = c['available'] == true;
      final changed = online != _online;
      setState(() {
        _approved = c['approved'] == true;
        _online = online;
      });
      if (changed) _applyOnlineSideEffects(online);
    } catch (_) {}
  }

  /// Onlayn holatga mos yon ta'sir: joylashuv oqimi (fonda ham).
  void _applyOnlineSideEffects(bool online) {
    if (online) {
      _startLocationReporting();
    } else {
      _stopLocationReporting();
    }
  }

  Future<void> _loadMaps() async {
    try {
      await ensureGoogleMapsLoaded(api.mapsApiKey);
      if (mounted) setState(() => _mapsReady = true);
    } catch (_) {
      if (mounted) setState(() => _mapsFailed = true);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _offerCountdown?.cancel();
    _countdownTicker?.cancel();
    final ws = _ws;
    if (ws != null) unawaited(ws.dispose());
    final reporter = _reporter;
    if (reporter != null) unawaited(reporter.stop());
    _offerPlayer.dispose();
    _voicePlayer.dispose();
    _countdown.dispose();
    super.dispose();
  }

  /// Oldinga chiqqanda joriy buyurtma va holat serverdan QAYTA so'raladi:
  /// fonda WebSocket uzilib, `order_status` hodisasi yo'qolgan bo'lishi mumkin.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.paused || state == AppLifecycleState.hidden) {
      if (!_inForeground) return;
      _inForeground = false;
      // Taklif ochiq turganda ilovadan chiqildi: ilova ichidagi ovoz
      // o'rniga takrorlanuvchi bildirishnoma (qolgan vaqtgacha).
      _switchOfferAlert();
      return;
    }
    if (state != AppLifecycleState.resumed) return;
    if (!_inForeground) {
      _inForeground = true;
      _switchOfferAlert();
    }
    final id = _activeOrder?['id'] as String?;
    if (id != null) {
      unawaited(_loadActiveOrder(id));
    } else {
      unawaited(_restoreActiveOrder());
    }
    unawaited(_syncCourierState());
    // Token saqlanmay qolgan bo'lsa (tarmoq yo'q edi) — qayta yoziladi.
    unawaited(registerCourierPush(onOfferHint: () => unawaited(_recoverPendingOffer())));
    // Fonda turgan paytda ko'rsatilgan taklifning qolgan vaqti (yoki uni
    // boshqa kuryer olib ulgurgani) serverdan.
    unawaited(_recoverPendingOffer());
  }

  /// Panelning qotirilgan amal tugmasi (taklif / joriy buyurtma bosqichi).
  Widget? _buildFooterAction() {
    final offer = _offer;
    if (offer != null) {
      return AcceptCountdownButton(
        secondsLeft: (offer['seconds_left'] as num? ?? 0).toInt(),
        totalSeconds: (offer['total_seconds'] as num? ?? 1).toInt(),
        busy: _respondingOffer,
        onAccept: _acceptOffer,
      );
    }
    final order = _activeOrder;
    if (_loadingActiveOrder || order == null) return null;
    final status = order['status'] as String? ?? 'ready';
    final pickedUp = status == 'picked_up';
    final ready = status == 'ready';
    // Har bosqich tugmasi O'Z kaliti bilan: busiz Flutter oldingi bosqich
    // tugmasining holatini qayta ishlatib, keyingisini abadiy "aylanayotgan"
    // holda qoldirardi (2026-09-15 lokal sinov).
    final orderId = order['id'] as String? ?? '';

    if (pickedUp) {
      return SlideToConfirm(
        key: ValueKey('slide-delivered-$orderId'),
        label: 'Mijozga yetkazdim',
        busy: _transitioning,
        onConfirm: _markDelivered,
      );
    }
    //  1) Hali yetib bormagan — "Yetib keldim" (mahalliy, taom holatidan qat'i nazar).
    //  2) Yetib bordi, taom tayyor emas — passiv "tayyorlanmoqda".
    //  3) Yetib bordi va taom tayyor — "Buyurtma olindi" (backend picked_up).
    if (!_arrivedAtRestaurant) {
      return SlideToConfirm(
        key: ValueKey('slide-arrived-$orderId'),
        label: 'Yetib keldim',
        busy: false,
        onConfirm: _markArrivedAtRestaurant,
      );
    }
    if (!ready) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(vertical: 16),
        decoration: BoxDecoration(color: kCardBg, borderRadius: BorderRadius.circular(28)),
        child: const Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2, color: kInkMuted)),
            SizedBox(width: 10),
            Text('Restoran taomni tayyorlamoqda...', style: TextStyle(color: kInkMuted)),
          ],
        ),
      );
    }
    return SlideToConfirm(
      key: ValueKey('slide-picked-$orderId'),
      label: 'Buyurtma olindi',
      busy: _transitioning,
      onConfirm: _confirmPickedUp,
    );
  }

  // ---------- WebSocket ----------

  /// Bilet 401 bilan rad etilsa `api.onUnauthorized` login ekraniga olib
  /// o'tadi, `dispose` klientni to'xtatadi (kuryer ilovasi auditi, 5-band).
  void _connectWs() {
    final old = _ws;
    if (old != null) unawaited(old.dispose());
    final ws = WsClient(
      ticketProvider: api.wsTicket,
      urlBuilder: wsUrl,
      onEvent: _onWsEvent,
      // Kanal uzilgan paytda yuborilgan taklif qayta ulanganda tiklanadi.
      onStateChange: (connected) {
        if (connected) unawaited(_recoverPendingOffer());
      },
    );
    _ws = ws;
    unawaited(ws.connect());
  }

  void _onWsEvent(Map<String, dynamic> e) {
    if (!mounted) return;
    switch (e['type']) {
      // Akkaunt o'chirildi — server tokenni allaqachon bekor qilgan.
      case 'account_deleted':
        _forceLogout();
      case 'offer':
        _handleOfferReceived(e);
      case 'offer_cancelled':
        if (_offer != null && _offer!['order_id'] == e['order_id']) _dismissOffer();
      case 'order_status':
        _handleOrderStatus(e);
      case 'auto_offline':
        _handleAutoOffline(e);
    }
  }

  /// Server kuryerni ketma-ket javobsiz takliflardan keyin liniyadan chiqardi
  /// (`internal/couriers/missed.go`).
  void _handleAutoOffline(Map<String, dynamic> e) {
    if (!_online) return;
    setState(() => _online = false);
    _applyOnlineSideEffects(false);
    final missed = (e['missed'] as num?)?.toInt() ?? 3;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      duration: const Duration(seconds: 8),
      content: Text('Ketma-ket $missed ta taklifga javob berilmadi — liniyadan chiqarildingiz. '
          'Ishlashga tayyor bo\'lsangiz, qayta liniyaga chiqing.'),
    ));
  }

  void _handleOrderStatus(Map<String, dynamic> e) {
    final active = _activeOrder;
    if (active == null || active['id'] != e['order_id']) return;
    final status = e['status'] as String?;
    if (status == null) return;
    if (status == 'delivered' || status == 'cancelled' || status == 'rejected') {
      setState(() {
        _activeOrder = null;
        _arrivedAtRestaurant = false;
        _restaurantIcon = null;
        _resetRoute();
      });
      // Bekor qilish JIM bo'lmasligi kerak (kuryer ilovasi auditi, 9-band).
      if (status != 'delivered') {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          backgroundColor: kDanger,
          duration: Duration(seconds: 10),
          content: Text('Buyurtma bekor qilindi. Taom qo\'lingizda bo\'lsa, '
              'restoranga qaytaring yoki restoran bilan bog\'laning.'),
        ));
      }
      return;
    }
    setState(() => _activeOrder = {...active, 'status': status});
    // `force` YO'Q: "tayyorlanmoqda"/"tayyor" da manzil o'zgarmaydi — yo'l
    // qayta so'ralmaydi; o'zgarsa `RouteThrottle` o'zi aniqlaydi.
    unawaited(_updateRoute());
  }

  void _handleOfferReceived(Map<String, dynamic> e) {
    if (!mounted) return;
    final orderId = e['order_id'];
    if (orderId is! String || orderId.isEmpty) return;
    // Faol yetkazma bor — yangi taklif uni ekrandan siqib chiqarmasin.
    if (_activeOrder != null || _loadingActiveOrder) return;
    // Shu taklif allaqachon ekranda (WebSocket va tiklash bir vaqtda
    // keldi) — qayta ko'rsatilmaydi, ovoz boshidan chalinmaydi.
    if (_offer?['order_id'] == orderId) return;
    // Ilova "liniyada emas" deb turibdi, server esa taklif yubordi — holat
    // qayta o'qiladi, taklif faqat haqiqatan liniyada bo'lsa ko'rsatiladi
    // (kuryer ilovasi auditi, 3-band).
    if (!_online) {
      unawaited(_syncCourierState().then((_) {
        if (mounted && _online) _handleOfferReceived(e);
      }));
      return;
    }
    final seconds = (e['expires_in_sec'] as num? ?? 20).toInt();
    if (seconds <= 0) return;
    // Tiklangan taklifda qolgan vaqt to'liq muddatdan kam — tugmadagi
    // "eriyotgan" chiziq to'g'ri joydan boshlansin.
    final total = (e['total_sec'] as num?)?.toInt() ?? seconds;
    _offerShownAt = DateTime.now();
    setState(() {
      _offer = {
        'order_id': orderId,
        'seconds_left': seconds,
        'total_seconds': total < seconds ? seconds : total,
        'restaurant_name': e['restaurant_name'] ?? '',
        'restaurant_address': e['restaurant_address'] ?? '',
        'restaurant_logo_url': e['restaurant_logo_url'] ?? '',
        'eta_minutes': null,
      };
      // Taklif yig'ilgan panelda qolib ketmasin.
      _panelExpanded = true;
    });
    _startOfferAlert();
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

  /// Taklif kelgan zahoti restorangacha haqiqiy yo'l vaqti.
  Future<void> _loadOfferEta(String orderId, Map<String, dynamic> e) async {
    final lat = (e['restaurant_lat'] as num?)?.toDouble();
    final lng = (e['restaurant_lng'] as num?)?.toDouble();
    final me = _myPosition;
    if (me == null || lat == null || lng == null) return;
    final result = await api.route(me.latitude, me.longitude, lat, lng, mode: _routeMode());
    if (!mounted || _offer == null || _offer!['order_id'] != orderId) return;
    final duration = result.durationSeconds;
    if (duration == null) return;
    setState(() => _offer = {..._offer!, 'eta_minutes': (duration / 60).round()});
  }

  String _routeMode() => switch (_vehicleType) {
        'foot' => 'walking',
        'bike' => 'bicycling',
        _ => 'driving',
      };

  /// Serverdagi OCHIQ taklifni tiklaydi: ilova ochilganda, oldinga
  /// chiqqanda, jonli kanal qayta ulanganda va push kelganda/bosilganda.
  ///
  /// ┌─ NEGA (2026-09-15) ────────────────────────────────────────────────┐
  /// Taklif avval FAQAT WebSocket xabari sifatida xotirada turardi. Kuryer
  /// taklifni ko'rib ilovadan chiqsa va 20 soniya ichida qaytsa ham taklif
  /// yo'qolardi, server esa uni ochiq deb kutardi. Endi haqiqat manbai —
  /// server (`GET /couriers/{id}/offer`), qolgan soniyalar ham serverdan.
  /// └────────────────────────────────────────────────────────────────────┘
  Future<void> _recoverPendingOffer() async {
    if (_recoveringOffer || !_online || _activeOrder != null || _loadingActiveOrder) return;
    _recoveringOffer = true;
    final requestedAt = DateTime.now();
    try {
      final server = await api.pendingOffer(widget.courierId);
      if (!mounted) return;
      switch (offerRecoveryAction(
        shown: _offer,
        shownAt: _offerShownAt,
        server: server,
        requestedAt: requestedAt,
      )) {
        case OfferRecoveryAction.none:
          break;
        case OfferRecoveryAction.dismiss:
          _dismissOffer();
        case OfferRecoveryAction.updateSeconds:
          final left = (server?['expires_in_sec'] as num?)?.toInt();
          final current = _offer;
          if (left == null || current == null) break;
          if (left <= 0) {
            _dismissOffer();
          } else {
            setState(() => _offer = {...current, 'seconds_left': left});
          }
        case OfferRecoveryAction.show:
          if (server != null) _handleOfferReceived(server);
      }
    } catch (_) {
      // Tarmoq xatosi — jonli kanal va keyingi urinish (oldinga chiqish,
      // qayta ulanish) ishlaydi.
    } finally {
      _recoveringOffer = false;
    }
  }

  /// "Orqaga": Profil tabida — Xaritaga; asosiy ekranda — ilova YOPILMAYDI,
  /// fonga o'tadi (WebSocket, joylashuv va takliflar ishlashda davom etadi).
  Future<void> _handleBack() async {
    if (_tabIndex != 0) {
      setState(() => _tabIndex = 0);
      return;
    }
    try {
      await _appChannel.invokeMethod<bool>('moveToBack');
    } catch (_) {
      // Android bo'lmagan platforma — hech narsa qilinmaydi.
    }
  }

  void _dismissOffer() {
    final orderId = _offer?['order_id'];
    _offerCountdown?.cancel();
    if (mounted) setState(() => _offer = null);
    unawaited(_offerPlayer.stop());
    if (orderId is String) unawaited(cancelOfferAlert(orderId));
  }

  /// Taklif signalini boshlaydi: ilova ekranda — ilova ichidagi mp3
  /// (taklif tugaguncha takrorlanadi); fonda — takrorlanuvchi bildirishnoma.
  void _startOfferAlert() {
    final offer = _offer;
    if (offer == null) return;
    if (_inForeground) {
      unawaited(_playOfferSound());
      return;
    }
    final name = (offer['restaurant_name'] as String?)?.trim() ?? '';
    unawaited(showOfferAlert(
      orderId: offer['order_id'] as String,
      title: 'Yangi buyurtma',
      body: name.isEmpty ? 'Qabul qilish uchun ilovani oching' : '$name — qabul qilish uchun ilovani oching',
      timeout: Duration(seconds: (offer['seconds_left'] as num? ?? 0).toInt()),
    ));
  }

  /// Ilova oldinga/fonga o'tganda signal turini almashtiradi (ikkalasi bir
  /// vaqtda jiringlamasin).
  void _switchOfferAlert() {
    final orderId = _offer?['order_id'];
    if (orderId is! String) return;
    if (_inForeground) {
      unawaited(cancelOfferAlert(orderId));
    } else {
      unawaited(_offerPlayer.stop());
    }
    _startOfferAlert();
  }

  Future<void> _playOfferSound() async {
    try {
      await _offerPlayer.stop();
      await _offerPlayer.setReleaseMode(ReleaseMode.loop);
      await _offerPlayer.play(AssetSource('sound/kuryersound.mp3'));
    } catch (_) {
      // Ovoz ijro etilmasa ham taklif ishlashda davom etadi.
    }
  }

  // ---------- Liniya (onlayn/oflayn) ----------

  /// Power tugmasi.
  ///
  /// Holat server TASDIQLAGACH o'zgaradi (avval optimistik edi va xatoda
  /// orqaga "sakrardi"). So'rov davomida tugma aylanadi va ikkinchi bosish
  /// e'tiborsiz — ketma-ket bosishlar serverga qarama-qarshi so'rovlar
  /// yubormaydi.
  Future<void> _toggleOnline(bool value) async {
    if (_togglingOnline) return;
    // Yetkazma o'rtasida liniyadan chiqish — TAQIQ (server ham 409 qaytaradi).
    if (!value && (_activeOrder != null || _loadingActiveOrder)) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content: Text('Yakunlanmagan buyurtmangiz bor — avval uni yetkazing, keyin liniyadan chiqing'),
      ));
      return;
    }
    setState(() => _togglingOnline = true);
    try {
      // Liniyaga chiqishdan OLDIN GPS va ruxsat (kuryer ilovasi auditi, 10-band).
      if (value) {
        final problem = await LocationReporter.readinessProblem();
        if (!mounted) return;
        if (problem != null) {
          _locationSnack(problem.message, actionLabel: problem.actionLabel, onAction: problem.action);
          return;
        }
      }
      await api.setAvailable(widget.courierId, value);
      if (!mounted) return;
      setState(() => _online = value);
      _applyOnlineSideEffects(value);
    } on ApiException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Serverga ulanib bo\'lmadi — internetni tekshiring')),
        );
      }
    } finally {
      if (mounted) setState(() => _togglingOnline = false);
    }
  }

  /// Onlayn paytda joylashuv oqimi — FONDA ham ishlaydi (kuryer auditi, 4-band).
  void _startLocationReporting() {
    _reporter ??= LocationReporter(
      onPosition: (lat, lng) {
        if (!mounted) return;
        setState(() {
          _myPosition = LatLng(lat, lng);
          _trimRoute();
        });
        unawaited(_updateRoute());
        _speakNavigation(LatLng(lat, lng));
      },
      send: (lat, lng) => api.updateLocation(widget.courierId, lat, lng),
    );
    // Faol buyurtma bo'lsa GPS yo'l ko'rsatish tezligida (har 2 soniya).
    if (_activeOrder != null) unawaited(_reporter!.setNavigationMode(true));
    unawaited(_reporter!.start());
  }

  void _stopLocationReporting() {
    final r = _reporter;
    if (r != null) unawaited(r.stop());
  }

  /// Joriy koordinatani BIR MARTA oladi (ilova ochilganda), muddat bilan.
  Future<void> _refreshMyLocation() async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) perm = await Geolocator.requestPermission();
      if (perm == LocationPermission.denied || perm == LocationPermission.deniedForever) return;
      final pos = await Geolocator.getCurrentPosition(locationSettings: LocationReporter.currentSettings);
      if (!mounted) return;
      setState(() => _myPosition = LatLng(pos.latitude, pos.longitude));
      unawaited(_updateRoute());
    } catch (_) {
      // Joylashuv aniqlanmadi — keyinroq oqim yoki tugma orqali yangilanadi.
    }
  }

  /// Hozirgi maqsad: taom olinguncha — restoran, keyin — mijoz.
  LatLng? _routeDestination(Map<String, dynamic> order) {
    final pickedUp = order['status'] == 'picked_up';
    final lat = pickedUp
        ? (order['delivery_lat'] as num?)?.toDouble()
        : (_activeRestaurant?['lat'] as num?)?.toDouble();
    final lng = pickedUp
        ? (order['delivery_lng'] as num?)?.toDouble()
        : (_activeRestaurant?['lng'] as num?)?.toDouble();
    if (lat == null || lng == null || (lat == 0 && lng == 0)) return null;
    return LatLng(lat, lng);
  }

  /// `setState` ichida chaqiriladi.
  void _setRoutePoints(List<LatLng> points) {
    _routePoints = points;
    _polylines = points.length < 2
        ? {}
        : {
            Polyline(
              polylineId: const PolylineId('route'),
              points: points,
              color: kBrandColor,
              width: 5,
            ),
          };
  }

  /// Chiziq va orqa sanoqni tozalaydi (`setState` ichida).
  void _clearRoute() {
    _setRoutePoints(const []);
    _countdown.value = null;
  }

  /// Buyurtma tugadi — keyingisi uchun marshrut hisobi ham noldan.
  void _resetRoute() {
    _clearRoute();
    _routeThrottle.reset();
    // Buyurtma tugadi — ovozli yo'l ko'rsatish va tezkor GPS o'chadi.
    _voiceNav.reset();
    _arrivalClip = null;
    _arrivalClipOrderId = null;
    unawaited(_voicePlayer.stop());
    final reporter = _reporter;
    if (reporter != null) unawaited(reporter.setNavigationMode(false));
  }

  /// Faol buyurtma boshlandi (yoki ilova qayta ochilib tiklandi): tezkor GPS
  /// va — taom hali olinmagan bo'lsa — restoran nomi iborasini oldindan
  /// yuklash (sintez ~10 s, kuryer yetib borguncha tayyor bo'ladi).
  void _startNavigation(Map<String, dynamic> order) {
    final reporter = _reporter;
    if (reporter != null) unawaited(reporter.setNavigationMode(true));
    final id = order['id'];
    if (id is! String || order['status'] == 'picked_up' || _arrivalClipOrderId == id) return;
    _arrivalClipOrderId = id;
    _arrivalClip = null;
    unawaited(_prefetchArrivalClip(id));
  }

  Future<void> _prefetchArrivalClip(String orderId) async {
    final clip = await api.arrivalVoice(widget.courierId, orderId);
    if (!mounted || _arrivalClipOrderId != orderId) return;
    _arrivalClip = clip;
  }

  /// Har GPS nuqtasida: aytiladigan ibora bo'lsa — aytadi.
  void _speakNavigation(LatLng position) {
    if (!_voiceEnabled || _activeOrder == null) return;
    final cue = _voiceNav.update(position, DateTime.now());
    if (cue != null) unawaited(_playCue(cue));
  }

  Future<void> _playCue(NavCue cue) async {
    final seq = ++_cueSeq;
    try {
      await _voicePlayer.stop();
      final clip = _arrivalClip;
      if (cue.arrivedAtRestaurant && clip != null) {
        await _voicePlayer.play(BytesSource(clip, mimeType: 'audio/wav'));
        return;
      }
      // Bo'laklar ketma-ket: "Ikki yuz metrdan keyin" + "O'ngga buriling."
      for (final key in cue.assetKeys) {
        if (seq != _cueSeq || !mounted) return;
        final done = _voicePlayer.onPlayerComplete.first;
        await _voicePlayer.play(AssetSource('voice/$key.wav'));
        await done.timeout(const Duration(seconds: 8), onTimeout: () {});
      }
    } catch (_) {
      // Bo'lak yo'q yoki ovoz chiqmadi — yo'l chizig'i va vaqt ishlashda
      // davom etadi.
    }
  }

  Future<void> _loadVoicePreference() async {
    try {
      final v = (await SharedPreferences.getInstance()).getBool(_voicePrefKey);
      if (v != null && mounted) setState(() => _voiceEnabled = v);
    } catch (_) {}
  }

  Future<void> _toggleVoice() async {
    final next = !_voiceEnabled;
    setState(() => _voiceEnabled = next);
    if (!next) unawaited(_voicePlayer.stop());
    try {
      await (await SharedPreferences.getInstance()).setBool(_voicePrefKey, next);
    } catch (_) {}
  }

  /// Kuryer yurgan sari chiziqning bosib o'tilgan boshini qirqadi —
  /// Google'ga so'rovsiz (`setState` ichida).
  void _trimRoute() {
    final me = _myPosition;
    if (me == null || _routePoints.length < 2) return;
    final trimmed = trimRouteToPosition(_routePoints, me);
    if (!identical(trimmed, _routePoints)) _setRoutePoints(trimmed);
  }

  /// Kuryerdan hozirgi maqsadgacha haqiqiy yo'l chizig'i va orqa sanoq.
  ///
  /// Har joylashuvda va holat o'zgarishida chaqiriladi, lekin Google
  /// Directions (PULLIK) faqat `RouteThrottle` ruxsat berganda so'raladi:
  /// maqsad o'zgarsa darhol, aks holda 300 m siljiganda yoki 90 s da bir
  /// (orasi kamida 30 s, bir maqsadga bir vaqtda bitta so'rov).
  /// `_routeRequestSeq` eskirgan javobni rad etadi.
  Future<void> _updateRoute() async {
    final me = _myPosition;
    final order = _activeOrder;
    final dest = order == null ? null : _routeDestination(order);
    if (me == null || dest == null) {
      ++_routeRequestSeq;
      _routeThrottle.abandon();
      if ((_routePoints.isNotEmpty || _countdown.value != null) && mounted) {
        setState(_clearRoute);
      }
      return;
    }
    final now = DateTime.now();
    if (!_routeThrottle.shouldFetch(me, dest, now)) return;
    final destinationChanged = _routeThrottle.destinationChanged(dest);
    final mySeq = ++_routeRequestSeq;
    _routeThrottle.begin(dest, now);
    RouteResult result;
    try {
      result = await api.route(me.latitude, me.longitude, dest.latitude, dest.longitude, mode: _routeMode());
    } catch (_) {
      // Tarmoq xatosi ham "natija yo'q" — busiz `begin` yopilmay, shu
      // maqsadga boshqa so'rov yuborilmay qolardi.
      result = const RouteResult(points: [], durationSeconds: null);
    }
    // Eskirgan javob: yangi so'rov yoki `abandon` holatni o'zi yakunlaydi.
    if (mySeq != _routeRequestSeq) return;
    if (result.points.isEmpty) {
      _routeThrottle.fail();
      // Maqsad o'zgargan bo'lsa eski chiziq noto'g'ri joyga olib boradi —
      // olib tashlanadi; aks holda oldingisi qoladi.
      if (destinationChanged && mounted) setState(_clearRoute);
      return;
    }
    _routeThrottle.succeed(me, dest, DateTime.now());
    _voiceNav.setRoute(
      steps: result.steps,
      destination: dest,
      target: _activeOrder?['status'] == 'picked_up' ? NavTarget.customer : NavTarget.restaurant,
    );
    if (!mounted) return;
    setState(() => _setRoutePoints([for (final p in result.points) LatLng(p.lat, p.lng)]));
    // Bufer: har km uchun +2 daqiqa (foydalanuvchi so'rovi).
    final duration = result.durationSeconds;
    _countdown.value = duration == null
        ? null
        : duration + (((result.distanceMeters ?? 0) / 1000.0) * 2 * 60).round();
  }

  void _startCountdownTicker() {
    _countdownTicker?.cancel();
    _countdownTicker = Timer.periodic(const Duration(seconds: 1), (_) {
      final left = _countdown.value;
      if (left == null || left <= 0) return;
      _countdown.value = left - 1;
    });
  }

  void _focusCamera(LatLng target) => unawaited(_animateTo(target, 16));

  Future<void> _animateTo(LatLng target, double zoom) async {
    _programmaticMoves++;
    try {
      await _mapController?.animateCamera(
        CameraUpdate.newCameraPosition(CameraPosition(target: target, zoom: zoom)),
      );
    } finally {
      _programmaticMoves--;
      _mapCenter = target;
      _currentZoom = zoom;
    }
  }

  void _zoom(double delta) {
    final newZoom = (_currentZoom + delta).clamp(2.0, 20.0);
    unawaited(_animateTo(_mapCenter, newZoom));
  }

  /// "Joriy joylashuvim" — har bir muvaffaqiyatsiz holat uchun aniq xabar.
  Future<void> _goToMyLocation() async {
    setState(() => _locatingMe = true);
    try {
      if (!await Geolocator.isLocationServiceEnabled()) {
        _locationSnack('Joylashuv xizmati (GPS) o\'chirilgan',
            actionLabel: 'Yoqish', onAction: Geolocator.openLocationSettings);
        return;
      }
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) perm = await Geolocator.requestPermission();
      if (perm == LocationPermission.deniedForever) {
        _locationSnack('Joylashuvga ruxsat berilmagan — sozlamalardan yoqing',
            actionLabel: 'Sozlamalar', onAction: Geolocator.openAppSettings);
        return;
      }
      if (perm == LocationPermission.denied) {
        _locationSnack('Joylashuvga ruxsat berilmadi');
        return;
      }
      final pos = await Geolocator.getCurrentPosition(locationSettings: LocationReporter.currentSettings);
      final latLng = LatLng(pos.latitude, pos.longitude);
      if (mounted) setState(() => _myPosition = latLng);
      await _animateTo(latLng, 16);
    } catch (_) {
      _locationSnack('Joylashuvni aniqlab bo\'lmadi');
    } finally {
      if (mounted) setState(() => _locatingMe = false);
    }
  }

  void _locationSnack(String message, {String? actionLabel, Future<bool> Function()? onAction}) {
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
    // Taklif shu lahzada yopilgan bo'lishi mumkin; ikki marta bosish e'tiborsiz.
    final orderId = _offer?['order_id'];
    if (orderId is! String || _respondingOffer) return;
    setState(() => _respondingOffer = true);
    try {
      await api.respond(widget.courierId, orderId, true);
      _dismissOffer();
      _arrivedAtRestaurant = false;
      await _loadActiveOrder(orderId);
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
      _dismissOffer();
    } finally {
      if (mounted) setState(() => _respondingOffer = false);
    }
  }

  /// Qabul qilingach buyurtma biriktirilishi bir oz vaqt oladi — qisqa
  /// oraliqda bir necha marta so'raladi.
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
        const SnackBar(content: Text('Buyurtma yuklanmadi — ilovani qayta oching')),
      );
      return;
    }
    // Shu buyurtmaning QAYTA yuklanishi (ilova oldinga chiqdi): restoran,
    // manzil va belgi saqlanadi. Avval har ilova almashtirishda ular
    // tozalanib, pullik geokodlash va yo'l so'rovi qayta ketardi (o'lchov,
    // 2026-09-15), panel esa har safar ochilib ketardi.
    final sameOrder = _activeOrder?['id'] == order['id'];
    setState(() {
      _activeOrder = order;
      if (!sameOrder) {
        _deliveryAddress = null;
        _deliveryAddressKey = null;
        _activeRestaurant = null;
        _restaurantIcon = null;
        _panelExpanded = true;
      }
      _loadingActiveOrder = false;
    });
    _startNavigation(order);
    unawaited(_enrichActiveOrder(order));
  }

  /// Restoran ma'lumoti, mijoz manzili matni va yo'l. Allaqachon
  /// yuklanganlari qayta so'ralmaydi (ilova har oldinga chiqqanda chaqiriladi).
  Future<void> _enrichActiveOrder(Map<String, dynamic> order) async {
    final restaurantId = order['restaurant_id'] as String?;
    final restaurantLoaded = restaurantId != null && _activeRestaurant?['id'] == restaurantId;
    if (restaurantId != null && restaurantId.isNotEmpty && !restaurantLoaded) {
      try {
        final rest = await api.restaurant(restaurantId);
        if (mounted) setState(() => _activeRestaurant = rest);
        final rLat = (rest['lat'] as num?)?.toDouble() ?? 0;
        final rLng = (rest['lng'] as num?)?.toDouble() ?? 0;
        if (rLat != 0 || rLng != 0) _focusCamera(LatLng(rLat, rLng));
        _buildRestaurantIcon(restaurantId, rest['logo_url'] as String?).then((icon) {
          if (mounted) setState(() => _restaurantIcon = icon);
        }).catchError((Object e, StackTrace st) {
          debugPrint('[restaurant-icon] yuklashda XATO: $e\n$st');
        });
      } catch (_) {}
    }
    final lat = (order['delivery_lat'] as num?)?.toDouble() ?? 0;
    final lng = (order['delivery_lng'] as num?)?.toDouble() ?? 0;
    final addressKey = '$lat,$lng';
    if ((lat != 0 || lng != 0) && _deliveryAddressKey != addressKey) {
      try {
        final addr = await api.reverseGeocode(lat, lng);
        if (mounted) {
          setState(() {
            _deliveryAddress = addr;
            _deliveryAddressKey = addressKey;
          });
        }
      } catch (_) {}
      _focusCamera(LatLng(lat, lng));
    }
    // Maqsad yangi bo'lsa `RouteThrottle` darhol so'raydi, o'sha bo'lsa — yo'q.
    unawaited(_updateRoute());
  }

  /// "Yetib keldim" — MAHALLIY holat (backend'da bu bosqich yo'q).
  Future<bool> _markArrivedAtRestaurant() async {
    setState(() => _arrivedAtRestaurant = true);
    return true;
  }

  /// "Buyurtma olindi" — server TO'LIQ buyurtmani (taomlar, mijoz manzili) qaytaradi.
  Future<bool> _confirmPickedUp() async {
    final orderId = _activeOrder?['id'];
    if (orderId is! String) return false;
    setState(() => _transitioning = true);
    try {
      final updated = await api.transition(orderId, 'picked_up');
      if (!mounted) return true;
      setState(() => _activeOrder = updated);
      unawaited(_enrichActiveOrder(updated));
      return true;
    } on ApiException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
      return false;
    } catch (_) {
      return false;
    } finally {
      if (mounted) setState(() => _transitioning = false);
    }
  }

  Future<bool> _markDelivered() async {
    final orderId = _activeOrder?['id'];
    if (orderId is! String) return false;
    setState(() => _transitioning = true);
    try {
      await api.transition(orderId, 'delivered');
      if (mounted) {
        setState(() {
          _activeOrder = null;
          _arrivedAtRestaurant = false;
          _restaurantIcon = null;
          _resetRoute();
        });
      }
      return true;
    } on ApiException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
      return false;
    } catch (_) {
      return false;
    } finally {
      if (mounted) setState(() => _transitioning = false);
    }
  }

  /// Akkaunt server tomonda o'chirilgan — `api.logout()` chaqirilmaydi
  /// (token allaqachon yaroqsiz).
  Future<void> _forceLogout() async {
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
      content: Text('Akkaunt o\'chirildi'),
      backgroundColor: kDanger,
    ));
    Navigator.of(context).pushReplacement(MaterialPageRoute(builder: (_) => const LoginScreen()));
  }

  Future<void> _logout() async {
    // Yakunlanmagan buyurtma bilan chiqish buyurtmani egasiz qoldirardi.
    if (_activeOrder != null || _loadingActiveOrder) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content: Text('Yakunlanmagan buyurtmangiz bor — avval uni yetkazing'),
      ));
      return;
    }
    // Push tokeni `logout` dan OLDIN o'chiriladi (so'rov hali amaldagi token
    // bilan ketadi).
    await unregisterCourierPush();
    await api.logout();
    await tokenStore.clear();
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushReplacement(MaterialPageRoute(builder: (_) => const LoginScreen()));
  }

  Set<Marker> _buildMarkers() {
    final markers = <Marker>{};
    final order = _activeOrder;

    LatLng? destination;
    if (order != null) {
      final pickedUp = order['status'] == 'picked_up';
      final lat = pickedUp
          ? (order['delivery_lat'] as num?)?.toDouble() ?? 0
          : (_activeRestaurant?['lat'] as num?)?.toDouble() ?? 0;
      final lng = pickedUp
          ? (order['delivery_lng'] as num?)?.toDouble() ?? 0
          : (_activeRestaurant?['lng'] as num?)?.toDouble() ?? 0;
      if (lat != 0 || lng != 0) destination = LatLng(lat, lng);
    }

    final me = _myPosition;
    if (me != null) {
      if (destination != null) _navBearing = _bearingBetween(me, destination);
      markers.add(Marker(
        markerId: const MarkerId('me'),
        position: me,
        icon: _navIcon ?? BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueAzure),
        anchor: const Offset(0.5, 0.5),
        rotation: _navBearing,
        flat: true,
        infoWindow: const InfoWindow(title: 'Siz'),
      ));
    }
    if (order != null) {
      final pickedUp = order['status'] == 'picked_up';
      if (!pickedUp) {
        final lat = (_activeRestaurant?['lat'] as num?)?.toDouble() ?? 0;
        final lng = (_activeRestaurant?['lng'] as num?)?.toDouble() ?? 0;
        if (lat != 0 || lng != 0) {
          markers.add(Marker(
            markerId: const MarkerId('restaurant'),
            position: LatLng(lat, lng),
            icon: _restaurantIcon ?? BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueOrange),
            anchor: _restaurantIcon != null ? const Offset(0.5, _restaurantIconAnchorY) : const Offset(0.5, 1.0),
            infoWindow: InfoWindow(title: _activeRestaurant?['name'] as String? ?? 'Restoran'),
          ));
        }
      } else {
        final lat = (order['delivery_lat'] as num?)?.toDouble() ?? 0;
        final lng = (order['delivery_lng'] as num?)?.toDouble() ?? 0;
        if (lat != 0 || lng != 0) {
          markers.add(Marker(
            markerId: const MarkerId('delivery'),
            position: LatLng(lat, lng),
            icon: BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueGreen),
            infoWindow: const InfoWindow(title: 'Mijoz manzili'),
          ));
        }
      }
    }
    return markers;
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(backgroundColor: kSurface, body: Center(child: CircularProgressIndicator()));
    }
    // Holat yuklanmadi va hali hech narsa ma'lum emas — sabab va "qayta
    // urinish" (kuryer ilovasi auditi, 8-band).
    final loadError = _loadError;
    if (loadError != null && _approved == null) {
      return Scaffold(
        backgroundColor: kSurface,
        body: SafeArea(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.wifi_off_rounded, size: 56, color: kInkFaint),
                  const SizedBox(height: 16),
                  Text(loadError, textAlign: TextAlign.center, style: const TextStyle(color: kInk)),
                  const SizedBox(height: 20),
                  FilledButton.icon(
                    onPressed: () {
                      setState(() => _loading = true);
                      unawaited(_init());
                    },
                    icon: const Icon(Icons.refresh),
                    label: const Text('Qayta urinish'),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) unawaited(_handleBack());
      },
      child: Scaffold(
      backgroundColor: kSurface,
      appBar: _tabIndex == 1 ? AppBar(title: Text(_name.isEmpty ? 'Kuryer' : _name)) : null,
      body: IndexedStack(
        index: _tabIndex,
        children: [
          _buildMapTab(),
          ProfileTab(
            name: _name,
            restaurantName: _homeRestaurantName,
            approved: _approved,
            onLogout: _logout,
          ),
        ],
      ),
      // Liniyada — pastki menyu YASHIRIN (ish vaqtida chalg'itmasin).
      bottomNavigationBar: _online
          ? null
          : NavigationBar(
              selectedIndex: _tabIndex,
              onDestinationSelected: (i) => setState(() => _tabIndex = i),
              destinations: const [
                NavigationDestination(icon: Icon(Icons.map_outlined), selectedIcon: Icon(Icons.map), label: 'Xarita'),
                NavigationDestination(
                    icon: Icon(Icons.person_outline), selectedIcon: Icon(Icons.person), label: 'Profil'),
              ],
            ),
      ),
    );
  }

  Widget _buildMapTab() {
    final topInset = MediaQuery.of(context).padding.top + 12;
    return LayoutBuilder(
      builder: (context, constraints) {
        final approved = _approved == true;
        // Xarita tugmalari panel ustida; juda baland panelda ekrandan chiqib
        // ketmasin.
        final controlsBottom = math.min(_panelHeight + 12, math.max(12.0, constraints.maxHeight - 190));
        return Stack(
          children: [
            // MUHIM: xarita alohida qatlam — panel o'lchami o'zgarganda
            // qayta o'lchanmaydi (faqat `padding` yangilanadi).
            Positioned.fill(child: RepaintBoundary(child: _buildMapLayer())),
            // `PointerInterceptor` — veb'da xarita HTML platform-view: busiz
            // tugma ustidagi bosish pastdagi xaritaga "sizib o'tardi".
            Positioned(
              right: 12,
              top: topInset,
              child: PointerInterceptor(
                child: MapRoundButton(
                  tooltip: _mapType == MapType.normal ? 'Sputnik (3D) ko\'rinish' : 'Oddiy xarita (2D)',
                  icon: _mapType == MapType.normal ? Icons.threed_rotation : Icons.map,
                  onTap: () => setState(() {
                    _mapType = _mapType == MapType.normal ? MapType.hybrid : MapType.normal;
                  }),
                ),
              ),
            ),
            // Ovozli yo'l ko'rsatishni o'chirish/yoqish (faqat faol buyurtmada).
            if (_activeOrder != null)
              Positioned(
                right: 12,
                top: topInset + 56,
                child: PointerInterceptor(
                  child: MapRoundButton(
                    tooltip: _voiceEnabled
                        ? 'Ovozli yo\'l ko\'rsatishni o\'chirish'
                        : 'Ovozli yo\'l ko\'rsatishni yoqish',
                    icon: _voiceEnabled ? Icons.volume_up_rounded : Icons.volume_off_rounded,
                    onTap: _toggleVoice,
                  ),
                ),
              ),
            Positioned(
              right: 12,
              bottom: controlsBottom,
              child: PointerInterceptor(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    MapRoundButton(
                      tooltip: 'Joriy joylashuvim',
                      icon: _locatingMe ? Icons.hourglass_top : Icons.my_location,
                      onTap: _locatingMe ? null : _goToMyLocation,
                    ),
                    const SizedBox(height: 12),
                    ZoomBlock(onZoomIn: () => _zoom(1), onZoomOut: () => _zoom(-1)),
                  ],
                ),
              ),
            ),
            Align(
              alignment: Alignment.bottomCenter,
              child: SizeReporter(
                onSize: (size) {
                  if (!mounted || (size.height - _panelHeight).abs() < 1) return;
                  setState(() => _panelHeight = size.height);
                },
                child: PointerInterceptor(
                  child: CourierBottomPanel(
                    maxHeight: constraints.maxHeight * 0.62,
                    expanded: _panelExpanded,
                    onExpandedChanged: (v) => setState(() => _panelExpanded = v),
                    header: CourierStatusHeader(
                      online: _online,
                      enabled: approved,
                      busy: _togglingOnline,
                      onPowerTap: () => _toggleOnline(!_online),
                    ),
                    cards: approved
                        ? CourierInfoCards(
                            restaurantName: _homeRestaurantName,
                            restaurantLogoUrl: _homeRestaurantLogo,
                            activeOrder: _activeOrder,
                          )
                        : null,
                    body: _buildPanelBody(approved),
                    footer: approved ? _buildFooterAction() : null,
                  ),
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _buildPanelBody(bool approved) {
    if (!approved) return AccessClosedCard(onRetry: _init);
    final offer = _offer;
    if (offer != null) return OfferCard(offer: offer);
    if (_loadingActiveOrder) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    final order = _activeOrder;
    if (order != null) {
      return ActiveOrderCard(
        order: order,
        restaurant: _activeRestaurant,
        deliveryAddress: _deliveryAddress,
        countdown: _countdown,
        arrivedAtRestaurant: _arrivedAtRestaurant,
      );
    }
    return _online ? const WaitingCard() : const OfflineHintCard();
  }

  Widget _buildMapLayer() {
    if (_mapsFailed) return const Center(child: Text('Xarita yuklanmadi', style: TextStyle(color: kInkMuted)));
    if (!_mapsReady) return const Center(child: CircularProgressIndicator());
    return GoogleMap(
      initialCameraPosition: CameraPosition(target: _myPosition ?? _chustCenter, zoom: _currentZoom),
      mapType: _mapType,
      onMapCreated: (c) => _mapController = c,
      onCameraMove: (pos) {
        if (_programmaticMoves > 0) return;
        _mapCenter = pos.target;
        _currentZoom = pos.zoom;
      },
      // Xaritaning ko'rinadigan qismi panel ustida: kamera markazi va
      // marshrut panel ostiga yashirinmasin.
      padding: EdgeInsets.only(bottom: _panelHeight),
      myLocationEnabled: false,
      myLocationButtonEnabled: false,
      zoomControlsEnabled: false,
      mapToolbarEnabled: false,
      compassEnabled: false,
      markers: _buildMarkers(),
      polylines: _polylines,
    );
  }
}
