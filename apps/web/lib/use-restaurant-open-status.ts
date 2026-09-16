"use client";

import {
  openStatusAt,
  openStatusDetail,
  restaurantOpenStatus,
  type OpenStatus,
  type OpenStatusFields,
} from "./restaurant-status";
import { useNow } from "./use-now";

/**
 * Restoran holati — ko'rsatish uchun. Sahifa ochiq tursa ham `open_changes_at`
 * kelganda o'zi almashadi (masalan 22:00 da menyu "Yopiq" bo'ladi).
 *
 * `now` — shu hisobda ishlatilgan vaqt (gidratsiyada `null`).
 */
export function useRestaurantOpenStatus(r: OpenStatusFields): {
  status: OpenStatus;
  detail: string | null;
  now: Date | null;
} {
  const now = useNow();
  const base = restaurantOpenStatus(r);
  if (!now) return { status: base, detail: null, now };
  const status = openStatusAt(base, now);
  return { status, detail: openStatusDetail(status, now), now };
}
