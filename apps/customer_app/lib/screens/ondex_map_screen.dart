import 'dart:async';

import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import 'package:maplibre_gl/maplibre_gl.dart' as ml;

import '../map/map_backend.dart';
import '../map/ondex_brand.dart';
import '../map/ondex_style.dart';
import '../map/ondexmap_api.dart';
import '../map/map_controls.dart';
import '../map/map_sheet.dart';
import '../map/poi_icons.dart';
import '../map/route_sheet.dart';
import 'add_place_screen.dart';
import 'catalog_screen.dart' show kBrand;

/// ONDEX XARITA — super-ilovaning xarita bo'limi.
///
/// ┌─ NIMA BERADI ──────────────────────────────────────────────────────┐
/// Yandex/Google Maps mobil ilovalaridagi asosiy oqim:
///
///   1. to'liq ekran xarita (OnDexMap, sun'iy yo'ldosh rejimi bilan);
///   2. yuqorida qidiruv — yozilgan sari natija chiqadi;
///   3. natija tanlansa xaritada belgi va pastda uning kartochkasi;
///   4. marshrut tugmasi — A va B ni xaritadan tanlab, yo'l chizig'i,
///      masofa, vaqt va yetib borish soati.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ MARSHRUT: A/B XARITADAN TANLANADI ────────────────────────────────┐
/// OnDexMap saytida ham shunday (`web/src/components/panels/RoutePanel.tsx`):
/// A va B maydonlari MATN KIRITISH emas, xaritadan tanlash tugmasi.
///
/// Sabab kontraktda: qidiruv (`/v1/search`) nom bo'yicha joy topadi,
/// lekin ixtiyoriy manzil satridan koordinata yasay olmaydi. Ya'ni
/// «matn yozing» maydoni ko'p holatda hech narsa topmasdi.
///
/// Shuning uchun nuqta ikki yo'l bilan qo'yiladi: xaritani surib
/// markazdagi nishonni joylashtirish, yoki «Mening joylashuvim».
/// Qidiruvdan topilgan joy ham to'g'ridan-to'g'ri B ga tushadi.
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapScreen extends StatefulWidget {
  const OndexMapScreen({super.key});

  @override
  State<OndexMapScreen> createState() => _OndexMapScreenState();
}

/// Marshrut nuqtasi.
enum _Slot { a, b }

/// Xaritadan nuqta tanlash nima uchun so'ralayotgani.
///
/// Tanlash oqimi bitta: markazda nishon, xaritani suriladi,
/// «Tanlash» bosiladi. Farq faqat natijada — shuning uchun maqsad
/// alohida saqlanadi.
enum _PickFor { routeA, routeB, newPlace }

class _RoutePoint {
  _RoutePoint(this.lat, this.lng, this.label);
  final double lat;
  final double lng;
  final String label;
}

class _OndexMapScreenState extends State<OndexMapScreen> {
  // Chust markazi — boshlang'ich joylashuv aniq bo'lmaganda.
  static const _chust = ml.LatLng(41.0030, 71.2360);

  /// A/B nuqtalarining ranglari — saytdagi `RoutePanel` bilan bir xil.
  static const _dotA = Color(0xFFFF4D3A);
  static const _dotB = Color(0xFF3F3F46);

  final _api = OndexMapApi();
  final _searchCtrl = TextEditingController();
  final _searchFocus = FocusNode();

  ml.MapLibreMapController? _ctrl;
  bool _styleReady = false;
  bool _satellite = false;
  ml.LatLng _center = _chust;
  /// Joriy masshtab.
  ///
  /// Sun'iy yo'ldosh rejimi almashganda xarita vidjeti butunlay
  /// qayta quriladi (`key: ValueKey(_satellite)`) va
  /// `initialCameraPosition` qaytadan qo'llanadi. Bu yerda qattiq
  /// `13` turgan paytda foydalanuvchi rejimni almashtirsa xarita
  /// markazda qolib, masshtabi sakrab ketardi.
  double _zoomLevel = 13;

  // Qidiruv.
  Timer? _debounce;
  List<OndexPlace> _results = const [];
  bool _searching = false;
  String? _searchError;

  // Tanlangan joy (qidiruv natijasi).
  OndexPlace? _selected;

  // Marshrut.
  bool _routeMode = false;
  _RoutePoint? _a;
  _RoutePoint? _b;

  /// Hozir xaritadan qaysi nuqta tanlanayotgani. `null` — tanlash
  /// rejimi o'chiq.
  _PickFor? _picking;

  OndexRoute? _route;
  bool _routing = false;

  // Xaritadagi ob'ektlar — o'chirish uchun saqlanadi.
  final _symbols = <ml.Symbol>[];
  ml.Line? _routeLine;

  bool _locating = false;

  /// Joylashuvga ruxsat berilganmi.
  ///
  /// ┌─ NEGA HOLATDA SAQLANADI ───────────────────────────────────────┐
  /// `myLocationEnabled: true` bilan TUG'ILGAN xarita, ruxsat o'sha
  /// paytda hali berilmagan bo'lsa, ko'k nuqtani BOSHQA hech qachon
  /// chizmaydi. Sabab `MapLibreMapController` da:
  /// `enableLocationComponent()` uslub yuklanganda bir marta
  /// chaqiriladi va ruxsatni o'sha yerda tekshiradi; keyin ruxsat
  /// berilsa ham `setMyLocationEnabled(true)` qiymat o'zgarmaganini
  /// ko'rib DARHOL qaytadi va komponent yaratilmay qoladi.
  ///
  /// Shuning uchun xarita `false` bilan tug'iladi. Ruxsat
  /// tasdiqlangach `true` ga o'tadi — aynan shu `false -> true`
  /// o'tishi komponentni haqiqatan yoqadi.
  /// └────────────────────────────────────────────────────────────────┘
  bool _locPerm = false;

  @override
  void initState() {
    super.initState();
    _readLocPerm();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _searchCtrl.dispose();
    _searchFocus.dispose();
    _api.dispose();
    super.dispose();
  }

  /* ── Qidiruv ─────────────────────────────────────────────────────── */

  void _onQueryChanged(String q) {
    _debounce?.cancel();
    if (q.trim().length < 2) {
      setState(() {
        _results = const [];
        _searchError = null;
      });
      return;
    }
    _debounce = Timer(const Duration(milliseconds: 300), () => _runSearch(q));
  }

  Future<void> _runSearch(String q) async {
    setState(() {
      _searching = true;
      _searchError = null;
    });
    try {
      final r = await _api.search(q,
          near: (lat: _center.latitude, lng: _center.longitude));
      if (!mounted) return;
      setState(() {
        _results = r;
        _searching = false;
      });
    } on OndexMapException catch (e) {
      if (!mounted) return;
      setState(() {
        _searching = false;
        _searchError = e.message;
      });
    }
  }

  Future<void> _select(OndexPlace p) async {
    _searchFocus.unfocus();
    setState(() {
      _selected = p;
      _results = const [];
      _searchCtrl.text = p.name;
    });
    await _redraw();
    await _ctrl?.animateCamera(
      ml.CameraUpdate.newLatLngZoom(ml.LatLng(p.lat, p.lng), 16),
    );
  }

  /* ── Xaritadagi belgilar va chiziq ───────────────────────────────── */

  /// Hamma belgini va yo'lni qaytadan chizadi.
  ///
  /// MapLibre'da ular imperativ qo'shiladi, ya'ni vidjet qayta
  /// qurilganda o'zi yangilanmaydi. Ob'ektlar uchtadan oshmaydi,
  /// shuning uchun har safar hammasini qayta chizish yetarli va
  /// farqni hisoblashdan soddaroq.
  Future<void> _redraw() async {
    final ctrl = _ctrl;
    if (ctrl == null || !_styleReady) return;

    try {
      for (final s in _symbols) {
        await ctrl.removeSymbol(s);
      }
      _symbols.clear();

      // ⚠️ Belgi nomi `poi_icons.dart` da `addImage` bilan
      // qo'shilgan bo'lishi SHART. Uslubning o'z sprite'i bo'sh
      // (`tiles/sprite.json` → 404), shuning uchun u yerdan hech
      // qanday tayyor belgi olib bo'lmaydi.
      //
      // ⚠️ LANGAR ikki xil, saytdagi kabi: A/B — nuqta USTIDA
      // markazlashgan doira (`center`), tanlangan joy esa uchi bilan
      // nuqtaga sanchiladigan pin (`bottom`).
      Future<void> pin(double lat, double lng, String belgi, String langar) async {
        _symbols.add(await ctrl.addSymbol(ml.SymbolOptions(
          geometry: ml.LatLng(lat, lng),
          iconImage: belgi,
          iconAnchor: langar,
        )));
      }

      if (_routeMode) {
        if (_a != null) await pin(_a!.lat, _a!.lng, '${markerPrefix}a', 'center');
        if (_b != null) await pin(_b!.lat, _b!.lng, '${markerPrefix}b', 'center');
      } else if (_selected != null) {
        await pin(_selected!.lat, _selected!.lng, markerSelected, 'bottom');
      }

      if (_routeLine != null) {
        await ctrl.removeLine(_routeLine!);
        _routeLine = null;
      }
      final r = _route;
      if (r != null && _routeMode) {
        // Rang saytdagi `ROUTE_BLUE` (`useMapTools.ts`) — Yandex
        // marshruti kabi ko'k. Ilgari bu yerda brend to'q sarig'i
        // turardi va ikki mahsulot boshqa-boshqa ko'rinardi.
        _routeLine = await ctrl.addLine(ml.LineOptions(
          geometry: [for (final p in r.points) ml.LatLng(p.lat, p.lng)],
          lineColor: kOndexRouteLine,
          lineWidth: 6,
          lineOpacity: 0.92,
          lineJoin: 'round',
        ));
      }
    } catch (_) {
      // Xarita yopilgan bo'lsa chaqiruvlar xato beradi — ekran
      // baribir tark etilgan.
    }
  }

  /* ── Marshrut ────────────────────────────────────────────────────── */

  void _openRouteMode() {
    setState(() {
      _routeMode = true;
      _searchFocus.unfocus();
      _results = const [];
      // Qidiruvdan joy tanlangan bo'lsa u B ga tushadi — odam
      // odatda «shu yerga borish» uchun marshrut ochadi.
      final sel = _selected;
      if (sel != null && _b == null) {
        _b = _RoutePoint(sel.lat, sel.lng, sel.name);
      }
    });
    _redraw();
    // A bo'sh bo'lsa darhol joylashuvdan to'ldirishga urinamiz —
    // eng ko'p uchraydigan holat «men turgan joydan».
    if (_a == null) _useMyLocation(_Slot.a, jim: true);
  }

  void _closeRouteMode() {
    setState(() {
      _routeMode = false;
      _picking = null;
    });
    _redraw();
  }

  void _clearRoute() {
    setState(() {
      _a = null;
      _b = null;
      _route = null;
      _picking = null;
    });
    _redraw();
  }

  void _swap() {
    setState(() {
      final t = _a;
      _a = _b;
      _b = t;
      _route = null;
    });
    _redraw();
    _maybeRoute();
  }

  /// Xaritadan nuqta tanlash rejimini yoqadi.
  void _startPicking(_PickFor nima) {
    setState(() => _picking = nima);
  }

  /// Markazdagi nishonni tasdiqlaydi — maqsadga qarab.
  Future<void> _confirmPick() async {
    final nima = _picking;
    if (nima == null) return;
    final lat = _center.latitude;
    final lng = _center.longitude;

    if (nima == _PickFor.newPlace) {
      setState(() => _picking = null);
      await _openAddPlace(lat, lng);
      return;
    }

    final p = _RoutePoint(lat, lng, 'Xaritadagi nuqta');
    setState(() {
      if (nima == _PickFor.routeA) {
        _a = p;
      } else {
        _b = p;
      }
      _picking = null;
      _route = null;
    });
    await _redraw();
    _maybeRoute();
  }

  /* ── Ob'ekt qo'shish ─────────────────────────────────────────────── */

  /// Xaritada nuqta tanlashni boshlaydi — keyin forma ochiladi.
  void _startAddPlace() {
    setState(() {
      // Marshrut rejimidan chiqamiz: ikkala rejim bir vaqtda
      // ishlashi chalkash bo'lardi (ikkalasi ham markaz nishonini
      // ishlatadi).
      _routeMode = false;
      _selected = null;
      _picking = _PickFor.newPlace;
    });
    _redraw();
  }

  Future<void> _openAddPlace(double lat, double lng) async {
    final yuborildi = await Navigator.of(context).push<bool>(
      MaterialPageRoute(builder: (_) => AddPlaceScreen(lat: lat, lng: lng)),
    );
    if (yuborildi == true && mounted) {
      _say('Rahmat! Ob\'ekt moderatsiyaga yuborildi');
    }
  }

  Future<void> _useMyLocation(_Slot slot, {bool jim = false}) async {
    final pos = await _myPosition(jim: jim);
    if (pos == null || !mounted) return;
    final p = _RoutePoint(pos.latitude, pos.longitude, 'Mening joylashuvim');
    setState(() {
      if (slot == _Slot.a) {
        _a = p;
      } else {
        _b = p;
      }
      _picking = null;
      _route = null;
    });
    await _redraw();
    _maybeRoute();
  }

  /// Ikkala nuqta tayyor bo'lsa marshrutni hisoblaydi.
  void _maybeRoute() {
    if (_a != null && _b != null) _buildRoute();
  }

  Future<void> _buildRoute() async {
    final a = _a, b = _b;
    if (a == null || b == null || _routing) return;

    // Qamrov tekshiruvi so'rovdan OLDIN: server tashqaridagi nuqta
    // uchun `400` qaytaradi va sabab foydalanuvchiga tushunarsiz
    // bo'lardi.
    if (!ondexRouteCovers(a.lat, a.lng)) {
      _say('Boshlanish nuqtasi O\'zbekiston hududidan tashqarida');
      return;
    }
    if (!ondexRouteCovers(b.lat, b.lng)) {
      _say('Tugash nuqtasi O\'zbekiston hududidan tashqarida');
      return;
    }

    setState(() => _routing = true);
    try {
      final r = await _api.route(
        fromLat: a.lat,
        fromLng: a.lng,
        toLat: b.lat,
        toLng: b.lng,
      );
      if (!mounted) return;
      setState(() => _route = r);
      await _redraw();
      await _fitRoute(r);
    } on OndexMapException catch (e) {
      _say(e.message);
    } finally {
      if (mounted) setState(() => _routing = false);
    }
  }

  Future<void> _fitRoute(OndexRoute r) async {
    var minLat = r.points.first.lat, maxLat = minLat;
    var minLng = r.points.first.lng, maxLng = minLng;
    for (final p in r.points) {
      if (p.lat < minLat) minLat = p.lat;
      if (p.lat > maxLat) maxLat = p.lat;
      if (p.lng < minLng) minLng = p.lng;
      if (p.lng > maxLng) maxLng = p.lng;
    }
    try {
      await _ctrl?.animateCamera(ml.CameraUpdate.newLatLngBounds(
        ml.LatLngBounds(
          southwest: ml.LatLng(minLat, minLng),
          northeast: ml.LatLng(maxLat, maxLng),
        ),
        left: 48,
        right: 48,
        top: 140,
        // Pastda marshrut paneli turadi — yo'l uning ostida
        // qolib ketmasin.
        bottom: 260,
      ));
    } catch (_) {}
  }

  /* ── Joylashuv ───────────────────────────────────────────────────── */

  /// Ruxsat holatini JIMGINA o'qiydi — dialog chiqarmaydi.
  ///
  /// Ekran ochilishi bilan so'rash noto'g'ri bo'lardi: odam hali nima
  /// uchun kerakligini bilmaydi. Ilgari ruxsat bergan bo'lsa esa ko'k
  /// nuqta darhol chiqishi kerak — shuning uchun tekshiruv bor.
  Future<void> _readLocPerm() async {
    try {
      _applyLocPerm(await Geolocator.checkPermission());
    } catch (_) {}
  }

  void _applyLocPerm(LocationPermission perm) {
    final bor = perm == LocationPermission.always ||
        perm == LocationPermission.whileInUse;
    if (bor == _locPerm || !mounted) return;
    setState(() => _locPerm = bor);
  }

  Future<Position?> _myPosition({bool jim = false}) async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      // ⚠️ Natija xaritaga SHU YERDA yetkaziladi: ruxsat endi
      // berilgan bo'lsa ko'k nuqta shundan keyin paydo bo'ladi
      // (`_locPerm` izohiga qarang).
      _applyLocPerm(perm);
      if (perm == LocationPermission.denied ||
          perm == LocationPermission.deniedForever) {
        if (!jim) _say('Joylashuvga ruxsat berilmadi');
        return null;
      }
      return await Geolocator.getCurrentPosition();
    } catch (_) {
      if (!jim) _say('Joylashuvni aniqlab bo\'lmadi');
      return null;
    }
  }

  Future<void> _goToMyLocation() async {
    if (_locating) return;
    setState(() => _locating = true);
    try {
      final p = await _myPosition();
      if (p == null) return;
      await _ctrl?.animateCamera(
        ml.CameraUpdate.newLatLngZoom(ml.LatLng(p.latitude, p.longitude), 16),
      );
    } finally {
      if (mounted) setState(() => _locating = false);
    }
  }

  void _zoom(double delta) {
    _ctrl?.animateCamera(
      delta > 0 ? ml.CameraUpdate.zoomIn() : ml.CameraUpdate.zoomOut(),
    );
  }

  void _say(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), behavior: SnackBarBehavior.floating),
    );
  }

  /* ── Ko'rinish ───────────────────────────────────────────────────── */

  /// Pastki panelning taxminiy balandligi — yon tugmalar va
  /// marshrut kadri shunga qarab joylashadi.
  double get _sheetHeight => switch (true) {
        _ when _picking != null => 132.0,
        _ when _routeMode => 300.0,
        _ when _selected != null => 190.0,
        _ => kMapSheetHeight,
      };

  @override
  Widget build(BuildContext context) {
    final pastki = MediaQuery.paddingOf(context).bottom;
    return Scaffold(
      // Klaviatura ochilganda panel ko'tarilmasin: xarita ostidagi
      // kadr sakrab ketardi.
      resizeToAvoidBottomInset: false,
      body: Stack(
        children: [
          _buildMap(),
          // Brend pastki panel USTIDA: aks holda u panel ortida
          // qolib, umuman ko'rinmasdi.
          OndexMapBrand(bottom: _sheetHeight + pastki + 4),
          if (_picking != null) _buildCrosshair(),
          // Orqaga qaytish — xarita ustida, chap yuqorida.
          _buildBackButton(),
          // Qidiruv natijalari pastki panel USTIDA ochiladi.
          if (!_routeMode &&
              (_results.isNotEmpty || _searching || _searchError != null))
            _buildResults(pastki),
          _buildSideButtons(pastki),
          // ── Pastki panel ────────────────────────────────────────
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: _buildSheet(),
          ),
        ],
      ),
    );
  }

  /// Ekranning pastki paneli — holatga qarab uch ko'rinish.
  Widget _buildSheet() {
    if (_picking != null) return _buildPickSheet();
    if (_routeMode) {
      return RouteSheet(
        onClose: _closeRouteMode,
        onClear: _clearRoute,
        onSwap: _swap,
        onPickA: () => _startPicking(_PickFor.routeA),
        onPickB: () => _startPicking(_PickFor.routeB),
        onUnavailableMode: (nom) =>
            _say('$nom marshruti hozircha hisoblanmaydi'),
        labelA: _a?.label,
        labelB: _b?.label,
        routing: _routing,
        summary: _route == null
            ? null
            : '${ondexDistanceText(_route!.distanceMeters)} · '
                '${ondexDurationText(_route!.durationSeconds)}',
        arrival: _route == null ? null : _arrivalText(_route!.durationSeconds),
      );
    }
    if (_selected != null) return _buildPlaceSheet();
    return MapSearchSheet(
      controller: _searchCtrl,
      focusNode: _searchFocus,
      onChanged: _onQueryChanged,
      onSubmitted: _runSearch,
      searching: _searching,
      onClear: () {
        setState(() {
          _searchCtrl.clear();
          _results = const [];
          _selected = null;
        });
        _redraw();
      },
      onChip: (q) {
        _searchCtrl.text = q;
        _runSearch(q);
      },
      onAddPlace: _startAddPlace,
    );
  }

  /// «14:35 da» — yetib borish soati; saytdagi marshrut paneli ham
  /// shuni ko'rsatadi.
  String _arrivalText(double durationSeconds) {
    final t = DateTime.now().add(Duration(seconds: durationSeconds.round()));
    String ikki(int n) => n.toString().padLeft(2, '0');
    return '${ikki(t.hour)}:${ikki(t.minute)} da';
  }

  Widget _buildBackButton() {
    return Positioned(
      left: 12,
      top: MediaQuery.paddingOf(context).top + 10,
      child: MapSquareButton(
        icon: LucideIcons.arrowLeft,
        tooltip: 'Orqaga',
        onTap: () => Navigator.of(context).pop(),
      ),
    );
  }

  Widget _buildMap() {
    return ml.MapLibreMap(
      key: ValueKey(_satellite),
      styleString: _satellite ? ondexSatelliteStyle : kOndexStyleUrl,
      initialCameraPosition:
          ml.CameraPosition(target: _center, zoom: _zoomLevel),
      trackCameraPosition: true,
      onMapCreated: (c) => _ctrl = c,
      onStyleLoadedCallback: () async {
        _styleReady = true;
        // Uslub almashgach belgilar va chiziq yo'qoladi — qaytaramiz.
        _symbols.clear();
        _routeLine = null;
        // ⚠️ Joy belgilari (do'kon, kafe, shifoxona…) uslubda EMAS —
        // ularni mijoz chizib beradi (`poi_icons.dart` izohiga
        // qarang). Busiz xaritada na belgi, na joy NOMI chiqadi.
        final ctrl = _ctrl;
        if (ctrl != null && mounted) {
          await loadPoiIcons(ctrl);
        }
        await _redraw();
      },
      onCameraMove: (pos) {
        _center = pos.target;
        _zoomLevel = pos.zoom;
      },
      // Kamera mamlakatdan chiqmaydi: tashqarida xarita ma'lumoti
      // yo'q va foydalanuvchi oq bo'shliq ko'rardi.
      cameraTargetBounds: ondexCameraBounds,
      minMaxZoomPreference:
          const ml.MinMaxZoomPreference(kOndexMinZoom, null),
      compassEnabled: false,
      logoEnabled: false,
      // Qayerda turganimni ko'rsatadigan ko'k nuqta.
      //
      // ⚠️ BU YERGA `true` YOZMANG. Ruxsat hali berilmagan bo'lsa
      // MapLibre komponentni umuman yaratmaydi va keyingi ruxsat ham
      // yordam bermaydi — `_locPerm` izohiga qarang.
      myLocationEnabled: _locPerm,
      myLocationRenderMode: ml.MyLocationRenderMode.normal,
      tiltGesturesEnabled: false,
      attributionButtonMargins: kOndexHideAttribution,
    );
  }

  /// Nuqta tanlash rejimidagi markaz nishoni.
  Widget _buildCrosshair() {
    final rang = switch (_picking!) {
      _PickFor.routeA => _dotA,
      _PickFor.routeB => _dotB,
      _PickFor.newPlace => kBrand,
    };
    return IgnorePointer(
      child: Center(
        child: Padding(
          // Belgining uchi markazda tursin.
          padding: const EdgeInsets.only(bottom: 34),
          child: Icon(Icons.location_on, size: 46, color: rang),
        ),
      ),
    );
  }

  /// Qidiruv natijalari — pastki panel USTIDA ochiladi.
  ///
  /// Qidiruv maydoni pastda turgani uchun natijalar ham o'sha yerdan
  /// yuqoriga qarab chiqadi: ko'z maydondan ro'yxatga to'g'ri o'tadi.
  Widget _buildResults(double pastki) {
    return Positioned(
      left: 12,
      right: 12,
      bottom: kMapSheetHeight + pastki + 6,
      child: Material(
        color: Colors.white,
        elevation: 4,
        borderRadius: BorderRadius.circular(16),
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * 0.42,
          ),
          child: _searchError != null
              ? Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(_searchError!,
                      style: const TextStyle(color: Color(0xFF757575))),
                )
              : ListView.separated(
                  shrinkWrap: true,
                  padding: const EdgeInsets.symmetric(vertical: 4),
                  itemCount: _results.length,
                  separatorBuilder: (_, _) =>
                      const Divider(height: 1, indent: 52),
                  itemBuilder: (_, i) {
                    final p = _results[i];
                    return ListTile(
                      leading: const Icon(LucideIcons.mapPin,
                          size: 20, color: kBrand),
                      title: Text(p.name,
                          maxLines: 1, overflow: TextOverflow.ellipsis),
                      subtitle: p.label.isEmpty ? null : Text(p.label),
                      onTap: () => _select(p),
                    );
                  },
                ),
        ),
      ),
    );
  }

  Widget _buildSideButtons(double pastki) {
    // Tugmalar pastki panel USTIDA turadi, uning balandligiga qarab
    // ko'chadi. Yana 62 px — o'ng pastki burchakdagi «OnDex» yozuvi
    // va krediti uchun; aks holda ular ustma-ust tushardi.
    return Positioned(
      right: 12,
      bottom: _sheetHeight + pastki + 66,
      width: 48,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (!_routeMode) ...[
            MapSquareButton(
              tooltip: 'Marshrut',
              // Lucide — saytdagi xarita belgilar bilan bir oila.
              icon: LucideIcons.route,
              onTap: _openRouteMode,
            ),
            const SizedBox(height: 10),
          ],
          MapSquareButton(
            tooltip: _satellite ? 'Oddiy xarita' : 'Sun\'iy yo\'ldosh',
            icon: _satellite ? LucideIcons.map : LucideIcons.satellite,
            active: _satellite,
            onTap: () {
              // Uslub almashadi: belgilar `onStyleLoaded` da qaytadi.
              _symbols.clear();
              _routeLine = null;
              _styleReady = false;
              setState(() => _satellite = !_satellite);
            },
          ),
          const SizedBox(height: 10),
          MapSquareButton(
            tooltip: 'Joriy joylashuvim',
            icon: _locating ? LucideIcons.loader : LucideIcons.locateFixed,
            onTap: _locating ? null : _goToMyLocation,
          ),
          const SizedBox(height: 10),
          MapZoomButtons(onIn: () => _zoom(1), onOut: () => _zoom(-1)),
        ],
      ),
    );
  }

  /// Nuqta tanlash paneli — qolgan panellar bilan bir xil sheet.
  Widget _buildPickSheet() {
    final nima = _picking!;
    final sarlavha = switch (nima) {
      _PickFor.routeA => 'Boshlanish nuqtasini tanlang',
      _PickFor.routeB => 'Tugash nuqtasini tanlang',
      _PickFor.newPlace => 'Ob\'ekt joyini belgilang',
    };
    // «Joylashuvim» faqat marshrut uchun: yangi ob'ektni odam
    // ko'rib turgan joyga qo'yadi, o'zi turgan joyga emas.
    final slot = switch (nima) {
      _PickFor.routeA => _Slot.a,
      _PickFor.routeB => _Slot.b,
      _PickFor.newPlace => null,
    };
    return _Sheet(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(sarlavha,
              style:
                  const TextStyle(fontSize: 16.5, fontWeight: FontWeight.w700)),
          const SizedBox(height: 3),
          const Text('Xaritani suring — nishon markazda qoladi',
              style: TextStyle(fontSize: 13.5, color: Color(0xFF71717A))),
          const SizedBox(height: 14),
          Row(
            children: [
              if (slot != null) ...[
                Expanded(
                  child: OutlinedButton.icon(
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size.fromHeight(46),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(13)),
                    ),
                    onPressed: () => _useMyLocation(slot),
                    icon: const Icon(LucideIcons.locateFixed, size: 18),
                    label: const Text('Joylashuvim'),
                  ),
                ),
                const SizedBox(width: 10),
              ] else ...[
                Expanded(
                  child: OutlinedButton(
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size.fromHeight(46),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(13)),
                    ),
                    onPressed: () => setState(() => _picking = null),
                    child: const Text('Bekor qilish'),
                  ),
                ),
                const SizedBox(width: 10),
              ],
              Expanded(
                child: FilledButton(
                  style: FilledButton.styleFrom(
                    minimumSize: const Size.fromHeight(46),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(13)),
                  ),
                  onPressed: _confirmPick,
                  child: Text(
                      nima == _PickFor.newPlace ? 'Davom etish' : 'Tanlash'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  /// Tanlangan joy kartochkasi — u ham pastki panel.
  Widget _buildPlaceSheet() {
    final p = _selected!;
    return _Sheet(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(p.name,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 18, fontWeight: FontWeight.w700)),
                    if (p.label.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 3),
                        child: Text(p.label,
                            style: const TextStyle(
                                fontSize: 13.5, color: Color(0xFF71717A))),
                      ),
                  ],
                ),
              ),
              IconButton(
                icon: const Icon(LucideIcons.x, size: 19),
                color: const Color(0xFF71717A),
                onPressed: () {
                  setState(() {
                    _selected = null;
                    _searchCtrl.clear();
                  });
                  _redraw();
                },
                tooltip: 'Yopish',
              ),
            ],
          ),
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              style: FilledButton.styleFrom(
                minimumSize: const Size.fromHeight(48),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(13)),
              ),
              onPressed: _openRouteMode,
              icon: const Icon(LucideIcons.route, size: 19),
              label: const Text('Marshrut'),
            ),
          ),
        ],
      ),
    );
  }
}

/// Pastki panel qobig'i — grabber, oq fon, yumaloq tepа.
///
/// Uchala panel (qidiruv, joy, nuqta tanlash) bir xil ko'rinishda
/// bo'lishi kerak; marshrut paneli o'z faylida, lekin ayni qobiq
/// bilan.
class _Sheet extends StatelessWidget {
  const _Sheet({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final pastki = MediaQuery.paddingOf(context).bottom;
    return Container(
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(18)),
        boxShadow: [
          BoxShadow(color: Colors.black26, blurRadius: 14, offset: Offset(0, -2)),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 36,
            height: 4,
            margin: const EdgeInsets.only(top: 8, bottom: 10),
            decoration: BoxDecoration(
              color: const Color(0xFFD4D4D8),
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          Padding(
            padding: EdgeInsets.fromLTRB(16, 0, 16, 14 + pastki),
            child: child,
          ),
        ],
      ),
    );
  }
}

