// Ochiq qayta yo'naltirishni (open redirect) oldini oladi: foydalanuvchi
// bergan `redirect`/`next` qiymati ko'r-ko'rona ishlatilsa, hujumchi
// `?next=https://evil.example` orqali mijozni ilova ichidagidek
// ko'rinadigan begona saytga olib chiqishi mumkin edi. Faqat SHU ilova
// ichidagi nisbiy yo'lga ruxsat beriladi — `/api/bridge` va `/login`
// ikkalasi ham shu bitta joydan foydalanadi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 50-band) ─────────────────────────────┐
// Avvalgi tekshiruv SATR PREFIKSIGA qaragan edi:
//
//	if (!raw.startsWith("/") || raw.startsWith("//")) return "/";
//
// U `//evil.com` ni to'sardi, lekin `/\evil.com` ni O'TKAZIB
// YUBORARDI. WHATWG URL spetsifikatsiyasi `http`/`https` uchun `\`
// ni `/` ga TENG deb qaraydi, ya'ni:
//
//	safeRedirect("/\evil.com")  →  "/\evil.com"
//	new URL("/\evil.com", origin)  →  https://evil.com/
//
// Natija to'g'ridan-to'g'ri `NextResponse.redirect(new URL(...))` ga
// borardi. Fishing uchun klassik vosita: havola HAQIQIY domeningizda
// ko'rinadi.
//
// Endi satr prefiksi emas, URL'ning O'ZI tahlil qilinadi: nisbiy yo'l
// mos yozuvlar origin'iga nisbatan yechiladi va origin O'ZGARMAGANI
// tekshiriladi. Bu yondashuv brauzer qanday tahlil qilsa, xuddi
// shunday tahlil qiladi — ya'ni keyingi shu turdagi hiyla (`/\/`,
// `/\t/`, kodlangan variantlar) ham o'z-o'zidan yopiladi.
// └────────────────────────────────────────────────────────────────────┘

// Mos yozuvlar origin'i — hech qachon haqiqiy domen emas. Faqat
// "origin o'zgardimi?" savoliga javob berish uchun kerak, shuning
// uchun rezervlangan `.invalid` TLD ishlatiladi (RFC 2606).
const REFERENCE_ORIGIN = "https://ondex.invalid";

export function safeRedirect(raw: string | null | undefined): string {
  if (!raw) return "/";
  // Nisbiy yo'l `/` bilan boshlanishi SHART: `https://evil.com` ham,
  // `evil.com` ham (u `REFERENCE_ORIGIN` ga nisbatan yechilib,
  // origin'ni o'zgartirmasa ham) shu yerda kesiladi.
  if (!raw.startsWith("/")) return "/";
  try {
    const u = new URL(raw, REFERENCE_ORIGIN);
    // Origin o'zgargan bo'lsa — `//host`, `/\host` va shu turdagi
    // barcha variantlar — rad etiladi.
    if (u.origin !== REFERENCE_ORIGIN) return "/";
    // Faqat ilova ichidagi qism qaytariladi (origin tashlanadi).
    return u.pathname + u.search + u.hash;
  } catch {
    return "/";
  }
}
