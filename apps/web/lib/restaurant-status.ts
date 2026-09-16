import { tashkentClock, tashkentDayNumber, tashkentWeekday } from "./tashkent-time";
import type { Restaurant } from "./types";

// Restoranning "Ochiq/Yopiq" holati — web'dagi YAGONA manba.
//
// ┌─ NEGA (2026-09-15) ────────────────────────────────────────────────┐
// Kartalar va menyu `restaurant.open` ga qarardi — bu restoranning qo'lda
// bosadigan tugmasi, xolos. Ish vaqti tugagan restoran hamma joyda
// "Ochiq" ko'rinib, mijoz taom qo'shgach faqat savatda "yopiq" javobini
// olardi. Endi holatni SERVER hisoblaydi (`open_now`, `closed_reason`,
// `open_changes_at` — `internal/catalog/hours.go`); bu modul faqat o'qiydi
// va matnga aylantiradi. Ish vaqti qoidasi bu yerda QAYTA YOZILMAYDI.
//
// Flutter ilovalarida aynan shu vazifani
// `packages/ondex_core/lib/src/restaurant_status.dart` bajaradi.
// └────────────────────────────────────────────────────────────────────┘

export type ClosedReason = "manual" | "hours";

export type OpenStatus = {
  open: boolean;
  /** Yopiq bo'lsa sababi, ochiq bo'lsa `null`. */
  reason: ClosedReason | null;
  /** Holat o'z-o'zidan o'zgaradigan payt (server hisoblagan). */
  changesAt: Date | null;
};

export type OpenStatusFields = Pick<
  Restaurant,
  "open" | "open_now" | "closed_reason" | "open_changes_at"
>;

/** "Yopiladi" izohi yopilishga shuncha qolganda ko'rsatiladi. */
export const CLOSING_SOON_MS = 60 * 60 * 1000;

export function restaurantOpenStatus(r: OpenStatusFields): OpenStatus {
  // `open_now` bo'lmagan eski javob — avvalgi xatti-harakat (qo'lda tugma).
  const open = typeof r.open_now === "boolean" ? r.open_now : Boolean(r.open);
  let reason: ClosedReason | null = null;
  if (!open) {
    if (r.closed_reason === "hours" || r.closed_reason === "manual") {
      reason = r.closed_reason;
    } else {
      reason = r.open ? "hours" : "manual";
    }
  }
  const at = r.open_changes_at ? new Date(r.open_changes_at) : null;
  return {
    open,
    reason,
    changesAt: at && !Number.isNaN(at.getTime()) ? at : null,
  };
}

/**
 * Sahifa ochiq turgan paytda `changesAt` o'tib ketsa — holat jadval
 * bo'yicha almashadi (server shu paytni aynan shu qoida bilan hisoblagan).
 * Keyingi o'zgarish vaqti noma'lum bo'lib qoladi. Qo'lda yopilgan
 * restoran vaqt bilan ochilmaydi.
 */
export function openStatusAt(s: OpenStatus, now: Date): OpenStatus {
  if (!s.changesAt || s.reason === "manual" || now < s.changesAt) return s;
  return s.open
    ? { open: false, reason: "hours", changesAt: null }
    : { open: true, reason: null, changesAt: null };
}

/**
 * Qisqa izoh: "09:00 da ochiladi", "Ertaga 09:00 da ochiladi",
 * "Dushanba 09:00 da ochiladi", yopilishga bir soatdan kam qolganda
 * "22:00 da yopiladi". Aytadigan narsa bo'lmasa `null`.
 */
export function openStatusDetail(s: OpenStatus, now: Date): string | null {
  const at = s.changesAt;
  if (!at) return null;
  const clock = tashkentClock(at);
  if (s.open) {
    return at.getTime() - now.getTime() <= CLOSING_SOON_MS
      ? `${clock} da yopiladi`
      : null;
  }
  const days = tashkentDayNumber(at) - tashkentDayNumber(now);
  if (days <= 0) return `${clock} da ochiladi`;
  if (days === 1) return `Ertaga ${clock} da ochiladi`;
  return `${tashkentWeekday(at)} ${clock} da ochiladi`.trim();
}

/** Menyu ogohlantirishi uchun to'liq jumla (faqat yopiq holatda ma'noli). */
export function closedMessage(s: OpenStatus, now: Date | null): string {
  const detail = now ? openStatusDetail(s, now) : null;
  if (detail) return `Restoran hozir yopiq · ${detail}`;
  return s.reason === "manual"
    ? "Restoran hozir buyurtma qabul qilmayapti"
    : "Restoran hozir yopiq — ish vaqti tugagan";
}
