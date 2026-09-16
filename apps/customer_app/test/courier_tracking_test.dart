import 'package:chust_customer/delivery_tracking.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;

void main() {
  group('parseGeoPoint', () {
    test('to\'g\'ri koordinata', () {
      final p = parseGeoPoint(41.0012, 71.2345);
      expect(p?.latitude, 41.0012);
      expect(p?.longitude, 71.2345);
    });

    test('noto\'g\'ri qiymatlar xaritaga tushmaydi', () {
      for (final (lat, lng) in <(Object?, Object?)>[
        (null, 71.2),
        ('41', 71.2),
        (double.nan, 71.2),
        (41.0, double.infinity),
        (91.0, 71.2),
        (41.0, 181.0),
        (0, 0),
      ]) {
        expect(parseGeoPoint(lat, lng), isNull, reason: '$lat,$lng');
      }
    });
  });

  test('jonli hodisa faqat SHU buyurtmaniki bo\'lsa qabul qilinadi', () {
    final e = {'type': 'courier_location', 'order_id': 'o1', 'lat': 41.0, 'lng': 71.2};
    expect(courierPointFromEvent(e, 'o1'), isNotNull);
    expect(courierPointFromEvent(e, 'o2'), isNull);
    expect(courierPointFromEvent({...e, 'type': 'order_status'}, 'o1'), isNull);
  });

  test('courierPointFromOrder', () {
    expect(courierPointFromOrder({'courier_location': {'lat': 41.0, 'lng': 71.2}}), isNotNull);
    expect(courierPointFromOrder({}), isNull);
    expect(courierPointFromOrder({'courier_location': 'x'}), isNull);
  });

  Map<String, dynamic> order(String status, {String type = 'delivery', String courier = 'k'}) =>
      {'status': status, 'type': type, 'courier_id': courier};

  test('courierTrackable — faqat yetkazma davomida', () {
    for (final s in ['accepted', 'preparing', 'ready', 'picked_up']) {
      expect(courierTrackable(order(s)), isTrue, reason: s);
    }
    expect(courierTrackable(order('delivered')), isFalse);
    expect(courierTrackable(order('cancelled')), isFalse);
    expect(courierTrackable(order('preparing', courier: '')), isFalse);
    expect(courierTrackable(order('ready', type: 'dine_in')), isFalse);
  });

  test('deliveryMapVisible — yetkazilgandan keyin ham xarita qoladi', () {
    expect(deliveryMapVisible(order('picked_up')), isTrue);
    expect(deliveryMapVisible(order('delivered')), isTrue);
    expect(deliveryMapVisible(order('delivered', courier: '')), isFalse);
    expect(deliveryMapVisible(order('cancelled')), isFalse);
    expect(deliveryMapVisible(order('served', type: 'dine_in')), isFalse);
  });

  group('DeliveryTracking.fromJson', () {
    final computed = DateTime.utc(2026, 9, 15, 12, 0, 0);
    final json = <String, dynamic>{
      'phase': 'live',
      'origin': {'lat': 41.0, 'lng': 71.23},
      'destination': {'lat': 41.02, 'lng': 71.25},
      'courier': {'lat': 41.005, 'lng': 71.235},
      'planned_route': {
        'points': [
          {'lat': 41.0, 'lng': 71.23},
          {'lat': 41.01, 'lng': 71.24},
          {'lat': 41.02, 'lng': 71.25},
        ],
        'distance_meters': 2400,
        'duration_seconds': 540,
      },
      'remaining_route': {
        'points': [
          {'lat': 41.005, 'lng': 71.235},
          {'lat': 'buzuq', 'lng': 71.24},
          {'lat': 41.02, 'lng': 71.25},
        ],
        'distance_meters': 1800,
        'duration_seconds': 420,
      },
      'eta_seconds': 420,
      'computed_at': computed.toIso8601String(),
    };

    test('to\'liq javob', () {
      final t = DeliveryTracking.fromJson(json);
      expect(t.phase, TrackingPhase.live);
      expect(t.origin, isNotNull);
      expect(t.destination, isNotNull);
      expect(t.courier, isNotNull);
      expect(t.plannedRoute?.points.length, 3);
      // Buzuq nuqta tashlanadi, qolgani chiziladi.
      expect(t.remainingRoute?.points.length, 2);
      expect(t.remainingRoute?.distanceMeters, 1800);
      expect(t.etaSeconds, 420);
    });

    test('qolgan vaqt o\'tgan vaqtni ayiradi va manfiy bo\'lmaydi', () {
      final t = DeliveryTracking.fromJson(json);
      expect(t.remainingSeconds(computed.add(const Duration(seconds: 120))), 300);
      expect(t.remainingSeconds(computed.add(const Duration(hours: 1))), 0);
      expect(DeliveryTracking.fromJson({'phase': 'live'}).remainingSeconds(computed), isNull);
    });

    test('noma\'lum/buzuq javob xavfsiz', () {
      final t = DeliveryTracking.fromJson({
        'phase': 'hack',
        'planned_route': {'points': 'x'},
        'remaining_route': {'points': [{'lat': 41.0, 'lng': 71.2}]}, // 1 nuqta — chiziq emas
        'eta_seconds': -5,
      });
      expect(t.phase, TrackingPhase.none);
      expect(t.plannedRoute, isNull);
      expect(t.remainingRoute, isNull);
      expect(t.etaSeconds, isNull);
    });

    test('juda ko\'p nuqta chegaralanadi', () {
      final pts = List.generate(TrackedRoute.maxPoints + 500, (i) => {'lat': 41.0 + i * 1e-5, 'lng': 71.2});
      final r = TrackedRoute.fromJson({'points': pts, 'distance_meters': 1, 'duration_seconds': 1});
      expect(r?.points.length, TrackedRoute.maxPoints);
    });
  });

  test('keepStaticFrom — A→B yo\'li va restoran logosi oldingi javobdan qoladi', () {
    final full = DeliveryTracking.fromJson({
      'phase': 'live',
      'restaurant_name': 'Book Cafe',
      'restaurant_logo_url': ' /uploads/book-cafe.png ',
      'planned_route': {
        'points': [
          {'lat': 41.0, 'lng': 71.23},
          {'lat': 41.02, 'lng': 71.25},
        ],
        'distance_meters': 2400,
        'duration_seconds': 540,
      },
    });
    expect(full.restaurantLogoUrl, '/uploads/book-cafe.png');
    final lite = DeliveryTracking.fromJson({
      'phase': 'live',
      'origin': {'lat': 41.0, 'lng': 71.23},
      'eta_seconds': 300,
    });
    expect(lite.plannedRoute, isNull);
    expect(lite.restaurantLogoUrl, isNull);
    final merged = lite.keepStaticFrom(full);
    expect(merged.plannedRoute, same(full.plannedRoute));
    expect(merged.restaurantLogoUrl, '/uploads/book-cafe.png');
    expect(merged.restaurantName, 'Book Cafe');
    expect(merged.phase, TrackingPhase.live);
    expect(merged.origin, lite.origin);
    expect(merged.etaSeconds, 300);
    expect(identical(lite.keepStaticFrom(null), lite), isTrue);
  });

  test('bearingDegrees — mashina harakat yo\'nalishiga buriladi', () {
    const a = LatLng(41.0, 71.23);
    expect(bearingDegrees(a, const LatLng(41.01, 71.23)), closeTo(0, 0.5)); // shimol
    expect(bearingDegrees(a, const LatLng(41.0, 71.24)), closeTo(90, 0.5)); // sharq
    expect(bearingDegrees(a, const LatLng(40.99, 71.23)), closeTo(180, 0.5)); // janub
    expect(bearingDegrees(a, const LatLng(41.0, 71.22)), closeTo(270, 0.5)); // g'arb
    expect(metersBetween(a, const LatLng(41.001, 71.23)), closeTo(111, 2));
  });

  test('matnlar', () {
    expect(etaText(30), '1 daqiqadan kam');
    expect(etaText(61), '~2 daqiqa');
    expect(etaText(420), '~7 daqiqa');
    expect(distanceText(850), '850 m');
    expect(distanceText(2400), '2.4 km');
    final a = DateTime(2026, 9, 15, 12, 0);
    expect(deliveryDurationText(a, a.add(const Duration(minutes: 12))), '12 daqiqada');
    expect(deliveryDurationText(a, a.subtract(const Duration(minutes: 1))), isNull);
    expect(deliveryDurationText(null, a), isNull);
  });
}
