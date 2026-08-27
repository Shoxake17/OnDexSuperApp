import type { Metadata } from "next";
import { publicFetch } from "@/lib/api";
import { safeJsonLdHtml } from "@/lib/json-ld";
import type { Restaurant } from "@/lib/types";
import HomeContent from "./home-content";

// Go'dagi Redis kesh TTL'siga mos revalidate (30s) — restoran ro'yxati
// GET /restaurants kabi ochiq (auth talab qilmaydi), shuning uchun SSR +
// ISR orqali qidiruv botlari ham to'liq HTML'ni ko'radi (SEO).
const REVALIDATE_SECONDS = 30;

export const metadata: Metadata = {
  title: "ChustApp — Chust bo'ylab taom yetkazib berish",
  description:
    "Chust shahridagi restoran va kafelardan taom buyurtma qiling — tez yetkazib berish, jonli buyurtma kuzatuvi.",
  openGraph: {
    title: "ChustApp — Chust bo'ylab taom yetkazib berish",
    description:
      "Chust shahridagi restoran va kafelardan taom buyurtma qiling.",
    type: "website",
  },
};

async function getHomeData(): Promise<{
  restaurants: Restaurant[];
  categories: string[];
}> {
  // try/catch: Docker build vaqtida (SSG prerender bosqichi) Go API hali
  // tarmoqda ochilmagan bo'lishi mumkin — bunday holatda bo'sh ro'yxat
  // bilan davom etamiz, `npm run build`ni butunlay yiqitmaymiz (ISR
  // keyin, konteyner jonli bo'lganda, haqiqiy ma'lumot bilan yangilaydi).
  try {
    const [restaurantsRes, categoriesRes] = await Promise.all([
      publicFetch("/restaurants", REVALIDATE_SECONDS),
      publicFetch("/categories", 300),
    ]);
    const restaurants: Restaurant[] = restaurantsRes.ok
      ? await restaurantsRes.json()
      : [];
    const categories: string[] = categoriesRes.ok
      ? await categoriesRes.json()
      : [];
    return { restaurants: restaurants ?? [], categories: categories ?? [] };
  } catch {
    return { restaurants: [], categories: [] };
  }
}

export default async function HomePage() {
  const { restaurants, categories } = await getHomeData();

  // JSON-LD (schema.org) — qidiruv tizimlariga restoranlar ro'yxatini
  // strukturaviy ma'lumot sifatida taqdim etadi (rich results imkoniyati).
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "ItemList",
    itemListElement: restaurants.map((r, i) => ({
      "@type": "ListItem",
      position: i + 1,
      item: {
        "@type": "Restaurant",
        name: r.name,
        address: r.address || undefined,
        url: `https://app.chustapp.uz/restaurants/${r.id}`,
      },
    })),
  };

  return (
    <>
      <script
        type="application/ld+json"
        // eslint-disable-next-line react/no-danger -- JSON-LD; `<` ESCAPE
        // qilingan (safeJsonLdHtml). Restoran nomi/manzili restoran egasi
        // kiritadigan matn, shuning uchun xom JSON.stringify unda
        // `</script>` bo'lsa saqlangan XSS berardi.
        dangerouslySetInnerHTML={{ __html: safeJsonLdHtml(jsonLd) }}
      />
      <HomeContent restaurants={restaurants} categories={categories} />
    </>
  );
}
