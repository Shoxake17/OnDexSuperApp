import 'dart:io';

import 'package:chust_courier/voice_navigation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;

void main() {
  const start = LatLng(41.0, 71.23);
  // Shimolga `m` metr.
  LatLng north(double m) => LatLng(start.latitude + m / 111320, start.longitude);
  final t0 = DateTime(2026, 9, 15, 12);
  DateTime at(int s) => t0.add(Duration(seconds: s));

  final turn = north(1500);
  final dest = north(2500);

  VoiceNavigator nav({NavTarget target = NavTarget.restaurant}) => VoiceNavigator()
    ..setRoute(steps: [NavStep('right', turn)], destination: dest, target: target);

  test('yaqinlashgan sari 1 km, 500 m, 200 m va "hozir" — masofa + manevr bo\'laklari, har biri bir marta', () {
    final n = nav();
    expect(n.update(north(100), at(0)), isNull); // 1400 m
    expect(n.update(north(400), at(10))?.assetKeys, ['dist_km1', 'man_right']); // 1100 m
    expect(n.update(north(500), at(20)), isNull); // hali 1 km bosqichi
    expect(n.update(north(950), at(30))?.assetKeys, ['dist_m500', 'man_right']);
    expect(n.update(north(1250), at(40))?.assetKeys, ['dist_m200', 'man_right']);
    expect(n.update(north(1450), at(50))?.assetKeys, ['dist_now', 'man_right']);
    expect(n.update(north(1480), at(55)), isNull); // bajarildi
  });

  test('yaqindan boshlangan yo\'lda eng yaqin bosqich, uzoqlari aytilmaydi', () {
    final n = nav();
    expect(n.update(north(1300), at(0))?.assetKeys, ['dist_m200', 'man_right']);
    expect(n.update(north(1310), at(10)), isNull);
  });

  test('iboralar orasi kamida 4 soniya — keyinroq aytiladi, yo\'qolmaydi', () {
    final n = VoiceNavigator()
      ..setRoute(
        steps: [NavStep('left', north(200)), NavStep('right', north(260))],
        destination: dest,
        target: NavTarget.customer,
      );
    expect(n.update(north(160), at(0))?.assetKeys, ['dist_now', 'man_left']);
    // Chap burilish bajarildi; o'ng burilishgacha 80 m ("200 m" bosqichi),
    // lekin oldingi iboradan 4 s o'tmagan — aytilmaydi.
    expect(n.update(north(180), at(1)), isNull);
    // 4 s o'tdi, burilishgacha 55 m — eng yaqin bosqich ("hozir") aytiladi.
    expect(n.update(north(205), at(4))?.assetKeys, ['dist_now', 'man_right']);
  });

  test('marshrut yangilansa o\'sha burilish qayta aytilmaydi', () {
    final n = nav();
    expect(n.update(north(400), at(0))?.assetKeys, ['dist_km1', 'man_right']);
    n.setRoute(steps: [NavStep('right', north(1505))], destination: dest, target: NavTarget.restaurant);
    expect(n.update(north(420), at(10)), isNull);
    expect(n.update(north(950), at(20))?.assetKeys, ['dist_m500', 'man_right']);
  });

  test('yetib kelish — bir marta; restoran bo\'lsa restoran nomi uchun belgi', () {
    final n = nav();
    final cue = n.update(north(2480), at(0));
    expect(cue?.assetKeys, [kArrivedRestaurantKey]);
    expect(cue?.arrivedAtRestaurant, isTrue);
    expect(n.update(north(2490), at(10)), isNull);

    // Taom olindi — yangi manzil: yetib kelish yana ishlaydi.
    n.setRoute(steps: const [], destination: north(4000), target: NavTarget.customer);
    final c2 = n.update(north(3990), at(20));
    expect(c2?.assetKeys, [kArrivedCustomerKey]);
    expect(c2?.arrivedAtRestaurant, isFalse);
  });

  test('noma\'lum manevr e\'tiborsiz; marshrut yo\'q — hech narsa', () {
    final n = VoiceNavigator()
      ..setRoute(steps: [NavStep('ferry', north(300))], destination: dest, target: NavTarget.customer);
    expect(n.update(north(250), at(0)), isNull);
    expect(VoiceNavigator().update(north(0), at(0)), isNull);
  });

  test('har bir bo\'lak uchun ovoz fayli ilovaga joylangan', () {
    expect(allVoiceAssetKeys().length, kNavBuckets.length + kNavManeuvers.length + 2);
    final missing = [
      for (final k in allVoiceAssetKeys())
        if (!File('assets/voice/$k.wav').existsSync()) k,
    ];
    expect(missing, isEmpty, reason: 'go run ./cmd/voicegen -delay 25s bilan yasang');
  });
}
