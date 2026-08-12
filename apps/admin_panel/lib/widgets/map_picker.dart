import 'package:flutter/material.dart';

import '../api.dart';
import 'map_surface.dart';

/// Xaritadan joy tanlash natijasi.
class MapPickResult {
  final double lat;
  final double lng;
  final String address;

  MapPickResult({required this.lat, required this.lng, required this.address});
}

/// Google Maps dialogini ochadi: joyni bosib belgilaysiz, manzil avtomatik
/// aniqlanadi, "Tanlash" bosilganda natija qaytadi.
///
/// Xaritaning O'ZI `MapSurface` da — u platformaga qarab ikki xil
/// chiziladi (web'da `google_maps_flutter`, Windows'da WebView2 ichida
/// Google Maps JS). Sabab `map_surface.dart` izohida.
///
/// Avval bu yerda `ensureGoogleMapsLoaded()` chaqirilardi. Endi u web
/// yuzasining o'z ichida — desktop varianti uchun u umuman kerak emas
/// va dialog ochilishini bekorga kechiktirardi.
Future<MapPickResult?> showMapPicker(BuildContext context) {
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
  double? _lat;
  double? _lng;
  String? _address;
  bool _resolving = false;

  Future<void> _onPick(double lat, double lng) async {
    setState(() {
      _lat = lat;
      _lng = lng;
      _resolving = true;
      _address = null;
    });
    try {
      // Teskari geokodlash — backend orqali (`/geocode/reverse`), chunki
      // Google Geocoding API'ni brauzerdan to'g'ridan-to'g'ri chaqirish
      // CORS tomonidan bloklanadi (bu API faqat server-server foydalanish
      // uchun mo'ljallangan).
      final addr =
          await api.reverseGeocode(lat, lng).timeout(const Duration(seconds: 10));
      if (!mounted) return;
      setState(() => _address = addr ?? '');
    } catch (_) {
      if (!mounted) return;
      setState(() => _address = ''); // koordinata bilan davom etamiz
    } finally {
      if (mounted) setState(() => _resolving = false);
    }
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
            Expanded(child: MapSurface(onPick: _onPick)),
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(child: _statusText(context)),
                  const SizedBox(width: 16),
                  FilledButton.icon(
                    icon: const Icon(Icons.check),
                    label: const Text('Tanlash'),
                    onPressed: _lat == null || _resolving
                        ? null
                        : () => Navigator.pop(
                              context,
                              MapPickResult(
                                lat: _lat!,
                                lng: _lng!,
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

  Widget _statusText(BuildContext context) {
    if (_lat == null) {
      return Text('Hali joy tanlanmadi',
          style: Theme.of(context).textTheme.bodySmall);
    }
    if (_resolving) {
      return const Row(children: [
        SizedBox(
            width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)),
        SizedBox(width: 8),
        Text('Manzil aniqlanmoqda...'),
      ]);
    }
    return Text(
      _address!.isEmpty
          ? 'Koordinata: ${_lat!.toStringAsFixed(5)}, ${_lng!.toStringAsFixed(5)}'
          : _address!,
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }
}
