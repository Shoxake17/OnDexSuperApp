// OnDexMap yo'li (`GOOGLE_MAP=false`) — MapLibre GL + PMTiles, `MapView`
// interfeysi ortida.
//
// `maplibre-gl` va `pmtiles` FAQAT shu yerda dinamik import qilinadi: Google
// rejimidagi foydalanuvchi ularni yuklamaydi. Uslub CSS'i esa komponentlar
// o'zi import qiladi (`import "maplibre-gl/dist/maplibre-gl.css"`).
//
// Zoom Google shkalasida: MapLibre 512 px plitka ishlatadi, shuning uchun
// bu yerda 1 ga ayriladi (`toMl`/`toGoogle`).

import type { GeoJSONSource, Map as MlMap } from "maplibre-gl";
import { bounds } from "./google-view";
import type { CircleStyle, LatLng, LineStyle, MapMarker, MapView, MapViewOptions } from "./types";

/** Uslub shuncha vaqtda yuklanmasa — OnDexMap ishlamayapti deb hisoblanadi. */
export const ONDEX_STYLE_TIMEOUT_MS = 10_000;

const toMl = (z: number) => z - 1;
const toGoogle = (z: number) => z + 1;

const EMPTY: GeoJSON.FeatureCollection = { type: "FeatureCollection", features: [] };

let protocolReady = false;

export async function createOndexView(
  container: HTMLElement,
  opts: MapViewOptions,
  cfg: { styleUrl: string },
): Promise<MapView> {
  const [maplibre, pmtiles] = await Promise.all([import("maplibre-gl"), import("pmtiles")]);
  if (!protocolReady) {
    maplibre.addProtocol("pmtiles", new pmtiles.Protocol().tile);
    protocolReady = true;
  }

  const map: MlMap = new maplibre.Map({
    container,
    style: cfg.styleUrl,
    center: [opts.center.lng, opts.center.lat],
    zoom: toMl(opts.zoom),
    // Buyurtma xaritasi 2D: qiyalik va burish faqat adashtiradi.
    maxPitch: 0,
    dragRotate: false,
    keyboard: false,
    attributionControl: { compact: true },
  });
  map.touchZoomRotate.disableRotation();

  // Uslub yuklanguncha kelgan xato yoki 10 soniya kutish — OnDexMap yiqilgan.
  // Yuklangandan keyingi alohida plitka xatolari xaritani buzmaydi.
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      fail(new Error("uslub 10 s ichida yuklanmadi"));
    }, ONDEX_STYLE_TIMEOUT_MS);
    const fail = (e: Error) => {
      clearTimeout(timer);
      map.off("load", onLoad);
      map.off("error", onError);
      map.remove();
      reject(e);
    };
    const onLoad = () => {
      clearTimeout(timer);
      map.off("error", onError);
      resolve();
    };
    const onError = (e: { error?: Error }) => fail(e.error ?? new Error("uslub xatosi"));
    map.once("load", onLoad);
    map.on("error", onError);
  });

  const lines = new Map<string, number>(); // key -> zIndex
  const markers = new Set<{ remove(): void }>();
  let circleSeq = 0;
  let destroyed = false;

  function layerBefore(zIndex: number): string | undefined {
    let best: { key: string; z: number } | null = null;
    for (const [key, z] of lines) {
      if (z > zIndex && (!best || z < best.z)) best = { key, z };
    }
    return best?.key;
  }

  return {
    provider: "ondexmap",
    getCenter() {
      const c = map.getCenter();
      return { lat: c.lat, lng: c.lng };
    },
    getZoom: () => toGoogle(map.getZoom()),
    setCenter: (p) => map.setCenter([p.lng, p.lat]),
    setZoom: (z) => map.setZoom(toMl(z)),
    panTo: (p) => map.easeTo({ center: [p.lng, p.lat] }),
    fitPoints(points, padding) {
      const b = bounds(points);
      if (!b) {
        if (points.length > 0) map.easeTo({ center: [points[0].lng, points[0].lat], zoom: toMl(16) });
        return;
      }
      map.fitBounds(
        [
          [b.minLng, b.minLat],
          [b.maxLng, b.maxLat],
        ],
        { padding, maxZoom: toMl(18), duration: 0 },
      );
    },
    onIdle(cb) {
      map.on("moveend", cb);
      // Google'dagi `idle` xarita birinchi tayyor bo'lganda ham chiqadi.
      map.once("idle", cb);
      return () => {
        map.off("moveend", cb);
        map.off("idle", cb);
      };
    },
    onUserDrag(cb) {
      map.on("dragstart", cb);
      return () => map.off("dragstart", cb);
    },
    addCircle(center, radiusM, s: CircleStyle) {
      const id = `ondex-circle-${circleSeq++}`;
      map.addSource(id, { type: "geojson", data: circlePolygon(center, radiusM) });
      map.addLayer({
        id: `${id}-fill`,
        type: "fill",
        source: id,
        paint: { "fill-color": s.color, "fill-opacity": s.fillOpacity },
      });
      map.addLayer({
        id: `${id}-line`,
        type: "line",
        source: id,
        paint: { "line-color": s.color, "line-opacity": s.strokeOpacity, "line-width": s.strokeWidth },
      });
    },
    addMarker(position, content, zIndex): MapMarker {
      const wrapper = document.createElement("div");
      wrapper.style.zIndex = String(zIndex);
      wrapper.appendChild(content);
      const marker = new maplibre.Marker({ element: wrapper, anchor: "center" })
        .setLngLat([position.lng, position.lat])
        .addTo(map);
      markers.add(marker);
      return {
        setPosition: (p) => marker.setLngLat([p.lng, p.lat]),
        setRotation: (deg) => {
          content.style.transform = `rotate(${deg}deg)`;
        },
        remove: () => {
          markers.delete(marker);
          marker.remove();
        },
      };
    },
    setLine(key, points: LatLng[] | null, style: LineStyle) {
      if (destroyed) return;
      const data: GeoJSON.FeatureCollection = points
        ? {
            type: "FeatureCollection",
            features: [
              {
                type: "Feature",
                properties: {},
                geometry: { type: "LineString", coordinates: points.map((p) => [p.lng, p.lat]) },
              },
            ],
          }
        : EMPTY;
      const src = map.getSource(key) as GeoJSONSource | undefined;
      if (!src) {
        if (!points) return;
        map.addSource(key, { type: "geojson", data });
        map.addLayer(
          {
            id: key,
            type: "line",
            source: key,
            layout: { "line-join": "round", "line-cap": "round" },
            paint: { "line-color": style.color, "line-width": style.width },
          },
          layerBefore(style.zIndex),
        );
        lines.set(key, style.zIndex);
        return;
      }
      src.setData(data);
      if (points) {
        map.setPaintProperty(key, "line-color", style.color);
        map.setPaintProperty(key, "line-width", style.width);
      }
    },
    destroy() {
      if (destroyed) return;
      destroyed = true;
      for (const m of markers) m.remove();
      markers.clear();
      map.remove();
    },
  };
}

/** Doira — ko'pburchak (MapLibre'da tayyor doira qatlami metrda emas, pikselda). */
function circlePolygon(center: LatLng, radiusM: number, steps = 64): GeoJSON.Feature<GeoJSON.Polygon> {
  const earth = 6371008.8;
  const lat = (center.lat * Math.PI) / 180;
  const ring: [number, number][] = [];
  for (let i = 0; i <= steps; i++) {
    const a = (i / steps) * 2 * Math.PI;
    const dLat = (radiusM / earth) * Math.cos(a);
    const dLng = (radiusM / (earth * Math.cos(lat))) * Math.sin(a);
    ring.push([center.lng + (dLng * 180) / Math.PI, center.lat + (dLat * 180) / Math.PI]);
  }
  return { type: "Feature", properties: {}, geometry: { type: "Polygon", coordinates: [ring] } };
}
