import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';

// ensureGoogleMapsLoaded va pi ikkalasi ham shu yerdan keladi:
// pi.dart ondex_core ni qayta eksport qiladi. Kuryer va mijoz
// ilovalarida ham xuddi shunday.
import '../api.dart';


/// Google Cloud'dan olingan vector Map ID (ixtiyoriy).
/// Bu ID bilan web'da 3D binolar va qiyalik (tilt) to'liq ishlaydi:
/// Console -> Google Maps Platform -> Map Management -> Create Map ID (Vector,
/// "Tilt" va "Rotation" ni yoqing). Bo'sh qoldirsangiz oddiy xarita ko'rinadi.
const kGoogleVectorMapId = '';

/// WEB uchun xarita yuzasi — `google_maps_flutter` (JS API).
///
/// Windows varianti uchun `map_surface_webview.dart` ga qarang; tanlash
/// `map_surface.dart` dagi conditional export orqali bo'ladi.
class MapSurface extends StatefulWidget {
  final void Function(double lat, double lng) onPick;

  const MapSurface({super.key, required this.onPick});

  @override
  State<MapSurface> createState() => _MapSurfaceState();
}

class _MapSurfaceState extends State<MapSurface> {
  // Chust markazi — xarita shu yerdan, 45° qiyalik (3D) bilan ochiladi.
  static const _chustCenter = LatLng(41.0030, 71.2360);
  static const _initialCamera =
      CameraPosition(target: _chustCenter, zoom: 15, tilt: 45);

  GoogleMapController? _ctrl;
  LatLng? _picked;
  bool _locating = false;
  bool _ready = false;
  String? _error;
  MapType _mapType = MapType.normal;

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// Maps JS kutubxonasi kalit bilan yuklanadi (kalit backenddan keladi,
  /// frontend kodida saqlanmaydi).
  Future<void> _load() async {
    try {
      await ensureGoogleMapsLoaded(api.mapsApiKey);
      if (mounted) setState(() => _ready = true);
    } catch (e) {
      if (mounted) setState(() => _error = 'Xarita yuklanmadi: $e');
    }
  }

  void _onTap(LatLng p) {
    setState(() => _picked = p);
    widget.onPick(p.latitude, p.longitude);
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

  void _zoom(double delta) => _ctrl?.animateCamera(CameraUpdate.zoomBy(delta));

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(_error!, textAlign: TextAlign.center),
        ),
      );
    }
    if (!_ready) return const Center(child: CircularProgressIndicator());

    return Stack(
      children: [
        GoogleMap(
          initialCameraPosition: _initialCamera,
          mapType: _mapType,
          mapId: kGoogleVectorMapId.isEmpty ? null : kGoogleVectorMapId,
          onMapCreated: (c) => _ctrl = c,
          onTap: _onTap,
          // Standart tugmalarni o'chiramiz — o'zimizniki chiroyliroq
          zoomControlsEnabled: false,
          myLocationButtonEnabled: false,
          markers: {
            if (_picked != null)
              Marker(markerId: const MarkerId('picked'), position: _picked!),
          },
        ),
        Positioned(
          right: 12,
          top: 12,
          child: Column(
            children: [
              _MapButton(
                tooltip: 'Joriy joylashuvim',
                icon: _locating ? Icons.hourglass_top : Icons.my_location,
                onTap: _locating ? null : _goToMyLocation,
              ),
              const SizedBox(height: 8),
              _MapButton(
                  tooltip: 'Kattalashtirish',
                  icon: Icons.add,
                  onTap: () => _zoom(1)),
              const SizedBox(height: 8),
              _MapButton(
                  tooltip: 'Kichiklashtirish',
                  icon: Icons.remove,
                  onTap: () => _zoom(-1)),
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
