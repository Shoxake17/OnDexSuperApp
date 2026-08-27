// JSON-LD ni <script type="application/ld+json"> ichiga XAVFSIZ
// joylashtirish uchun.
//
// ┌─ NEGA XOM JSON.stringify XAVFLI (saqlangan XSS) ──────────────────────┐
// JSON-LD ichidagi `name`/`address` restoran EGASI kiritadigan matn
// (admin panel orqali, HTML sanitatsiyasiz saqlanadi). `JSON.stringify`
// esa `<`, `>`, `&` belgilarini escape QILMAYDI. Shuning uchun nomi
//
//     </script><script>...zararli kod...</script>
//
// bo'lgan restoran JSON-LD script tegini YOPIB, o'z skriptini
// ochib yuborardi. CSP'da `'unsafe-inline'` bo'lgani uchun bu inline
// skript BAJARILARDI — ya'ni bitta restoran nomi butun ochiq bosh
// sahifani (barcha tashrifchilar uchun) zaharlab qo'yardi.
//
// `<` ni `<` ga aylantirish shu yagona chiqish yo'lini yopadi:
// JSON parseri escape'ni asl belgiga qaytaradi, ya'ni strukturaviy
// ma'lumot buzilmaydi, lekin brauzer `</script>` ni ko'rmaydi.
// `>` va `&` ham ehtiyot uchun escape qilinadi (`<!--`/`]]>` kabi
// kontekst chalkashliklariga qarshi).
// └──────────────────────────────────────────────────────────────────────┘
export function safeJsonLdHtml(data: unknown): string {
  return JSON.stringify(data)
    .replace(/</g, "\\u003c")
    .replace(/>/g, "\\u003e")
    .replace(/&/g, "\\u0026");
}
