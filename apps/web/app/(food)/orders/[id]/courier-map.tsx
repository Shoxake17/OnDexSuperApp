"use client";

import { useEffect, useRef, useState } from "react";
import { loadGoogleMaps } from "@/lib/gmaps";

// _CourierMap (tracking_screen.dart) bilan bir xil: kuryer (to'q sariq) +
// yetkazish manzili (ko'k) belgilari, kamera YANGI koordinataga SILLIQ
// suriladi (xarita qayta yaratilmaydi). ATAYLAB yo'l chizig'i/ETA yo'q —
// mijozga faqat "kuryer hozir qayerda" yetarli.
export default function CourierMap({
  courier,
  destination,
  heightClassName = "h-[200px]",
}: {
  /**
   * Kuryer koordinatasi. `null` — kuryer hali yo'lga chiqmagan: xarita
   * baribir chiziladi, lekin FAQAT yetkazish manzili belgisi bilan.
   *
   * Avval bu maydon majburiy edi va xarita faqat kuryer paydo
   * bo'lgandagina ko'rsatilardi. Kompyuter ko'rinishida esa manzil
   * xaritasi buyurtma holatining bir qismi sifatida DOIM kerak —
   * mijoz "qayerga yetkaziladi" ni birinchi daqiqadanoq ko'rishi
   * kerak.
   */
  courier: { lat: number; lng: number } | null;
  destination: { lat: number; lng: number } | null;
  /** Kompyuterda xarita balandroq bo'ladi. */
  heightClassName?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<google.maps.Map | null>(null);
  const courierMarkerRef = useRef<google.maps.Marker | null>(null);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);

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

  useEffect(() => {
    if (!ready || !containerRef.current) return;

    // Markaz: kuryer bo'lsa u, aks holda yetkazish manzili. Ikkalasi
    // ham bo'lmasa xarita yaratilmaydi — markazsiz xarita okean
    // o'rtasini ko'rsatardi.
    const center = courier ?? destination;
    if (!center) return;

    if (!mapRef.current) {
      mapRef.current = new google.maps.Map(containerRef.current, {
        center,
        zoom: 15,
        zoomControl: false,
        mapTypeControl: false,
        streetViewControl: false,
        fullscreenControl: false,
      });
    }

    if (!courier) return;
    if (!courierMarkerRef.current) {
      courierMarkerRef.current = new google.maps.Marker({
        position: courier,
        map: mapRef.current,
        icon: "http://maps.google.com/mapfiles/ms/icons/orange-dot.png",
      });
    } else {
      courierMarkerRef.current.setPosition(courier);
    }
    mapRef.current.panTo(courier);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, courier?.lat, courier?.lng, destination?.lat, destination?.lng]);

  // Yetkazish manzili belgisi — ALOHIDA effekt.
  //
  // Avval u xarita yaratilayotgan effekt ichida edi va o'sha effekt
  // faqat `[ready, courier.lat, courier.lng]`ga bog'langan edi. Buyurtma
  // ma'lumoti kechroq kelib `destination` keyin paydo bo'lsa (odatiy
  // holat: kuryer koordinatasi WS orqali darhol, manzil esa so'rov
  // javobida keladi), effekt qayta ishga tushsa ham `mapRef.current`
  // allaqachon mavjud bo'lgani uchun `else` shoxiga tushardi — ya'ni
  // manzil belgisi HECH QACHON chizilmasdi.
  const destMarkerRef = useRef<google.maps.Marker | null>(null);
  useEffect(() => {
    if (!ready || !mapRef.current || !destination) return;
    if (!destMarkerRef.current) {
      destMarkerRef.current = new google.maps.Marker({
        position: destination,
        map: mapRef.current,
        icon: "http://maps.google.com/mapfiles/ms/icons/blue-dot.png",
      });
    } else {
      destMarkerRef.current.setPosition(destination);
    }
  }, [ready, destination]);

  if (failed) return null;

  return (
    <div
      ref={containerRef}
      className={`${heightClassName} w-full overflow-hidden rounded-2xl bg-neutral-100 dark:bg-neutral-800`}
    />
  );
}
