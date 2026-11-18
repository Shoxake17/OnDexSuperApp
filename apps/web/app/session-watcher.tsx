"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// ┌─ SESSIYA TUGASA — AVTOMATIK CHIQISH (2026-11-18) ─────────────────────┐
// BFF (`/api/proxy`) 401 da cookie'larni allaqachon tozalaydi, lekin OCHIQ
// sahifa buni bilmasdi: foydalanuvchi boshqa sahifaga o'tmaguncha "kirgan"
// ko'rinishda qolib, har so'rovda xato olardi.
//
// Endi proksi `X-Ondex-Session-Expired: 1` qo'yadi; bu komponent uni ko'rib
// `router.refresh()` qiladi — server komponentlari "kirmagan" holatda qayta
// chiziladi. Telegram Mini App'da `TelegramAuth` (signedIn=false) initData
// bilan O'ZI qayta kiradi, shuning uchun u yerda xabar chiqarilmaydi.
// └───────────────────────────────────────────────────────────────────────┘

const HEADER = "x-ondex-session-expired";
const EVENT = "ondex:session-expired";

let patched = false;

function patchFetch() {
  if (patched || typeof window === "undefined") return;
  patched = true;
  const orig = window.fetch.bind(window);
  window.fetch = async (...args: Parameters<typeof fetch>) => {
    const res = await orig(...args);
    try {
      if (res.status === 401 && res.headers.get(HEADER) === "1") {
        window.dispatchEvent(new Event(EVENT));
      }
    } catch {
      // sarlavha o'qilmasa — oddiy javob
    }
    return res;
  };
}

function inTelegram(): boolean {
  const w = window as unknown as { Telegram?: { WebApp?: { initData?: string } } };
  return Boolean(w.Telegram?.WebApp?.initData);
}

export default function SessionWatcher() {
  const router = useRouter();
  const [show, setShow] = useState(false);

  useEffect(() => {
    patchFetch();
    let busy = false;
    const onExpired = () => {
      if (busy) return; // parallel 401 lar — bitta reaksiya
      busy = true;
      if (!inTelegram()) setShow(true);
      router.refresh();
      window.setTimeout(() => {
        busy = false;
      }, 3000);
    };
    window.addEventListener(EVENT, onExpired);
    return () => window.removeEventListener(EVENT, onExpired);
  }, [router]);

  useEffect(() => {
    if (!show) return;
    const t = window.setTimeout(() => setShow(false), 6000);
    return () => window.clearTimeout(t);
  }, [show]);

  if (!show) return null;
  return (
    <div
      role="status"
      className="fixed inset-x-4 top-4 z-[1000] mx-auto max-w-md rounded-2xl bg-neutral-900 px-4 py-3 text-center text-sm font-medium text-white shadow-lg"
    >
      Sessiya tugadi — xavfsizlik uchun qaytadan kiring
    </div>
  );
}
