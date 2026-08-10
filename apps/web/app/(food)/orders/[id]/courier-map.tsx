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
}: {
  courier: { lat: number; lng: number };
  destination: { lat: number; lng: number } | null;
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
    if (!mapRef.current) {
      mapRef.current = new google.maps.Map(containerRef.current, {
        center: courier,
        zoom: 15,
        zoomControl: false,
        mapTypeControl: false,
        streetViewControl: false,
        fullscreenControl: false,
      });
      courierMarkerRef.current = new google.maps.Marker({
        position: courier,
        map: mapRef.current,
        icon: "http://maps.google.com/mapfiles/ms/icons/orange-dot.png",
      });
    } else {
      courierMarkerRef.current?.setPosition(courier);
      mapRef.current.panTo(courier);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, courier.lat, courier.lng]);

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
      className="h-[200px] w-full overflow-hidden rounded-2xl bg-neutral-100 dark:bg-neutral-800"
    />
  );
}
