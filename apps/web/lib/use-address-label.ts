"use client";

import { useEffect, useState } from "react";

/// Manzil saqlanmagan (yoki mehmon) holatdagi yorliq.
///
/// Avval bu yerda shahar nomi — "Chust" — qattiq yozilgan edi: platforma
/// bitta shahar uchun bo'lganda bu doim to'g'ri edi. Toshkent qo'shilgach
/// manzilsiz Toshkent mijoziga ham "Chust" ko'rinardi, shuning uchun
/// endi shahar taxmin qilinmaydi — foydalanuvchi o'zi tanlaydi.
export const NO_ADDRESS_LABEL = "Manzilni tanlang";

/// Sarlavhadagi manzil yorlig'i: saqlangan manzil matni, bo'lmasa
/// `NO_ADDRESS_LABEL`. Kompyuter navbar'i va mobil bosh sahifa BIR XIL
/// manbadan oladi. Ikkinchi qiymat — manzil shu ekranda saqlanganda
/// yorliqni darhol yangilash uchun.
export function useAddressLabel(
  signedIn: boolean,
): [string, (label: string) => void] {
  const [label, setLabel] = useState(NO_ADDRESS_LABEL);

  useEffect(() => {
    if (!signedIn) return;
    let cancelled = false;
    fetch("/api/proxy/me/address")
      .then((r) => (r.ok ? r.json() : null))
      .then((a: { text?: string } | null) => {
        const text = a?.text?.trim();
        if (text && !cancelled) setLabel(text);
      })
      .catch(() => {
        // Manzil ko'rsatilmaydi, xolos — zaxira yorliq qoladi.
      });
    return () => {
      cancelled = true;
    };
  }, [signedIn]);

  return [label, setLabel];
}
