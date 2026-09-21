import 'package:chust_admin/ondexmap/editor_geometry.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:latlong2/latlong.dart';

// Muharrir geometriyasi: GeoJSON ↔ nuqtalar, chizishni tekshirish, bosishni aniqlash.

const _sq = [
  LatLng(41.00, 71.20),
  LatLng(41.00, 71.21),
  LatLng(41.01, 71.21),
  LatLng(41.01, 71.20),
];

void main() {
  group('toGeoJson', () {
    test('poligon halqasi YOPILADI, tartib [lng, lat]', () {
      final g = toGeoJson(GeomKind.polygon, _sq);
      expect(g['type'], 'Polygon');
      final ring = ((g['coordinates'] as List).first as List);
      expect(ring, hasLength(5));
      expect(ring.first, [71.2, 41.0], reason: 'lng birinchi');
      expect(ring.first, ring.last);
    });

    test('chiziq yopilmaydi', () {
      final g = toGeoJson(GeomKind.line, _sq.take(3).toList());
      expect(g['type'], 'LineString');
      expect(g['coordinates'], hasLength(3));
    });

    test('koordinata 6 xonagacha yaxlitlanadi', () {
      final g = toGeoJson(GeomKind.line, const [LatLng(41.123456789, 71.987654321), LatLng(41.2, 71.3)]);
      expect((g['coordinates'] as List).first, [71.987654, 41.123457]);
    });
  });

  group('parseGeometry', () {
    test('Polygon: yopuvchi takror nuqta olib tashlanadi', () {
      final s = parseGeometry(toGeoJson(GeomKind.polygon, _sq))!;
      expect(s.kind, GeomKind.polygon);
      expect(s.parts.single, hasLength(4));
    });

    test('MultiPolygon va MultiLineString (server shunday qaytaradi)', () {
      final mp = parseGeometry({
        'type': 'MultiPolygon',
        'coordinates': [
          [
            [[71.2, 41.0], [71.21, 41.0], [71.21, 41.01], [71.2, 41.0]],
          ],
        ],
      })!;
      expect(mp.kind, GeomKind.polygon);
      expect(mp.parts.single, hasLength(3));

      final ml = parseGeometry({
        'type': 'MultiLineString',
        'coordinates': [
          [[71.2, 41.0], [71.21, 41.0]],
          [[71.3, 41.1], [71.31, 41.1], [71.32, 41.12]],
        ],
      })!;
      expect(ml.kind, GeomKind.line);
      expect(ml.parts, hasLength(2));
    });

    test('yaroqsiz kirishlar null (yiqilmaydi)', () {
      for (final bad in <Object?>[
        null,
        'matn',
        42,
        {},
        {'type': 'Point', 'coordinates': [71, 41]},
        {'type': 'Polygon', 'coordinates': []},
        {'type': 'Polygon', 'coordinates': [[[71.2, 41.0], [71.21, 41.0]]]},
        {'type': 'LineString', 'coordinates': [[71.2, 41.0]]},
        {'type': 'LineString', 'coordinates': [[71.2, 'x'], [71.3, 41.0]]},
        {'type': 'LineString', 'coordinates': [[71.2, 95.0], [71.3, 41.0]]},
        {'type': 'LineString', 'coordinates': [[double.nan, 41.0], [71.3, 41.0]]},
        {'type': 'LineString', 'coordinates': 'x'},
      ]) {
        expect(parseGeometry(bad), isNull, reason: '$bad');
      }
    });

    test('nuqtalar soni chegaralanadi (ekran qotmasin)', () {
      final huge = {
        'type': 'LineString',
        'coordinates': [for (var i = 0; i < maxVertices + 5; i++) [71.0 + i * 1e-7, 41.0]],
      };
      expect(parseGeometry(huge), isNull);
    });
  });

  group('drawingProblem', () {
    test('yaroqli poligon va chiziq', () {
      expect(drawingProblem(GeomKind.polygon, _sq), isNull);
      expect(drawingProblem(GeomKind.line, _sq.take(2).toList()), isNull);
    });

    test('kam nuqta', () {
      expect(drawingProblem(GeomKind.polygon, _sq.take(2).toList()), contains('3 nuqta'));
      expect(drawingProblem(GeomKind.line, _sq.take(1).toList()), contains('2 nuqta'));
    });

    test('o\'zini kesib o\'tuvchi poligon (galstuk) rad etiladi', () {
      final bow = [
        const LatLng(41.00, 71.20),
        const LatLng(41.01, 71.21),
        const LatLng(41.00, 71.21),
        const LatLng(41.01, 71.20),
      ];
      expect(drawingProblem(GeomKind.polygon, bow), contains('kesib'));
    });

    test('qo\'shni tomonlar kesishgan deb hisoblanmaydi; qavariq bo\'lmagan poligon yaroqli', () {
      final l = [
        const LatLng(41.00, 71.20),
        const LatLng(41.00, 71.24),
        const LatLng(41.02, 71.24),
        const LatLng(41.02, 71.22),
        const LatLng(41.01, 71.22),
        const LatLng(41.01, 71.20),
      ];
      expect(drawingProblem(GeomKind.polygon, l), isNull);
    });

    test('bir chiziqdagi nuqtalar va takror nuqta rad etiladi', () {
      final flat = [const LatLng(41, 71.2), const LatLng(41, 71.21), const LatLng(41, 71.22)];
      expect(drawingProblem(GeomKind.polygon, flat), isNotNull);
      final dup = [_sq[0], _sq[0], _sq[1], _sq[2]];
      expect(drawingProblem(GeomKind.polygon, dup), contains('bir xil'));
      expect(drawingProblem(GeomKind.line, [_sq[0], _sq[0]]), contains('bir xil'));
    });
  });

  group('bosishni aniqlash', () {
    final poly = parseGeometry(toGeoJson(GeomKind.polygon, _sq))!;
    final line = parseGeometry(toGeoJson(GeomKind.line, [_sq[0], _sq[1]]))!;

    test('poligon ichi / tashqarisi', () {
      expect(shapeHit(poly, const LatLng(41.005, 71.205), 0), isTrue);
      expect(shapeHit(poly, const LatLng(41.02, 71.205), 0), isFalse);
      expect(shapeHit(poly, const LatLng(41.005, 71.25), 0), isFalse);
    });

    test('chiziq: ruxsat etilgan masofa ichida', () {
      // Chiziq lat=41.00 bo'ylab; 0.0001° ≈ 11 m.
      expect(shapeHit(line, const LatLng(41.00005, 71.205), 10), isTrue);
      expect(shapeHit(line, const LatLng(41.0005, 71.205), 10), isFalse);
      // Kesma tashqarisi (uchidan uzoq).
      expect(shapeHit(line, const LatLng(41.0, 71.30), 10), isFalse);
    });

    test('metersPerPixel: zoom oshsa kamayadi; 16-zoomda ~2.4 m (41° kenglik)', () {
      final z16 = metersPerPixel(41, 16);
      expect(z16, closeTo(1.8, 0.2));
      expect(metersPerPixel(41, 17), closeTo(z16 / 2, 1e-9));
    });
  });

  group('yorliq nuqtasi', () {
    test('poligon markazi ichkarida', () {
      final s = parseGeometry(toGeoJson(GeomKind.polygon, _sq))!;
      final c = s.labelPoint;
      expect(c.latitude, closeTo(41.005, 1e-9));
      expect(c.longitude, closeTo(71.205, 1e-9));
    });

    test('chiziq: o\'rta nuqta', () {
      final s = parseGeometry(toGeoJson(GeomKind.line, [_sq[0], _sq[1]]))!;
      expect(s.labelPoint.longitude, closeTo(71.205, 1e-6));
    });

    test('chegaralar', () {
      final b = parseGeometry(toGeoJson(GeomKind.polygon, _sq))!.bounds;
      expect(b, [41.0, 71.2, 41.01, 71.21]);
    });
  });
}
