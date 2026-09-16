import { tashkentClock } from "./tashkent-time";

// Buyurtma kuzatuvi (`GET /orders/{id}/tracking`) — ma'lumotni tekshiruvchi
// sof funksiyalar, UI dan ajratilgan.
//
// Qoidalar mijoz ilovasidagi `apps/customer_app/lib/delivery_tracking.dart`
// bilan AYNAN bir xil: web'dagi xarita mobil ilova bilan bir xil bosqichlarda
// bir xil narsani ko'rsatadi va serverdan kelgan noto'g'ri qiymat (matn,
// NaN, chegaradan tashqari, "0,0") xaritaga tushmaydi.

export type LatLng = { lat: number; lng: number };

export function parseGeoPoint(lat: unknown, lng: unknown): LatLng | null {
  if (typeof lat !== "number" || typeof lng !== "number") return null;
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) return null;
  if (lat < -90 || lat > 90 || lng < -180 || lng > 180) return null;
  // "0,0" — joylashuv hali yuborilmagan.
  if (lat === 0 && lng === 0) return null;
  return { lat, lng };
}

function pointFrom(raw: unknown): LatLng | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  return parseGeoPoint(r.lat, r.lng);
}

/** Kuzatuv uchun buyurtmaning kerakli maydonlari. */
export type TrackableOrder = {
  status: string;
  type?: string;
  courier_id?: string;
  courier_location?: unknown;
};

const hasCourier = (o: TrackableOrder) =>
  o.type !== "dine_in" && Boolean(o.courier_id);

const TRACKABLE_STATUSES = new Set(["accepted", "preparing", "ready", "picked_up"]);

/** Kuryer jonli kuzatiladimi: yetkazish, kuryer biriktirilgan, yakunlanmagan. */
export function courierTrackable(o: TrackableOrder): boolean {
  return hasCourier(o) && TRACKABLE_STATUSES.has(o.status);
}

/** Kuzatuv xaritasi ko'rsatiladimi. Yetkazilgandan keyin ham — yo'l qoladi. */
export function deliveryMapVisible(o: TrackableOrder): boolean {
  return courierTrackable(o) || (hasCourier(o) && o.status === "delivered");
}

/** `GET /orders/{id}` dagi `courier_location`. */
export function courierPointFromOrder(o: TrackableOrder): LatLng | null {
  return pointFrom(o.courier_location);
}

export type TrackingPhase = "none" | "waiting" | "live" | "delivered";

export type TrackedRoute = {
  points: LatLng[];
  distanceMeters: number;
  durationSeconds: number;
};

/** Nuqtalar chegarasi — server bilan bir xil (`tracking.MaxRoutePoints`). */
export const MAX_ROUTE_POINTS = 2000;

export function parseTrackedRoute(raw: unknown): TrackedRoute | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  if (!Array.isArray(r.points)) return null;
  const points: LatLng[] = [];
  for (const p of r.points) {
    const point = pointFrom(p);
    if (point) points.push(point);
    if (points.length >= MAX_ROUTE_POINTS) break;
  }
  if (points.length < 2) return null;
  const nonNegative = (v: unknown) =>
    typeof v === "number" && v >= 0 ? Math.trunc(v) : 0;
  return {
    points,
    distanceMeters: nonNegative(r.distance_meters),
    durationSeconds: nonNegative(r.duration_seconds),
  };
}

export type DeliveryTracking = {
  phase: TrackingPhase;
  /** A — restoran, B — mijoz manzili. */
  origin: LatLng | null;
  destination: LatLng | null;
  courier: LatLng | null;
  plannedRoute: TrackedRoute | null;
  remainingRoute: TrackedRoute | null;
  etaSeconds: number | null;
  computedAt: Date | null;
  pickedUpAt: Date | null;
  deliveredAt: Date | null;
  /** A nuqta belgisi — faqat to'liq javobda keladi. */
  restaurantName: string | null;
  restaurantLogoUrl: string | null;
};

export function parseDeliveryTracking(raw: unknown): DeliveryTracking | null {
  if (!raw || typeof raw !== "object") return null;
  const j = raw as Record<string, unknown>;
  const time = (v: unknown) => {
    if (typeof v !== "string") return null;
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? null : d;
  };
  const text = (v: unknown) =>
    typeof v === "string" && v.trim() ? v.trim() : null;
  const phase: TrackingPhase =
    j.phase === "waiting" || j.phase === "live" || j.phase === "delivered"
      ? j.phase
      : "none";
  return {
    phase,
    origin: pointFrom(j.origin),
    destination: pointFrom(j.destination),
    courier: pointFrom(j.courier),
    plannedRoute: parseTrackedRoute(j.planned_route),
    remainingRoute: parseTrackedRoute(j.remaining_route),
    etaSeconds:
      typeof j.eta_seconds === "number" && j.eta_seconds >= 0
        ? Math.trunc(j.eta_seconds)
        : null,
    computedAt: time(j.computed_at),
    pickedUpAt: time(j.picked_up_at),
    deliveredAt: time(j.delivered_at),
    restaurantName: text(j.restaurant_name),
    restaurantLogoUrl: text(j.restaurant_logo_url),
  };
}

/**
 * O'zgarmaydigan qismlar (A→B yo'li, restoran nomi/logosi) oldingi javobdan
 * olinadi: keyingi so'rovlar ularni qayta yuklamaydi (`?planned=0`).
 */
export function keepStaticFrom(
  next: DeliveryTracking,
  previous: DeliveryTracking | null,
): DeliveryTracking {
  if (!previous) return next;
  return {
    ...next,
    plannedRoute: next.plannedRoute ?? previous.plannedRoute,
    restaurantName: next.restaurantName ?? previous.restaurantName,
    restaurantLogoUrl: next.restaurantLogoUrl ?? previous.restaurantLogoUrl,
  };
}

/**
 * Hozirgi paytda qolgan soniya: server hisoblaganidan beri o'tgan vaqt
 * ayiriladi. Ma'lumot yo'q bo'lsa `null`.
 */
export function remainingSeconds(
  t: DeliveryTracking | null,
  now: Date,
): number | null {
  if (!t || t.etaSeconds === null || !t.computedAt) return null;
  const left =
    t.etaSeconds - Math.floor((now.getTime() - t.computedAt.getTime()) / 1000);
  return left < 0 ? 0 : left;
}

/** "~7 daqiqa" kabi qolgan vaqt. */
export function etaText(seconds: number): string {
  return seconds <= 60 ? "1 daqiqadan kam" : `~${Math.ceil(seconds / 60)} daqiqa`;
}

export function distanceText(meters: number): string {
  return meters < 1000 ? `${meters} m` : `${(meters / 1000).toFixed(1)} km`;
}

/** Taom olingandan yetkazilguncha ketgan vaqt ("12 daqiqada"). */
export function deliveryDurationText(
  pickedUp: Date | null,
  delivered: Date | null,
): string | null {
  if (!pickedUp || !delivered) return null;
  const ms = delivered.getTime() - pickedUp.getTime();
  if (ms < 0) return null;
  const minutes = Math.floor(ms / 60_000);
  return minutes < 1 ? "1 daqiqadan kamda" : `${minutes} daqiqada`;
}

export const clockText = tashkentClock;

const EARTH_RADIUS_M = 6_371_000;
const rad = (d: number) => (d * Math.PI) / 180;

/** Ikki nuqta orasidagi masofa (metr, haversine). */
export function metersBetween(a: LatLng, b: LatLng): number {
  const dLat = rad(b.lat - a.lat);
  const dLng = rad(b.lng - a.lng);
  const h =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(rad(a.lat)) * Math.cos(rad(b.lat)) * Math.sin(dLng / 2) ** 2;
  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(h)));
}

/** `from` dan `to` ga yo'nalish, gradus (0 — shimol, soat mili bo'yicha). */
export function bearingDegrees(from: LatLng, to: LatLng): number {
  const lat1 = rad(from.lat);
  const lat2 = rad(to.lat);
  const dLng = rad(to.lng - from.lng);
  const y = Math.sin(dLng) * Math.cos(lat2);
  const x =
    Math.cos(lat1) * Math.sin(lat2) -
    Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLng);
  return ((Math.atan2(y, x) * 180) / Math.PI + 360) % 360;
}
