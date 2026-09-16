// Toshkent vaqti bo'yicha ko'rsatish — web'dagi YAGONA formatlovchi.
//
// Xizmat shahri bitta va server ham qat'iy +05:00 bilan hisoblaydi
// (`internal/catalog/hours.go`). Mijoz brauzeri boshqa mintaqaga
// sozlangan bo'lsa ham "09:00 da ochiladi" yoki "taxminan 14:20 gacha"
// restoran/kuryer turgan shahar soatini bildirishi kerak.

const TIME_ZONE = "Asia/Tashkent";

const clockFormat = new Intl.DateTimeFormat("en-GB", {
  timeZone: TIME_ZONE,
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});
const dayFormat = new Intl.DateTimeFormat("en-CA", {
  timeZone: TIME_ZONE,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});
const weekdayFormat = new Intl.DateTimeFormat("en-US", {
  timeZone: TIME_ZONE,
  weekday: "short",
});
const WEEKDAYS: Record<string, string> = {
  Mon: "Dushanba",
  Tue: "Seshanba",
  Wed: "Chorshanba",
  Thu: "Payshanba",
  Fri: "Juma",
  Sat: "Shanba",
  Sun: "Yakshanba",
};

/** "09:05" */
export function tashkentClock(d: Date): string {
  return clockFormat.format(d);
}

/** Toshkent kalendaridagi kun tartib raqami — kunlar farqini hisoblash uchun. */
export function tashkentDayNumber(d: Date): number {
  const [y, m, day] = dayFormat.format(d).split("-").map(Number);
  return Date.UTC(y, m - 1, day) / 86_400_000;
}

/** "Dushanba" */
export function tashkentWeekday(d: Date): string {
  return WEEKDAYS[weekdayFormat.format(d)] ?? "";
}
