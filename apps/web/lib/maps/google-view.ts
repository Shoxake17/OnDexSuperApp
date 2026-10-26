// Google Maps yo'li — `MapView` interfeysi ortida. Xarita sozlamalari va
// belgilar avvalgi komponentlardagi bilan AYNAN bir xil (o'zgarishsiz
// ko'chirilgan): Google rejimida foydalanuvchi hech narsa o'zgarganini
// sezmasligi kerak.

import { createHtmlMapMarker } from "../html-map-marker";
import type { CircleStyle, LatLng, LineStyle, MapMarker, MapView, MapViewOptions } from "./types";

/** `loadGoogleMaps()` tugagandan keyingina chaqiriladi. */
export function createGoogleView(container: HTMLElement, opts: MapViewOptions): MapView {
  // Xarita uslubi ATAYLAB o'zgartirilmaydi — Google'ning standart OQ
  // ko'rinishi (Flutter'dagi `MapType.normal` bilan bir xil bo'lsin).
  const map = new google.maps.Map(container, {
    center: opts.center,
    zoom: opts.zoom,
    disableDefaultUI: true,
    keyboardShortcuts: false,
    gestureHandling: `greedy`,
    clickableIcons: false,
  });
  const lines = new Map<string, google.maps.Polyline>();
  const circles: google.maps.Circle[] = [];
  const listeners: google.maps.MapsEventListener[] = [];

  function listen(event: string, cb: () => void): () => void {
    const l = map.addListener(event, cb);
    listeners.push(l);
    return () => l.remove();
  }

  return {
    provider: `google`,
    getCenter() {
      const c = map.getCenter();
      return c ? { lat: c.lat(), lng: c.lng() } : opts.center;
    },
    getZoom: () => map.getZoom() ?? opts.zoom,
    setCenter: (p) => map.setCenter(p),
    setZoom: (z) => map.setZoom(z),
    panTo: (p) => map.panTo(p),
    fitPoints(points, padding) {
      const b = bounds(points);
      if (!b) {
        if (points.length > 0) {
          map.panTo(points[0]);
          map.setZoom(16);
        }
        return;
      }
      map.fitBounds({ south: b.minLat, west: b.minLng, north: b.maxLat, east: b.maxLng }, padding);
    },
    onIdle: (cb) => listen(`idle`, cb),
    onUserDrag: (cb) => listen(`dragstart`, cb),
    addCircle(center, radiusM, s: CircleStyle) {
      circles.push(
        new google.maps.Circle({
          map,
          center,
          radius: radiusM,
          strokeColor: s.color,
          strokeOpacity: s.strokeOpacity,
          strokeWeight: s.strokeWidth,
          fillColor: s.color,
          fillOpacity: s.fillOpacity,
          clickable: false,
        }),
      );
    },
    addMarker(position, content, zIndex): MapMarker {
      return createHtmlMapMarker(map, position, content, zIndex);
    },
    setLine(key, points: LatLng[] | null, style: LineStyle) {
      const line = lines.get(key);
      if (!points) {
        line?.setMap(null);
        lines.delete(key);
        return;
      }
      const opts2 = { strokeColor: style.color, strokeWeight: style.width, zIndex: style.zIndex };
      if (!line) {
        lines.set(
          key,
          new google.maps.Polyline({ map, path: points, clickable: false, strokeOpacity: 1, ...opts2 }),
        );
        return;
      }
      line.setOptions({ path: points, ...opts2 });
      if (!line.getMap()) line.setMap(map);
    },
    destroy() {
      for (const l of listeners) l.remove();
      for (const l of lines.values()) l.setMap(null);
      for (const c of circles) c.setMap(null);
      lines.clear();
    },
  };
}

/** Nuqtalarni o'rab turgan to'rtburchak; hammasi deyarli bir joyda bo'lsa `null`. */
export function bounds(points: LatLng[]) {
  if (points.length === 0) return null;
  let minLat = points[0].lat;
  let maxLat = minLat;
  let minLng = points[0].lng;
  let maxLng = minLng;
  for (const p of points) {
    if (p.lat < minLat) minLat = p.lat;
    if (p.lat > maxLat) maxLat = p.lat;
    if (p.lng < minLng) minLng = p.lng;
    if (p.lng > maxLng) maxLng = p.lng;
  }
  if (maxLat - minLat < 1e-4 && maxLng - minLng < 1e-4) return null;
  return { minLat, maxLat, minLng, maxLng };
}
