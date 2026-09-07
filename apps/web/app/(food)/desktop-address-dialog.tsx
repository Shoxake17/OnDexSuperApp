"use client";

import { Crosshair, MapPin, Minus, Plus, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { getCurrentPosition } from "@/lib/geo";
import { loadGoogleMaps, UnauthorizedError } from "@/lib/gmaps";
import { cityFor, DEFAULT_CENTER, SERVICE_CITIES } from "@/lib/service-area";

// Yetkazish manzili — KOMPYUTER uchun modal oyna (image/maps.png).
//
// ┌─ NEGA MODAL, ALOHIDA SAHIFA EMAS ──────────────────────────────────┐
// Telefonda `/address` to'liq ekranli sahifa — u yerda boshqa iloji
// yo'q. Kompyuterda esa manzil tanlash — QISQA, oraliq amal: mijoz
// menyuni yig'ayotgan bo'ladi va sahifa almashsa, o'sha kontekst
// yo'qoladi. Namunada (image/maps.png) ham u aynan modal.
// Mobil sahifaga TEGILMAYDI — u o'z holicha ishlaydi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────────┐
// Maps kaliti bu yerga TUSHMAYDI: `loadGoogleMaps` uni serverdan
// (`/api/proxy/config/maps`, auth talab qiladi) oladi va faqat skript
// manzilida ishlatadi. Manzil `POST /api/proxy/me/address` orqali
// saqlanadi — ya'ni sessiya cookie'si bilan, o'z profiliga.
// Hudud tekshiruvi IKKI joyda: tugma o'chiriladi VA saqlashdan oldin
// qayta tekshiriladi (`lib/service-area.ts`).
// └────────────────────────────────────────────────────────────────────┘

type AddressDetails = {
  text?: string;
  lat?: number;
  lng?: number;
  entrance?: string;
  floor?: string;
  apartment?: string;
  intercom?: string;
  comment?: string;
};

export default function DesktopAddressDialog({
  onClose,
  onSaved,
}: {
  onClose: () => void;
  onSaved?: (text: string) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<google.maps.Map | null>(null);
  const geocodeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastGeocoded = useRef("");

  const [mapFailed, setMapFailed] = useState(false);
  const [mapReady, setMapReady] = useState(false);
  const [addressLoaded, setAddressLoaded] = useState(false);
  const [savedPoint, setSavedPoint] = useState<{ lat: number; lng: number } | null>(null);

  const [addressText, setAddressText] = useState("");
  const [resolving, setResolving] = useState(false);
  const [outOfArea, setOutOfArea] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [entrance, setEntrance] = useState("");
  const [floor, setFloor] = useState("");
  const [apartment, setApartment] = useState("");
  const [intercom, setIntercom] = useState("");
  const [comment, setComment] = useState("");

  // Escape bilan yopish + oyna ochiq turganda orqadagi sahifa
  // skrollini to'xtatish (modal odatiy xatti-harakati).
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [onClose]);

  const reverseGeocode = useCallback((lat: number, lng: number) => {
    // Hudud tekshiruvi DARHOL (geokodlashni kutmasdan) — javob bir
    // zumda ko'rinadi.
    setOutOfArea(cityFor(lat, lng) === null);

    const key = `${lat.toFixed(5)},${lng.toFixed(5)}`;
    if (key === lastGeocoded.current) return;
    lastGeocoded.current = key;
    if (geocodeTimer.current) clearTimeout(geocodeTimer.current);
    setResolving(true);
    geocodeTimer.current = setTimeout(() => {
      fetch(`/api/proxy/geocode/reverse?lat=${lat}&lng=${lng}`)
        .then((r) => (r.ok ? r.json() : null))
        .then((d) =>
          setAddressText(d?.address || `${lat.toFixed(5)}, ${lng.toFixed(5)}`),
        )
        .catch(() => setAddressText(`${lat.toFixed(5)}, ${lng.toFixed(5)}`))
        .finally(() => setResolving(false));
    }, 500);
  }, []);

  useEffect(() => {
    fetch("/api/proxy/me/address")
      .then((r) => (r.ok ? r.json() : null))
      .then((a: AddressDetails | null) => {
        if (a) {
          setEntrance(a.entrance ?? "");
          setFloor(a.floor ?? "");
          setApartment(a.apartment ?? "");
          setIntercom(a.intercom ?? "");
          setComment(a.comment ?? "");
          if (a.lat && a.lng) setSavedPoint({ lat: a.lat, lng: a.lng });
        }
      })
      .catch(() => {})
      .finally(() => setAddressLoaded(true));
  }, []);

  useEffect(() => {
    let cancelled = false;
    loadGoogleMaps()
      .then(() => !cancelled && setMapReady(true))
      .catch((e) => {
        if (cancelled) return;
        // Sessiya oyna ochiq turganda tugagan bo'lsa — "internet
        // uzildi" deb chalg'itmasdan kirish sahifasiga.
        if (e instanceof UnauthorizedError) {
          window.location.replace(
            `/login?next=${encodeURIComponent(window.location.pathname)}`,
          );
          return;
        }
        setMapFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Xarita FAQAT saqlangan manzil so'rovi tugagach quriladi — markaz
  // bir marta to'g'ri qo'yiladi va "sakrash" ko'rinmaydi.
  useEffect(() => {
    if (!mapReady || !addressLoaded || !containerRef.current || mapRef.current) {
      return;
    }
    const map = new google.maps.Map(containerRef.current, {
      center: savedPoint ?? DEFAULT_CENTER,
      zoom: 17,
      disableDefaultUI: true,
      keyboardShortcuts: false,
      gestureHandling: "greedy",
      clickableIcons: false,
    });
    mapRef.current = map;

    // Xizmat hududi ko'rinadi — mijoz tanlashdan OLDIN biladi.
    for (const c of SERVICE_CITIES) {
      new google.maps.Circle({
        map,
        center: { lat: c.lat, lng: c.lng },
        radius: c.radiusKm * 1000,
        strokeColor: "#F4511E",
        strokeOpacity: 0.95,
        strokeWeight: 3,
        fillColor: "#F4511E",
        fillOpacity: 0.07,
        clickable: false,
      });
    }

    map.addListener("idle", () => {
      const c = map.getCenter();
      if (c) reverseGeocode(c.lat(), c.lng());
    });

    if (!savedPoint) {
      getCurrentPosition()
        .then(({ lat, lng }) => {
          map.setCenter({ lat, lng });
          map.setZoom(17);
        })
        .catch(() => {
          const c = map.getCenter();
          if (c) reverseGeocode(c.lat(), c.lng());
        });
    }
  }, [mapReady, addressLoaded, savedPoint, reverseGeocode]);

  function zoom(delta: number) {
    const map = mapRef.current;
    if (!map) return;
    map.setZoom((map.getZoom() ?? 17) + delta);
  }

  async function goToMyLocation() {
    setError(null);
    try {
      const { lat, lng } = await getCurrentPosition();
      mapRef.current?.panTo({ lat, lng });
      mapRef.current?.setZoom(17);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Joylashuvni aniqlab bo'lmadi");
    }
  }

  async function save() {
    const c = mapRef.current?.getCenter();
    if (!c) {
      setError("Xarita hali tayyor emas");
      return;
    }
    // Ikkinchi himoya qatlami — tugma o'chirilgan bo'lsa ham, hudud
    // tashqarisidagi manzil HECH QACHON saqlanmaydi.
    if (cityFor(c.lat(), c.lng()) === null) {
      setError("Bu manzilga hozircha yetkazmaymiz");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const text = addressText || `${c.lat().toFixed(5)}, ${c.lng().toFixed(5)}`;
      const res = await fetch("/api/proxy/me/address", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          lat: c.lat(),
          lng: c.lng(),
          text,
          entrance,
          floor,
          apartment,
          intercom,
          comment,
        }),
      });
      if (!res.ok) throw new Error("Manzilni saqlab bo'lmadi");
      onSaved?.(text);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Manzilni saqlab bo'lmadi");
      setSaving(false);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Yetkazish manzili"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6"
      // Fon bosilganda yopiladi; ichkariga bosish yopmaydi.
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="flex max-h-[88vh] w-full max-w-[860px] flex-col overflow-hidden rounded-3xl border border-white/10 bg-[#1e1e1e] text-white shadow-2xl">
        <div className="flex items-center justify-between gap-4 px-6 pb-4 pt-5">
          <h2 className="text-[22px] font-extrabold">
            Buyurtma qayerga yetkazilsin?
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Yopish"
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white/60 transition-colors hover:bg-white/10 hover:text-white"
          >
            <X size={20} />
          </button>
        </div>

        {/* Manzil satri + "Tayyor" — namunadagi kabi xarita USTIDA. */}
        <div className="flex items-center gap-3 px-6 pb-4">
          <div className="flex h-12 min-w-0 flex-1 items-center gap-2.5 rounded-2xl bg-white/10 px-4">
            <MapPin size={18} className="shrink-0 text-white/50" />
            <span className="truncate text-[15px]">
              {resolving && !addressText
                ? "Aniqlanmoqda…"
                : addressText || "Xaritani surib, joyni belgilang"}
            </span>
          </div>
          <button
            type="button"
            onClick={save}
            disabled={saving || outOfArea || mapFailed}
            className="h-12 shrink-0 rounded-2xl bg-brand px-7 text-[15px] font-bold text-white transition-colors hover:bg-brand-light disabled:cursor-not-allowed disabled:bg-white/10 disabled:text-white/40"
          >
            {saving ? "Saqlanmoqda…" : "Tayyor"}
          </button>
        </div>

        <div className="ondex-scroll min-h-0 flex-1 overflow-y-auto px-6 pb-6">
          {/* ── Xarita ─────────────────────────────────────────────── */}
          <div className="relative h-[380px] w-full overflow-hidden rounded-2xl bg-white/5">
            {mapFailed ? (
              <div className="flex h-full items-center justify-center px-8 text-center text-sm text-white/45">
                Xaritani yuklab bo&apos;lmadi. Internet aloqasini tekshiring.
              </div>
            ) : (
              <>
                <div ref={containerRef} className="h-full w-full" />

                {/* Markazdagi QOTIRILGAN belgi: xarita suriladi, belgi
                    joyida qoladi — tanlangan nuqta doim markazda
                    (mobil sahifadagi bilan bir xil naqsh). */}
                <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
                  <div className="-translate-y-4 flex flex-col items-center">
                    <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-brand shadow-lg">
                      <MapPin size={20} className="text-white" />
                    </div>
                    <div className="h-4 w-0.5 bg-brand" />
                  </div>
                </div>

                {/* Zoom va "mening joylashuvim" — namunadagi joylashuv. */}
                <div className="absolute bottom-4 right-4 flex flex-col gap-2">
                  <div className="overflow-hidden rounded-xl bg-[#1e1e1e] shadow-lg ring-1 ring-white/10">
                    <button
                      type="button"
                      onClick={() => zoom(1)}
                      aria-label="Kattalashtirish"
                      className="flex h-10 w-10 items-center justify-center hover:bg-white/10"
                    >
                      <Plus size={18} />
                    </button>
                    <div className="h-px bg-white/10" />
                    <button
                      type="button"
                      onClick={() => zoom(-1)}
                      aria-label="Kichraytirish"
                      className="flex h-10 w-10 items-center justify-center hover:bg-white/10"
                    >
                      <Minus size={18} />
                    </button>
                  </div>
                  <button
                    type="button"
                    onClick={goToMyLocation}
                    aria-label="Mening joylashuvim"
                    className="flex h-10 w-10 items-center justify-center rounded-xl bg-[#1e1e1e] shadow-lg ring-1 ring-white/10 hover:bg-white/10"
                  >
                    <Crosshair size={18} />
                  </button>
                </div>
              </>
            )}
          </div>

          {outOfArea && (
            <p className="mt-3 rounded-xl bg-white/5 px-4 py-3 text-[13px] text-white/60">
              Bu yerda hali ishlamaymiz. Hozircha faqat{" "}
              <span className="font-semibold text-white">Chust</span> shahriga
              yetkazamiz — chegara xaritada belgilangan.
            </p>
          )}

          {/* ── Kuryer uchun tafsilotlar ───────────────────────────── */}
          <div className="mt-4 grid grid-cols-4 gap-3">
            <Field label="Podyezd" value={entrance} onChange={setEntrance} />
            <Field label="Qavat" value={floor} onChange={setFloor} />
            <Field label="Kvartira" value={apartment} onChange={setApartment} />
            <Field label="Domofon" value={intercom} onChange={setIntercom} />
          </div>
          <div className="mt-3">
            <Field
              label="Kuryer uchun izoh"
              value={comment}
              onChange={setComment}
            />
          </div>

          {error && <p className="mt-3 text-sm text-red-400">{error}</p>}
        </div>
      </div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-[12px] font-medium text-white/40">
        {label}
      </span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="h-11 w-full rounded-xl bg-white/[0.07] px-3.5 text-[14px] outline-none transition-colors focus:bg-white/10"
      />
    </label>
  );
}
