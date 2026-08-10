import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { publicFetch } from "@/lib/api";
import type { ActivePromotion, Product, Restaurant } from "@/lib/types";
import MenuContent from "./menu-content";

const REVALIDATE_SECONDS = 30;

async function getMenuData(id: string) {
  const [restaurantRes, menuRes, promotionsRes] = await Promise.all([
    publicFetch(`/restaurants/${id}`, REVALIDATE_SECONDS),
    publicFetch(`/restaurants/${id}/menu`, REVALIDATE_SECONDS),
    publicFetch(`/restaurants/${id}/active-promotions`, REVALIDATE_SECONDS),
  ]);
  if (!restaurantRes.ok) return null;
  const restaurant: Restaurant = await restaurantRes.json();
  const menu: Product[] = menuRes.ok ? await menuRes.json() : [];
  const promotions: ActivePromotion[] = promotionsRes.ok
    ? await promotionsRes.json()
    : [];
  return { restaurant, menu: menu ?? [], promotions: promotions ?? [] };
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const data = await getMenuData(id).catch(() => null);
  if (!data) return { title: "Restoran topilmadi — ChustApp" };
  return {
    title: `${data.restaurant.name} — ChustApp`,
    description:
      `${data.restaurant.name} menyusi — ${data.restaurant.address || "Chust"}. ` +
      "Onlayn buyurtma bering, tez yetkazib berish.",
  };
}

export default async function MenuPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await getMenuData(id).catch(() => null);
  if (!data) notFound();
  const { restaurant, menu, promotions } = data;

  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "Restaurant",
    name: restaurant.name,
    address: restaurant.address || undefined,
    url: `https://app.chustapp.uz/restaurants/${restaurant.id}`,
  };

  return (
    <>
      <script
        type="application/ld+json"
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <MenuContent
        restaurant={restaurant}
        menu={menu}
        promotions={promotions}
      />
    </>
  );
}
