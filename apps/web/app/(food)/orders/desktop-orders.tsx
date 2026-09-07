"use client";

import { ChevronRight, Store } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { statusStyleOf } from "@/lib/order-status";
import DesktopShell, { DesktopEmpty } from "../desktop-shell";

// "Buyurtmalarim" — kompyuter ko'rinishi.
//
// Mobil variantdan farqi faqat JOYLASHUVDA: u yerda kartalar ustma-ust
// tizilgan tor ustun, bu yerda esa keng qator — logotip, restoran va
// tarkib, holat va summa bir qatorga sig'adi va ko'z bir harakatda
// hammasini o'qiydi. Ma'lumot manbai bir xil (`GET /me/orders`).
type OrderListEntry = {
  id: string;
  restaurant_id: string;
  restaurant_name?: string;
  restaurant_logo_url?: string;
  items: { name: string; qty: number }[];
  total_tiyin: number;
  status: string;
  created_at: string;
  /** "dine_in" — stol (QR) buyurtmasi; bo'sh = yetkazib berish. */
  type?: string;
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const two = (n: number) => n.toString().padStart(2, "0");
  return `${two(d.getDate())}.${two(d.getMonth() + 1)}.${d.getFullYear()} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

export default function DesktopOrders({ signedIn }: { signedIn: boolean }) {
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
    <DesktopShell
      title="Buyurtmalarim"
      subtitle={
        orders && orders.length > 0 ? `${orders.length} ta buyurtma` : undefined
      }
      signedIn={signedIn}
    >
      {failed ? (
        <DesktopEmpty
          title="Buyurtmalarni yuklab bo'lmadi"
          text="Internet aloqasini tekshirib, sahifani yangilang."
          action={
            <button
              type="button"
              onClick={() => window.location.reload()}
              className="rounded-xl bg-white/10 px-4 py-2.5 text-sm font-semibold hover:bg-white/[0.16]"
            >
              Qaytadan urinish
            </button>
          }
        />
      ) : orders === null ? (
        // Skelet — bo'sh ekran o'rniga: yuklanayotgani ko'rinib turadi.
        <div className="space-y-3 p-5">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-[86px] animate-pulse rounded-2xl bg-white/5" />
          ))}
        </div>
      ) : orders.length === 0 ? (
        <DesktopEmpty
          title="Hozircha buyurtma yo'q"
          text="Birinchi buyurtmangizni bering — u shu yerda ko'rinadi."
          action={
            <Link
              href="/"
              className="inline-flex rounded-xl bg-brand px-5 py-2.5 text-sm font-bold text-white hover:bg-brand-light"
            >
              Restoranlarni ko'rish
            </Link>
          }
        />
      ) : (
        <div className="p-3">
          {orders.map((o) => {
            const style = statusStyleOf(o.status, o.type === "dine_in");
            return (
              <Link
                key={o.id}
                href={`/orders/${o.id}`}
                className="flex items-center gap-4 rounded-2xl p-3 transition-colors hover:bg-white/5"
              >
                <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-white/5">
                  {o.restaurant_logo_url ? (
                    // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                    <img
                      src={fullImageUrl(o.restaurant_logo_url)}
                      alt=""
                      aria-hidden="true"
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <Store size={22} className="text-white/30" />
                  )}
                </div>

                <div className="min-w-0 flex-1">
                  <p className="truncate text-[15px] font-bold">
                    {o.restaurant_name || "Restoran"}
                  </p>
                  <p className="mt-0.5 truncate text-[13px] text-white/45">
                    {o.items.map((i) => `${i.name} × ${i.qty}`).join(", ")}
                  </p>
                  <p className="mt-1 text-[12px] text-white/30">
                    {formatDate(o.created_at)}
                    {o.type === "dine_in" && " · Stol buyurtmasi"}
                  </p>
                </div>

                <div className="shrink-0 text-right">
                  <p className="text-[15px] font-bold">
                    {formatSum(o.total_tiyin)}
                  </p>
                  <span
                    className="mt-1.5 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-semibold"
                    // Rang holat bilan keladi (`lib/order-status.tsx`) —
                    // mobil ro'yxat bilan bir xil manbadan, ya'ni ikkala
                    // ko'rinishda bir xil holat bir xil rangda.
                    style={{ backgroundColor: `${style.color}22`, color: style.color }}
                  >
                    <style.Icon size={13} />
                    {style.label}
                  </span>
                </div>

                <ChevronRight size={18} className="shrink-0 text-white/25" />
              </Link>
            );
          })}
        </div>
      )}
    </DesktopShell>
  );
}
