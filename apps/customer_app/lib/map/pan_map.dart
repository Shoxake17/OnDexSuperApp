import 'dart:async';

import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gm;
import 'package:maplibre_gl/maplibre_gl.dart' as ml;
import 'package:ondex_core/ondex_core.dart' show MapConfigService;

import 'map_backend.dart';
import 'ondex_brand.dart';
import 'ondex_style.dart';
import 'poi_icons.dart';

/// MANZIL TANLASH XARITASI — ikki motor, bitta xatti-harakat.
///
/// Ekranning markazida qimirlamas pin turadi, foydalanuvchi xaritani
/// suradi. Shuning uchun bu yerda marker ham, chizilgan yo'l ham kerak
/// emas — faqat kamera: qayerdaligi va uni surish.
///
/// ┌─ NEGA `gm.LatLng` TASHQI TURI ─────────────────────────────────────┐
/// Chaqiruvchi ekran (`AddressScreen`) va buyurtma modeli allaqachon
/// `google_maps_flutter` ning `LatLng` turida yozilgan. Yangi
/// "neytral" nuqta turini kiritish butun oqimni (model, API, kuzatuv)
/// o'zgartirishni talab qilardi — bu production ilovada keraksiz xavf.
///
/// Shuning uchun tashqi shartnoma o'zgarmaydi, konvertatsiya esa FAQAT
/// shu faylning ichida, bitta joyda bo'ladi. `google_maps_flutter`
/// baribir paketda qoladi: flag o'chiq bo'lsa xarita o'sha paket
/// bilan chiziladi.
/// └────────────────────────────────────────────────────────────────────┘

/// Xaritani dastur tomonidan surish.
///
/// Ikkala motor uchun bitta interfeys: chaqiruvchi qaysi biri
/// ishlayotganini bilmaydi.
abstract class PanMapController {
  /// Kamerani berilgan nuqtaga va masshtabga olib boradi.
  Future<void> animateTo(gm.LatLng target, double zoom);
}

class PanMap extends StatefulWidget {
  const PanMap({
    super.key,
    required this.initialCenter,
    required this.initialZoom,
    required this.satellite,
    required this.onCameraMove,
    required this.onCameraIdle,
    required this.onReady,
  });

  final gm.LatLng initialCenter;
  final double initialZoom;

  /// Sun'iy yo'ldosh ko'rinishi.
  final bool satellite;

  /// Kamera surilganda — HAR kadrda chaqiriladi.
  final void Function(gm.LatLng center, double zoom) onCameraMove;

  /// Surish to'xtaganda.
  final VoidCallback onCameraIdle;

  /// Xarita tayyor bo'lganda boshqaruvchi beriladi.
  final void Function(PanMapController controller) onReady;

  @override
  State<PanMap> createState() => _PanMapState();
}

class _PanMapState extends State<PanMap> {
  /// Motor ekran ochilganda BIR MARTA tanlanadi (`GOOGLE_MAP`, serverdan).
  /// Xarita ish paytida o'zi almashmaydi — faqat OnDexMap xatosida
  /// Google'ga tushadi (`_ondexFailed`).
  late bool _ondex = useOndexMapNow;

  void _ondexFailed(String reason) {
    if (!_ondex || !mounted) return;
    MapConfigService.instance.reportOndexFailure('address', reason);
    setState(() => _ondex = false);
  }

  @override
  Widget build(BuildContext context) => _ondex
      ? _OndexPanMap(widget, onFailure: _ondexFailed)
      : _GooglePanMap(widget);
}

/* ══════════════════════════════════════════════════════════════════════
   Google Maps — HOZIRGI yo'l.

   Bu qism `address_screen.dart` da nima bo'lgan bo'lsa, o'shaning
   o'zi: bir xil parametrlar, bir xil callback'lar. Flag o'chiq
   bo'lsa ilova aynan shu kodni ishlatadi.
   ══════════════════════════════════════════════════════════════════════ */

class _GooglePanMap extends StatefulWidget {
  const _GooglePanMap(this.cfg);
  final PanMap cfg;

  @override
  State<_GooglePanMap> createState() => _GooglePanMapState();
}

class _GooglePanMapState extends State<_GooglePanMap> implements PanMapController {
  gm.GoogleMapController? _ctrl;

  @override
  Future<void> animateTo(gm.LatLng target, double zoom) async {
    await _ctrl?.animateCamera(
      gm.CameraUpdate.newCameraPosition(
        gm.CameraPosition(target: target, zoom: zoom),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final cfg = widget.cfg;
    return gm.GoogleMap(
      initialCameraPosition: gm.CameraPosition(
        target: cfg.initialCenter,
        zoom: cfg.initialZoom,
      ),
      mapType: cfg.satellite ? gm.MapType.hybrid : gm.MapType.normal,
      onMapCreated: (c) {
        _ctrl = c;
        cfg.onReady(this);
      },
      onCameraMove: (pos) => cfg.onCameraMove(pos.target, pos.zoom),
      onCameraIdle: cfg.onCameraIdle,
      zoomControlsEnabled: false,
      myLocationButtonEnabled: false,
      mapToolbarEnabled: false,
      compassEnabled: false,
    );
  }
}

/* ══════════════════════════════════════════════════════════════════════
   OnDexMap (MapLibre Native).

   Uslub — `maps.ondex.uz/tiles/style.json` (vektor, PMTiles).
   Sun'iy yo'ldosh — o'z raster manbamiz; uslub `ondex_style.dart` da
   quriladi.
   ══════════════════════════════════════════════════════════════════════ */

class _OndexPanMap extends StatefulWidget {
  const _OndexPanMap(this.cfg, {required this.onFailure});
  final PanMap cfg;

  /// OnDexMap ishlamadi — chaqiruvchi Google'ga o'tadi.
  final void Function(String reason) onFailure;

  @override
  State<_OndexPanMap> createState() => _OndexPanMapState();
}

class _OndexPanMapState extends State<_OndexPanMap> implements PanMapController {
  ml.MapLibreMapController? _ctrl;
  Timer? _watchdog;

  @override
  void initState() {
    super.initState();
    _armWatchdog();
  }

  /// Uslub [kOndexStyleTimeout] ichida yuklanmasa — OnDexMap ishlamayapti.
  /// `maplibre_gl` uslub xatosi uchun callback bermaydi, shuning uchun
  /// "yuklandi" signalining kelmasligi yagona ishonchli belgi.
  void _armWatchdog() {
    _watchdog?.cancel();
    _watchdog = Timer(kOndexStyleTimeout, () {
      if (mounted) {
        widget.onFailure('uslub ${kOndexStyleTimeout.inSeconds} s ichida yuklanmadi');
      }
    });
  }

  @override
  void didUpdateWidget(_OndexPanMap old) {
    super.didUpdateWidget(old);
    // Oddiy ↔ sun'iy yo'ldosh: xarita qayta yaratiladi, uslub qayta yuklanadi.
    if (old.cfg.satellite != widget.cfg.satellite) _armWatchdog();
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
        ml.CameraPosition(target: ml.LatLng(target.latitude, target.longitude), zoom: zoom),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final cfg = widget.cfg;
    return Stack(
      children: [
        _map(cfg),
        // MapLibre'ning «i» tugmasi o'rniga ilovaning o'z yorlig'i.
        const OndexMapBrand(),
      ],
    );
  }

  Widget _map(PanMap cfg) {
    return ml.MapLibreMap(
      // ⚠️ `ValueKey` SHART: uslub almashganda (oddiy ↔ sun'iy yo'ldosh)
      // MapLibre uni joyida yangilamaydi, vidjet qayta yaratilishi kerak.
      key: ValueKey(cfg.satellite),
      styleString: cfg.satellite ? ondexSatelliteStyle : kOndexStyleUrl,
      initialCameraPosition: ml.CameraPosition(
        target: ml.LatLng(cfg.initialCenter.latitude, cfg.initialCenter.longitude),
        zoom: cfg.initialZoom,
      ),
      // Kamera holatini kuzatish YOQILADI: usiz `onCameraMove`
      // chaqirilmaydi va markaz hech qachon yangilanmasdi.
      trackCameraPosition: true,
      onMapCreated: (c) {
        _ctrl = c;
        cfg.onReady(this);
      },
      // Joy belgilari uslubda emas, mijoz tomonidan beriladi —
      // `poi_icons.dart` izohiga qarang. Busiz xarita bo'm-bo'sh
      // ko'rinadi: na do'kon, na kafe nomi.
      onStyleLoadedCallback: () async {
        _watchdog?.cancel();
        MapConfigService.instance.reportOndexRecovered();
        final c = _ctrl;
        if (c != null && mounted) {
          await loadPoiIcons(c);
        }
      },
      onCameraIdle: cfg.onCameraIdle,
      onCameraMove: (pos) =>
          cfg.onCameraMove(gm.LatLng(pos.target.latitude, pos.target.longitude), pos.zoom),
      // Kamera mamlakatdan chiqmaydi — tashqarida xarita ma'lumoti yo'q.
      cameraTargetBounds: ondexCameraBounds,
      minMaxZoomPreference: const ml.MinMaxZoomPreference(kOndexMinZoom, null),
      compassEnabled: false,
      // Atribut tugmasi ekrandan chiqariladi, o'rniga ilovaning
      // «OnDex» yorlig'i qo'yiladi (`ondex_brand.dart` izohiga qarang).
      attributionButtonMargins: kOndexHideAttribution,
      logoEnabled: false,
      myLocationEnabled: false,
      // Xarita 2D: manzil tanlashda qiyalik faqat adashtiradi.
      tiltGesturesEnabled: false,
      rotateGesturesEnabled: false,
    );
  }
}
