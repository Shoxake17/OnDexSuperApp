import 'dart:async';

import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:posthog_flutter/posthog_flutter.dart';

import '../api.dart';
import '../widgets/app_text_field.dart';
import '../widgets/common.dart';
import 'address_search_screen.dart';
import 'catalog_screen.dart' show kBrand;

/// Xaritadagi OnDex pinasi va tugmalari.
///
/// ┌─ NEGA `scheme.primary` EMAS ───────────────────────────────────────┐
/// Ilgari pin va tugmalar mavzudan olingan rangda edi. `ColorScheme`
/// urug'dan hosil qilinadi, ya'ni chiqqan rang brend rangining AYNAN
/// o'zi emas — xaritada u pushti-qizg'ish tusga kirardi va ilovaning
/// qolgan qismidan farq qilardi. Bu yerda brend rangi TO'G'RIDAN-
/// TO'G'RI olinadi.
/// └────────────────────────────────────────────────────────────────────┘
const _mapPin = 'assets/services/map.png';

/// Yetkazib berish manzilini xaritadan tanlash ekrani (Yandex Go uslubi):
/// xarita tepada, markazda qimirlamas pin turadi (foydalanuvchi xaritani
/// suradi, pin joyida qoladi), pastda manzil matni + podъezd/qavat/
/// kvartira/domofon/izoh maydonlari, "Tayyor" bosilganda akkauntga saqlanadi.
class AddressScreen extends StatefulWidget {
  final double? initialLat;
  final double? initialLng;

  const AddressScreen({super.key, this.initialLat, this.initialLng});

  @override
  State<AddressScreen> createState() => _AddressScreenState();
}

class _AddressScreenState extends State<AddressScreen> {
  // Chust markazi — boshlang'ich joylashuv aniq bo'lmasa shu yerdan boshlanadi.
  static const _chustCenter = LatLng(41.0030, 71.2360);

  late CameraPosition _initialCamera;
  LatLng _center = _chustCenter;
  GoogleMapController? _ctrl;
  bool _mapsReady = false;
  bool _mapsFailed = false;
  bool _resolvingAddress = false;
  bool _locating = false;
  bool _saving = false;
  MapType _mapType = MapType.normal;
  int _geocodeGen = 0;
  LatLng? _lastResolvedCenter;
  Timer? _debounce;
  double _currentZoom = 16;
  // Dastur o'zi animateCamera chaqirganda (zoom, "joriy joylashuvim",
  // qidiruvdan tanlash) Google xaritasi animatsiya DAVOMIDA ham
  // `onCameraMove` orqali oraliq (intermediate) koordinatalarni yuboradi.
  // Agar shu oraliq qiymatlarni `_center`ga yozib borsak, ketma-ket tez
  // bosishda markaz asta-sekin "siljib" ketadi (compounding drift) — aynan
  // shu "+/- bosilganda tomonga suzib ketish" xatosining haqiqiy sababi.
  // Bu hisoblagich >0 bo'lganda `onCameraMove` markazni e'tiborsiz
  // qoldiradi — markaz FAQAT haqiqiy foydalanuvchi surganda yangilanadi.
  int _programmaticMoves = 0;

  final _addressCtrl = TextEditingController();
  final _entranceCtrl = TextEditingController();
  final _floorCtrl = TextEditingController();
  final _apartmentCtrl = TextEditingController();
  final _intercomCtrl = TextEditingController();
  final _commentCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    _center = (widget.initialLat != null && widget.initialLng != null)
        ? LatLng(widget.initialLat!, widget.initialLng!)
        : _chustCenter;
    _initialCamera = CameraPosition(target: _center, zoom: 16);
    _loadMaps();
  }

  Future<void> _loadMaps() async {
    try {
      await ensureGoogleMapsLoaded(api.mapsApiKey);
      if (!mounted) return;
      setState(() => _mapsReady = true);
      _scheduleResolve(_center);
    } catch (_) {
      if (mounted) setState(() => _mapsFailed = true);
    }
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _addressCtrl.dispose();
    _entranceCtrl.dispose();
    _floorCtrl.dispose();
    _apartmentCtrl.dispose();
    _intercomCtrl.dispose();
    _commentCtrl.dispose();
    super.dispose();
  }

  /// Xarita tez-tez `onCameraIdle` chiqarishi mumkin (masalan boshlang'ich
  /// yuklanishda 2-3 marta ketma-ket) — har birida darhol so'rov yubormay,
  /// 350ms kutib, faqat OXIRGI pozitsiya uchun bitta so'rov yuboramiz.
  void _scheduleResolve(LatLng p) {
    _debounce?.cancel();
    _debounce =
        Timer(const Duration(milliseconds: 350), () => _resolveAddress(p));
  }

  /// Teskari geokodlash — backend orqali (`/geocode/reverse`), chunki Google
  /// Geocoding API'ni brauzerdan to'g'ridan-to'g'ri chaqirish CORS tomonidan
  /// bloklanadi (bu API faqat server-server foydalanish uchun mo'ljallangan
  /// — aynan shu sabab "Manzil aniqlanmoqda..." abadiy osilib qolar edi:
  /// so'rov hech qachon muvaffaqiyatli yakunlanmasdi). `_geocodeGen` — eski
  /// so'rov natijasi keyingi (yangiroq) so'rovni ustidan bosib qolmasligi
  /// uchun; masofa tekshiruvi — deyarli bir xil nuqta uchun qayta so'ramaslik.
  Future<void> _resolveAddress(LatLng p) async {
    if (_lastResolvedCenter != null) {
      final d = Geolocator.distanceBetween(_lastResolvedCenter!.latitude,
          _lastResolvedCenter!.longitude, p.latitude, p.longitude);
      if (d < 3) return;
    }
    _lastResolvedCenter = p;
    final myGen = ++_geocodeGen;
    if (mounted) setState(() => _resolvingAddress = true);
    String? result;
    try {
      result = await api
          .reverseGeocode(p.latitude, p.longitude)
          .timeout(const Duration(seconds: 10));
    } catch (_) {
      result =
          null; // Internet/server muammosida koordinata bilan davom etamiz.
    }
    if (!mounted || myGen != _geocodeGen) return; // eskirgan javob — e'tiborsiz
    final resolved = result;
    setState(() {
      if (resolved != null && resolved.isNotEmpty) _addressCtrl.text = resolved;
      _resolvingAddress = false;
    });
  }

  Future<void> _goToMyLocation() async {
    setState(() => _locating = true);
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied ||
          perm == LocationPermission.deniedForever) {
        snack('Joylashuvga ruxsat berilmadi');
        return;
      }
      final pos = await Geolocator.getCurrentPosition();
      final here = LatLng(pos.latitude, pos.longitude);
      await _animateTo(here, 17);
    } catch (_) {
      snack('Joylashuvni aniqlab bo\'lmadi');
    } finally {
      if (mounted) setState(() => _locating = false);
    }
  }


  /// Barcha DASTUR o'zi buyuradigan kamera harakatlari shu orqali o'tadi —
  /// `_programmaticMoves` hisoblagichi animatsiya davomida `onCameraMove`ning
  /// oraliq qiymatlari `_center`ni buzib qo'yishining oldini oladi.
  Future<void> _animateTo(LatLng target, double zoom) async {
    _programmaticMoves++;
    try {
      await _ctrl?.animateCamera(CameraUpdate.newCameraPosition(
          CameraPosition(target: target, zoom: zoom)));
    } finally {
      _programmaticMoves--;
      _center = target;
      _currentZoom = zoom;
    }
  }

  /// `CameraUpdate.zoomBy()` xaritaning JORIY holatiga nisbatan hisoblaydi —
  /// tez-tez bosilganda (animatsiya hali tugamagan holda) markaz asta-sekin
  /// yon tomonga "surilib" ketadi (google_maps_flutter_web'da tasdiqlangan
  /// muammo). Shuning uchun har doim BIZ o'zimiz kuzatib turgan `_center`
  /// (pinning nuqtasi) va `_currentZoom`ga asoslanib, ANIQ target bilan
  /// yangi kamera pozitsiyasini beramiz — `_animateTo` esa animatsiya
  /// davomidagi oraliq hodisalarni ham e'tiborsiz qoldiradi, shuning uchun
  /// qanchalik tez ketma-ket bossangiz ham markaz hech qachon siljimaydi.
  void _zoom(double delta) {
    final newZoom = (_currentZoom + delta).clamp(2.0, 20.0);
    _animateTo(_center, newZoom);
  }

  /// Manzil maydoniga bosilganda qidiruv ekrani ochiladi (Yandex Go
  /// uslubi) — natija tanlansa xarita o'sha nuqtaga suriladi. Agar aniq
  /// manzil matni bilan qaytsa (oddiy taklif tanlanganda), uni qayta
  /// teskari geokodlashning hojati yo'q — `_lastResolvedCenter` shu
  /// nuqtaga o'rnatiladi, `onCameraIdle` uni qayta so'ramaydi.
  Future<void> _openAddressSearch() async {
    final result = await Navigator.of(context).push<Map<String, dynamic>>(
      MaterialPageRoute(builder: (_) => const AddressSearchScreen()),
    );
    if (result == null || !mounted) return;
    final lat = (result['lat'] as num?)?.toDouble();
    final lng = (result['lng'] as num?)?.toDouble();
    if (lat == null || lng == null) return;
    final newCenter = LatLng(lat, lng);
    final addr = result['address'] as String?;
    if (addr != null && addr.isNotEmpty) {
      _lastResolvedCenter = newCenter;
      setState(() => _addressCtrl.text = addr);
    }
    await _animateTo(newCenter, _currentZoom);
  }

  Future<void> _save() async {
    if (_addressCtrl.text.trim().isEmpty) {
      snack('Manzil aniqlanmadi — xaritada joyni tanlang');
      return;
    }
    setState(() => _saving = true);
    try {
      await api.saveAddress(
        lat: _center.latitude,
        lng: _center.longitude,
        text: _addressCtrl.text.trim(),
        entrance: _entranceCtrl.text.trim(),
        floor: _floorCtrl.text.trim(),
        apartment: _apartmentCtrl.text.trim(),
        intercom: _intercomCtrl.text.trim(),
        comment: _commentCtrl.text.trim(),
      );
      if (!mounted) return;
      Navigator.of(context).pop(_addressCtrl.text.trim());
    } on ApiException catch (e) {
      snack(e.message);
    } catch (_) {
      snack('Saqlab bo\'lmadi — internetni tekshiring');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      resizeToAvoidBottomInset: true,
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  if (_mapsFailed)
                    const Center(child: Text('Xarita yuklanmadi'))
                  else if (!_mapsReady)
                    const Center(child: CircularProgressIndicator())
                  else
                    GoogleMap(
                      initialCameraPosition: _initialCamera,
                      mapType: _mapType,
                      onMapCreated: (c) => _ctrl = c,
                      onCameraMove: (pos) {
                        // Dastur o'zi kamerani suriyotgan bo'lsa (zoom,
                        // "joriy joylashuvim", qidiruv) — oraliq qiymatlarni
                        // e'tiborsiz qoldiramiz, faqat haqiqiy foydalanuvchi
                        // surganda markazni yangilaymiz.
                        if (_programmaticMoves > 0) return;
                        _center = pos.target;
                        _currentZoom = pos.zoom;
                      },
                      onCameraIdle: () => _scheduleResolve(_center),
                      zoomControlsEnabled: false,
                      myLocationButtonEnabled: false,
                      mapToolbarEnabled: false,
                      compassEnabled: false,
                    ),
                  // Markazda qimirlamas pin — xarita ostida suriladi.
                  if (_mapsReady)
                    IgnorePointer(
                      child: Center(
                        child: Padding(
                          padding: const EdgeInsets.only(bottom: 36),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              // OnDex xarita belgisi — bosh sahifadagi
                              // "OnDex Xarita" plitkasi bilan AYNAN
                              // bitta rasm. Foydalanuvchi ikki joyda
                              // bir xil belgini ko'radi.
                              Image.asset(_mapPin,
                                  height: 52,
                                  // Rasm topilmasa xarita pinsiz
                                  // qolmasin.
                                  errorBuilder: (_, __, ___) => const Icon(
                                      Icons.location_on,
                                      size: 44,
                                      color: kBrand)),
                              Container(
                                width: 6,
                                height: 6,
                                decoration: const BoxDecoration(
                                  color: Colors.black45,
                                  shape: BoxShape.circle,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  Positioned(
                    left: 12,
                    top: 12,
                    child: RoundIconButton(
                      icon: Icons.arrow_back,
                      onTap: () => Navigator.of(context).pop(),
                    ),
                  ),
                  // 3D/2D — yuqori o'ng burchakda, yolg'iz.
                  Positioned(
                    right: 12,
                    top: 12,
                    child: RoundIconButton(
                      tooltip: _mapType == MapType.normal
                          ? 'Sputnik (3D) ko\'rinish'
                          : 'Oddiy xarita (2D)',
                      icon: _mapType == MapType.normal
                          ? Icons.threed_rotation
                          : Icons.map,
                      onTap: () => setState(() {
                        _mapType = _mapType == MapType.normal
                            ? MapType.hybrid
                            : MapType.normal;
                      }),
                    ),
                  ),
                  // Joriy joylashuv + zoom — pastki o'ng burchakda, manzil
                  // panelining tepasida. Qat'iy kenglik (48) — aks holda
                  // _ZoomBlock ichidagi chiziq to'liq kenglikni egallashga
                  // urinib, butun xaritani kesib o'tib cho'zilib ketadi.
                  Positioned(
                    right: 12,
                    bottom: 12,
                    width: 48,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.end,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        RoundIconButton(
                          tooltip: 'Joriy joylashuvim',
                          icon: _locating
                              ? Icons.hourglass_top
                              : Icons.my_location,
                          onTap: _locating ? null : _goToMyLocation,
                        ),
                        const SizedBox(height: 14),
                        _ZoomBlock(
                            onZoomIn: () => _zoom(1),
                            onZoomOut: () => _zoom(-1)),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            SingleChildScrollView(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _resolvingAddress
                        ? const Row(
                            children: [
                              SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                      strokeWidth: 2)),
                              SizedBox(width: 8),
                              Text('Manzil aniqlanmoqda...'),
                            ],
                          )
                        : PostHogMaskWidget(
                            // Bu yerda mijozning haqiqiy uy manzili
                            // ko'rsatiladi — PostHog seans yozuvida
                            // niqoblanadi.
                            child: AppTextField(
                              controller: _addressCtrl,
                              hint: 'Manzilni qidirish uchun bosing',
                              icon: Icons.search,
                              // Bu maydon TUGMA: bosilganda manzil qidiruv
                              // ekrani ochiladi, o'zida yozilmaydi.
                              readOnly: true,
                              onTap: _openAddressSearch,
                            ),
                          ),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        Expanded(
                            child: _FieldBox(
                                controller: _entranceCtrl, label: 'Podyezd')),
                        const SizedBox(width: 16),
                        Expanded(
                            child: _FieldBox(
                                controller: _floorCtrl, label: 'Qavat')),
                      ],
                    ),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        Expanded(
                            child: _FieldBox(
                                controller: _apartmentCtrl, label: 'Kvartira')),
                        const SizedBox(width: 16),
                        Expanded(
                            child: _FieldBox(
                                controller: _intercomCtrl, label: 'Domofon')),
                      ],
                    ),
                    const SizedBox(height: 12),
                    _FieldBox(
                        controller: _commentCtrl, label: 'Kuryer uchun izoh'),
                    const SizedBox(height: 20),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton(
                        style: FilledButton.styleFrom(
                          minimumSize: const Size.fromHeight(52),
                          shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(16)),
                        ),
                        onPressed: _saving ? null : _save,
                        child: _saving
                            ? const SizedBox(
                                width: 20,
                                height: 20,
                                child: CircularProgressIndicator(
                                    strokeWidth: 2, color: Colors.white))
                            : const Text('Tayyor',
                                style: TextStyle(fontWeight: FontWeight.bold)),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _FieldBox extends StatelessWidget {
  final TextEditingController controller;
  final String label;
  const _FieldBox({required this.controller, required this.label});

  @override
  Widget build(BuildContext context) {
    // Ilgari tagi chizilgan (`UnderlineInputBorder`) edi va ekrandagi
    // qolgan maydonlardan ajralib turardi. Endi umumiy ko'rinish.
    //
    // Podyezd/qavat/kvartira/domofon/izoh — hammasi manzil tafsiloti,
    // PostHog seans yozuvida niqoblanadi.
    return PostHogMaskWidget(
      child: AppTextField(controller: controller, hint: label),
    );
  }
}


/// Zoom bloki — faqat kattalashtirish/kichiklashtirish, boshqa ikon yo'q.
class _ZoomBlock extends StatelessWidget {
  final VoidCallback onZoomIn;
  final VoidCallback onZoomOut;
  const _ZoomBlock({required this.onZoomIn, required this.onZoomOut});

  @override
  Widget build(BuildContext context) {
    // Qat'iy kenglik — Divider (yoki shunga o'xshash chiziq) mavjud
    // kenglikni to'liq egallashga urinishi mumkin; SizedBox bunga yo'l
    // qo'ymaydi, blok har doim ikkita dumaloq tugma o'lchamida qoladi.
    return SizedBox(
      width: 44,
      child: Material(
        // Qat'iy oq — pastki menyu paneli bilan bir xil. `colorScheme`
        // qurilma mavzusiga bog'liq va kulrang tusga kirardi.
        color: Colors.white,
        borderRadius: BorderRadius.circular(22),
        elevation: 3,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            InkWell(
              borderRadius:
                  const BorderRadius.vertical(top: Radius.circular(22)),
              onTap: onZoomIn,
              child: const Padding(
                padding: EdgeInsets.symmetric(vertical: 16, horizontal: 10),
                child: Icon(Icons.add, size: 20),
              ),
            ),
            Container(
              width: 22,
              height: 1,
              color: Colors.grey.shade400,
            ),
            InkWell(
              borderRadius:
                  const BorderRadius.vertical(bottom: Radius.circular(22)),
              onTap: onZoomOut,
              child: const Padding(
                padding: EdgeInsets.symmetric(vertical: 16, horizontal: 10),
                child: Icon(Icons.remove, size: 20),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
