import 'dart:math' as math;

import 'package:latlong2/latlong.dart';

/// OnDexMap muharriri: geometriya bilan sof (UI'siz) ishlash.
///
/// Xaritada chizilgan chegara/chiziqni GeoJSON'ga aylantirish, serverdan
/// kelgan GeoJSON'ni o'qish, chizilgan shaklni TEKSHIRISH va xaritada
/// bosilgan joy qaysi obyektga tegishli ekanini aniqlash. Hammasi alohida
/// fayl va testlanadi: ekran o'zgarganda ham bu mantiq buzilmasin.
///
/// Koordinata tartibi: GeoJSON'da `[lng, lat]`, `LatLng` da (lat, lng) —
/// ikkalasini adashtirish eng ko'p uchraydigan xato, shuning uchun
/// aylantirish FAQAT shu yerda.

enum GeomKind { polygon, line }

/// Chizilgan yoki saqlangan shakl (ko'rsatish va bosishni aniqlash uchun).
class EditorShape {
  final GeomKind kind;

  /// Poligon: tashqi konturlar (yopuvchi takror nuqtasiz). Chiziq: chiziqlar.
  final List<List<LatLng>> parts;

  const EditorShape(this.kind, this.parts);

  /// Barcha nuqtalar.
  Iterable<LatLng> get points sync* {
    for (final p in parts) {
      yield* p;
    }
  }

  /// [minLat, minLng, maxLat, maxLng]
  List<double> get bounds {
    var minLat = 90.0, minLng = 180.0, maxLat = -90.0, maxLng = -180.0;
    for (final p in points) {
      minLat = math.min(minLat, p.latitude);
      maxLat = math.max(maxLat, p.latitude);
      minLng = math.min(minLng, p.longitude);
      maxLng = math.max(maxLng, p.longitude);
    }
    return [minLat, minLng, maxLat, maxLng];
  }

  /// Yorliq qo'yiladigan nuqta: poligon — eng katta bo'lakning og'irlik
  /// markazi; chiziq — eng uzun chiziqning o'rta nuqtasi.
  LatLng get labelPoint {
    if (parts.isEmpty || parts.every((p) => p.isEmpty)) return const LatLng(0, 0);
    if (kind == GeomKind.polygon) {
      final biggest = parts.reduce((a, b) => _area(a).abs() >= _area(b).abs() ? a : b);
      return _centroid(biggest);
    }
    final longest = parts.reduce((a, b) => lineLengthMeters(a) >= lineLengthMeters(b) ? a : b);
    return _pointAtHalf(longest);
  }
}

/// Bitta shaklda ko'pi bilan shuncha nuqta o'qiladi. Buzuq/dushman
/// ma'lumot (millionlab nuqta) ekranni qotirib qo'ymasin.
const maxVertices = 50000;

/// GeoJSON geometriyasini o'qiydi. Yaroqsiz bo'lsa `null` (yiqilmaydi).
///
/// Qo'llab-quvvatlanadi: Polygon, MultiPolygon, LineString, MultiLineString.
/// Poligonning TEShIKLARI ko'rsatilmaydi (muharrir teshik yaratmaydi).
EditorShape? parseGeometry(Object? json) {
  if (json is! Map) return null;
  final type = json['type'];
  final coords = json['coordinates'];
  var budget = maxVertices;

  List<LatLng>? line(Object? raw, {bool ring = false}) {
    if (raw is! List) return null;
    final out = <LatLng>[];
    for (final c in raw) {
      if (c is! List || c.length < 2 || c[0] is! num || c[1] is! num) return null;
      final lng = (c[0] as num).toDouble();
      final lat = (c[1] as num).toDouble();
      if (!lat.isFinite || !lng.isFinite || lat.abs() > 90 || lng.abs() > 180) return null;
      if (--budget < 0) return null;
      out.add(LatLng(lat, lng));
    }
    // GeoJSON halqasi birinchi nuqtani OXIRIDA takrorlaydi — ichkarida u kerak emas.
    if (ring && out.length > 1 && out.first == out.last) out.removeLast();
    return out;
  }

  switch (type) {
    case 'Polygon':
      if (coords is! List || coords.isEmpty) return null;
      final outer = line(coords.first, ring: true);
      if (outer == null || outer.length < 3) return null;
      return EditorShape(GeomKind.polygon, [outer]);
    case 'MultiPolygon':
      if (coords is! List) return null;
      final parts = <List<LatLng>>[];
      for (final poly in coords) {
        if (poly is! List || poly.isEmpty) return null;
        final outer = line(poly.first, ring: true);
        if (outer == null || outer.length < 3) return null;
        parts.add(outer);
      }
      return parts.isEmpty ? null : EditorShape(GeomKind.polygon, parts);
    case 'LineString':
      final l = line(coords);
      if (l == null || l.length < 2) return null;
      return EditorShape(GeomKind.line, [l]);
    case 'MultiLineString':
      if (coords is! List) return null;
      final parts = <List<LatLng>>[];
      for (final raw in coords) {
        final l = line(raw);
        if (l == null || l.length < 2) return null;
        parts.add(l);
      }
      return parts.isEmpty ? null : EditorShape(GeomKind.line, parts);
  }
  return null;
}

/// Chizilgan nuqtalardan GeoJSON. Poligon halqasi YOPILADI (birinchi nuqta
/// oxirida takrorlanadi) va koordinatalar `[lng, lat]` tartibida.
Map<String, Object> toGeoJson(GeomKind kind, List<LatLng> pts) {
  List<double> c(LatLng p) => [_round6(p.longitude), _round6(p.latitude)];
  if (kind == GeomKind.polygon) {
    final ring = [for (final p in pts) c(p)];
    ring.add(c(pts.first));
    return {'type': 'Polygon', 'coordinates': [ring]};
  }
  return {'type': 'LineString', 'coordinates': [for (final p in pts) c(p)]};
}

double _round6(double v) => (v * 1e6).roundToDouble() / 1e6;

/// Chizilgan shakl yaroqlimi. `null` — yaroqli; aks holda foydalanuvchiga
/// ko'rsatiladigan sabab.
///
/// Nega mijozda tekshiriladi: yaroqsiz (o'zini kesib o'tuvchi) poligon
/// PostGIS'ga tushib, keyin maydon/kesishma hisoblarini jimgina buzadi.
String? drawingProblem(GeomKind kind, List<LatLng> pts) {
  if (kind == GeomKind.line) {
    if (pts.length < 2) return 'Chiziq uchun kamida 2 nuqta kerak';
    for (var i = 1; i < pts.length; i++) {
      if (pts[i] == pts[i - 1]) return 'Ketma-ket bir xil nuqta bor';
    }
    return null;
  }
  if (pts.length < 3) return 'Chegara uchun kamida 3 nuqta kerak';
  for (var i = 0; i < pts.length; i++) {
    if (pts[i] == pts[(i + 1) % pts.length]) return 'Ketma-ket bir xil nuqta bor';
  }
  // Avval kesishish: «galstuk» shaklining ishorali maydoni nolga yaqin bo'lishi
  // mumkin va u "bir chiziqda" deb NOTO'G'RI sabab bilan rad etilardi.
  if (_selfIntersects(pts)) return 'Chegara o\'zini kesib o\'tmasligi kerak';
  if (_area(pts).abs() < 1e-12) return 'Nuqtalar bir chiziqda — maydon hosil bo\'lmadi';
  return null;
}

bool _selfIntersects(List<LatLng> ring) {
  final n = ring.length;
  for (var i = 0; i < n; i++) {
    final a1 = ring[i], a2 = ring[(i + 1) % n];
    for (var j = i + 1; j < n; j++) {
      // Qo'shni tomonlar umumiy uchga ega — ular kesishmaydi.
      if (j == i + 1 || (i == 0 && j == n - 1)) continue;
      final b1 = ring[j], b2 = ring[(j + 1) % n];
      if (_segmentsIntersect(a1, a2, b1, b2)) return true;
    }
  }
  return false;
}

double _cross(LatLng o, LatLng a, LatLng b) =>
    (a.longitude - o.longitude) * (b.latitude - o.latitude) -
    (a.latitude - o.latitude) * (b.longitude - o.longitude);

bool _onSegment(LatLng a, LatLng b, LatLng p) =>
    p.longitude >= math.min(a.longitude, b.longitude) &&
    p.longitude <= math.max(a.longitude, b.longitude) &&
    p.latitude >= math.min(a.latitude, b.latitude) &&
    p.latitude <= math.max(a.latitude, b.latitude);

bool _segmentsIntersect(LatLng p1, LatLng p2, LatLng p3, LatLng p4) {
  final d1 = _cross(p3, p4, p1);
  final d2 = _cross(p3, p4, p2);
  final d3 = _cross(p1, p2, p3);
  final d4 = _cross(p1, p2, p4);
  if (((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
      ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))) {
    return true;
  }
  // Chiziq ustida yotish (kollinear) holatlari.
  if (d1 == 0 && _onSegment(p3, p4, p1)) return true;
  if (d2 == 0 && _onSegment(p3, p4, p2)) return true;
  if (d3 == 0 && _onSegment(p1, p2, p3)) return true;
  if (d4 == 0 && _onSegment(p1, p2, p4)) return true;
  return false;
}

/// Ishorali maydon (koordinata birligida; faqat tartib/nol tekshiruvi uchun).
double _area(List<LatLng> r) {
  var s = 0.0;
  for (var i = 0; i < r.length; i++) {
    final a = r[i], b = r[(i + 1) % r.length];
    s += a.longitude * b.latitude - b.longitude * a.latitude;
  }
  return s / 2;
}

LatLng _centroid(List<LatLng> r) {
  final a = _area(r);
  if (a.abs() < 1e-14) {
    return LatLng(
      r.map((p) => p.latitude).reduce((x, y) => x + y) / r.length,
      r.map((p) => p.longitude).reduce((x, y) => x + y) / r.length,
    );
  }
  // Boshlang'ich nuqtaga NISBATAN hisoblanadi: koordinatalar ~71°/41° bo'lganda
  // ayirmalar katta sonlarni yo'qotib aniqlikni buzardi.
  final o = r.first;
  var cx = 0.0, cy = 0.0;
  for (var i = 0; i < r.length; i++) {
    final px = r[i].longitude - o.longitude, py = r[i].latitude - o.latitude;
    final qx = r[(i + 1) % r.length].longitude - o.longitude;
    final qy = r[(i + 1) % r.length].latitude - o.latitude;
    final f = px * qy - qx * py;
    cx += (px + qx) * f;
    cy += (py + qy) * f;
  }
  final area = a; // ishorali maydon: siljish uni o'zgartirmaydi
  return LatLng(o.latitude + cy / (6 * area), o.longitude + cx / (6 * area));
}

// ── Masofa va bosishni aniqlash ─────────────────────────────────────────

/// Ikki nuqta orasidagi masofa (metr; kichik masofalar uchun tekis yaqinlashuv).
double _meters(LatLng a, LatLng b) {
  final dx = (b.longitude - a.longitude) * math.cos(a.latitude * math.pi / 180) * 111320;
  final dy = (b.latitude - a.latitude) * 110540;
  return math.sqrt(dx * dx + dy * dy);
}

double lineLengthMeters(List<LatLng> l) {
  var s = 0.0;
  for (var i = 1; i < l.length; i++) {
    s += _meters(l[i - 1], l[i]);
  }
  return s;
}

LatLng _pointAtHalf(List<LatLng> l) {
  final half = lineLengthMeters(l) / 2;
  var run = 0.0;
  for (var i = 1; i < l.length; i++) {
    final seg = _meters(l[i - 1], l[i]);
    if (run + seg >= half && seg > 0) {
      final t = (half - run) / seg;
      return LatLng(
        l[i - 1].latitude + (l[i].latitude - l[i - 1].latitude) * t,
        l[i - 1].longitude + (l[i].longitude - l[i - 1].longitude) * t,
      );
    }
    run += seg;
  }
  return l.last;
}

/// Bitta ekran pikseli necha metr (Web Mercator, 256 px tile).
double metersPerPixel(double lat, double zoom) =>
    156543.03392 * math.cos(lat * math.pi / 180) / math.pow(2, zoom);

/// Nuqta halqa ichidami (nur tashlash usuli).
bool pointInRing(LatLng p, List<LatLng> ring) {
  var inside = false;
  for (var i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    final a = ring[i], b = ring[j];
    final crosses = (a.latitude > p.latitude) != (b.latitude > p.latitude) &&
        p.longitude <
            (b.longitude - a.longitude) * (p.latitude - a.latitude) / (b.latitude - a.latitude) +
                a.longitude;
    if (crosses) inside = !inside;
  }
  return inside;
}

/// Nuqtadan kesmagacha masofa (metr).
double distanceToSegmentMeters(LatLng p, LatLng a, LatLng b) {
  final kx = math.cos(p.latitude * math.pi / 180) * 111320;
  const ky = 110540.0;
  const px = 0.0, py = 0.0;
  final ax = (a.longitude - p.longitude) * kx, ay = (a.latitude - p.latitude) * ky;
  final bx = (b.longitude - p.longitude) * kx, by = (b.latitude - p.latitude) * ky;
  final dx = bx - ax, dy = by - ay;
  final len2 = dx * dx + dy * dy;
  var t = len2 == 0 ? 0.0 : ((px - ax) * dx + (py - ay) * dy) / len2;
  t = t.clamp(0.0, 1.0);
  final cx = ax + dx * t, cy = ay + dy * t;
  return math.sqrt(cx * cx + cy * cy);
}

/// Bosilgan nuqta shaklga tegadimi. [toleranceMeters] — chiziq uchun ruxsat
/// etilgan masofa (odatda ~10 piksel; qarang `metersPerPixel`).
bool shapeHit(EditorShape s, LatLng p, double toleranceMeters) {
  if (s.kind == GeomKind.polygon) {
    return s.parts.any((r) => pointInRing(p, r));
  }
  for (final l in s.parts) {
    for (var i = 1; i < l.length; i++) {
      if (distanceToSegmentMeters(p, l[i - 1], l[i]) <= toleranceMeters) return true;
    }
  }
  return false;
}
