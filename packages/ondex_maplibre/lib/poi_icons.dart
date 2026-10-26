import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show MissingPluginException, rootBundle;
import 'package:lucide_icons_flutter/lucide_icons.dart';
import 'package:maplibre_gl/maplibre_gl.dart' as ml;

/// XARITADAGI JOY BELGILARI (POI).
///
/// ┌─ NEGA KOD BILAN, USLUBDAN EMAS ────────────────────────────────────┐
/// OnDexMap uslubi belgilarni NOM bilan so'raydi:
///
///     "icon-image": ["concat", "ondex-poi-", ["get", "class"]]
///
/// ya'ni `ondex-poi-cafe`, `ondex-poi-hospital` va hokazo. Bu nomlar
/// uslubning sprite faylida YO'Q — sprite umuman bo'sh. Ularni
/// mijozning o'zi chizib, `addImage` bilan beradi.
///
/// Saytda shunday qilinadi (`web/src/components/map/poiIcons.ts`), va
/// sabab: bitta chizuvchi OnDexMap ob'ektlari uchun ham, OSM'dan
/// kelgan joylar uchun ham ishlatiladi — xaritada «bizniki» va
/// «OSM'niki» degan farq qolmaydi.
///
/// ⚠️ Ilovada rasm BERILMASA, belgi ham, uning YOZUVI ham chiqmaydi:
/// uslubda `text-optional: true`, ya'ni matn ikonkaga biriktirilgan.
/// Aynan shu sabab xarita «bo'm-bo'sh» ko'rinardi — na do'kon, na
/// kafe, na shifoxona nomi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA HAMMASI OLDINDAN YUKLANADI ──────────────────────────────────┐
/// MapLibre GL JS da `styleimagemissing` hodisasi bor: rasm kerak
/// bo'lganda so'raladi. MapLibre Native'ning Flutter o'ramida bunday
/// hodisa YO'Q, shuning uchun barcha belgilar uslub yuklangach bir
/// marta qo'shiladi. Ular kichik (52×52 px, ~1 KB), jami ~60 KB.
/// └────────────────────────────────────────────────────────────────────┘

/// Uslub kutayotgan nom prefiksi.
const String poiPrefix = 'ondex-poi-';

/// PNG pin ramkasi (dip) — saytdagi `PIN_SIZE` bilan bir xil.
const double _pinSize = 30;

/// Doira belgisi (dip) — saytdagi `BADGE_SIZE`.
const double _badgeSize = 20;

/// Doira ichidagi belgi o'lchami — saytdagi `BADGE_GLYPH`.
const double _glyphSize = 12;

/// Rasm shuncha marta zich chiziladi (`loadPoiIcons` izohiga qarang).
///
/// Chegara ataylab: dpr juda katta bo'lsa 54 ta belgi xotirada behuda
/// joy egallardi, 4 dan yuqorisi esa amalda uchramaydi.
double get _dpr =>
    (ui.PlatformDispatcher.instance.implicitView?.devicePixelRatio ?? 1.0)
        .clamp(1.0, 4.0);

/// Testlar uchun: bitta doira belgisining PNG baytlari.
///
/// Belgining EKRANDAGI o'lchami faqat shu baytlardan bilinadi —
/// `addImage` ni testda chaqirib bo'lmaydi (platforma kanali).
@visibleForTesting
Future<Uint8List> debugBadgePng(String klass) => _badge(_poi[klass]!);

/// Testlar uchun: doira belgisining dip o'lchami.
@visibleForTesting
const double debugBadgeSize = _badgeSize;

/// Testlar uchun: marshrut nuqtasining (A yoki B) PNG baytlari.
@visibleForTesting
Future<Uint8List> debugRouteDotPng(String slot) =>
    _routeDot(_routeDots[slot]!, slot.toUpperCase());

/// `w`×`h` dip li tuval, `dpr` baravar zich. Chizish dip da bajariladi.
Future<Uint8List> _chiz(
  double w,
  double h,
  void Function(Canvas) chizuvchi,
) async {
  final k = _dpr;
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder)..scale(k);
  chizuvchi(canvas);
  final img = await recorder.endRecording().toImage(
    (w * k).round(),
    (h * k).round(),
  );
  final data = await img.toByteData(format: ui.ImageByteFormat.png);
  return data!.buffer.asUint8List();
}

/// Qaysi klass uchun TAYYOR PNG pin bor.
///
/// ┌─ NEGA FAQAT SHULAR ────────────────────────────────────────────────┐
/// Bu ro'yxat `web/src/components/map/poiIcons.ts` dagi `PNG_ICON`
/// bilan AYNAN bir xil. Saytda ham shu o'n bitta klass tayyor rasm
/// bilan chiziladi, qolgan 43 tasi — rangli doira.
///
/// Rasmlar `web/public/org-icons/` dan ko'chirilgan va 128 px ga
/// kichraytirilgan (xaritada ular 30 dip, ya'ni dpr=3 da 90 px).
/// Nisbatlari saqlangan: `hospital` yotiq, `cafe` tik — kvadratga
/// tenglansa buziladi.
/// └────────────────────────────────────────────────────────────────────┘
const Map<String, String> _pngPin = {
  'restaurant': 'restaurant.png',
  'cafe': 'cafe.png',
  'grocery': 'grocery.png',
  'hospital': 'hospital.png',
  'pharmacy': 'pharmacy.png',
  'doctors': 'doctors.png',
  'dentist': 'dentist.png',
  'lodging': 'hotel.png',
  'school': 'school.png',
  'college': 'college.png',
  'library': 'library.png',
};

@immutable
class _Poi {
  const _Poi(this.color, {this.icon, this.letter, this.stroke});
  final Color color;
  final IconData? icon;
  final String? letter;
  final double? stroke;
}

/// Klass → rang va belgi.
///
/// Ranglar `web/src/components/map/poiIcons.ts` dan AYNAN ko'chirilgan:
/// bitta xarita ikki mahsulotda bir xil ko'rinishi kerak.
const Map<String, _Poi> _poi = {
  // Ovqat va ichimlik
  'restaurant': _Poi(Color(0xFFF2611D), icon: LucideIcons.utensils),
  'fast_food': _Poi(Color(0xFFF59E42), icon: LucideIcons.utensils),
  'cafe': _Poi(Color(0xFFF2611D), icon: LucideIcons.coffee),
  'bar': _Poi(Color(0xFFA855F7), icon: LucideIcons.coffee),
  'beer': _Poi(Color(0xFFA855F7), icon: LucideIcons.beer),
  'alcohol_shop': _Poi(Color(0xFFA855F7), icon: LucideIcons.wine),
  'ice_cream': _Poi(Color(0xFFF59E42), icon: LucideIcons.iceCreamCone),
  'bakery': _Poi(Color(0xFFF97316), icon: LucideIcons.shoppingBag),

  // Savdo
  'grocery': _Poi(Color(0xFFF2611D), icon: LucideIcons.shoppingCart),
  'shop': _Poi(Color(0xFF6FB9EF), icon: LucideIcons.shoppingBag),
  'clothing_store': _Poi(Color(0xFF6FB9EF), icon: LucideIcons.shoppingBag),
  'laundry': _Poi(Color(0xFF38BDF8), icon: LucideIcons.shirt),

  // Tibbiyot
  'pharmacy': _Poi(Color(0xFFE2231F), icon: LucideIcons.cross),
  'hospital': _Poi(Color(0xFFE2231F), icon: LucideIcons.hospital),
  'doctors': _Poi(Color(0xFFE2231F), icon: LucideIcons.cross),
  'dentist': _Poi(Color(0xFFE2231F), icon: LucideIcons.cross),

  // Moliya va idora
  'bank': _Poi(Color(0xFFE8453C), icon: LucideIcons.creditCard),
  'atm': _Poi(Color(0xFFE8453C), icon: LucideIcons.creditCard),
  'office': _Poi(Color(0xFF78716C), icon: LucideIcons.briefcase),

  // Transport
  'fuel': _Poi(Color(0xFF9B8CF5), icon: LucideIcons.fuel),
  'car': _Poi(Color(0xFF64748B), icon: LucideIcons.car),
  'bus': _Poi(Color(0xFF3178E2), icon: LucideIcons.bus),
  'parking': _Poi(Color(0xFF3B82F6), letter: 'P'),
  'railway': _Poi(Color(0xFF0EA5E9), icon: LucideIcons.trainFront),
  'aerialway': _Poi(Color(0xFF0EA5E9), icon: LucideIcons.cableCar),
  'harbor': _Poi(Color(0xFF0EA5E9), icon: LucideIcons.anchor),
  'entrance': _Poi(Color(0xFF94A3B8), icon: LucideIcons.logIn),

  // Ta'lim
  'school': _Poi(Color(0xFFE6A700), icon: LucideIcons.graduationCap),
  'college': _Poi(Color(0xFFE6A700), icon: LucideIcons.graduationCap),
  'library': _Poi(Color(0xFFE6A700), icon: LucideIcons.graduationCap),

  // Turar joy va dam olish
  'lodging': _Poi(Color(0xFF3C434E), icon: LucideIcons.bedDouble),
  'campsite': _Poi(Color(0xFF4CC15F), icon: LucideIcons.tent),

  // Diniy va tarixiy
  'place_of_worship': _Poi(Color(0xFF8B7355), icon: LucideIcons.landmark),
  'castle': _Poi(Color(0xFF8B7355), icon: LucideIcons.castle),
  'cemetery': _Poi(Color(0xFF8B7355), icon: LucideIcons.flower),
  'art_gallery': _Poi(Color(0xFFA78BFA), icon: LucideIcons.palette),

  // Davlat xizmatlari
  'police': _Poi(Color(0xFF3B82F6), icon: LucideIcons.shield),
  'town_hall': _Poi(Color(0xFF3B82F6), icon: LucideIcons.landmark),
  'post': _Poi(Color(0xFF3B82F6), icon: LucideIcons.mail),

  // Tabiat va sport
  'park': _Poi(Color(0xFF4CC15F), icon: LucideIcons.trees),
  'garden': _Poi(Color(0xFF4CC15F), icon: LucideIcons.trees),
  'golf': _Poi(Color(0xFF4CC15F), icon: LucideIcons.flag),
  'zoo': _Poi(Color(0xFF4CC15F), icon: LucideIcons.pawPrint),
  'pitch': _Poi(Color(0xFF4CC15F), icon: LucideIcons.volleyball),
  'stadium': _Poi(Color(0xFF4CC15F), icon: LucideIcons.volleyball),
  'sport': _Poi(Color(0xFF4CC15F), icon: LucideIcons.volleyball),
  'sports_centre': _Poi(Color(0xFF4CC15F), icon: LucideIcons.volleyball),
  'swimming': _Poi(Color(0xFF38BDF8), icon: LucideIcons.volleyball),
  'swimming_pool': _Poi(Color(0xFF38BDF8), icon: LucideIcons.waves),

  // Qolgani
  'toilets': _Poi(Color(0xFF94A3B8), icon: LucideIcons.toilet),
  'attraction': _Poi(Color(0xFFF59E42), icon: LucideIcons.info),
  'information': _Poi(Color(0xFF94A3B8), icon: LucideIcons.info),
  // ⚠️ `other` — uslub noma'lum klass uchun shuni so'raydi
  // (`coalesce(class, "other")`). U bo'lmasa noma'lum turdagi
  // joylarning hammasi yo'qolardi.
  'other': _Poi(Color(0xFF94A3B8), icon: LucideIcons.dot, stroke: 3.4),
};

/// Xaritaning O'Z belgisi — tanlangan joy, marshrutning A va B
/// nuqtalari uchun.
///
/// ⚠️ Uslubda TAYYOR belgi yo'q: `sprite` maydoni bo'sh va
/// `tiles/sprite.json` 404 qaytaradi. Ilgari bu yerda `marker-15`
/// (OpenMapTiles sprite'ining standart nomi) ishlatilardi va u
/// hech qachon chizilmasdi — pinlar jimgina yo'qolardi.
const String markerPrefix = 'ondex-marker-';

/// Marshrut chizig'i — saytdagi `ROUTE_BLUE` (`useMapTools.ts`).
/// Yandex marshruti kabi ko'k, brend to'q sarig'i emas.
const String kOndexRouteLine = '#2f6bff';

/// Marshrut nuqtalarining ranglari — saytdagi `ROUTE_DOT` bilan
/// AYNAN bir xil (`useMapTools.ts`).
const Map<String, Color> _routeDots = {
  'a': Color(0xFFFF4D3A), // marshrut boshlanishi
  'b': Color(0xFF3F3F46), // marshrut tugashi
};

/// Tanlangan joy belgisining nomi (`_logoPin` bilan chiziladi).
const String markerSelected = '${markerPrefix}selected';

/// OnDex pin rasmi — OnDexMap saytidagi `public/ondexmap-pin.png` dan
/// nusxa (96×96 px).
const String _logoPinFile = 'ondex_pin.png';

/// Pin o'lchami (dip) — saytdagi `PIN_SIZE` (`SearchBar.tsx`).
const double _logoPinSize = 46;

/// Barcha belgilarni xaritaga yuklaydi.
///
/// Uslub yuklangandan KEYIN chaqiriladi: `addImage` uslubga yoziladi,
/// va uslub almashsa (sun'iy yo'ldosh ↔ oddiy) qaytadan kerak bo'ladi.
///
/// ┌─ NEGA RASM `dpr` BARAVAR ZICH CHIZILADI ───────────────────────────┐
/// Saytda tuval `PIN_SIZE * BADGE_RATIO` piksel qilib chiziladi va
/// MapLibre'ga `addImage(..., { pixelRatio: 2 })` deb beriladi — ya'ni
/// «bu rasm ikki barobar zich» deyiladi va xaritada u `PIN_SIZE` CSS
/// px bo'lib ko'rinadi.
///
/// Flutter o'ramida `addImage` da `pixelRatio` parametri YO'Q, Android
/// tarafi esa rasmni `inScaled = false`, `inDensity = 0` bilan o'qiydi
/// (`MapLibreMapController.java`: `style#addImage`). Zichlik haqida
/// ma'lumot yo'qolgani uchun rasm pikseli ekran pikseliga 1:1 tushadi.
///
/// Shuning uchun `_pinSize = 30` bilan chizilgan rasm dpr=2.8 li
/// qurilmada atigi ~10.7 dip bo'lib ko'rinardi — ko'rinmas darajada
/// kichik. Rasmni `dpr` baravar zich chizsak, u yana kerakli 30 dip
/// bo'ladi.
///
/// Chizish koordinatalari DIP da qoladi: tuval bir marta
/// `canvas.scale(dpr)` bilan kattalashtiriladi. Ikki joyda (ham
/// o'lchamda, ham tuvalda) ko'paytirish esa rasmni dpr² baravar
/// kattalashtirib yuborardi.
/// └────────────────────────────────────────────────────────────────────┘
Future<void> loadPoiIcons(ml.MapLibreMapController ctrl) async {
  // ┌─ XARITA YO'Q BO'LSA — DARHOL TO'XTAYMIZ ──────────────────────────┐
  // `onStyleLoadedCallback` ba'zan ALLAQACHON yo'q qilingan xarita
  // ko'rinishi uchun ishga tushadi (foydalanuvchi ekranni tezda
  // yopsa yoki uslub almashsa). Nativ tomonda `MethodChannel`
  // ishlovchisi `dispose()` da o'chiriladi, shuning uchun har bir
  // `addImage` `MissingPluginException` beradi.
  //
  // Avval sikl shunday bo'lsa ham oxirigacha ishlardi: 40 dan ortiq
  // rasm bekorga chizilib, logga 40 ta bir xil xato tushardi
  // (telefon logida kuzatilgan, 2026-10-20). Endi birinchi shunday
  // xatoda to'xtaymiz va BITTA qator yozamiz — tirik xarita uslubi
  // yuklanganda belgilar baribir qaytadan qo'yiladi.
  // └───────────────────────────────────────────────────────────────────┘
  var gone = false;

  Future<void> qosh(String nom, Future<Uint8List> Function() chiz) async {
    if (gone) return;
    try {
      await ctrl.addImage(nom, await chiz());
    } on MissingPluginException {
      gone = true;
      debugPrint('[ondexmap] xarita yopilgan — belgilar qo\'yilmadi');
    } catch (e) {
      // Jim qolmaydi: aks holda rasm yo'qligi (masalan asset
      // `pubspec.yaml` ga qo'shilmagani) xaritadagi bo'shliq bo'lib
      // ko'rinardi va sababi topilmasdi.
      debugPrint('[ondexmap] belgi qo\'shilmadi: $nom — $e');
    }
  }

  for (final e in _poi.entries) {
    // Tayyor PNG bo'lsa — u, aks holda rangli doira. Saytdagi
    // tartib ham shunday: `loadPoiPngIcon` → `makePoiIcon`.
    final fayl = _pngPin[e.key];
    await qosh(
      '$poiPrefix${e.key}',
      () => fayl != null ? _pngIcon(fayl) : _badge(e.value),
    );
    if (gone) return;
  }

  for (final e in _routeDots.entries) {
    await qosh(
      '$markerPrefix${e.key}',
      () => _routeDot(e.value, e.key.toUpperCase()),
    );
    if (gone) return;
  }
  await qosh(markerSelected, _logoPin);
}

/// Marshrut nuqtasi — oq halqali rangli doira, ichida «A» yoki «B».
///
/// ┌─ SAYTDAGI SHAKLNING AYNAN O'ZI ────────────────────────────────────┐
/// `web/src/components/map/useMapTools.ts` dagi `LYR.halo` va
/// `LYR.labels` qatlamlari: `circle-radius: 10`,
/// `circle-stroke-width: 2` (oq), ustida `text-size: 11` oq qalin
/// harf. Doira nuqtaning USTIDA markazlashadi — shuning uchun bu
/// belgi `iconAnchor: 'center'` bilan qo'yiladi, tomchi kabi
/// «bottom» bilan emas.
///
/// ⚠️ Ilgari bu yerda QO'LDA chizilgan tomchi bor edi. U saytga
/// umuman o'xshamasdi: saytda A/B — doira, tomchi esa faqat
/// tanlangan joy uchun (u ham OnDex logotipi, qo'lda chizilgan
/// shakl emas — `_logoPin` ga qarang).
/// └────────────────────────────────────────────────────────────────────┘
Future<Uint8List> _routeDot(Color fill, String label) {
  // Tashqi oq halqa doiradan 1 dip chiqadi (10 radius + 2 chiziq,
  // chiziq markazi radiusda — ya'ni tashqarisi 11).
  const r = 10.0;
  const s = (r + 1) * 2;
  const c = Offset(s / 2, s / 2);

  final tp = TextPainter(
    text: TextSpan(
      text: label,
      style: const TextStyle(
        fontSize: 11,
        fontWeight: FontWeight.w700,
        color: Colors.white,
        height: 1,
      ),
    ),
    textDirection: TextDirection.ltr,
  )..layout();

  return _chiz(s, s, (canvas) {
    canvas.drawCircle(c, r + 1, Paint()..color = Colors.white);
    canvas.drawCircle(c, r, Paint()..color = fill);
    tp.paint(canvas, c - Offset(tp.width / 2, tp.height / 2));
  });
}

/// Tanlangan joy — OnDexMap logotipi shaklidagi pin.
///
/// Saytda ham AYNAN shu fayl ishlatiladi (`logoPin.ts` →
/// `/ondexmap-pin.png`, `anchor: "bottom"`). Rasmning o'tkir uchi
/// tuvalning pastki chetiga yaqin, shuning uchun belgi nuqtaga
/// «sanchiladi».
Future<Uint8List> _logoPin() async {
  final data = await rootBundle.load(
    'packages/ondex_maplibre/assets/poi/$_logoPinFile',
  );
  final codec = await ui.instantiateImageCodec(data.buffer.asUint8List());
  final img = (await codec.getNextFrame()).image;

  return _chiz(_logoPinSize, _logoPinSize, (canvas) {
    canvas.drawImageRect(
      img,
      Rect.fromLTWH(0, 0, img.width.toDouble(), img.height.toDouble()),
      const Rect.fromLTWH(0, 0, _logoPinSize, _logoPinSize),
      Paint()..filterQuality = FilterQuality.high,
    );
  });
}

/// Tayyor PNG pinni kerakli o'lchamga keltiradi.
///
/// Rasm o'z NISBATINI saqlaydi va kvadrat ramkaga markazlashtiriladi
/// («contain», CSS'dagi kabi) — saytdagi `loadPngPin` ham shunday
/// qiladi. Tenglab cho'zilsa yotiq rasmlar (shifoxona, dorixona)
/// tikka cho'zilgandek ko'rinardi.
Future<Uint8List> _pngIcon(String fayl) async {
  final data = await rootBundle.load(
    'packages/ondex_maplibre/assets/poi/$fayl',
  );
  const px = _pinSize;

  final codec = await ui.instantiateImageCodec(data.buffer.asUint8List());
  final img = (await codec.getNextFrame()).image;

  final k = math.min(px / img.width, px / img.height);
  final w = img.width * k;
  final h = img.height * k;

  return _chiz(px, px, (canvas) {
    canvas.drawImageRect(
      img,
      Rect.fromLTWH(0, 0, img.width.toDouble(), img.height.toDouble()),
      Rect.fromLTWH((px - w) / 2, (px - h) / 2, w, h),
      Paint()..filterQuality = FilterQuality.high,
    );
  });
}

/// Oq halqali rangli doira, ichida oq belgi yoki harf.
///
/// ┌─ SAYTDAGI `makeBadge` NING AYNAN O'ZI ─────────────────────────────┐
/// Tayyor PNG yo'q klasslar (43 ta) shu bilan chiziladi — saytda ham
/// shunday. Chizish tartibi bir xil: tashqi oq doira (0.5 dip
/// chetdan), ichida 1.25 dip kichikroq rangli doira, markazda oq
/// belgi.
///
/// ⚠️ Bu yerga TOMCHI shakli chizilmaydi. Bir marta urinib ko'rilgan
/// va natija tayyor PNG pinlarga umuman o'xshamagan edi: ularda
/// gradient, aniq hoshiya va pishiq chizilgan belgi bor. Qo'lda
/// chizilgan taqlid ikki xil ko'rinish hosil qilardi, bittasi emas.
/// └────────────────────────────────────────────────────────────────────┘
Future<Uint8List> _badge(_Poi spec) {
  const s = _badgeSize;
  const c = Offset(s / 2, s / 2);

  final TextSpan span;
  if (spec.letter != null) {
    span = TextSpan(
      text: spec.letter,
      style: const TextStyle(
        fontSize: s * 0.66,
        fontWeight: FontWeight.w700,
        color: Colors.white,
        height: 1,
      ),
    );
  } else {
    final icon = spec.icon!;
    span = TextSpan(
      text: String.fromCharCode(icon.codePoint),
      style: TextStyle(
        fontFamily: icon.fontFamily,
        package: icon.fontPackage,
        fontSize: _glyphSize,
        color: Colors.white,
        height: 1,
      ),
    );
  }

  final tp = TextPainter(text: span, textDirection: TextDirection.ltr)
    ..layout();

  return _chiz(s, s, (canvas) {
    canvas.drawCircle(c, s / 2 - 0.5, Paint()..color = Colors.white);
    canvas.drawCircle(c, s / 2 - 1.25, Paint()..color = spec.color);
    tp.paint(canvas, c - Offset(tp.width / 2, tp.height / 2));
  });
}
