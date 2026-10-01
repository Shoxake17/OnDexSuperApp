import 'dart:async';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gm;
import 'package:maplibre_gl/maplibre_gl.dart' as ml;
import 'package:ondex_core/ondex_core.dart' show MapConfigService;
import 'package:ondex_maplibre/ondex_maplibre.dart';

/// KURYER XARITASI — ikki motor, bitta xatti-harakat.
///
/// Qaysi motor ishlashini serverdagi `GOOGLE_MAP` bayrog'i hal qiladi
/// (`ondex_core` → `MapConfigService`):
///   * `true`  — Google Maps (avvalgidek);
///   * `false` — OnDexMap (MapLibre).
///
/// OnDexMap ishlamasa (uslub 10 soniyada yuklanmasa) xarita o'zi Google'ga
/// tushadi va ilova keyingi 5 daqiqa Google'da turadi: kuryer xarita
/// o'rniga xato ko'rmaydi — u buyurtma yetkazayotgan bo'ladi.
///
/// Tashqi shartnoma `google_maps_flutter` ning `LatLng` turida qoldi: ekran
/// (`CourierShell`) butunlay shu turda yozilgan, konvertatsiya faqat shu
/// fayl ichida.

/// Xaritani dastur tomonidan surish (ikkala motor uchun bitta interfeys).
abstract class CourierMapController {
  Future<void> animateTo(gm.LatLng target, double zoom);
}

/// OnDexMap belgisi uchun rasm (PNG) va uning o'lchamlari.
@immutable
class CourierMarkerImage {
  const CourierMarkerImage({
    required this.bytes,
    required this.widthPx,
    required this.heightPx,
    required this.sizeDip,
  });

  final Uint8List bytes;
  final int widthPx;
  final int heightPx;

  /// Rasmning ekrandagi kengligi (dip).
  final double sizeDip;
}

/// Xaritadagi bitta belgi. Ikkala motor uchun kerakli hamma narsa shu yerda:
/// Google `googleIcon`/`googleHue` dan, OnDexMap `image` yoki `fallbackColor`
/// dan foydalanadi.
@immutable
class CourierMarker {
  const CourierMarker({
    required this.id,
    required this.point,
    this.rotation = 0,
    this.flat = false,
    this.anchor = const Offset(0.5, 1.0),
    this.title,
    this.googleIcon,
    this.googleHue = gm.BitmapDescriptor.hueRed,
    this.imageKey = '',
    this.image,
    this.fallbackColor = const Color(0xFFF97316),
  });

  final String id;
  final gm.LatLng point;
  final double rotation;
  final bool flat;

  /// Rasmning qaysi nuqtasi `point` ga to'g'ri keladi (0..1).
  final Offset anchor;
  final String? title;

  // ---- Google ----
  final gm.BitmapDescriptor? googleIcon;
  final double googleHue;

  // ---- OnDexMap ----
  /// Rasm nomi: bir xil rasm xaritaga bir marta yuklanadi.
  final String imageKey;
  final CourierMarkerImage? image;

  /// Rasm bo'lmasa shu rangdagi nuqta chiziladi.
  final Color fallbackColor;
}

class CourierMap extends StatefulWidget {
  const CourierMap({
    super.key,
    required this.initialCenter,
    required this.initialZoom,
    required this.satellite,
    required this.markers,
    required this.route,
    required this.routeColor,
    required this.bottomPadding,
    required this.onCameraMove,
    required this.onReady,
  });

  final gm.LatLng initialCenter;
  final double initialZoom;

  /// Sun'iy yo'ldosh ko'rinishi (Google'da `hybrid`).
  final bool satellite;
  final List<CourierMarker> markers;

  /// Marshrut chizig'i (ikkitadan kam nuqta — chiziq yo'q).
  final List<gm.LatLng> route;
  final Color routeColor;

  /// Panel egallagan pastki qism: kamera markazi va marshrut panel ostiga
  /// yashirinmasin.
  final double bottomPadding;

  /// Kamera surilganda. Zoom Google shkalasida (OnDexMap'da +1 ga
  /// aylantirilgan, `_mlToGoogleZoom`).
  final void Function(gm.LatLng center, double zoom) onCameraMove;
  final void Function(CourierMapController controller) onReady;

  @override
  State<CourierMap> createState() => _CourierMapState();
}

class _CourierMapState extends State<CourierMap> {
  /// Motor ekran ochilganda BIR MARTA tanlanadi; faqat OnDexMap xatosida
  /// Google'ga tushadi.
  late bool _ondex = MapConfigService.instance.useOndexMap;

  void _ondexFailed(String reason) {
    if (!_ondex || !mounted) return;
    MapConfigService.instance.reportOndexFailure('courier', reason);
    setState(() => _ondex = false);
  }

  @override
  Widget build(BuildContext context) => _ondex
      ? _OndexCourierMap(widget, onFailure: _ondexFailed)
      : _GoogleCourierMap(widget);
}

/* ══════════════════════════════════════════════════════════════════════
   Google Maps — AVVALGI yo'l (kodning mazmuni `courier_shell.dart` dan
   ko'chirilgan, o'zgarishsiz).
   ══════════════════════════════════════════════════════════════════════ */

class _GoogleCourierMap extends StatefulWidget {
  const _GoogleCourierMap(this.cfg);
  final CourierMap cfg;

  @override
  State<_GoogleCourierMap> createState() => _GoogleCourierMapState();
}

class _GoogleCourierMapState extends State<_GoogleCourierMap>
    implements CourierMapController {
  gm.GoogleMapController? _ctrl;

  @override
  Future<void> animateTo(gm.LatLng target, double zoom) async {
    await _ctrl?.animateCamera(
      gm.CameraUpdate.newCameraPosition(
        gm.CameraPosition(target: target, zoom: zoom),
      ),
    );
  }

  Set<gm.Marker> _markers() => {
        for (final m in widget.cfg.markers)
          gm.Marker(
            markerId: gm.MarkerId(m.id),
            position: m.point,
            icon: m.googleIcon ?? gm.BitmapDescriptor.defaultMarkerWithHue(m.googleHue),
            anchor: m.anchor,
            rotation: m.rotation,
            flat: m.flat,
            infoWindow: m.title == null ? gm.InfoWindow.noText : gm.InfoWindow(title: m.title),
          ),
      };

  Set<gm.Polyline> _polylines() => widget.cfg.route.length < 2
      ? {}
      : {
          gm.Polyline(
            polylineId: const gm.PolylineId('route'),
            points: widget.cfg.route,
            color: widget.cfg.routeColor,
            width: 5,
          ),
        };

  @override
  Widget build(BuildContext context) {
    final cfg = widget.cfg;
    return gm.GoogleMap(
      initialCameraPosition:
          gm.CameraPosition(target: cfg.initialCenter, zoom: cfg.initialZoom),
      mapType: cfg.satellite ? gm.MapType.hybrid : gm.MapType.normal,
      onMapCreated: (c) {
        _ctrl = c;
        cfg.onReady(this);
      },
      onCameraMove: (pos) => cfg.onCameraMove(pos.target, pos.zoom),
      // Xaritaning ko'rinadigan qismi panel ustida: kamera markazi va
      // marshrut panel ostiga yashirinmasin.
      padding: EdgeInsets.only(bottom: cfg.bottomPadding),
      myLocationEnabled: false,
      myLocationButtonEnabled: false,
      zoomControlsEnabled: false,
      mapToolbarEnabled: false,
      compassEnabled: false,
      markers: _markers(),
      polylines: _polylines(),
    );
  }
}

/* ══════════════════════════════════════════════════════════════════════
   OnDexMap (MapLibre Native).
   ══════════════════════════════════════════════════════════════════════ */

/// MapLibre zoomi Google'nikidan 1 ga kam: u 512 px, Google 256 px plitka
/// ishlatadi. Ekran Google shkalasida o'ylaydi, shuning uchun almashtiramiz.
double _googleToMlZoom(double z) => z - 1;
double _mlToGoogleZoom(double z) => z + 1;

class _OndexCourierMap extends StatefulWidget {
  const _OndexCourierMap(this.cfg, {required this.onFailure});
  final CourierMap cfg;

  /// OnDexMap ishlamadi — chaqiruvchi Google'ga o'tadi.
  final void Function(String reason) onFailure;

  @override
  State<_OndexCourierMap> createState() => _OndexCourierMapState();
}

class _OndexCourierMapState extends State<_OndexCourierMap>
    implements CourierMapController {
  ml.MapLibreMapController? _ctrl;
  bool _styleReady = false;
  Timer? _watchdog;

  /// Xaritaga yuklangan rasmlar (`addImage` nomi bo'yicha).
  final _loadedImages = <String>{};
  final _symbols = <String, ml.Symbol>{};
  ml.Line? _routeLine;

  /// Qayta chizish navbati: ketma-ket yangilanishlar bir-birini bosmasin.
  Future<void>? _pending;

  @override
  void initState() {
    super.initState();
    _armWatchdog();
  }

  /// Uslub [kOndexStyleTimeout] ichida yuklanmasa — OnDexMap ishlamayapti.
  void _armWatchdog() {
    _watchdog?.cancel();
    _watchdog = Timer(kOndexStyleTimeout, () {
      if (mounted && !_styleReady) {
        widget.onFailure('uslub ${kOndexStyleTimeout.inSeconds} s ichida yuklanmadi');
      }
    });
  }

  @override
  void dispose() {
    _watchdog?.cancel();
    super.dispose();
  }

  @override
  Future<void> animateTo(gm.LatLng target, double zoom) async {
    await _ctrl?.animateCamera(
      ml.CameraUpdate.newCameraPosition(
        ml.CameraPosition(
          target: ml.LatLng(target.latitude, target.longitude),
          zoom: _googleToMlZoom(zoom),
        ),
      ),
    );
  }

  @override
  void didUpdateWidget(_OndexCourierMap old) {
    super.didUpdateWidget(old);
    if (old.cfg.satellite != widget.cfg.satellite) {
      // Uslub almashadi: xarita qayta yaratiladi, belgilar `onStyleLoaded` da qaytadi.
      _styleReady = false;
      _symbols.clear();
      _routeLine = null;
      _loadedImages.clear();
      _armWatchdog();
      return;
    }
    _scheduleSync();
  }

  void _scheduleSync() {
    if (!_styleReady) return;
    _pending = (_pending ?? Future<void>.value()).then((_) => _sync());
  }

  Future<void> _sync() async {
    final ctrl = _ctrl;
    if (ctrl == null || !mounted) return;
    final cfg = widget.cfg;
    try {
      // ---- marshrut chizig'i ----
      if (cfg.route.length < 2) {
        final line = _routeLine;
        if (line != null) {
          await ctrl.removeLine(line);
          _routeLine = null;
        }
      } else {
        final geometry = [for (final p in cfg.route) ml.LatLng(p.latitude, p.longitude)];
        final line = _routeLine;
        if (line == null) {
          _routeLine = await ctrl.addLine(ml.LineOptions(
            geometry: geometry,
            lineColor: _hex(cfg.routeColor),
            lineWidth: 5,
            lineJoin: 'round',
          ));
        } else {
          await ctrl.updateLine(line, ml.LineOptions(geometry: geometry));
        }
      }

      // ---- belgilar ----
      final ids = {for (final m in cfg.markers) m.id};
      for (final stale in _symbols.keys.where((k) => !ids.contains(k)).toList()) {
        await ctrl.removeSymbol(_symbols.remove(stale)!);
      }
      for (final m in cfg.markers) {
        final img = await _imageFor(ctrl, m);
        // Rasm bo'lmasa rangli nuqta markazda turadi.
        final anchor = (m.image != null && m.imageKey.isNotEmpty) ? m.anchor : const Offset(0.5, 0.5);
        final options = ml.SymbolOptions(
          geometry: ml.LatLng(m.point.latitude, m.point.longitude),
          iconImage: img.name,
          iconSize: img.size,
          iconRotate: m.rotation,
          iconAnchor: 'center',
          // Rasmning `anchor` nuqtasi `point` da tursin (markazdan siljitish).
          iconOffset: Offset(
            -(anchor.dx - 0.5) * img.widthPx,
            -(anchor.dy - 0.5) * img.heightPx,
          ),
        );
        final existing = _symbols[m.id];
        if (existing == null) {
          _symbols[m.id] = await ctrl.addSymbol(options);
        } else {
          await ctrl.updateSymbol(existing, options);
        }
      }
    } catch (e) {
      // Xarita yopilgan bo'lsa chaqiruvlar xato beradi — kutilgan holat.
      debugPrint('[ondexmap] kuryer xaritasi qayta chizilmadi: $e');
    }
  }

  /// Belgi uchun rasmni xaritaga bir marta yuklaydi.
  Future<_Img> _imageFor(ml.MapLibreMapController ctrl, CourierMarker m) async {
    final provided = m.image;
    if (provided != null && m.imageKey.isNotEmpty) {
      final name = 'courier-${m.imageKey}';
      if (_loadedImages.add(name)) await ctrl.addImage(name, provided.bytes);
      return _Img(name, provided.sizeDip / provided.widthPx, provided.widthPx, provided.heightPx);
    }
    // Rasm yo'q — rangli nuqta.
    final name = 'courier-dot-${m.fallbackColor.toARGB32()}';
    if (_loadedImages.add(name)) await ctrl.addImage(name, await _dotPng(m.fallbackColor));
    return _Img(name, _dotDip / _dotPx, _dotPx, _dotPx);
  }

  @override
  Widget build(BuildContext context) {
    final cfg = widget.cfg;
    return Stack(
      children: [
        // Panel egallagan qism xaritadan chiqarib tashlanadi: `maplibre_gl`
        // da Google'dagi kabi ichki `padding` yo'q.
        Positioned.fill(
          bottom: cfg.bottomPadding,
          child: ml.MapLibreMap(
            // ⚠️ `ValueKey` SHART: uslub almashganda (oddiy ↔ sun'iy
            // yo'ldosh) MapLibre uni joyida yangilamaydi.
            key: ValueKey(cfg.satellite),
            styleString: cfg.satellite ? ondexSatelliteStyle : kOndexStyleUrl,
            initialCameraPosition: ml.CameraPosition(
              target: ml.LatLng(cfg.initialCenter.latitude, cfg.initialCenter.longitude),
              zoom: _googleToMlZoom(cfg.initialZoom),
            ),
            trackCameraPosition: true,
            onMapCreated: (c) {
              _ctrl = c;
              cfg.onReady(this);
            },
            // ⚠️ Belgilarni uslub yuklangunga QADAR qo'shib bo'lmaydi.
            onStyleLoadedCallback: () async {
              _styleReady = true;
              _watchdog?.cancel();
              MapConfigService.instance.reportOndexRecovered();
              _loadedImages.clear();
              _symbols.clear();
              _routeLine = null;
              // Joy belgilari uslubda emas — busiz do'kon/kafe ko'rinmaydi.
              final c = _ctrl;
              if (c != null && mounted) await loadPoiIcons(c);
              _scheduleSync();
            },
            onCameraMove: (pos) => cfg.onCameraMove(
              gm.LatLng(pos.target.latitude, pos.target.longitude),
              _mlToGoogleZoom(pos.zoom),
            ),
            // Kamera mamlakatdan chiqmaydi — tashqarida xarita ma'lumoti yo'q.
            cameraTargetBounds: ondexCameraBounds,
            minMaxZoomPreference: const ml.MinMaxZoomPreference(kOndexMinZoom, null),
            compassEnabled: false,
            logoEnabled: false,
            myLocationEnabled: false,
            // Kuryer xaritasi 2D: qiyalik va burish faqat adashtiradi.
            tiltGesturesEnabled: false,
            rotateGesturesEnabled: false,
            attributionButtonMargins: kOndexHideAttribution,
          ),
        ),
        // Atribut yorlig'i panel USTIDA ko'rinsin.
        OndexMapBrand(bottom: cfg.bottomPadding + 4),
      ],
    );
  }

  static String _hex(Color c) {
    int ch(double v) => (v * 255).round() & 0xff;
    return '#${ch(c.r).toRadixString(16).padLeft(2, '0')}'
        '${ch(c.g).toRadixString(16).padLeft(2, '0')}'
        '${ch(c.b).toRadixString(16).padLeft(2, '0')}';
  }
}

class _Img {
  const _Img(this.name, this.size, this.widthPx, this.heightPx);
  final String name;

  /// `iconSize` — rasmni dip o'lchamiga keltiruvchi koeffitsient.
  final double size;
  final int widthPx;
  final int heightPx;
}

const int _dotPx = 56;
const double _dotDip = 22;

/// Rasm berilmagan belgi uchun: oq hoshiyali rangli nuqta.
Future<Uint8List> _dotPng(Color color) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  const c = Offset(_dotPx / 2, _dotPx / 2);
  canvas.drawCircle(c, _dotPx / 2, Paint()..color = Colors.white);
  canvas.drawCircle(c, _dotPx / 2 - 6, Paint()..color = color);
  final image = await recorder.endRecording().toImage(_dotPx, _dotPx);
  final data = await image.toByteData(format: ui.ImageByteFormat.png);
  return data!.buffer.asUint8List();
}
