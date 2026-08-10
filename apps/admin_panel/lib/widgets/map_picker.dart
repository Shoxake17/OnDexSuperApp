import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';

import '../api.dart';
import 'maps_loader.dart';

/// Google Cloud'dan olingan vector Map ID (ixtiyoriy).
/// Bu ID bilan web'da 3D binolar va qiyalik (tilt) to'liq ishlaydi:
/// Console -> Google Maps Platform -> Map Management -> Create Map ID (Vector,
/// "Tilt" va "Rotation" ni yoqing). Bo'sh qoldirsangiz oddiy xarita ko'rinadi.
const kGoogleVectorMapId = '';

/// Xaritadan joy tanlash natijasi.
class MapPickResult {
  final double lat;
  final double lng;
  final String address;

  MapPickResult({required this.lat, required this.lng, required this.address});
}

/// Google Maps dialogini ochadi: joyni bosib belgilaysiz, manzil avtomatik
/// aniqlanadi, "Tanlash" bosilganda natija qaytadi.
/// Avval Maps skripti backend'dan olingan kalit bilan yuklanadi.
Future<MapPickResult?> showMapPicker(BuildContext context) async {
  try {
    await ensureGoogleMapsLoaded();
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Xarita yuklanmadi: $e')));
    }
    return null;
  }
  if (!context.mounted) return null;
  return showDialog<MapPickResult>(
    context: context,
    builder: (_) => const _MapPickerDialog(),
  );
}

class _MapPickerDialog extends StatefulWidget {
  const _MapPickerDialog();

  @override
  State<_MapPickerDialog> createState() => _MapPickerDialogState();
}

class _MapPickerDialogState extends State<_MapPickerDialog> {
  // Chust markazi — xarita shu yerdan, 45° qiyalik (3D) bilan ochiladi.
  static const _chustCenter = LatLng(41.0030, 71.2360);
  static const _initialCamera =
      CameraPosition(target: _chustCenter, zoom: 15, tilt: 45);

  GoogleMapController? _ctrl;
  LatLng? _picked;
  String? _address;
  bool _resolving = false;
  bool _locating = false;
  MapType _mapType = MapType.normal;

  Future<void> _onTap(LatLng p) async {
    setState(() {
      _picked = p;
      _resolving = true;
      _address = null;
    });
    try {
      // Teskari geokodlash — backend orqali (`/geocode/reverse`), chunki
      // Google Geocoding API'ni brauzerdan to'g'ridan-to'g'ri chaqirish
      // CORS tomonidan bloklanadi (bu API faqat server-server foydalanish
      // uchun mo'ljallangan).
      final addr = await api
          .reverseGeocode(p.latitude, p.longitude)
          .timeout(const Duration(seconds: 10));
      if (!mounted) return;
      setState(() => _address = addr ?? '');
    } catch (_) {
      if (!mounted) return;
      setState(() => _address = ''); // koordinata bilan davom etamiz
    } finally {
      if (mounted) setState(() => _resolving = false);
    }
  }

  /// Joriy joylashuvga o'tish (brauzer ruxsat so'raydi).
  Future<void> _goToMyLocation() async {
    setState(() => _locating = true);
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied ||
          perm == LocationPermission.deniedForever) {
        _snack('Joylashuvga ruxsat berilmadi');
        return;
      }
      final pos = await Geolocator.getCurrentPosition();
      final here = LatLng(pos.latitude, pos.longitude);
      await _ctrl?.animateCamera(CameraUpdate.newCameraPosition(
          CameraPosition(target: here, zoom: 17, tilt: 45)));
      _onTap(here); // joriy joyni tanlangan nuqta sifatida ham belgilaymiz
    } catch (_) {
      _snack('Joylashuvni aniqlab bo\'lmadi');
    } finally {
      if (mounted) setState(() => _locating = false);
    }
  }

  void _snack(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  void _zoom(double delta) {
    _ctrl?.animateCamera(CameraUpdate.zoomBy(delta));
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      child: SizedBox(
        width: 760,
        height: 580,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 16, 8, 8),
              child: Row(
                children: [
                  const Icon(Icons.location_on),
                  const SizedBox(width: 8),
                  Text('Restoran joyini xaritadan bosing',
                      style: Theme.of(context).textTheme.titleMedium),
                  const Spacer(),
                  IconButton(
                    icon: const Icon(Icons.close),
                    onPressed: () => Navigator.pop(context),
                  ),
                ],
              ),
            ),
            Expanded(
              child: Stack(
                children: [
                  GoogleMap(
                    initialCameraPosition: _initialCamera,
                    mapType: _mapType,
                    mapId: kGoogleVectorMapId.isEmpty
                        ? null
                        : kGoogleVectorMapId,
                    onMapCreated: (c) => _ctrl = c,
                    onTap: _onTap,
                    // Standart tugmalarni o'chiramiz — o'zimizniki chiroyliroq
                    zoomControlsEnabled: false,
                    myLocationButtonEnabled: false,
                    markers: {
                      if (_picked != null)
                        Marker(
                          markerId: const MarkerId('picked'),
                          position: _picked!,
                        ),
                    },
                  ),
                  // O'ng tomonda boshqaruv tugmalari
                  Positioned(
                    right: 12,
                    top: 12,
                    child: Column(
                      children: [
                        _MapButton(
                          tooltip: 'Joriy joylashuvim',
                          icon: _locating
                              ? Icons.hourglass_top
                              : Icons.my_location,
                          onTap: _locating ? null : _goToMyLocation,
                        ),
                        const SizedBox(height: 8),
                        _MapButton(
                          tooltip: 'Kattalashtirish',
                          icon: Icons.add,
                          onTap: () => _zoom(1),
                        ),
                        const SizedBox(height: 8),
                        _MapButton(
                          tooltip: 'Kichiklashtirish',
                          icon: Icons.remove,
                          onTap: () => _zoom(-1),
                        ),
                        const SizedBox(height: 8),
                        _MapButton(
                          tooltip: _mapType == MapType.normal
                              ? 'Sputnik (3D) ko\'rinish'
                              : 'Oddiy xarita',
                          icon: _mapType == MapType.normal
                              ? Icons.threed_rotation
                              : Icons.map,
                          onTap: () => setState(() {
                            _mapType = _mapType == MapType.normal
                                ? MapType.hybrid
                                : MapType.normal;
                          }),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: _picked == null
                        ? Text('Hali joy tanlanmadi',
                            style: Theme.of(context).textTheme.bodySmall)
                        : _resolving
                            ? const Row(children: [
                                SizedBox(
                                    width: 16,
                                    height: 16,
                                    child: CircularProgressIndicator(
                                        strokeWidth: 2)),
                                SizedBox(width: 8),
                                Text('Manzil aniqlanmoqda...'),
                              ])
                            : Text(
                                _address!.isEmpty
                                    ? 'Koordinata: ${_picked!.latitude.toStringAsFixed(5)}, ${_picked!.longitude.toStringAsFixed(5)}'
                                    : _address!,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),
                  ),
                  const SizedBox(width: 16),
                  FilledButton.icon(
                    icon: const Icon(Icons.check),
                    label: const Text('Tanlash'),
                    onPressed: _picked == null || _resolving
                        ? null
                        : () => Navigator.pop(
                              context,
                              MapPickResult(
                                lat: _picked!.latitude,
                                lng: _picked!.longitude,
                                address: _address ?? '',
                              ),
                            ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Xarita ustidagi dumaloq boshqaruv tugmasi.
class _MapButton extends StatelessWidget {
  final String tooltip;
  final IconData icon;
  final VoidCallback? onTap;

  const _MapButton({
    required this.tooltip,
    required this.icon,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: Theme.of(context).colorScheme.surface,
        shape: const CircleBorder(),
        elevation: 3,
        child: InkWell(
          customBorder: const CircleBorder(),
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.all(10),
            child: Icon(icon, size: 22),
          ),
        ),
      ),
    );
  }
}
