"use client";

import { Bell } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import DesktopShell, { DesktopEmpty } from "../desktop-shell";

// "Bildirishnomalar" — kompyuter ko'rinishi.
//
// Mobil variant bilan bir xil ma'lumot (`GET /notifications?limit=50`)
// va bir xil qoida: ro'yxat MUVAFFAQIYATLI ko'rsatilgandan KEYINGINA
// "hammasi o'qildi" yuboriladi. Aks holda so'rov yiqilganda ham
// hisoblagich nolga tushib, mijoz xabarlarni ko'rmay qolardi.
type Notification = {
  id: string;
  module?: string;
  kind?: string;
  title: string;
  body: string;
  data?: Record<string, string>;
  read_at?: string;
  created_at: string;
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const two = (n: number) => n.toString().padStart(2, "0");
  return `${two(d.getDate())}.${two(d.getMonth() + 1)}.${d.getFullYear()} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

/** Xabar buyurtmaga tegishli bo'lsa — o'sha buyurtma sahifasiga. */
function linkFor(n: Notification): string | null {
  const orderId = n.data?.order_id;
  return orderId ? `/orders/${orderId}` : null;
}

export default function DesktopNotifications({
  signedIn,
}: {
  signedIn: boolean;
}) {
  const [items, setItems] = useState<Notification[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/notifications?limit=50");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        const list = ((await res.json()) ?? []) as Notification[];
        setItems(list);
        if (list.some((n) => !n.read_at)) {
          await fetch("/api/proxy/notifications/read-all", { method: "POST" });
        }
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  const unread = items?.filter((n) => !n.read_at).length ?? 0;

  return (
    <DesktopShell
      title="Bildirishnomalar"
      subtitle={unread > 0 ? `${unread} ta yangi` : undefined}
      signedIn={signedIn}
    >
      {failed ? (
        <DesktopEmpty
          title="Bildirishnomalarni yuklab bo'lmadi"
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
      ) : items === null ? (
        <div className="space-y-3 p-5">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-[74px] animate-pulse rounded-2xl bg-white/5" />
          ))}
        </div>
      ) : items.length === 0 ? (
        <DesktopEmpty
          title="Bildirishnoma yo'q"
          text="Buyurtma holati o'zgarganda xabar shu yerda paydo bo'ladi."
        />
      ) : (
        <div className="p-3">
          {items.map((n) => {
            const href = linkFor(n);
            const row = (
              <>
                <div
                  className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-full ${
                    n.read_at ? "bg-white/5 text-white/35" : "bg-brand/15 text-brand"
                  }`}
                >
                  <Bell size={18} />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[15px] font-bold">{n.title}</p>
                  <p className="mt-0.5 text-[13px] leading-relaxed text-white/50">
                    {n.body}
                  </p>
                  <p className="mt-1 text-[12px] text-white/30">
                    {formatDate(n.created_at)}
                  </p>
                </div>
                {/* O'qilmagan belgisi — ochiq nuqta. Ro'yxat ochilishi
                    bilan hammasi o'qilgan deb belgilanadi, shuning uchun
                    u faqat SHU ochilishda yangi bo'lganlarda ko'rinadi. */}
                {!n.read_at && (
                  <span className="mt-1 h-2 w-2 shrink-0 self-start rounded-full bg-brand" />
                )}
              </>
            );

            return href ? (
              <Link
                key={n.id}
                href={href}
                className="flex items-start gap-4 rounded-2xl p-3 transition-colors hover:bg-white/5"
              >
                {row}
              </Link>
            ) : (
              <div key={n.id} className="flex items-start gap-4 rounded-2xl p-3">
                {row}
              </div>
            );
          })}
        </div>
      )}
    </DesktopShell>
  );
}
