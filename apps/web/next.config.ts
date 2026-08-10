import type { NextConfig } from "next";

// MUHIM (haqiqiy Android qurilmada topilgan HAQIQIY sabab — soatlab
// izlangan "hech qanaqa tugma ishlamayapti" bug'ining ILDIZI): Next.js
// 15+/16 dev-server xavfsizlik uchun standart holatda LAN IP kabi
// "begona" origin'lardan kelgan so'rovlarni _next/static JS chunk'lariga
// BLOKLAYDI ("Blocked cross-origin request to Next.js dev resource").
// Bu React'ning "hydration"ini (butun interaktivlikni — onClick va h.k.)
// butunlay ishga tushirilmay qoldirardi — sahifa VIZUAL to'g'ri
// ko'rinardi (SSR HTML yetib kelgan), lekin HECH BIR tugma ishlamasdi,
// chunki client-side JS umuman yuklanmagan edi. Faqat DEV rejimida
// muhim (production build bunday cheklovga ega emas).
const nextConfig: NextConfig = {
  output: "standalone",
  allowedDevOrigins: ["127.0.0.1", "localhost"],
};

export default nextConfig;
