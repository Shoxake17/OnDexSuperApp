import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('MapConfig.fromJson', () {
    test('to\'liq javob', () {
      final c = MapConfig.fromJson({
        'provider': 'ondexmap',
        'google_available': false,
        'ondexmap': {
          'api_base': 'https://maps.example.uz',
          'style_url': 'https://maps.example.uz/tiles/style.json',
        },
      });
      expect(c.provider, MapProvider.ondexmap);
      expect(c.googleAvailable, isFalse);
      expect(c.ondexApiBase, 'https://maps.example.uz');
      expect(c.ondexStyleUrl, 'https://maps.example.uz/tiles/style.json');
    });

    test('eski server (provider yo\'q) — Google, kalit bor', () {
      final c = MapConfig.fromJson({'maps_api_key': 'k'});
      expect(c.provider, MapProvider.google);
      expect(c.googleAvailable, isTrue);
      expect(c.ondexApiBase, MapConfig.defaults.ondexApiBase);
    });

    test('buzuq javob hech qachon xato otmaydi va Google beradi', () {
      for (final bad in [
        null,
        5,
        'x',
        <int>[],
        {'provider': 7},
      ]) {
        expect(
          MapConfig.fromJson(bad).provider,
          MapProvider.google,
          reason: '$bad',
        );
      }
    });

    test('https bo\'lmagan yoki noto\'g\'ri manzil qabul qilinmaydi', () {
      final c = MapConfig.fromJson({
        'provider': 'ondexmap',
        'ondexmap': {
          'api_base': 'http://evil.example',
          'style_url': 'javascript:alert(1)',
        },
      });
      expect(c.ondexApiBase, MapConfig.defaults.ondexApiBase);
      expect(c.ondexStyleUrl, MapConfig.defaults.ondexStyleUrl);
    });
  });

  group('MapConfigService', () {
    late DateTime now;
    late MapConfigService svc;
    var probeOk = true;
    final probed = <String>[];
    Future<bool> probe(String url) async {
      probed.add(url);
      return probeOk;
    }

    setUp(() {
      SharedPreferences.setMockInitialValues({});
      now = DateTime(2026, 10, 2, 12);
      probeOk = true;
      probed.clear();
      svc = MapConfigService(clock: () => now, probe: probe);
    });

    Map<String, dynamic> ondexJson() => {
      'provider': 'ondexmap',
      'google_available': true,
      'ondexmap': {
        'api_base': 'https://maps.ondex.uz',
        'style_url': 'https://maps.ondex.uz/tiles/style.json',
      },
    };

    test('hech narsa ma\'lum emas — Google', () {
      expect(svc.useOndexMap, isFalse);
      expect(svc.config.provider, MapProvider.google);
    });

    test(
      'refresh serverdan oladi va saqlaydi; yangi nusxa keshdan o\'qiydi',
      () async {
        await svc.refresh((m, p, [b]) async => ondexJson());
        expect(svc.useOndexMap, isTrue);

        final other = MapConfigService(clock: () => now, probe: probe);
        expect(other.useOndexMap, isFalse, reason: 'init() dan oldin');
        await other.init();
        expect(
          other.useOndexMap,
          isTrue,
          reason: 'server javob bermasa ham oxirgi qiymat qoladi',
        );
      },
    );

    test('server xatosi oxirgi qiymatni buzmaydi', () async {
      await svc.refresh((m, p, [b]) async => ondexJson());
      await svc.refresh((m, p, [b]) async => throw Exception('503'));
      expect(svc.useOndexMap, isTrue);
    });

    test(
      'OnDexMap xatosi — cooldown davomida Google, keyin qayta urinish',
      () async {
        await svc.refresh((m, p, [b]) async => ondexJson());
        svc.reportOndexFailure('address', 'uslub yuklanmadi');
        expect(svc.useOndexMap, isFalse);

        now = now.add(
          MapConfigService.failureCooldown - const Duration(seconds: 1),
        );
        expect(svc.useOndexMap, isFalse);
        now = now.add(const Duration(seconds: 2));
        expect(
          svc.useOndexMap,
          isTrue,
          reason: 'cooldown tugadi — qayta urinamiz',
        );
      },
    );

    test('reportOndexRecovered cooldownni darhol tugatadi', () async {
      await svc.refresh((m, p, [b]) async => ondexJson());
      svc.reportOndexFailure('tracking', 'x');
      svc.reportOndexRecovered();
      expect(svc.useOndexMap, isTrue);
    });

    test('xato serverga xabar qilinadi, xabar yiqilsa ham jim', () async {
      final calls = <String>[];
      await svc.refresh((m, p, [b]) async {
        if (m == 'GET') return ondexJson();
        calls.add('$m $p ${b?['surface']}');
        throw Exception('tarmoq yo\'q');
      });
      svc.reportOndexFailure('courier', 'timeout');
      await Future<void>.delayed(Duration.zero);
      expect(calls, ['POST /config/maps/fallback courier']);
    });

    test(
      'OnDexMap uslubi javob bermasa — xarita ochilmasdan Google, serverga xabar',
      () async {
        probeOk = false;
        final posts = <String>[];
        await svc.refresh((m, p, [b]) async {
          if (m == 'GET') return ondexJson();
          posts.add('${b?['surface']}');
          return null;
        });
        expect(probed, ['https://maps.ondex.uz/tiles/style.json']);
        expect(svc.config.provider, MapProvider.ondexmap);
        expect(svc.useOndexMap, isFalse, reason: 'probe yiqildi → Google');
        await Future<void>.delayed(Duration.zero);
        expect(posts, ['probe']);
      },
    );

    test('probe qayta muvaffaqiyatli bo\'lsa OnDexMap qaytadi', () async {
      probeOk = false;
      await svc.refresh((m, p, [b]) async => ondexJson());
      expect(svc.useOndexMap, isFalse);
      probeOk = true;
      await svc.refresh((m, p, [b]) async => ondexJson());
      expect(svc.useOndexMap, isTrue);
    });

    test('Google rejimida probe umuman yuborilmaydi', () async {
      await svc.refresh((m, p, [b]) async => {'provider': 'google'});
      expect(probed, isEmpty);
    });

    test(
      'provider google bo\'lsa xato ham hech narsani o\'zgartirmaydi',
      () async {
        await svc.refresh((m, p, [b]) async => {'provider': 'google'});
        svc.reportOndexFailure('address', 'x');
        expect(svc.useOndexMap, isFalse);
      },
    );
  });
}
