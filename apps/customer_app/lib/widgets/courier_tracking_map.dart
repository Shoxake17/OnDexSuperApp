import 'dart:async';
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:http/http.dart' as http;

import '../api.dart';
import '../delivery_tracking.dart';
import '../screens/catalog_screen.dart' show kBrand;

const _ink = Color(0xFF111827);
const _muted = Color(0xFF757575);
const _plannedLine = Color(0xFFB0B7C3);

/// Kuryer mashinasi — yuqoridan ko'rinish, old tomoni TEPAGA qaragan
/// (harakat yo'nalishiga `rotation` bilan buriladi).
const _carAsset = 'assets/map/kuryer_car.png';

/// Restoran logosini yuklash chegarasi (kuryer ilovasidagi bilan bir xil).
const _maxLogoBytes = 2 * 1024 * 1024;

/// Buyurtma kuzatuvidagi xarita ("Yetkazib berish" bloki ostida).
///
///  * Kuryer taomni olguncha — kuryer mashinasi va manzil belgilari.
///  * Kuryer yo'lda — restoran (logosi) → mijoz (uy belgisi) yo'li, kuryerdan
///    manzilgacha qolgan yo'l brend rangida, jonli mashina va qolgan vaqt.
///  * Yetkazilgach — xarita YO'QOLMAYDI: buyurtmaga biriktirilgan yo'l,
///    yetkazish vaqti va masofasi bilan.
///
/// Jonli ma'lumot [courier] va [tracking] TINGLANADI: ular o'zgarganda faqat
/// shu kartochka qayta chiziladi, butun sahifa emas.
///
/// Mijoz xaritani o'zi sursa kamera uni "tortib qaytarmaydi" — joylashuv
/// tugmasi bilan kuzatuv qayta yoqiladi.
class CourierTrackingMap extends StatefulWidget {
  const CourierTrackingMap({
    super.key,
    required this.orderStatus,
    required this.courier,
    required this.destination,
    required this.tracking,
    required this.courierName,
  });

  final String orderStatus;

  /// Jonli joylashuv (WebSocket hodisasi). Bo'lmasa kuzatuvdagisi olinadi.
  final ValueListenable<LatLng?> courier;
  final LatLng? destination;
  final ValueListenable<DeliveryTracking?> tracking;
  final String courierName;

  @override
  State<CourierTrackingMap> createState() => _CourierTrackingMapState();
}

enum _Stage { waiting, live, delivered }

class _CourierTrackingMapState extends State<CourierTrackingMap> {
  static const _chustCenter = LatLng(41.0030, 71.2360);

  /// Mashina shundan kam siljisa burilmaydi (GPS "titrashi" uni aylantirmasin).
  static const _minMoveForBearing = 8.0;

  GoogleMapController? _ctrl;
  bool _mapsReady = false;
  bool _mapsFailed = false;
  bool _follow = true;
  // Dastur o'zi kamerani surayotganda `onCameraMoveStarted` kuzatuvni
  // o'chirib qo'ymasin.
  int _programmaticMoves = 0;

  /// Qolgan vaqt matnini daqiqalar o'tishi bilan yangilaydi (faqat yo'lda).
  Timer? _etaTicker;

  BitmapDescriptor? _iconRestaurant;
  // Qaysi logo uchun belgi so'ralgan ('' — logosiz zaxira belgi).
  String? _iconRestaurantFor;
  BitmapDescriptor? _iconHome;
  BitmapDescriptor? _iconCar;
  bool _iconsRequested = false;
  double _dpr = 2;

  LatLng? _lastCourier;
  double _bearing = 0;

  _Stage get _stage => switch (widget.orderStatus) {
        'delivered' => _Stage.delivered,
        'picked_up' => _Stage.live,
        _ => _Stage.waiting,
      };

  DeliveryTracking? get _t => widget.tracking.value;

  LatLng? get _courier =>
      _stage == _Stage.delivered ? null : (widget.courier.value ?? _t?.courier);

  LatLng? get _destination => _t?.destination ?? widget.destination;

  LatLng? get _origin => _stage == _Stage.waiting ? null : _t?.origin;

  String get _restaurantName => _t?.restaurantName ?? 'Restoran';

  @override
  void initState() {
    super.initState();
    _listen(widget);
    _updateBearing();
    _loadMaps();
  }

  void _listen(CourierTrackingMap w) {
    w.courier.addListener(_onLiveChanged);
    w.tracking.addListener(_onLiveChanged);
    _syncTicker();
  }

  void _unlisten(CourierTrackingMap w) {
    w.courier.removeListener(_onLiveChanged);
    w.tracking.removeListener(_onLiveChanged);
  }

  void _onLiveChanged() {
    if (!mounted) return;
    _updateBearing();
    _ensureRestaurantIcon();
    setState(() {});
    _syncTicker();
    if (_follow) _fit();
  }

  /// Mashina yo'nalishi: oldingi va hozirgi joylashuvdan; birinchi marta —
  /// qolgan yo'lning boshidan (yoki manzil tomonga).
  void _updateBearing() {
    final c = _courier;
    if (c == null) return;
    final prev = _lastCourier;
    if (prev == null) {
      final road = _t?.remainingRoute?.points;
      final dest = _destination;
      if (road != null && road.length >= 2) {
        _bearing = bearingDegrees(road[0], road[1]);
      } else if (dest != null) {
        _bearing = bearingDegrees(c, dest);
      }
      _lastCourier = c;
    } else if (metersBetween(prev, c) >= _minMoveForBearing) {
      _bearing = bearingDegrees(prev, c);
      _lastCourier = c;
    }
  }

  void _syncTicker() {
    final live = _stage == _Stage.live && _t?.etaSeconds != null;
    if (live) {
      _etaTicker ??= Timer.periodic(const Duration(seconds: 15), (_) {
        if (mounted) setState(() {});
      });
    } else {
      _etaTicker?.cancel();
      _etaTicker = null;
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_iconsRequested) return;
    _iconsRequested = true;
    _dpr = MediaQuery.devicePixelRatioOf(context);
    final config = createLocalImageConfiguration(context);
    Future.wait([
      _circleIcon(_dpr, color: kBrand, icon: Icons.home_rounded, size: 38),
      BitmapDescriptor.asset(config, _carAsset, width: 22, height: 48),
    ]).then((icons) {
      if (!mounted) return;
      setState(() {
        _iconHome = icons[0];
        _iconCar = icons[1];
      });
    }).catchError((Object e) {
      // Belgilar chizilmasa standart belgilar qoladi.
      debugPrint('[kuzatuv] belgi chizilmadi: $e');
    });
    _ensureRestaurantIcon();
  }

  /// Restoran belgisi: logosi (bir marta yuklanadi), bo'lmasa "A".
  void _ensureRestaurantIcon() {
    if (!_iconsRequested) return;
    final key = _t?.restaurantLogoUrl ?? '';
    if (_iconRestaurantFor == key) return;
    _iconRestaurantFor = key;
    final future = key.isEmpty
        ? _circleIcon(_dpr, color: _ink, label: 'A')
        : _logoIcon(key, _dpr);
    future.then((icon) {
      if (mounted && _iconRestaurantFor == key) setState(() => _iconRestaurant = icon);
    }).catchError((Object e) {
      debugPrint('[kuzatuv] restoran belgisi chizilmadi: $e');
    });
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
  void didUpdateWidget(CourierTrackingMap old) {
    super.didUpdateWidget(old);
    if (old.courier != widget.courier || old.tracking != widget.tracking) {
      _unlisten(old);
      _listen(widget);
    }
    final stageChanged = old.orderStatus != widget.orderStatus;
    if (stageChanged) {
      _follow = true;
      _updateBearing();
      _syncTicker();
    }
    if (_follow && (stageChanged || old.destination != widget.destination)) {
      _fit();
    }
  }

  @override
  void dispose() {
    _unlisten(widget);
    _etaTicker?.cancel();
    super.dispose();
  }

  /// Kamerada ko'rinishi kerak bo'lgan nuqtalar.
  List<LatLng> _focusPoints() {
    final t = _t;
    final origin = _origin;
    final destination = _destination;
    final courier = _courier;
    switch (_stage) {
      case _Stage.delivered:
        return [
          ...?t?.plannedRoute?.points,
          if (origin != null) origin,
          if (destination != null) destination,
        ];
      case _Stage.live:
        return [
          if (courier != null) courier,
          if (destination != null) destination,
          ...?t?.remainingRoute?.points,
        ];
      case _Stage.waiting:
        return [
          if (courier != null) courier,
          if (destination != null) destination,
        ];
    }
  }

  Future<void> _fit() async {
    final ctrl = _ctrl;
    if (ctrl == null) return;
    final points = _focusPoints();
    if (points.isEmpty) return;
    var minLat = points.first.latitude, maxLat = minLat;
    var minLng = points.first.longitude, maxLng = minLng;
    for (final p in points) {
      if (p.latitude < minLat) minLat = p.latitude;
      if (p.latitude > maxLat) maxLat = p.latitude;
      if (p.longitude < minLng) minLng = p.longitude;
      if (p.longitude > maxLng) maxLng = p.longitude;
    }
    final CameraUpdate update;
    if ((maxLat - minLat).abs() < 1e-4 && (maxLng - minLng).abs() < 1e-4) {
      update = CameraUpdate.newLatLngZoom(points.first, 16);
    } else {
      update = CameraUpdate.newLatLngBounds(
        LatLngBounds(
          southwest: LatLng(minLat, minLng),
          northeast: LatLng(maxLat, maxLng),
        ),
        48,
      );
    }
    _programmaticMoves++;
    try {
      await ctrl.animateCamera(update);
    } catch (_) {
      // Xarita hali o'lchanmagan bo'lsa `newLatLngBounds` xato beradi —
      // keyingi yangilanishda qayta urinadi.
    } finally {
      _programmaticMoves--;
    }
  }

  Set<Marker> _markers() {
    final name = widget.courierName.trim();
    final origin = _origin;
    final destination = _destination;
    final courier = _courier;
    const center = Offset(0.5, 0.5);
    const bottom = Offset(0.5, 1);
    return {
      if (origin != null)
        Marker(
          markerId: const MarkerId('origin'),
          position: origin,
          icon: _iconRestaurant ??
              BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueViolet),
          anchor: _iconRestaurant != null ? center : bottom,
          infoWindow: InfoWindow(title: _restaurantName),
          zIndexInt: 1,
        ),
      if (destination != null)
        Marker(
          markerId: const MarkerId('destination'),
          position: destination,
          icon: _iconHome ??
              BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueGreen),
          anchor: _iconHome != null ? center : bottom,
          infoWindow: const InfoWindow(title: 'Sizning manzilingiz'),
          zIndexInt: 2,
        ),
      if (courier != null)
        Marker(
          markerId: const MarkerId('courier'),
          position: courier,
          icon: _iconCar ??
              BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueOrange),
          anchor: _iconCar != null ? center : bottom,
          // Mashina xarita bilan birga "yotadi" va yo'nalishga buriladi.
          flat: _iconCar != null,
          rotation: _iconCar != null ? _bearing : 0,
          infoWindow: InfoWindow(title: name.isEmpty ? 'Kuryer' : name),
          zIndexInt: 3,
        ),
    };
  }

  Set<Polyline> _polylines() {
    final t = _t;
    if (t == null || _stage == _Stage.waiting) return const {};
    final planned = t.plannedRoute;
    final remaining = t.remainingRoute;
    return {
      if (planned != null)
        Polyline(
          polylineId: const PolylineId('planned'),
          points: planned.points,
          // Yetkazilgan buyurtmada yo'l bosib o'tilgan — brend rangida;
          // yo'lda — kulrang "butun yo'l", ustida qolgan qism.
          color: _stage == _Stage.delivered ? kBrand : _plannedLine,
          width: _stage == _Stage.delivered ? 6 : 5,
          jointType: JointType.round,
          startCap: Cap.roundCap,
          endCap: Cap.roundCap,
          zIndex: 1,
        ),
      if (_stage == _Stage.live && remaining != null)
        Polyline(
          polylineId: const PolylineId('remaining'),
          points: remaining.points,
          color: kBrand,
          width: 6,
          jointType: JointType.round,
          startCap: Cap.roundCap,
          endCap: Cap.roundCap,
          zIndex: 2,
        ),
    };
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFE5E5E5)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _Header(
            stage: _stage,
            tracking: _t,
            courierName: widget.courierName,
            hasCourier: _courier != null,
          ),
          const SizedBox(height: 10),
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: SizedBox(height: 260, child: _buildMap()),
          ),
          if (_stage != _Stage.waiting) ...[
            const SizedBox(height: 10),
            Wrap(
              spacing: 16,
              runSpacing: 6,
              children: [
                _Legend(
                  leading: _LogoDot(url: _t?.restaurantLogoUrl),
                  label: _restaurantName,
                ),
                const _Legend(
                  leading: _IconDot(icon: Icons.home_rounded, color: kBrand),
                  label: 'Sizning manzilingiz',
                ),
                if (_stage == _Stage.live)
                  _Legend(
                    leading: Image.asset(_carAsset, height: 20),
                    label: 'Kuryer',
                  ),
              ],
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildMap() {
    if (_mapsFailed) {
      return const ColoredBox(
        color: Color(0xFFF5F5F5),
        child: Center(
          child: Text('Xarita yuklanmadi', style: TextStyle(color: Color(0xFF9E9E9E))),
        ),
      );
    }
    if (!_mapsReady) {
      return const ColoredBox(
        color: Color(0xFFF5F5F5),
        child: Center(child: CircularProgressIndicator(color: kBrand)),
      );
    }
    final focus = _focusPoints();
    return Stack(
      children: [
        GoogleMap(
          initialCameraPosition: CameraPosition(
            target: focus.isNotEmpty ? focus.first : _chustCenter,
            zoom: 14,
          ),
          markers: _markers(),
          polylines: _polylines(),
          onMapCreated: (c) {
            _ctrl = c;
            _fit();
          },
          onCameraMoveStarted: () {
            if (_programmaticMoves == 0 && _follow) {
              setState(() => _follow = false);
            }
          },
          // Xarita sahifa ichida (ListView): busiz barmoq xaritani emas,
          // sahifani surardi.
          gestureRecognizers: <Factory<OneSequenceGestureRecognizer>>{
            Factory<OneSequenceGestureRecognizer>(() => EagerGestureRecognizer()),
          },
          myLocationEnabled: false,
          myLocationButtonEnabled: false,
          zoomControlsEnabled: false,
          mapToolbarEnabled: false,
          compassEnabled: false,
        ),
        // Kuryer ilovasidagi "joriy joylashuv" tugmasi uslubida.
        if (!_follow)
          Positioned(
            right: 10,
            bottom: 10,
            child: Tooltip(
              message: 'Kuryerni kuzatish',
              child: Material(
                color: Colors.white,
                shape: const CircleBorder(),
                elevation: 3,
                child: InkWell(
                  key: const ValueKey('courier-map-follow'),
                  customBorder: const CircleBorder(),
                  onTap: () {
                    setState(() => _follow = true);
                    _fit();
                  },
                  child: const Padding(
                    padding: EdgeInsets.all(11),
                    child: Icon(Icons.my_location, size: 22, color: _ink),
                  ),
                ),
              ),
            ),
          ),
      ],
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({
    required this.stage,
    required this.tracking,
    required this.courierName,
    required this.hasCourier,
  });

  final _Stage stage;
  final DeliveryTracking? tracking;
  final String courierName;
  final bool hasCourier;

  @override
  Widget build(BuildContext context) {
    final name = courierName.trim();
    final who = name.isEmpty ? 'Kuryer' : name;
    final t = tracking;

    final String caption;
    final String headline;
    String? detail;
    switch (stage) {
      case _Stage.waiting:
        caption = 'Kuryer qayerda';
        headline = hasCourier
            ? '$who restoranda buyurtmani oladi'
            : 'Kuryer joylashuvi kutilmoqda…';
      case _Stage.live:
        caption = '$who yo\'lda';
        final left = t?.remainingSeconds(DateTime.now());
        if (left == null) {
          headline = 'Yetib kelish vaqti hisoblanmoqda…';
        } else if (left == 0) {
          headline = 'Kuryer yetib keldi';
        } else {
          headline = '${etaText(left)}da yetib keladi';
          detail = 'Taxminan ${clockText(DateTime.now().add(Duration(seconds: left)))} gacha';
        }
        final remaining = t?.remainingRoute;
        if (remaining != null && left != 0) {
          final km = '${distanceText(remaining.distanceMeters)} qoldi';
          detail = detail == null ? km : '$detail · $km';
        }
      case _Stage.delivered:
        caption = 'Yetkazish yo\'li';
        final at = t?.deliveredAt;
        headline = at == null ? 'Yetkazildi' : 'Yetkazildi · ${clockText(at)}';
        final took = deliveryDurationText(t?.pickedUpAt, t?.deliveredAt);
        final planned = t?.plannedRoute;
        detail = [
          if (took != null) took,
          if (planned != null) distanceText(planned.distanceMeters),
        ].join(' · ');
        if (detail.isEmpty) detail = null;
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(caption,
            style: const TextStyle(
                fontSize: 13, fontWeight: FontWeight.w600, color: _muted)),
        const SizedBox(height: 4),
        Text(headline,
            key: const ValueKey('courier-map-headline'),
            style: TextStyle(
              fontSize: stage == _Stage.live ? 20 : 15,
              fontWeight: FontWeight.w800,
              color: stage == _Stage.live ? kBrand : _ink,
            )),
        if (detail != null) ...[
          const SizedBox(height: 2),
          Text(detail, style: const TextStyle(fontSize: 13, color: _muted)),
        ],
      ],
    );
  }
}

class _Legend extends StatelessWidget {
  const _Legend({required this.leading, required this.label});

  final Widget leading;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(width: 20, height: 20, child: Center(child: leading)),
        const SizedBox(width: 6),
        Text(label, style: const TextStyle(fontSize: 12.5, color: _muted)),
      ],
    );
  }
}

class _IconDot extends StatelessWidget {
  const _IconDot({required this.icon, required this.color});

  final IconData icon;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 20,
      height: 20,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
      child: Icon(icon, size: 13, color: Colors.white),
    );
  }
}

/// Restoran logosi (kichik); bo'lmasa yoki yuklanmasa — do'kon belgisi.
class _LogoDot extends StatelessWidget {
  const _LogoDot({required this.url});

  final String? url;

  @override
  Widget build(BuildContext context) {
    const fallback = _IconDot(icon: Icons.storefront_rounded, color: _ink);
    final u = url ?? '';
    if (u.isEmpty) return fallback;
    return ClipOval(
      child: Image.network(
        fullImageUrl(u),
        width: 20,
        height: 20,
        fit: BoxFit.cover,
        cacheWidth: 60,
        errorBuilder: (_, __, ___) => fallback,
      ),
    );
  }
}

/// Oq hoshiyali dumaloq belgi: harf yoki ikonka.
Future<BitmapDescriptor> _circleIcon(
  double dpr, {
  required Color color,
  String? label,
  IconData? icon,
  double size = 34,
}) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder)..scale(dpr);
  final center = Offset(size / 2, size / 2);
  canvas.drawCircle(center, size / 2, Paint()..color = Colors.white);
  canvas.drawCircle(center, size / 2 - 3, Paint()..color = color);
  final span = icon != null
      ? TextSpan(
          text: String.fromCharCode(icon.codePoint),
          style: TextStyle(
            fontFamily: icon.fontFamily,
            package: icon.fontPackage,
            fontSize: size * 0.58,
            color: Colors.white,
          ),
        )
      : TextSpan(
          text: label ?? '',
          style: TextStyle(
            fontSize: size * 0.47,
            fontWeight: FontWeight.w800,
            color: Colors.white,
          ),
        );
  final tp = TextPainter(text: span, textDirection: TextDirection.ltr)..layout();
  tp.paint(canvas, center - Offset(tp.width / 2, tp.height / 2));
  final px = (size * dpr).ceil();
  final image = await recorder.endRecording().toImage(px, px);
  final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
  return BitmapDescriptor.bytes(bytes!.buffer.asUint8List(), width: size, height: size);
}

/// Restoran logosidan dumaloq belgi. Yuklanmasa (tarmoq, hajm, format) —
/// "A" harfli zaxira belgi; xarita hech qachon belgisiz qolmaydi.
Future<BitmapDescriptor> _logoIcon(String url, double dpr, {double size = 40}) async {
  try {
    final resp = await http
        .get(Uri.parse(fullImageUrl(url)))
        .timeout(const Duration(seconds: 10));
    if (resp.statusCode != 200 || resp.bodyBytes.length > _maxLogoBytes) {
      throw StateError('logo yuklanmadi: ${resp.statusCode}');
    }
    final codec = await ui.instantiateImageCodec(resp.bodyBytes,
        targetWidth: (size * dpr).ceil());
    final image = (await codec.getNextFrame()).image;

    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder)..scale(dpr);
    final center = Offset(size / 2, size / 2);
    canvas.drawCircle(center, size / 2, Paint()..color = Colors.white);
    final inner = Rect.fromCircle(center: center, radius: size / 2 - 3);
    canvas.save();
    canvas.clipPath(Path()..addOval(inner));
    final side = (image.width < image.height ? image.width : image.height).toDouble();
    final src = Rect.fromLTWH(
        (image.width - side) / 2, (image.height - side) / 2, side, side);
    canvas.drawImageRect(image, src, inner, Paint()..filterQuality = FilterQuality.medium);
    canvas.restore();

    final px = (size * dpr).ceil();
    final out = await recorder.endRecording().toImage(px, px);
    final bytes = await out.toByteData(format: ui.ImageByteFormat.png);
    return BitmapDescriptor.bytes(bytes!.buffer.asUint8List(), width: size, height: size);
  } catch (_) {
    return _circleIcon(dpr, color: _ink, label: 'A');
  }
}
