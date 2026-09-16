import 'package:chust_courier/route_throttle.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;

void main() {
  final t0 = DateTime(2026, 9, 15, 12);
  const start = LatLng(41.0, 71.23);
  const dest = LatLng(41.02, 71.25);
  const customer = LatLng(41.05, 71.3);
  // 0.001° kenglik ≈ 111 m.
  LatLng north(double deg) => LatLng(start.latitude + deg, start.longitude);
  Duration s(int seconds) => Duration(seconds: seconds);

  test('metersBetween', () {
    expect(metersBetween(start, north(0.001)), closeTo(111, 2));
    expect(metersBetween(start, start), 0);
  });

  group('RouteThrottle', () {
    late RouteThrottle th;
    setUp(() {
      th = RouteThrottle()
        ..begin(dest, t0)
        ..succeed(start, dest, t0);
    });

    test('birinchi marta so\'raladi', () {
      expect(RouteThrottle().shouldFetch(start, dest, t0), isTrue);
    });

    test('30 s ichida qayta so\'ralmaydi — kuryer uzoqlashsa ham', () {
      expect(th.shouldFetch(north(0.005), dest, t0.add(s(10))), isFalse);
    });

    test('300 m dan ko\'p siljisa so\'raladi, kam siljisa 90 s gacha kutadi', () {
      final t31 = t0.add(s(31));
      expect(th.shouldFetch(north(0.001), dest, t31), isFalse); // ~111 m
      expect(th.shouldFetch(north(0.003), dest, t31), isTrue); // ~333 m
      expect(th.shouldFetch(start, dest, t0.add(s(91))), isTrue);
    });

    // O'lchovdagi haqiqiy holat: restoran "tayyorlanmoqda"/"tayyor" bosdi,
    // kuryer ilovani qayta ochdi — manzil o'sha, so'rov KERAK EMAS.
    test('holat o\'zgarishi yoki qayta ochish (maqsad o\'sha) — so\'ralmaydi', () {
      for (final sec in [1, 13, 16, 29]) {
        expect(th.shouldFetch(start, dest, t0.add(s(sec))), isFalse, reason: '+$sec s');
      }
    });

    test('maqsad o\'zgarsa (taom olindi) — darhol', () {
      expect(th.destinationChanged(customer), isTrue);
      expect(th.shouldFetch(start, customer, t0.add(s(1))), isTrue);
    });

    test('shu maqsadga so\'rov ketayotganda ikkinchisi yuborilmaydi', () {
      th.begin(customer, t0.add(s(1)));
      // Masalan "taom olindi" javobi va WebSocket hodisasi bir vaqtda keldi.
      expect(th.shouldFetch(start, customer, t0.add(s(2))), isFalse);
      expect(th.shouldFetch(start, customer, t0.add(s(120))), isFalse);
      th.fail();
      expect(th.shouldFetch(start, customer, t0.add(s(120))), isTrue);
    });

    test('abandon — javob e\'tiborsiz qolsa ham maqsad qulflanib qolmaydi', () {
      th.begin(customer, t0.add(s(1)));
      th.abandon();
      expect(th.shouldFetch(start, customer, t0.add(s(40))), isTrue);
    });

    test('xatodan keyin ham 30 s kutiladi', () {
      final failed = RouteThrottle()
        ..begin(dest, t0)
        ..fail();
      expect(failed.shouldFetch(start, dest, t0.add(s(10))), isFalse);
      expect(failed.shouldFetch(start, dest, t0.add(s(31))), isTrue);
    });

    test('reset — keyingi buyurtma darhol so\'raydi', () {
      th.reset();
      expect(th.shouldFetch(start, dest, t0.add(s(1))), isTrue);
    });
  });

  group('trimRouteToPosition', () {
    final line = [for (var i = 0; i <= 10; i++) north(i * 0.001)];

    test('bosib o\'tilgan bosh qirqiladi, kuryer chiziq boshida', () {
      final pos = north(0.0032);
      final got = trimRouteToPosition(line, pos);
      expect(got.first, pos);
      expect(got[1], line[4]);
      expect(got.last, line.last);
      expect(got.length, 1 + 7);
    });

    test('yo\'ldan uzoq bo\'lsa o\'zgarmaydi', () {
      const far = LatLng(41.004, 71.25); // ~1.7 km sharqda
      expect(identical(trimRouteToPosition(line, far), line), isTrue);
    });

    test('oxirgi nuqtada ham manzil qoladi', () {
      final got = trimRouteToPosition(line, line.last);
      expect(got.length, 2);
      expect(got.last, line.last);
    });

    test('qisqa ro\'yxat', () {
      final one = [start];
      expect(identical(trimRouteToPosition(one, start), one), isTrue);
    });
  });
}
