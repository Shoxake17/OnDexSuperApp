"use client";

import { ArrowLeft, LocateFixed } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { getCurrentPosition } from "@/lib/geo";
import { loadGoogleMaps } from "@/lib/gmaps";
import { goBack } from "@/lib/nav";
import { cityFor, DEFAULT_CENTER, SERVICE_CITIES } from "@/lib/service-area";
import { AppButton } from "../ui";

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

// Boshlang'ich markaz va xizmat hududi — `lib/service-area.ts` da
// (hozircha faqat Chust). Yangi shahar qo'shish uchun shu faylga
// bitta qator qo'shiladi.
const CHUST_CENTER = DEFAULT_CENTER;

// map.png namunasiga mos: to'liq ekranli xarita, MARKAZDA qotirilgan
// belgi (Yandex Go uslubi — belgi surilmaydi, XARITA suriladi), pastda
// manzil + kuryer uchun tafsilotlar paneli va "Tayyor" tugmasi.
function AddressPicker() {
  const router = useRouter();
  const searchParams = useSearchParams();
  // ?next=/checkout — "Tayyor"dan keyin qayerga o'tish. Standart: checkout.
  const next = searchParams.get("next") ?? "/checkout";

  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<google.maps.Map | null>(null);
  const geocodeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastGeocoded = useRef<string>("");

  const [mapReady, setMapReady] = useState(false);
  const [mapFailed, setMapFailed] = useState(false);
  const [addressText, setAddressText] = useState("");
  const [resolving, setResolving] = useState(false);
  // Xarita markazi xizmat hududida (Chust) emasligini bildiradi.
  const [outOfArea, setOutOfArea] = useState(false);
  const [locating, setLocating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [entrance, setEntrance] = useState("");
  const [floor, setFloor] = useState("");
  const [apartment, setApartment] = useState("");
  const [intercom, setIntercom] = useState("");
  const [comment, setComment] = useState("");

  // Xarita markazidagi koordinata bo'yicha manzil matnini aniqlash.
  // Debounce — surish paytida har kadrda so'rov yubormaslik uchun.
  const reverseGeocode = useCallback((lat: number, lng: number) => {
    // Hudud tekshiruvi — DARHOL (geokodlashni kutmasdan), shunda
    // foydalanuvchi xaritani surganda javob bir zumda ko'rinadi.
    setOutOfArea(cityFor(lat, lng) === null);

    const key = `${lat.toFixed(5)},${lng.toFixed(5)}`;
    if (key === lastGeocoded.current) return;
    lastGeocoded.current = key;
    if (geocodeTimer.current) clearTimeout(geocodeTimer.current);
    setResolving(true);
    geocodeTimer.current = setTimeout(() => {
      fetch(`/api/proxy/geocode/reverse?lat=${lat}&lng=${lng}`)
        .then((r) => (r.ok ? r.json() : null))
        .then((d) => setAddressText(d?.address || `${lat.toFixed(5)}, ${lng.toFixed(5)}`))
        .catch(() => setAddressText(`${lat.toFixed(5)}, ${lng.toFixed(5)}`))
        .finally(() => setResolving(false));
    }, 500);
  }, []);

  // Saqlangan manzil koordinatasi — xarita hali yaratilmagan bo'lishi
  // mumkin (bu effekt mount'da, xarita esa skript yuklangach quriladi),
  // shuning uchun to'g'ridan-to'g'ri xaritaga yozmaymiz, holatda saqlab
  // qo'yamiz va xarita tayyor bo'lganda qo'llaymiz.
  const [savedPoint, setSavedPoint] = useState<{ lat: number; lng: number } | null>(null);
  const [addressLoaded, setAddressLoaded] = useState(false);

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
      .catch(() => !cancelled && setMapFailed(true));
    return () => {
      cancelled = true;
    };
  }, []);

  // Xarita FAQAT saqlangan manzil so'rovi tugagach quriladi — shunda
  // boshlang'ich markazni bir marta to'g'ri qo'yamiz va foydalanuvchi
  // "sakrab o'tish"ni ko'rmaydi.
  useEffect(() => {
    if (!mapReady || !addressLoaded || !containerRef.current || mapRef.current) return;
    // Xarita uslubi ATAYLAB o'zgartirilmaydi — Google'ning standart OQ
    // ko'rinishi. Profil → "Manzillarim" ekrani (address_screen.dart) ham
    // aynan shunday (`MapType.normal`), ikkalasi bir xil ko'rinishi kerak.
    const map = new google.maps.Map(containerRef.current, {
      center: savedPoint ?? CHUST_CENTER,
      zoom: 17,
      disableDefaultUI: true,
      keyboardShortcuts: false,
      gestureHandling: "greedy",
      clickableIcons: false,
    });
    mapRef.current = map;

    // Xizmat hududi xaritada KO'RINADI — sariq shaffof doira + chegara.
    // Foydalanuvchi qayerda ishlashimizni tanlashdan OLDIN ko'radi
    // (Yandex Eats'da ham zona shunday ko'rsatiladi).
    for (const c of SERVICE_CITIES) {
      new google.maps.Circle({
        map,
        center: { lat: c.lat, lng: c.lng },
        radius: c.radiusKm * 1000,
        strokeColor: "#FFD100",
        strokeOpacity: 0.95,
        strokeWeight: 3,
        fillColor: "#FFD100",
        // Juda past shaffoflik: yaqin zoomda (17) butun ekran doira
        // ichida bo'ladi va kuchli to'ldirish xaritani sarg'aytirib,
        // ko'chalarni o'qishni qiyinlashtirardi. Chegara chizig'i esa
        // uzoqroq zoomda — aynan kerak bo'lganda — yaqqol ko'rinadi.
        fillOpacity: 0.07,
        clickable: false,
      });
    }

    map.addListener("idle", () => {
      const c = map.getCenter();
      if (c) reverseGeocode(c.lat(), c.lng());
    });
    // Saqlangan manzil BO'LMASA — joriy joylashuvga o'tamiz (birinchi
    // buyurtma holati). Saqlangan bo'lsa unga tegmaymiz, aks holda
    // foydalanuvchi oldin tanlagan nuqta yo'qolardi.
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

  async function goToMyLocation() {
    setLocating(true);
    setError(null);
    try {
      const { lat, lng } = await getCurrentPosition();
      mapRef.current?.panTo({ lat, lng });
      mapRef.current?.setZoom(17);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Joylashuvni aniqlab bo'lmadi");
    } finally {
      setLocating(false);
    }
  }

  async function save() {
    const c = mapRef.current?.getCenter();
    if (!c) {
      setError("Xarita hali tayyor emas");
      return;
    }
    // Ikkinchi himoya qatlami — tugma allaqachon o'chirilgan bo'lsa ham,
    // hudud tashqarisidagi manzil HECH QACHON saqlanmasligi kerak.
    if (cityFor(c.lat(), c.lng()) === null) {
      setError("Bu manzilga hozircha yetkazmaymiz");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const res = await fetch("/api/proxy/me/address", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          lat: c.lat(),
          lng: c.lng(),
          text: addressText || `${c.lat().toFixed(5)}, ${c.lng().toFixed(5)}`,
          entrance,
          floor,
          apartment,
          intercom,
          comment,
        }),
      });
      if (!res.ok) throw new Error("Manzilni saqlab bo'lmadi");
      router.push(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Manzilni saqlab bo'lmadi");
      setSaving(false);
    }
  }

  return (
    <div className="flex h-dvh flex-col bg-[#121212]">
      <div className="relative min-h-0 flex-1">
        {mapFailed ? (
          <div className="flex h-full items-center justify-center px-8 text-center text-sm text-neutral-400">
            Xaritani yuklab bo'lmadi. Internet aloqasini tekshiring.
          </div>
        ) : (
          <div ref={containerRef} className="h-full w-full" />
        )}

        {/* Markazdagi QOTIRILGAN belgi — xarita surilganda u qimirlamaydi,
            tanlangan nuqta doim ekran markazida (map.png naqshi). */}
        {!mapFailed && (
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
            <div className="-translate-y-5 flex flex-col items-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#FFD100] shadow-lg">
                <svg width="26" height="26" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M14 5.5a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z M9.2 8.3 6 10.2v3.3h2v-2.1l2-1.2-.7 3.6 3.4 3v4.7h2v-6.2l-2.6-2.4.9-4.1c1 1.2 2.6 2 4.3 2v-2c-1.5 0-2.8-.8-3.5-2l-1-1.6c-.4-.6-1-1-1.7-1-.3 0-.5.1-.8.2L5.5 6.4v4.1h2V7.8l1.7-.7"
                    fill="#111"
                  />
                </svg>
              </div>
              <div className="h-5 w-[3px] rounded-full bg-white/90" />
            </div>
          </div>
        )}

        {/* `safe-top` (padding) ISHLATILMAYDI — u ikonkani doira ichida
            pastga surib, markazdan chiqarib yuborardi. Doira oddiy
            joylashuv bilan qo'yiladi, ikonka esa flex bilan aniq
            markazda qoladi. */}
        <button
          type="button"
          onClick={() => goBack(() => router.back())}
          className="absolute left-3 top-3 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur"
          aria-label="Orqaga"
        >
          <ArrowLeft size={22} />
        </button>

        <button
          type="button"
          onClick={goToMyLocation}
          disabled={locating}
          className="absolute bottom-3 right-3 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur disabled:opacity-60"
          aria-label="Mening joylashuvim"
        >
          <LocateFixed size={22} className={locating ? "animate-pulse" : ""} />
        </button>
      </div>

      <div className="safe-bottom shrink-0 rounded-t-[20px] bg-[#1A1A1A] px-4 pt-4">
        {outOfArea ? (
          // Yandex Eats naqshi: xizmat qamrab olmagan hudud tanlansa —
          // aniq tushuntirish va davom etishga ruxsat berilmaydi.
          <>
            <p className="text-lg font-semibold text-white">
              Bu yerda hali ishlamaymiz
            </p>
            <p className="mt-0.5 text-sm text-neutral-400">
              Hozircha faqat <span className="text-white">Chust</span> shahriga
              yetkazamiz. Iltimos, Chust ichidan boshqa manzil tanlang.
            </p>
          </>
        ) : (
          <>
            <p className="truncate text-lg font-semibold text-white">
              {resolving && !addressText
                ? "Aniqlanmoqda..."
                : addressText || "Manzilni tanlang"}
            </p>
            <p className="mt-0.5 text-sm text-neutral-500">
              Xaritani surib, aniq joyni belgilang
            </p>
          </>
        )}

        {/* Hudud tashqarisida qo'shimcha maydonlarni ko'rsatishning
            ma'nosi yo'q — bunday manzilga baribir yetkazilmaydi. */}
        {!outOfArea && (
          <>
            <div className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3">
              <Field label="Podyezd" value={entrance} onChange={setEntrance} />
              <Field label="Qavat" value={floor} onChange={setFloor} />
              <Field label="Kvartira" value={apartment} onChange={setApartment} />
              <Field label="Domofon" value={intercom} onChange={setIntercom} />
            </div>
            <div className="mt-3">
              <Field label="Kuryer uchun izoh" value={comment} onChange={setComment} />
            </div>
          </>
        )}

        {error && <p className="mt-3 text-sm text-red-500">{error}</p>}

        {/* FAQAT saqlash paytida o'chiriladi. Avval `resolving` ham
            shartda edi, lekin manzil matni fon rejimida yangilanib
            turgani uchun tugma keraksiz o'chib qolardi — koordinata esa
            xarita markazidan HAR DOIM ma'lum, matn topilmasa `save()`
            koordinataning o'ziga tushadi. */}
        <div className="mb-1 mt-4">
          <AppButton onClick={save} disabled={saving || outOfArea}>
            {saving
              ? "Saqlanmoqda..."
              : outOfArea
                ? "Chust ichidan tanlang"
                : "Tayyor"}
          </AppButton>
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
    <label className="block border-b border-neutral-700 pb-1.5">
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={label}
        className="w-full bg-transparent text-base text-white outline-none placeholder:text-neutral-500"
      />
    </label>
  );
}

export default function AddressPage() {
  return (
    <Suspense
      fallback={
        <div className="flex h-dvh items-center justify-center bg-[#121212] text-neutral-500">
          Yuklanmoqda...
        </div>
      }
    >
      <AddressPicker />
    </Suspense>
  );
}
