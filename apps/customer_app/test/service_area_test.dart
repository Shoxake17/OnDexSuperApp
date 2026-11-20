import 'package:flutter_test/flutter_test.dart';
import 'package:chust_customer/data/service_area.dart';

// bug.md 90: mobil ilova xizmat hududini server/web bilan bir xil hisoblaydi.
void main() {
  test('Chust va Toshkent markazi hududda', () {
    expect(cityFor(41.0004, 71.2394)?.name, 'Chust');
    expect(cityFor(41.3111, 69.2797)?.name, 'Toshkent');
  });
  test('hududdan tashqari — null', () {
    expect(cityFor(39.65, 66.96), isNull); // Samarqand
    expect(cityFor(41.0004, 71.40), isNull); // Chustdan ~13 km
  });
  test('masofa haversine bilan', () {
    expect(distanceKm(41, 71, 41, 71), 0);
    expect(distanceKm(41.0004, 71.2394, 41.3111, 69.2797), closeTo(167, 3));
  });
}