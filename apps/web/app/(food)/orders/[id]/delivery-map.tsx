"use client";

import { Home, LocateFixed, Store } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  bearingDegrees,
  clockText,
  deliveryDurationText,
  distanceText,
  etaText,
  metersBetween,
  parseGeoPoint,
  remainingSeconds,
  type DeliveryTracking,
  type LatLng,
  type TrackedRoute,
} from "@/lib/delivery-tracking";
import { loadGoogleMaps } from "@/lib/gmaps";
import { createHtmlMapMarker, type HtmlMapMarker } from "@/lib/html-map-marker";
import { fullImageUrl } from "@/lib/images";
import { useNow } from "@/lib/use-now";
import type { Order } from "@/lib/use-order-tracking";

// Buyurtma kuzatuvi xaritasi — mobil va kompyuter ko'rinishlari uchun BITTA
// komponent (avvalgi `courier-map.tsx` o'rniga).
//
// Mijoz ilovasidagi `widgets/courier_tracking_map.dart` bilan bir xil
// bosqichlar va belgilar:
//   * kuryer biriktirilmagan — faqat yetkazish manzili;
//   * kuryer taomni olguncha — kuryer mashinasi va manzil;
//   * kuryer yo'lda — restoran (logosi) → mijoz (uy) yo'li kulrangda,
//     kuryerdan manzilgacha qolgan yo'l brend rangida, jonli mashina va
//     qolgan vaqt;
//   * yetkazilgach — xarita YO'QOLMAYDI: buyurtmaga biriktirilgan yo'l,
//     yetkazish vaqti va masofasi.
//
// Mijoz xaritani o'zi sursa kamera uni "tortib qaytarmaydi" — kuzatish
// tugmasi bilan qayta yoqiladi.

/** `tailwind.config.ts` dagi `brand`. */
const BRAND = "#F4511E";
const PLANNED_LINE = "#B0B7C3";
const INK = "#111827";
/** Mashina — yuqoridan ko'rinish, old tomoni TEPAGA qaragan. */
const CAR_SRC = "/map/kuryer-car.png";
const CHUST_CENTER: LatLng = { lat: 41.003, lng: 71.236 };
/** Mashina shundan kam siljisa burilmaydi (GPS "titrashi" aylantirmasin). */
const MIN_MOVE_FOR_BEARING_M = 8;

type Stage = "pending" | "waiting" | "live" | "delivered";

export default function DeliveryMap({
  order,
  courier,
  tracking,
  variant,
  mapHeightClassName = "h-[260px]",
}: {
  order: Order;
  /** Jonli joylashuv (WebSocket yoki buyurtma javobi). */
  courier: LatLng | null;
  tracking: DeliveryTracking | null;
  /** `light` — mobil sahifa, `dark` — kompyuter ko'rinishi. */
  variant: "light" | "dark";
  mapHeightClassName?: string;
}) {
  const now = useNow() ?? new Date();

  const stage: Stage =
    order.status === "delivered"
      ? "delivered"
      : !order.courier_id
        ? "pending"
        : order.status === "picked_up"
          ? "live"
          : "waiting";
  const courierPos =
    stage === "waiting" || stage === "live"
      ? (courier ?? tracking?.courier ?? null)
      : null;
  const destination =
    tracking?.destination ?? parseGeoPoint(order.delivery_lat, order.delivery_lng);
  const origin =
    stage === "live" || stage === "delivered" ? (tracking?.origin ?? null) : null;
  const restaurantName = tracking?.restaurantName ?? "Restoran";
  const logoUrl = tracking?.restaurantLogoUrl ?? null;
  const courierName = order.courier_name?.trim() || "Kuryer";

  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<google.maps.Map | null>(null);
  const markersRef = useRef<{
    origin?: HtmlMapMarker;
    originIdentity?: string;
    destination?: HtmlMapMarker;
    courier?: HtmlMapMarker;
  }>({});
  const linesRef = useRef<{
    planned?: google.maps.Polyline;
    remaining?: google.maps.Polyline;
  }>({});
  const bearingRef = useRef<{ last: LatLng | null; deg: number }>({
    last: null,
    deg: 0,
  });
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const [follow, setFollow] = useState(true);
  const followRef = useRef(true);

  useEffect(() => {
    followRef.current = follow;
  }, [follow]);

  useEffect(() => {
    let cancelled = false;
    loadGoogleMaps()
      .then(() => {
        if (!cancelled) setReady(true);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Xarita BIR MARTA yaratiladi — keyin faqat belgilar va yo'llar yangilanadi.
  useEffect(() => {
    const el = containerRef.current;
    if (!ready || !el) return;
    const map = new google.maps.Map(el, {
      center: CHUST_CENTER,
      zoom: 14,
      disableDefaultUI: true,
      clickableIcons: false,
      keyboardShortcuts: false,
    });
    // Mijoz xaritani o'zi surdi — kamera endi uni tortib qaytarmaydi.
    const drag = map.addListener("dragstart", () => setFollow(false));
    mapRef.current = map;
    return () => {
      drag.remove();
      const m = markersRef.current;
      m.origin?.remove();
      m.destination?.remove();
      m.courier?.remove();
      linesRef.current.planned?.setMap(null);
      linesRef.current.remaining?.setMap(null);
      markersRef.current = {};
      linesRef.current = {};
      mapRef.current = null;
    };
  }, [ready]);

  /** Kamerada ko'rinishi kerak bo'lgan nuqtalar (mobil ilova bilan bir xil). */
  function focusPoints(): LatLng[] {
    switch (stage) {
      case "delivered":
        return [
          ...(tracking?.plannedRoute?.points ?? []),
          ...(origin ? [origin] : []),
          ...(destination ? [destination] : []),
        ];
      case "live":
        return [
          ...(courierPos ? [courierPos] : []),
          ...(destination ? [destination] : []),
          ...(tracking?.remainingRoute?.points ?? []),
        ];
      default:
        return [
          ...(courierPos ? [courierPos] : []),
          ...(destination ? [destination] : []),
        ];
    }
  }

  const courierKey = courierPos ? `${courierPos.lat},${courierPos.lng}` : "";
  const destinationKey = destination ? `${destination.lat},${destination.lng}` : "";
  const originKey = origin ? `${origin.lat},${origin.lng}` : "";

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    const markers = markersRef.current;

    // A — restoran (logosi; logo keyinroq kelsa belgi qayta quriladi).
    const originIdentity = `${logoUrl ?? ""}|${restaurantName}`;
    if (origin) {
      if (!markers.origin || markers.originIdentity !== originIdentity) {
        markers.origin?.remove();
        markers.origin = createHtmlMapMarker(
          map,
          origin,
          restaurantMarker(logoUrl, restaurantName),
          1,
        );
        markers.originIdentity = originIdentity;
      } else {
        markers.origin.setPosition(origin);
      }
    } else {
      markers.origin?.remove();
      markers.origin = undefined;
    }

    // B — mijoz manzili.
    if (destination) {
      if (!markers.destination) {
        markers.destination = createHtmlMapMarker(map, destination, homeMarker(), 2);
      } else {
        markers.destination.setPosition(destination);
      }
    } else {
      markers.destination?.remove();
      markers.destination = undefined;
    }

    // Kuryer — harakat yo'nalishiga buriladi; birinchi marta qolgan yo'lning
    // boshidan (yoki manzil tomonga).
    if (courierPos) {
      const b = bearingRef.current;
      if (!b.last) {
        const road = tracking?.remainingRoute?.points;
        if (road && road.length >= 2) b.deg = bearingDegrees(road[0], road[1]);
        else if (destination) b.deg = bearingDegrees(courierPos, destination);
        b.last = courierPos;
      } else if (metersBetween(b.last, courierPos) >= MIN_MOVE_FOR_BEARING_M) {
        b.deg = bearingDegrees(b.last, courierPos);
        b.last = courierPos;
      }
      if (!markers.courier) {
        markers.courier = createHtmlMapMarker(map, courierPos, carMarker(courierName), 3);
      } else {
        markers.courier.setPosition(courierPos);
      }
      markers.courier.setRotation(b.deg);
    } else {
      markers.courier?.remove();
      markers.courier = undefined;
      bearingRef.current = { last: null, deg: 0 };
    }

    // Yo'llar: yetkazilganda bosib o'tilgan yo'l brend rangida; yo'lda —
    // kulrang "butun yo'l", ustida qolgan qism.
    const lines = linesRef.current;
    lines.planned = syncPolyline(
      map,
      lines.planned,
      stage === "live" || stage === "delivered" ? (tracking?.plannedRoute ?? null) : null,
      {
        strokeColor: stage === "delivered" ? BRAND : PLANNED_LINE,
        strokeWeight: stage === "delivered" ? 6 : 5,
        zIndex: 1,
      },
    );
    lines.remaining = syncPolyline(
      map,
      lines.remaining,
      stage === "live" ? (tracking?.remainingRoute ?? null) : null,
      { strokeColor: BRAND, strokeWeight: 6, zIndex: 2 },
    );

    if (followRef.current) fitTo(map, focusPoints());
    // `focusPoints` shu qiymatlardan hisoblanadi — kalitlar yetarli.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, stage, courierKey, destinationKey, originKey, logoUrl, restaurantName, courierName, tracking]);

  const header = headerFor(stage, order.status, tracking, courierName, courierPos !== null, now);
  const dark = variant === "dark";
  const muted = dark ? "text-white/50" : "text-neutral-500";

  return (
    <div
      className={
        dark
          ? "rounded-2xl border border-white/10 bg-white/[0.03] p-4"
          : "rounded-2xl border border-neutral-200 p-3.5 dark:border-neutral-800"
      }
    >
      <p className={`text-[13px] font-semibold ${muted}`}>{header.caption}</p>
      <p
        className={`mt-1 font-extrabold ${
          stage === "live" ? "text-xl text-brand" : "text-[15px]"
        }`}
      >
        {header.headline}
      </p>
      {header.detail && (
        <p className={`mt-0.5 text-[13px] ${muted}`}>{header.detail}</p>
      )}

      <div
        className={`relative mt-3 w-full overflow-hidden rounded-xl ${mapHeightClassName} ${
          dark ? "bg-white/5" : "bg-neutral-100 dark:bg-neutral-800"
        }`}
      >
        {failed ? (
          <div className="flex h-full items-center justify-center text-sm text-neutral-400">
            Xarita yuklanmadi
          </div>
        ) : (
          <>
            <div ref={containerRef} className="absolute inset-0" />
            {!ready && (
              <div className="absolute inset-0 flex items-center justify-center text-sm text-neutral-400">
                Xarita yuklanmoqda…
              </div>
            )}
            {ready && !follow && (
              <button
                type="button"
                onClick={() => {
                  setFollow(true);
                  if (mapRef.current) fitTo(mapRef.current, focusPoints());
                }}
                aria-label="Kuryerni kuzatish"
                title="Kuryerni kuzatish"
                className="absolute bottom-2.5 right-2.5 flex h-11 w-11 items-center justify-center rounded-full bg-white text-[#111827] shadow-md active:scale-95"
              >
                <LocateFixed size={20} />
              </button>
            )}
          </>
        )}
      </div>

      {(stage === "live" || stage === "delivered") && (
        <div className={`mt-2.5 flex flex-wrap gap-x-4 gap-y-1.5 text-[12.5px] ${muted}`}>
          <Legend icon={<LogoDot url={logoUrl} />} label={restaurantName} />
          <Legend
            icon={
              <span
                className="flex h-5 w-5 items-center justify-center rounded-full"
                style={{ background: BRAND }}
              >
                <Home size={12} className="text-white" />
              </span>
            }
            label="Sizning manzilingiz"
          />
          {stage === "live" && (
            <Legend
              // eslint-disable-next-line @next/next/no-img-element -- statik fayl
              icon={<img src={CAR_SRC} alt="" className="h-5 w-auto" />}
              label="Kuryer"
            />
          )}
        </div>
      )}
    </div>
  );
}

function headerFor(
  stage: Stage,
  status: string,
  t: DeliveryTracking | null,
  courierName: string,
  hasCourier: boolean,
  now: Date,
): { caption: string; headline: string; detail: string | null } {
  switch (stage) {
    case "pending":
      return {
        caption: "Yetkazish manzili",
        headline:
          status === "created"
            ? "Restoran buyurtmani tasdiqlashi kutilmoqda"
            : "Kuryer biriktirilishi kutilmoqda",
        detail: null,
      };
    case "waiting":
      return {
        caption: "Kuryer qayerda",
        headline: hasCourier
          ? `${courierName} restoranda buyurtmani oladi`
          : "Kuryer joylashuvi kutilmoqda…",
        detail: null,
      };
    case "live": {
      const left = remainingSeconds(t, now);
      let headline: string;
      let detail: string | null = null;
      if (left === null) {
        headline = "Yetib kelish vaqti hisoblanmoqda…";
      } else if (left === 0) {
        headline = "Kuryer yetib keldi";
      } else {
        headline = `${etaText(left)}da yetib keladi`;
        detail = `Taxminan ${clockText(new Date(now.getTime() + left * 1000))} gacha`;
      }
      const remaining = t?.remainingRoute;
      if (remaining && left !== 0) {
        const km = `${distanceText(remaining.distanceMeters)} qoldi`;
        detail = detail ? `${detail} · ${km}` : km;
      }
      return { caption: `${courierName} yo'lda`, headline, detail };
    }
    case "delivered": {
      const at = t?.deliveredAt ?? null;
      const parts = [
        deliveryDurationText(t?.pickedUpAt ?? null, at),
        t?.plannedRoute ? distanceText(t.plannedRoute.distanceMeters) : null,
      ].filter((x): x is string => Boolean(x));
      return {
        caption: "Yetkazish yo'li",
        headline: at ? `Yetkazildi · ${clockText(at)}` : "Yetkazildi",
        detail: parts.length > 0 ? parts.join(" · ") : null,
      };
    }
  }
}

function Legend({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <span className="flex min-w-0 items-center gap-1.5">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center">{icon}</span>
      <span className="truncate">{label}</span>
    </span>
  );
}

/** Restoran logosi (kichik); bo'lmasa yoki yuklanmasa — do'kon belgisi. */
function LogoDot({ url }: { url: string | null }) {
  const [broken, setBroken] = useState(false);
  if (!url || broken) {
    return (
      <span className="flex h-5 w-5 items-center justify-center rounded-full bg-[#111827]">
        <Store size={11} className="text-white" />
      </span>
    );
  }
  return (
    // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik (R2/lokal disk)
    <img
      src={fullImageUrl(url)}
      alt=""
      onError={() => setBroken(true)}
      className="h-5 w-5 rounded-full object-cover"
    />
  );
}

// ── Xarita belgilari (DOM, `innerHTML` siz) ─────────────────────────────

function circle(size: number, background: string): HTMLDivElement {
  const el = document.createElement("div");
  Object.assign(el.style, {
    width: `${size}px`,
    height: `${size}px`,
    borderRadius: "9999px",
    background,
    border: "3px solid #fff",
    boxShadow: "0 2px 6px rgba(0,0,0,.35)",
    boxSizing: "border-box",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
  });
  return el;
}

/** Restoran belgisi: logosi, bo'lmasa yoki yuklanmasa — "A". */
function restaurantMarker(logoUrl: string | null, name: string): HTMLElement {
  const el = circle(40, INK);
  el.title = name;
  const letter = () => {
    el.textContent = "A";
    Object.assign(el.style, { color: "#fff", fontWeight: "800", fontSize: "17px" });
  };
  if (!logoUrl) {
    letter();
    return el;
  }
  const img = document.createElement("img");
  img.src = fullImageUrl(logoUrl);
  img.alt = "";
  img.decoding = "async";
  img.referrerPolicy = "no-referrer";
  Object.assign(img.style, { width: "100%", height: "100%", objectFit: "cover" });
  img.onerror = () => {
    img.remove();
    letter();
  };
  el.appendChild(img);
  return el;
}

const SVG_NS = "http://www.w3.org/2000/svg";

/** Uy belgisi — brend rangidagi doira (lucide `house`). */
function homeMarker(): HTMLElement {
  const el = circle(38, BRAND);
  el.title = "Sizning manzilingiz";
  const svg = document.createElementNS(SVG_NS, "svg");
  const attrs: Record<string, string> = {
    width: "19",
    height: "19",
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "#fff",
    "stroke-width": "2.5",
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
  };
  for (const [k, v] of Object.entries(attrs)) svg.setAttribute(k, v);
  for (const d of [
    "M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8",
    "M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z",
  ]) {
    const path = document.createElementNS(SVG_NS, "path");
    path.setAttribute("d", d);
    svg.appendChild(path);
  }
  el.appendChild(svg);
  return el;
}

function carMarker(name: string): HTMLElement {
  const img = document.createElement("img");
  img.src = CAR_SRC;
  img.alt = "";
  img.title = name;
  Object.assign(img.style, {
    width: "22px",
    height: "48px",
    display: "block",
    filter: "drop-shadow(0 2px 3px rgba(0,0,0,.35))",
    transition: "transform .4s ease",
  });
  return img;
}

function syncPolyline(
  map: google.maps.Map,
  line: google.maps.Polyline | undefined,
  route: TrackedRoute | null,
  style: { strokeColor: string; strokeWeight: number; zIndex: number },
): google.maps.Polyline | undefined {
  if (!route) {
    line?.setMap(null);
    return undefined;
  }
  if (!line) {
    return new google.maps.Polyline({
      map,
      path: route.points,
      clickable: false,
      strokeOpacity: 1,
      ...style,
    });
  }
  line.setOptions({ path: route.points, ...style });
  if (!line.getMap()) line.setMap(map);
  return line;
}

function fitTo(map: google.maps.Map, points: LatLng[]) {
  if (points.length === 0) return;
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
  if (maxLat - minLat < 1e-4 && maxLng - minLng < 1e-4) {
    map.panTo(points[0]);
    map.setZoom(16);
    return;
  }
  map.fitBounds({ south: minLat, west: minLng, north: maxLat, east: maxLng }, 48);
}
