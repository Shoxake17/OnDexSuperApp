"use client";

import { Bell, Heart, Store, UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { statusStyleOf } from "@/lib/order-status";
import type { ProductSearchResult } from "@/lib/types";

// Navbardan ochiladigan panellar: bildirishnomalar, buyurtmalar,
// sevimlilar.
//
// ┌─ NEGA PANEL, SAHIFA EMAS ──────────────────────────────────────────┐
// Savat bilan bir xil sabab: kompyuterda bu uchala ro'yxat ham QISQA
// ma'lumot — "yangi xabar bormi", "buyurtmam qayerda", "nimani
// saqlagandim". Ular uchun butun sahifani almashtirish mijozni
// menyudan uzoqlashtiradi va orqaga qaytishga majbur qiladi.
//
// Sahifalar (`/orders`, `/favorites`, `/notifications`) O'CHIRILMADI:
// ular mobil ko'rinish uchun kerak va to'g'ridan-to'g'ri havola bilan
// ham ochiladi. Panel pastida "Hammasini ko'rish" — to'liq ro'yxat
// kerak bo'lganda.
// └────────────────────────────────────────────────────────────────────┘

/** Panellarning umumiy qobig'i — savat bloki bilan bir xil o'lcham. */
export function Panel({
  title,
  moreHref,
  moreLabel = "Hammasini ko'rish",
  onNavigate,
  children,
}: {
  title: string;
  moreHref: string;
  moreLabel?: string;
  onNavigate: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="absolute right-0 top-[calc(100%+10px)] flex max-h-[calc(100vh-96px)] w-[380px] flex-col overflow-hidden rounded-2xl bg-[#1e1e1e] text-white shadow-2xl ring-1 ring-white/10">
      <p className="shrink-0 px-4 pb-2 pt-4 text-[17px] font-bold">{title}</p>

      <div className="ondex-scroll min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 pb-2">
        {children}
      </div>

      <Link
        href={moreHref}
        onClick={onNavigate}
        className="shrink-0 border-t border-white/10 px-4 py-3 text-center text-[13px] font-semibold text-white/60 transition-colors hover:bg-white/5 hover:text-white"
      >
        {moreLabel}
      </Link>
    </div>
  );
}

function Loading({ height }: { height: string }) {
  return (
    <div className="space-y-2 p-2">
      {[0, 1, 2].map((i) => (
        <div
          key={i}
          className={`${height} animate-pulse rounded-xl bg-white/5`}
        />
      ))}
    </div>
  );
}

function Empty({ text }: { text: string }) {
  return (
    <p className="px-4 py-10 text-center text-[13px] text-white/40">{text}</p>
  );
}

// ── Bildirishnomalar ─────────────────────────────────────────────────
type Notification = {
  id: string;
  title: string;
  body: string;
  data?: Record<string, string>;
  read_at?: string;
  created_at: string;
};

export function NotificationsPanel({ onNavigate }: { onNavigate: () => void }) {
  const [items, setItems] = useState<Notification[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/notifications?limit=20");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        const list = ((await res.json()) ?? []) as Notification[];
        setItems(list);
        // "Hammasi o'qildi" FAQAT ro'yxat muvaffaqiyatli kelgandan
        // keyin — aks holda so'rov yiqilganda ham hisoblagich nolga
        // tushib, mijoz xabarlarni ko'rmay qolardi.
        if (list.some((n) => !n.read_at)) {
          await fetch("/api/proxy/notifications/read-all", { method: "POST" });
        }
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  return (
    <Panel title="Bildirishnomalar" moreHref="/notifications" onNavigate={onNavigate}>
      {failed ? (
        <Empty text="Bildirishnomalarni yuklab bo'lmadi." />
      ) : items === null ? (
        <Loading height="h-[66px]" />
      ) : items.length === 0 ? (
        <Empty text="Hozircha xabar yo'q." />
      ) : (
        items.map((n) => {
          const orderId = n.data?.order_id;
          const body = (
            <>
              <div
                className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full ${
                  n.read_at ? "bg-white/5 text-white/35" : "bg-brand/15 text-brand"
                }`}
              >
                <Bell size={16} />
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[14px] font-semibold">{n.title}</p>
                <p className="mt-0.5 line-clamp-2 text-[12.5px] leading-snug text-white/50">
                  {n.body}
                </p>
              </div>
            </>
          );
          return orderId ? (
            <Link
              key={n.id}
              href={`/orders/${orderId}`}
              onClick={onNavigate}
              className="flex gap-3 rounded-xl p-2.5 transition-colors hover:bg-white/5"
            >
              {body}
            </Link>
          ) : (
            <div key={n.id} className="flex gap-3 rounded-xl p-2.5">
              {body}
            </div>
          );
        })
      )}
    </Panel>
  );
}

// ── Buyurtmalar ──────────────────────────────────────────────────────
type OrderListEntry = {
  id: string;
  restaurant_name?: string;
  restaurant_logo_url?: string;
  items: { name: string; qty: number }[];
  total_tiyin: number;
  status: string;
  type?: string;
};

export function OrdersPanel({ onNavigate }: { onNavigate: () => void }) {
  const [orders, setOrders] = useState<OrderListEntry[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/me/orders");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        setOrders((await res.json()) ?? []);
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  return (
    <Panel title="Buyurtmalarim" moreHref="/orders" onNavigate={onNavigate}>
      {failed ? (
        <Empty text="Buyurtmalarni yuklab bo'lmadi." />
      ) : orders === null ? (
        <Loading height="h-[62px]" />
      ) : orders.length === 0 ? (
        <Empty text="Hozircha buyurtma yo'q." />
      ) : (
        orders.slice(0, 8).map((o) => {
          const style = statusStyleOf(o.status, o.type === "dine_in");
          return (
            <Link
              key={o.id}
              href={`/orders/${o.id}`}
              onClick={onNavigate}
              className="flex items-center gap-3 rounded-xl p-2.5 transition-colors hover:bg-white/5"
            >
              <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white/5">
                {o.restaurant_logo_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                  <img
                    src={fullImageUrl(o.restaurant_logo_url)}
                    alt=""
                    aria-hidden="true"
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <Store size={18} className="text-white/30" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[14px] font-semibold">
                  {o.restaurant_name || "Restoran"}
                </p>
                <span
                  className="mt-1 inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11.5px] font-semibold"
                  style={{ backgroundColor: `${style.color}22`, color: style.color }}
                >
                  <style.Icon size={12} />
                  {style.label}
                </span>
              </div>
              <p className="shrink-0 text-[13.5px] font-bold">
                {formatSum(o.total_tiyin)}
              </p>
            </Link>
          );
        })
      )}
    </Panel>
  );
}

// ── Sevimlilar ───────────────────────────────────────────────────────
export function FavoritesPanel({ onNavigate }: { onNavigate: () => void }) {
  const [items, setItems] = useState<ProductSearchResult[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/favorites");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        setItems((await res.json()) ?? []);
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  return (
    <Panel title="Sevimlilar" moreHref="/favorites" onNavigate={onNavigate}>
      {failed ? (
        <Empty text="Sevimlilarni yuklab bo'lmadi." />
      ) : items === null ? (
        <Loading height="h-[62px]" />
      ) : items.length === 0 ? (
        <div className="px-4 py-10 text-center">
          <Heart size={26} className="mx-auto text-white/20" />
          <p className="mt-3 text-[13px] text-white/40">
            Menyudagi yurakni bosing — taom shu yerga tushadi.
          </p>
        </div>
      ) : (
        items.slice(0, 8).map((p) => (
          <Link
            key={p.id}
            href={`/restaurants/${p.restaurant_id}`}
            onClick={onNavigate}
            className={`flex items-center gap-3 rounded-xl p-2.5 transition-colors hover:bg-white/5 ${
              p.restaurant_open ? "" : "opacity-40"
            }`}
          >
            <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white/5">
              {p.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                <img
                  src={fullImageUrl(p.image_url)}
                  alt={p.name}
                  className="h-full w-full object-cover"
                />
              ) : (
                <UtensilsCrossed size={18} className="text-white/25" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[14px] font-semibold">{p.name}</p>
              <p className="mt-0.5 truncate text-[12.5px] text-white/40">
                {p.restaurant_open
                  ? p.restaurant_name
                  : `${p.restaurant_name} · yopiq`}
              </p>
            </div>
            <p className="shrink-0 text-[13.5px] font-bold">
              {formatSum(p.price_tiyin)}
            </p>
          </Link>
        ))
      )}
    </Panel>
  );
}
