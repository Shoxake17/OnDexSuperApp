"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { useCart } from "@/lib/cart-context";
import { restaurantOpenStatus } from "@/lib/restaurant-status";
import { lines, proxy, registerTools, sum } from "@/lib/webmcp";
import type { Product, ProductSearchResult, Restaurant } from "@/lib/types";
import { useAgentActivity } from "./agent-activity";

/**
 * OnDex amallarini brauzer agentiga ochadi (WebMCP).
 *
 * ┌─ NIMA UCHUN BU SAHIFA AGENT UCHUN QULAY ───────────────────────────┐
 * Odam taom buyurtma qilish uchun: qidiradi, restoranni tanlaydi,
 * menyuni ochadi, savatga qo'shadi, manzil va to'lovni tekshiradi,
 * tasdiqlaydi. Bu ketma-ketlik ko'p qadamli va har qadamda kontekst
 * kerak — aynan agent yaxshi bajaradigan ish.
 *
 * Ilgari buni avtomatlashtirish uchun har bir sayt uchun alohida
 * integratsiya (yoki ekranni "ko'rib" bosadigan mo'rt skript) kerak
 * edi. WebMCP bilan sahifaning O'ZI nima qila olishini aytadi.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ AGENT KO'RINMAS ISHLAMAYDI ───────────────────────────────────────┐
 * Har bir amal `activity` ga yoziladi va ekranning tepasida lenta
 * bo'lib turadi. Bundan tashqari amallar sahifani HAQIQATAN
 * boshqaradi: menyuni ochadi, savatga o'tadi. Ya'ni foydalanuvchi
 * "nimadir bo'ldi" emas, "mana nima bo'ldi" ni ko'radi.
 *
 * Bu qulaylik emas, ishonch masalasi: pul sarflaydigan oqimda odam
 * har qadamni ko'rishi va to'xtata olishi kerak.
 * └────────────────────────────────────────────────────────────────────┘
 */
export default function WebMcpTools() {
  const cart = useCart();
  const router = useRouter();
  const activity = useAgentActivity();

  useEffect(() => {
    // Har render'da qayta ro'yxatga olinmasin uchun bog'liqliklar
    // `ref` orqali emas, `useEffect` ichida joriy qiymat bilan
    // o'qiladi — savat holati `execute` chaqirilganda kerak.
    const say = activity.push;

    const unregister = registerTools([
      {
        name: "search_food",
        description:
          "Search dishes across all OnDex restaurants by name or category " +
          "(for example 'osh', 'lagman', 'pizza'). Returns dish ids that " +
          "other tools need. Always call this before adding anything to " +
          "the cart — never invent ids.",
        inputSchema: {
          type: "object",
          properties: {
            query: { type: "string", description: "Dish name or category." },
          },
          required: ["query"],
        },
        execute: async (input) => {
          const query = String((input as { query?: string }).query ?? "").trim();
          if (!query) return "Query is empty.";
          say(`"${query}" qidirilyapti`);

          const r = await proxy<ProductSearchResult[]>(
            `products/search?q=${encodeURIComponent(query)}`,
          );
          if (!r.ok) return `Search failed: ${r.error}`;
          const found = (r.data ?? []).filter((p) => p.available).slice(0, 8);
          if (found.length === 0) return `Nothing found for "${query}".`;

          say(`${found.length} ta natija topildi`);
          return lines(
            `Found ${found.length} dish(es) for "${query}":`,
            ...found.map(
              (p) =>
                `- ${p.name} — ${sum(
                  p.discount_price_tiyin > 0 &&
                    p.discount_price_tiyin < p.price_tiyin
                    ? p.discount_price_tiyin
                    : p.price_tiyin,
                )} at ${p.restaurant_name}${
                  p.restaurant_open ? "" : " (closed now)"
                } [dish_id=${p.id}, restaurant_id=${p.restaurant_id}]`,
            ),
          );
        },
      },

      {
        name: "list_restaurants",
        description:
          "List OnDex restaurants with their open/closed state and ids.",
        inputSchema: { type: "object", properties: {} },
        execute: async () => {
          say("Restoranlar ro'yxati olinyapti");
          const r = await proxy<Restaurant[]>("restaurants");
          if (!r.ok) return `Could not load restaurants: ${r.error}`;
          const list = (r.data ?? []).slice(0, 20);
          if (list.length === 0) return "No restaurants available.";
          return lines(
            `${list.length} restaurant(s):`,
            ...list.map(
              (x) =>
                `- ${x.name}${restaurantOpenStatus(x).open ? "" : " (closed)"} [restaurant_id=${x.id}]`,
            ),
          );
        },
      },

      {
        name: "open_restaurant_menu",
        description:
          "Open a restaurant's menu page in the browser and return its " +
          "dishes. Use the restaurant_id returned by search_food or " +
          "list_restaurants. This also navigates the user's screen so they " +
          "can see the menu.",
        inputSchema: {
          type: "object",
          properties: {
            restaurant_id: { type: "string" },
          },
          required: ["restaurant_id"],
        },
        execute: async (input) => {
          const id = String(
            (input as { restaurant_id?: string }).restaurant_id ?? "",
          ).trim();
          if (!id) return "restaurant_id is empty.";

          const r = await proxy<Product[]>(
            `restaurants/${encodeURIComponent(id)}/menu`,
          );
          if (!r.ok) return `Could not load the menu: ${r.error}`;

          // Ekranni ham ochamiz — foydalanuvchi nima haqida gap
          // ketayotganini ko'rsin.
          say("Menyu ochilyapti");
          router.push(`/restaurants/${encodeURIComponent(id)}`);

          const menu = (r.data ?? []).filter((p) => p.available).slice(0, 25);
          if (menu.length === 0) return "This restaurant has no dishes yet.";
          return lines(
            `Menu (${menu.length} dishes):`,
            ...menu.map(
              (p) => `- ${p.name} — ${sum(p.price_tiyin)} [dish_id=${p.id}]`,
            ),
          );
        },
      },

      {
        name: "add_to_cart",
        description:
          "Add a dish to the cart by dish_id. All dishes in one order must " +
          "come from the same restaurant; adding a dish from another " +
          "restaurant clears the cart first (the user is told). Does NOT " +
          "place an order.",
        inputSchema: {
          type: "object",
          properties: {
            dish_id: { type: "string" },
            restaurant_id: {
              type: "string",
              description: "Restaurant the dish belongs to.",
            },
            quantity: { type: "integer", minimum: 1, maximum: 20 },
          },
          required: ["dish_id", "restaurant_id"],
        },
        execute: async (input) => {
          const i = input as {
            dish_id?: string;
            restaurant_id?: string;
            quantity?: number;
          };
          const dishId = String(i.dish_id ?? "").trim();
          const restaurantId = String(i.restaurant_id ?? "").trim();
          const qty = Math.min(Math.max(Number(i.quantity ?? 1) || 1, 1), 20);
          if (!dishId || !restaurantId) {
            return "dish_id and restaurant_id are required.";
          }

          // ★ ID ni TEKSHIRAMIZ. Agent ID to'qishi mumkin; menyudan
          // tasdiqlanmasa savatga bo'lmagan taom tushib, keyin
          // rasmiylashtirishda tushunarsiz xato chiqardi.
          const menu = await proxy<Product[]>(
            `restaurants/${encodeURIComponent(restaurantId)}/menu`,
          );
          if (!menu.ok) return `Could not verify the dish: ${menu.error}`;
          const dish = (menu.data ?? []).find((p) => p.id === dishId);
          if (!dish) {
            return "No such dish in this restaurant. Call search_food first and use the ids it returns.";
          }
          if (!dish.available) return `"${dish.name}" is not available now.`;

          // Savatlar restoran bo'yicha ALOHIDA (`lib/cart-context.tsx`):
          // boshqa restorandan qo'shish eskisini o'chirmaydi, faqat
          // faol savatni almashtiradi.
          const switched =
            cart.restaurantId !== null && cart.restaurantId !== restaurantId;
          const current = cart.carts[restaurantId]?.[dishId] ?? 0;

          cart.setQty(restaurantId, dishId, current + qty);
          say(`"${dish.name}" savatga qo'shildi (${qty} ta)`);
          router.push(`/restaurants/${encodeURIComponent(restaurantId)}`);

          return lines(
            switched &&
              "Note: this restaurant now has its own cart and is the active one. Carts from other restaurants are kept (one order = one restaurant, but carts are separate).",
            `Added ${qty} x ${dish.name}. Cart now has ${current + qty} of this dish.`,
          );
        },
      },

      {
        name: "view_cart",
        description:
          "Show what is currently in the cart, with the priced total from " +
          "the server (discounts and delivery included).",
        inputSchema: { type: "object", properties: {} },
        execute: async () => {
          const ids = Object.keys(cart.items);
          if (!cart.restaurantId || ids.length === 0) {
            return "The cart is empty.";
          }
          say("Savat tekshirilyapti");

          const menu = await proxy<Product[]>(
            `restaurants/${encodeURIComponent(cart.restaurantId)}/menu`,
          );
          const byId = new Map(
            (menu.ok ? (menu.data ?? []) : []).map((p) => [p.id, p]),
          );

          const rows = ids.map((id) => {
            const p = byId.get(id);
            const qty = cart.items[id];
            return `- ${p?.name ?? id} x${qty}`;
          });

          // ★ SUMMA SERVERDAN. Mijoz tomonda hisoblansa aksiya va
          // yetkazib berish narxi hisobga olinmay, agent noto'g'ri
          // summa aytardi.
          const quote = await proxy<{ total_tiyin?: number }>(
            `restaurants/${encodeURIComponent(cart.restaurantId)}/quote`,
            {
              method: "POST",
              body: JSON.stringify({
                items: ids.map((id) => ({
                  product_id: id,
                  qty: cart.items[id],
                })),
              }),
            },
          );

          return lines(
            "Cart:",
            ...rows,
            quote.ok && typeof quote.data?.total_tiyin === "number"
              ? `Total: ${sum(quote.data.total_tiyin)}`
              : "Total: could not be calculated right now.",
          );
        },
      },

      {
        name: "open_checkout",
        description:
          "Open the checkout page so the user can review the address and " +
          "payment method and confirm the order themselves. This does NOT " +
          "place the order.",
        inputSchema: { type: "object", properties: {} },
        execute: async () => {
          if (!cart.restaurantId || Object.keys(cart.items).length === 0) {
            return "The cart is empty — add dishes first.";
          }
          say("Rasmiylashtirish ekraniga o'tilyapti");
          router.push("/cart");
          return lines(
            "Opened the cart page.",
            "The user reviews the address and payment method and presses the confirm button. You cannot place the order yourself.",
          );
        },
      },

      {
        name: "my_orders",
        description:
          "List the user's recent orders with restaurant name, status and total.",
        inputSchema: { type: "object", properties: {} },
        execute: async () => {
          say("Buyurtmalar olinyapti");
          const r = await proxy<
            Array<{
              id: string;
              order_number: string;
              status: string;
              total_tiyin: number;
              restaurant_name?: string;
            }>
          >("me/orders");
          if (!r.ok) return `Could not load orders: ${r.error}`;
          const list = (r.data ?? []).slice(0, 10);
          if (list.length === 0) return "No orders yet.";
          return lines(
            `${list.length} recent order(s):`,
            ...list.map(
              (o) =>
                `- ${o.restaurant_name ?? "order"} — ${sum(o.total_tiyin)}, status: ${
                  o.status
                } [order_id=${o.id}, number=${o.order_number}]`,
            ),
          );
        },
      },

      {
        name: "track_order",
        description:
          "Open an order's tracking page and return its current status. " +
          "Use an order_id from my_orders.",
        inputSchema: {
          type: "object",
          properties: { order_id: { type: "string" } },
          required: ["order_id"],
        },
        execute: async (input) => {
          const id = String(
            (input as { order_id?: string }).order_id ?? "",
          ).trim();
          if (!id) return "order_id is empty.";
          const r = await proxy<{ status?: string; total_tiyin?: number }>(
            `orders/${encodeURIComponent(id)}`,
          );
          if (!r.ok) return `Could not load the order: ${r.error}`;
          say("Buyurtma kuzatuvi ochilyapti");
          router.push(`/orders/${encodeURIComponent(id)}`);
          return `Order status: ${r.data?.status ?? "unknown"}${
            typeof r.data?.total_tiyin === "number"
              ? `, total ${sum(r.data.total_tiyin)}`
              : ""
          }.`;
        },
      },
    ]);

    return unregister;
    // `cart` har o'zgarishda yangilanadi — amallar joriy savatni
    // ko'rishi kerak, shuning uchun qayta ro'yxatga olish TO'G'RI.
  }, [cart, router, activity.push]);

  return null;
}
