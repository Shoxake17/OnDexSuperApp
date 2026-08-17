import type { Config } from "tailwindcss";

export default {
  content: ["./app/**/*.{ts,tsx}", "./lib/**/*.{ts,tsx}"],

  // ┌─ QORONG'I REJIM O'CHIRILGAN (2026-08-17) ────────────────────────┐
  // Ilova FAQAT yorug' (light) mavzuda ko'rinadi — kirish/ro'yxatdan
  // o'tish ekranlari bilan bir xil, oq fonda.
  //
  // NEGA SHU YERDA, 69 ta `dark:` sinfni o'chirib emas:
  // Tailwind'ning standart qiymati `"media"` — ya'ni `dark:` variant
  // TELEFON sozlamasiga qarab o'z-o'zidan yoqilardi. `"class"` ga
  // o'tkazilgach u faqat `<html class="dark">` bo'lganda ishlaydi, biz
  // esa bu sinfni hech qayerda qo'ymaymiz. Natijada mavjud `dark:`
  // sinflar KODDA QOLADI, lekin hech qachon qo'llanmaydi.
  //
  // KELAJAKDA qorong'i rejim kerak bo'lganda: bu qatorni o'chirish
  // (media'ga qaytadi) yoki `<html>` ga `dark` sinfini qo'shadigan
  // almashtirgich yozish yetarli — 69 ta sinfni qaytadan yozish
  // shart emas. Shu sabab ular ataylab o'chirilmadi.
  //
  // Diqqat: bu FAQAT Tailwind sinflarini boshqaradi. Qattiq yozilgan
  // ranglar uchun `app/globals.css` va `lib/telegram.ts` ga qarang —
  // ular ham shu bilan birga yorug' rejimga qulflangan.
  // └──────────────────────────────────────────────────────────────────┘
  darkMode: "class",

  theme: {
    extend: {
      colors: {
        // OnDex brend to'q sariq rangi — logotipdagi "Dex", faol pastki
        // menyu elementi va QR tugmasi shu rangda. Bitta joyda turgani
        // uchun rebrend paytida faqat shu qator o'zgaradi.
        brand: {
          DEFAULT: "#F4511E",
          light: "#FF7043",
        },
      },
    },
  },
  plugins: [],
} satisfies Config;
