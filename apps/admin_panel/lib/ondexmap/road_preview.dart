import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:latlong2/latlong.dart';

/// Foydalanuvchi chizgan YO'L — moderator tasdiqlashdan oldin xaritada ko'radi.
///
/// Chiziq to'liq ko'rinadigan qilib yaqinlashtiriladi; boshi yashil, oxiri ko'k
/// belgi. Xarita interaktiv (surish/yaqinlashtirish) — moderator atrofni ham
/// ko'rib, yo'l haqiqatan shu yerdan o'tishini tekshira oladi. Asos — OSM raster
/// (muharrir bilan bir xil); `tileProvider` faqat testlar uchun.
class RoadPreview extends StatelessWidget {
  final List<LatLng> points;
  final TileProvider? tileProvider;
  final double height;

  const RoadPreview(
      {super.key, required this.points, this.tileProvider, this.height = 260});

  @override
  Widget build(BuildContext context) {
    if (points.length < 2) return const SizedBox.shrink();
    final bounds = LatLngBounds.fromPoints(points);
    return ClipRRect(
      borderRadius: BorderRadius.circular(12),
      child: SizedBox(
        height: height,
        child: FlutterMap(
          options: MapOptions(
            initialCameraFit: CameraFit.bounds(
              bounds: bounds,
              padding: const EdgeInsets.all(36),
              maxZoom: 19,
            ),
            minZoom: 3,
            maxZoom: 20,
          ),
          children: [
            TileLayer(
              urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
              userAgentPackageName: 'uz.ondex.chust_admin',
              maxNativeZoom: 19,
              maxZoom: 20,
              tileProvider: tileProvider,
            ),
            PolylineLayer(
              polylines: [
                Polyline(
                  points: points,
                  strokeWidth: 5,
                  color: const Color(0xFF2F6BFF),
                  borderStrokeWidth: 2,
                  borderColor: Colors.white,
                  strokeCap: StrokeCap.round,
                  strokeJoin: StrokeJoin.round,
                ),
              ],
            ),
            MarkerLayer(
              markers: [
                _dot(points.first, const Color(0xFF16A34A)),
                _dot(points.last, const Color(0xFF2F6BFF)),
              ],
            ),
            const RichAttributionWidget(
              alignment: AttributionAlignment.bottomRight,
              showFlutterMapAttribution: false,
              attributions: [
                TextSourceAttribution('© OpenStreetMap contributors')
              ],
            ),
          ],
        ),
      ),
    );
  }

  Marker _dot(LatLng p, Color color) => Marker(
        point: p,
        width: 20,
        height: 20,
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: color,
            shape: BoxShape.circle,
            border: Border.all(color: Colors.white, width: 3),
            boxShadow: const [BoxShadow(blurRadius: 4, color: Colors.black38)],
          ),
        ),
      );
}
