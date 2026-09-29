import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gm;
import 'package:maplibre_gl/maplibre_gl.dart' as ml;

import 'map_backend.dart';
import 'ondex_brand.dart';
import 'ondex_style.dart';
import 'poi_icons.dart';

/// KURYER KUZATUVI XARITASI — ikki motor, bitta xatti-harakat.
///
/// Manzil tanlash xaritasidan farqli o'laroq bu yerda kamera emas,
/// USTIDAGI narsalar muhim: restoran, mijoz va kuryer belgilari hamda
/// yo'l chizig'i. Shuning uchun interfeys ham shunga qurilgan —
/// chaqiruvchi "nima ko'rinsin" deydi, "qanday chizilsin" demaydi.

/// Xaritadagi bitta belgi.
///
/// Rasm PNG BAYTLARI sifatida beriladi, `BitmapDescriptor` emas: ikkala
/// motor ham baytni qabul qiladi, lekin har biri o'z turiga o'raydi.
/// Chaqiruvchi esa belgilarni allaqachon `Canvas` da chizib, PNG
/// qilib olyapti — ya'ni bu qo'shimcha ish emas.
@immutable
class TrackMarker {
  const TrackMarker({
    required this.id,
    required this.point,
    required this.image,
    required this.sizeDip,
    required this.imageWidthPx,
    this.rotation = 0,
    this.zIndex = 0,
    this.title,
  });

  final String id;
  final gm.LatLng point;

  /// PNG baytlari. `null` bo'lsa motorning standart belgisi ishlatiladi.
  final Uint8List? image;

  /// Belgining ekrandagi o'lchami (dip).
  final double sizeDip;

  /// Rasmning HAQIQIY kengligi (piksel).
  ///
  /// ┌─ NEGA KERAK ─────────────────────────────────────────────────────┐
  /// Ikki motor rasmni har xil o'lchaydi: Google Maps `width`/`height`
  /// ni dip da oladi va o'zi miqyoslaydi, MapLibre esa rasmni PIKSEL
  /// o'lchamida chizadi va `iconSize` ko'paytmasini qo'llaydi.
  ///
  /// Belgilar `size * devicePixelRatio` piksel qilib chiziladi, ya'ni
  /// dpr=3 qurilmada MapLibre ularni uch barobar katta ko'rsatardi.
  /// Haqiqiy piksel kengligi ma'lum bo'lsa, to'g'ri `iconSize` ni
  /// hisoblab olish mumkin — taxminsiz.
  /// └──────────────────────────────────────────────────────────────────┘
  final double imageWidthPx;

  /// Gradus; mashina yo'nalishi uchun.
  final double rotation;

  final int zIndex;
  final String? title;

  @override
  bool operator ==(Object other) =>
      other is TrackMarker &&
      other.id == id &&
      other.point == point &&
      other.rotation == rotation &&
      identical(other.image, image);

  @override
  int get hashCode => Object.hash(id, point, rotation, image);
}

/// Xaritadagi chiziq (yo'l).
@immutable
class TrackLine {
  const TrackLine({
    required this.id,
    required this.points,
    required this.color,
    required this.width,
  });

  final String id;
  final List<gm.LatLng> points;
  final Color color;
  final double width;
}

/// Xaritani dastur tomonidan boshqarish.
abstract class TrackMapController {
  /// Berilgan nuqtalarning hammasi ko'rinadigan qilib kamerani sozlaydi.
  Future<void> fitPoints(List<gm.LatLng> points, {double padding = 48});
}

class TrackMap extends StatefulWidget {
  const TrackMap({
    super.key,
    required this.initialCenter,
    required this.markers,
    required this.lines,
    required this.onReady,
    required this.onUserMoved,
  });

  final gm.LatLng initialCenter;
  final List<TrackMarker> markers;
  final List<TrackLine> lines;
  final void Function(TrackMapController controller) onReady;

  /// Foydalanuvchi xaritani O'ZI surganda (dastur emas).
  final VoidCallback onUserMoved;

  @override
  State<TrackMap> createState() => _TrackMapState();
}

class _TrackMapState extends State<TrackMap> {
  @override
  Widget build(BuildContext context) =>
      kUseOnDexMap ? _OndexTrackMap(widget) : _GoogleTrackMap(widget);
}

/// Xarita sahifa ichida (`ListView`) turadi: busiz barmoq xaritani
/// emas, sahifani surardi. Ikkala motor ham shu to'plamni oladi.
const Set<Factory<OneSequenceGestureRecognizer>> _eagerGestures = {
  Factory<OneSequenceGestureRecognizer>(EagerGestureRecognizer.new),
};

/* ══════════════════════════════════════════════════════════════════════
   Google Maps — HOZIRGI yo'l.
   ══════════════════════════════════════════════════════════════════════ */

class _GoogleTrackMap extends StatefulWidget {
  const _GoogleTrackMap(this.cfg);
  final TrackMap cfg;

  @override
  State<_GoogleTrackMap> createState() => _GoogleTrackMapState();
}

class _GoogleTrackMapState extends State<_GoogleTrackMap>
    implements TrackMapController {
  gm.GoogleMapController? _ctrl;

  /// Dastur o'zi kamerani surayotganda `onCameraMoveStarted`
  /// foydalanuvchi harakati deb hisoblanmasin.
  int _programmatic = 0;

  @override
  Future<void> fitPoints(List<gm.LatLng> points, {double padding = 48}) async {
    final ctrl = _ctrl;
    if (ctrl == null || points.isEmpty) return;
    final b = _bounds(points);
    final update = b == null
        ? gm.CameraUpdate.newLatLngZoom(points.first, 16)
        : gm.CameraUpdate.newLatLngBounds(
            gm.LatLngBounds(
              southwest: gm.LatLng(b.minLat, b.minLng),
              northeast: gm.LatLng(b.maxLat, b.maxLng),
            ),
            padding,
          );
    _programmatic++;
    try {
      await ctrl.animateCamera(update);
    } catch (_) {
      // Xarita hali o'lchanmagan bo'lsa `newLatLngBounds` xato beradi —
      // keyingi yangilanishda qayta urinadi.
    } finally {
      _programmatic--;
    }
  }

  gm.BitmapDescriptor _icon(TrackMarker m) {
    final bytes = m.image;
    if (bytes == null) {
      return gm.BitmapDescriptor.defaultMarkerWithHue(gm.BitmapDescriptor.hueOrange);
    }
    return gm.BitmapDescriptor.bytes(bytes, width: m.sizeDip, height: m.sizeDip);
  }

  @override
  Widget build(BuildContext context) {
    final cfg = widget.cfg;
    return gm.GoogleMap(
      initialCameraPosition: gm.CameraPosition(target: cfg.initialCenter, zoom: 14),
      markers: {
        for (final m in cfg.markers)
          gm.Marker(
            markerId: gm.MarkerId(m.id),
            position: m.point,
            icon: _icon(m),
            anchor: m.image != null ? const Offset(0.5, 0.5) : const Offset(0.5, 1),
            flat: m.image != null && m.rotation != 0,
            rotation: m.rotation,
            infoWindow: m.title == null ? gm.InfoWindow.noText : gm.InfoWindow(title: m.title),
            zIndexInt: m.zIndex,
          ),
      },
      polylines: {
        for (final l in cfg.lines)
          gm.Polyline(
            polylineId: gm.PolylineId(l.id),
            points: l.points,
            color: l.color,
            width: l.width.round(),
            jointType: gm.JointType.round,
            startCap: gm.Cap.roundCap,
            endCap: gm.Cap.roundCap,
          ),
      },
      onMapCreated: (c) {
        _ctrl = c;
        cfg.onReady(this);
      },
      onCameraMoveStarted: () {
        if (_programmatic == 0) cfg.onUserMoved();
      },
      gestureRecognizers: _eagerGestures,
      myLocationEnabled: false,
      myLocationButtonEnabled: false,
      zoomControlsEnabled: false,
      mapToolbarEnabled: false,
      compassEnabled: false,
    );
  }
}

/* ══════════════════════════════════════════════════════════════════════
   OnDexMap (MapLibre Native).

   ┌─ FARQ: BELGILAR IMPERATIV QO'SHILADI ────────────────────────────┐
   Google Maps'da markerlar — vidjet parametri: ro'yxat o'zgarsa
   xarita o'zi yangilanadi. MapLibre'da esa ular boshqaruvchi orqali
   QO'SHILADI va OLIB TASHLANADI.

   Shuning uchun bu yerda `didUpdateWidget` da farq hisoblanadi.
   Soddalik uchun har yangilanishda hammasi qayta chiziladi: kuryer
   kuzatuvida belgilar uchtadan oshmaydi va yangilanish 15 soniyada
   bir marta keladi, ya'ni bu sezilarli yuk emas.
   └───────────────────────────────────────────────────────────────────┘
   ══════════════════════════════════════════════════════════════════════ */

class _OndexTrackMap extends StatefulWidget {
  const _OndexTrackMap(this.cfg);
  final TrackMap cfg;

  @override
  State<_OndexTrackMap> createState() => _OndexTrackMapState();
}

class _OndexTrackMapState extends State<_OndexTrackMap>
    implements TrackMapController {
  ml.MapLibreMapController? _ctrl;
  bool _styleReady = false;

  /// Xaritaga yuklangan rasmlar (`addImage` nomi bo'yicha).
  final _loadedImages = <String>{};

  int _programmatic = 0;

  /// Qayta chizish navbati: uslub tayyor bo'lmaguncha kutadi.
  Future<void>? _pending;

  @override
  Future<void> fitPoints(List<gm.LatLng> points, {double padding = 48}) async {
    final ctrl = _ctrl;
    if (ctrl == null || points.isEmpty) return;
    final b = _bounds(points);
    final update = b == null
        ? ml.CameraUpdate.newLatLngZoom(
            ml.LatLng(points.first.latitude, points.first.longitude), 16)
        : ml.CameraUpdate.newLatLngBounds(
            ml.LatLngBounds(
              southwest: ml.LatLng(b.minLat, b.minLng),
              northeast: ml.LatLng(b.maxLat, b.maxLng),
            ),
            left: padding,
            right: padding,
            top: padding,
            bottom: padding,
          );
    _programmatic++;
    try {
      await ctrl.animateCamera(update);
    } catch (_) {
      // Google tarafdagi bilan bir xil sabab.
    } finally {
      _programmatic--;
    }
  }

  @override
  void didUpdateWidget(_OndexTrackMap old) {
    super.didUpdateWidget(old);
    _scheduleRedraw();
  }

  void _scheduleRedraw() {
    if (!_styleReady) return;
    // Ketma-ket kelgan yangilanishlar bir-birini bosib ketmasin.
    _pending = (_pending ?? Future<void>.value()).then((_) => _redraw());
  }

  Future<void> _redraw() async {
    final ctrl = _ctrl;
    if (ctrl == null || !mounted) return;
    final cfg = widget.cfg;

    try {
      // Rasmlar bir marta yuklanadi — nomi belgi `id`si bilan bir xil.
      for (final m in cfg.markers) {
        final bytes = m.image;
        if (bytes == null || _loadedImages.contains(m.id)) continue;
        await ctrl.addImage(m.id, bytes);
        _loadedImages.add(m.id);
      }

      await ctrl.clearSymbols();
      await ctrl.clearLines();

      for (final l in cfg.lines) {
        await ctrl.addLine(ml.LineOptions(
          geometry: [
            for (final p in l.points) ml.LatLng(p.latitude, p.longitude),
          ],
          lineColor: _hex(l.color),
          lineWidth: l.width,
          lineJoin: 'round',
        ));
      }

      // `zIndex` o'rniga qo'shilish TARTIBI: MapLibre belgilarni
      // qo'shilgan ketma-ketligida chizadi.
      final byZ = [...cfg.markers]..sort((a, b) => a.zIndex.compareTo(b.zIndex));
      for (final m in byZ) {
        await ctrl.addSymbol(ml.SymbolOptions(
          geometry: ml.LatLng(m.point.latitude, m.point.longitude),
          // Rasm berilmagan bo'lsa — umumiy belgi. Uslubning o'z
          // sprite'i bo'sh, shuning uchun `marker-15` kabi tayyor
          // nomlar ishlamaydi (ular jimgina chizilmasdi).
          iconImage: m.image != null ? m.id : '${markerPrefix}selected',
          // Rasm piksel o'lchamida chiziladi — uni dip ga qaytaramiz,
          // aks holda yuqori zichlikdagi ekranda belgi ulkan bo'lardi.
          iconSize: m.image != null && m.imageWidthPx > 0
              ? m.sizeDip / m.imageWidthPx
              : 1,
          iconRotate: m.rotation,
          iconAnchor: 'center',
        ));
      }
    } catch (e) {
      // Xarita yopilgan bo'lsa (ekrandan chiqildi) chaqiruvlar xato
      // beradi — bu kutilgan holat, kuzatuv ekrani baribir yo'q.
      debugPrint('[ondexmap] qayta chizilmadi: $e');
    }
  }

  static String _hex(Color c) {
    int ch(double v) => (v * 255).round() & 0xff;
    return '#${ch(c.r).toRadixString(16).padLeft(2, '0')}'
        '${ch(c.g).toRadixString(16).padLeft(2, '0')}'
        '${ch(c.b).toRadixString(16).padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        _map(widget.cfg),
        const OndexMapBrand(),
      ],
    );
  }

  Widget _map(TrackMap cfg) {
    return ml.MapLibreMap(
      styleString: ondexVectorStyle,
      initialCameraPosition: ml.CameraPosition(
        target: ml.LatLng(cfg.initialCenter.latitude, cfg.initialCenter.longitude),
        zoom: 14,
      ),
      onMapCreated: (c) {
        _ctrl = c;
        cfg.onReady(this);
      },
      // ⚠️ Belgilarni uslub yuklangunga QADAR qo'shib bo'lmaydi:
      // `addImage` manba topa olmay xato beradi.
      onStyleLoadedCallback: () async {
        _styleReady = true;
        _loadedImages.clear();
        // Joy belgilari uslubda emas (`poi_icons.dart`) — busiz
        // kuzatuv xaritasi ham bo'm-bo'sh ko'rinadi.
        final c = _ctrl;
        if (c != null && mounted) {
          await loadPoiIcons(c);
        }
        _scheduleRedraw();
      },
      onCameraTrackingDismissed: cfg.onUserMoved,
      onCameraMove: (_) {
        if (_programmatic == 0) cfg.onUserMoved();
      },
      trackCameraPosition: true,
      gestureRecognizers: _eagerGestures,
      // Kamera mamlakatdan chiqmaydi — tashqarida xarita ma'lumoti yo'q.
      cameraTargetBounds: ondexCameraBounds,
      minMaxZoomPreference: const ml.MinMaxZoomPreference(kOndexMinZoom, null),
      compassEnabled: false,
      logoEnabled: false,
      myLocationEnabled: false,
      tiltGesturesEnabled: false,
      // Atribut tugmasi ekrandan chiqariladi — o'rniga «OnDex» yorlig'i.
      attributionButtonMargins: kOndexHideAttribution,
    );
  }
}

/* ══════════════════════════════════════════════════════════════════════
   Umumiy yordamchi
   ══════════════════════════════════════════════════════════════════════ */

@immutable
class _Bounds {
  const _Bounds(this.minLat, this.minLng, this.maxLat, this.maxLng);
  final double minLat, minLng, maxLat, maxLng;
}

/// Nuqtalarni o'rab turgan to'rtburchak.
///
/// Hamma nuqta deyarli bir joyda bo'lsa `null` qaytaradi: nol
/// o'lchamli to'rtburchak bilan `fitBounds` ma'nosiz masshtab beradi
/// (ikkala motorda ham).
_Bounds? _bounds(List<gm.LatLng> points) {
  var minLat = points.first.latitude, maxLat = minLat;
  var minLng = points.first.longitude, maxLng = minLng;
  for (final p in points) {
    if (p.latitude < minLat) minLat = p.latitude;
    if (p.latitude > maxLat) maxLat = p.latitude;
    if (p.longitude < minLng) minLng = p.longitude;
    if (p.longitude > maxLng) maxLng = p.longitude;
  }
  if ((maxLat - minLat).abs() < 1e-4 && (maxLng - minLng).abs() < 1e-4) {
    return null;
  }
  return _Bounds(minLat, minLng, maxLat, maxLng);
}
