"use client";

import { Store } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { statusStyleOf } from "@/lib/order-status";
import MobileSheet from "../mobile-sheet";
import { AppButtonLink } from "../ui";

// "Buyurtmalarim" — pastki menyudagi ikkinchi bo'lim.
//
// Ma'lumot `GET /me/orders` dan keladi (proksi allowlist'ida allaqachon
// bor). Javob `/orders/{id}` dagidan KAMROQ maydon qaytaradi: `history`,
// `type` va `order_number` yo'q — shuning uchun bu yerda bosqich chizig'i
// ham, stol/yetkazish farqi ham ko'rsatilmaydi. Tafsilot uchun mijoz
// kuzatuv sahifasiga o'tadi.

type OrderListEntry = {
  id: string;
  restaurant_id: string;
  restaurant_name?: string;
  restaurant_logo_url?: string;
  items: { name: string; qty: number }[];
  total_tiyin: number;
  status: string;
  created_at: string;
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const two = (n: number) => n.toString().padStart(2, "0");
  return `${two(d.getDate())}.${two(d.getMonth() + 1)}.${d.getFullYear()} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

export default function OrdersPage() {
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
    <MobileSheet className="px-4 pb-28 pt-3">
      <h1 className="text-2xl font-bold">Buyurtmalarim</h1>

      {failed ? (
        <div className="py-14 text-center">
          <p className="text-neutral-500">Buyurtmalarni yuklab bo&apos;lmadi.</p>
          <button
            onClick={() => window.location.reload()}
            className="mt-3 text-sm font-semibold text-brand"
          >
            Qaytadan urinish
          </button>
        </div>
      ) : orders === null ? (
        // Skelet — bo'sh ekran o'rniga. "Buyurtma yo'q" xabari faqat
        // HAQIQATAN javob kelgandan keyin chiqadi, aks holda yuklanish
        // paytida mijoz buyurtmalari yo'qolgandek tuyulardi.
        <div className="mt-4 flex flex-col gap-3">
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              className="h-24 animate-pulse rounded-2xl bg-neutral-100 dark:bg-neutral-800"
            />
          ))}
        </div>
      ) : orders.length === 0 ? (
        <div className="py-14 text-center">
          <p className="text-neutral-500">Hozircha buyurtma yo&apos;q</p>
          <div className="mx-auto mt-5 max-w-xs">
            <AppButtonLink href="/">Restoranlarni ko&apos;rish</AppButtonLink>
          </div>
        </div>
      ) : (
        <div className="mt-4 flex flex-col gap-3">
          {orders.map((o) => {
            const s = statusStyleOf(o.status);
            const itemCount = (o.items ?? []).reduce(
              (n, it) => n + (it.qty || 0),
              0,
            );
            return (
              <Link
                key={o.id}
                href={`/orders/${o.id}`}
                className="tg-surface flex gap-3 rounded-2xl border border-neutral-200 bg-white p-3 active:opacity-70 dark:border-neutral-800 dark:bg-neutral-900"
              >
                <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-neutral-100 dark:bg-neutral-800">
                  {o.restaurant_logo_url ? (
                    // eslint-disable-next-line @next/next/no-img-element -- rasm
                    // manzili muhitga qarab dinamik (R2/lokal disk).
                    <img
                      src={fullImageUrl(o.restaurant_logo_url)}
                      alt=""
                      aria-hidden="true"
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <Store size={20} className="text-neutral-400" />
                  )}
                </div>

                <div className="min-w-0 flex-1">
                  <div className="flex items-start justify-between gap-2">
                    <p className="truncate font-bold">
                      {o.restaurant_name || "Restoran"}
                    </p>
                    <span
                      className="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold"
                      style={{ backgroundColor: `${s.color}26`, color: s.color }}
                    >
                      {s.label}
                    </span>
                  </div>
                  <p className="tg-muted mt-0.5 text-xs text-neutral-500">
                    {formatDate(o.created_at)}
                    {itemCount > 0 && ` · ${itemCount} ta taom`}
                  </p>
                  <p className="mt-1 font-semibold">
                    {formatSum(o.total_tiyin)}
                  </p>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </MobileSheet>
  );
}
