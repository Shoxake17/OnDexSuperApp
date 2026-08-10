import type { MetadataRoute } from "next";
import { publicFetch } from "@/lib/api";
import type { Restaurant } from "@/lib/types";

// Ochiq (auth talab qilmaydigan) sahifalar — Home + har bir restoran
// (Menyu sahifasi 3-bosqichda quriladi, lekin URL strukturasi hoziroq
// belgilanadi). Savat/Checkout/Active-Order — auth talab qiladi,
// sitemap'ga kiritilmaydi (indekslashning ma'nosi yo'q).
const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "https://app.chustapp.uz";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  let restaurants: Restaurant[] = [];
  try {
    const res = await publicFetch("/restaurants", 300);
    restaurants = res.ok ? await res.json() : [];
  } catch {
    restaurants = [];
  }

  return [
    { url: SITE_URL, changeFrequency: "hourly", priority: 1 },
    ...restaurants.map((r) => ({
      url: `${SITE_URL}/restaurants/${r.id}`,
      changeFrequency: "hourly" as const,
      priority: 0.8,
    })),
  ];
}
