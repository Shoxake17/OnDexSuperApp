// Ochiq qayta yo'naltirishni (open redirect) oldini oladi: foydalanuvchi
// bergan `redirect`/`next` qiymati ko'r-ko'rona ishlatilsa, hujumchi
// `?next=https://evil.example` orqali mijozni ilova ichidagidek
// ko'rinadigan begona saytga olib chiqishi mumkin edi. Faqat SHU ilova
// ichidagi nisbiy yo'lga ruxsat beriladi — `/api/bridge` va `/login`
// ikkalasi ham shu bitta joydan foydalanadi.
export function safeRedirect(raw: string | null | undefined): string {
  if (!raw) return "/";
  // "//host" ham brauzer uchun mutlaq manzil (protocol-relative) —
  // shuning uchun rad etiladi.
  if (!raw.startsWith("/") || raw.startsWith("//")) return "/";
  return raw;
}
