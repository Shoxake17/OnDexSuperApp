"use client";

import {
  ArrowLeft,
  Bell,
  BellOff,
  Bike,
  ChefHat,
  Clock,
  PackageCheck,
  ShoppingBag,
  Ticket,
  UtensilsCrossed,
  Wallet,
  XCircle,
} from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { goBack } from "@/lib/nav";
import MobileSheet from "../mobile-sheet";

// Bildirishnomalar ro'yxati — maket: image/notification.png
//
// Backend allaqachon tayyor (`internal/httpapi/routes_notifications.go`,
// migration 0030). Bu sahifa uni ko'rsatadi.
//
// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
// WebSocket xabari FAQAT ilova ochiq bo'lganda yetadi. Ilova yopiq,
// tarmoq uzilgan yoki soket o'lik bo'lsa xabar yo'qolardi. Server
// ularni bazaga yozadi — bu sahifa esa o'sha tarixni ko'rsatadi.
// └────────────────────────────────────────────────────────────────────┘

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

export default function NotificationsPage() {
  const router = useRouter();
  const [items, setItems] = useState<Notification[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/notifications?limit=50");
        if (!res.ok) {
          // 401 ham shu yerga tushadi — bu sahifaga faqat kirgan
          // holatda kelinadi, shuning uchun "yuklab bo'lmadi" to'g'ri.
          setFailed(true);
          return;
        }
        const list = ((await res.json()) ?? []) as Notification[];
        setItems(list);

        // O'qilgan deb belgilash FAQAT ro'yxat MUVAFFAQIYATLI
        // ko'rsatilgandan keyin. Aks holda so'rov yiqilgan holatda ham
        // hisoblagich nolga tushib, mijoz xabarlarni ko'rmay qolardi.
        if (list.some((n) => !n.read_at)) {
          await fetch("/api/proxy/notifications/read-all", { method: "POST" });
        }
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  const onBack = useCallback(() => {
    // Native ekran ichida bo'lsa o'sha ekranni yopadi, aks holda
    // oddiy Next.js navigatsiyasi (`lib/nav.ts` izohiga qarang).
    goBack(() => router.back());
  }, [router]);

  return (
    // `pb-10` (`pb-28` EMAS): `bottom-nav.tsx` bu sahifada menyuni
    // ko'rsatmaydi (u faqat to'rtta asosiy sahifada chiziladi), shuning
    // uchun pastda menyu uchun joy ajratish kerak emas — ortiqcha
    // bo'shliq ro'yxatni uzib ko'rsatardi.
    <MobileSheet className="px-4 pb-10 pt-3">
      {/* ┌─ SARLAVHA ────────────────────────────────────────────────────┐
          Maketda orqaga tugmasi CHAPDA, sarlavha esa EKRAN o'rtasida.
          Shu sabab sarlavha `absolute` bilan markazlashtiriladi — oddiy
          flex'da u tugmaning kengligiga qarab siljib, o'ngga qarab
          qiyshayib qolardi.
          └───────────────────────────────────────────────────────────────┘ */}
      <div className="relative flex h-11 items-center">
        <button
          type="button"
          onClick={onBack}
          aria-label="Orqaga"
          className="flex h-11 w-11 items-center justify-center rounded-2xl border border-neutral-200 bg-white active:bg-neutral-100"
        >
          <ArrowLeft size={22} />
        </button>
        <h1 className="pointer-events-none absolute inset-x-0 text-center text-[19px] font-bold">
          Bildirishnomalar
        </h1>
      </div>

      {failed ? (
        <div className="py-14 text-center">
          <p className="text-neutral-500">
            Bildirishnomalarni yuklab bo&apos;lmadi.
          </p>
          <button
            onClick={() => window.location.reload()}
            className="mt-3 text-sm font-semibold text-brand"
          >
            Qaytadan urinish
          </button>
        </div>
      ) : items === null ? (
        <div className="mt-5 flex flex-col gap-3">
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-[104px] animate-pulse rounded-2xl bg-neutral-100"
            />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center px-6 py-16 text-center">
          <div className="flex h-20 w-20 items-center justify-center rounded-full bg-neutral-100">
            <BellOff size={36} className="text-neutral-400" />
          </div>
          <p className="mt-5 text-lg font-semibold">Bildirishnoma yo&apos;q</p>
          <p className="mt-2 max-w-xs text-sm leading-relaxed text-neutral-500">
            Buyurtma holati o&apos;zgarganda va aksiyalar chiqqanda shu
            yerda ko&apos;rasiz.
          </p>
        </div>
      ) : (
        <ul className="mt-5 flex flex-col gap-3">
          {items.map((n) => (
            <NotificationCard key={n.id} n={n} />
          ))}
        </ul>
      )}
    </MobileSheet>
  );
}

// ─── Bitta karta ──────────────────────────────────────────────────────

function NotificationCard({ n }: { n: Notification }) {
  const router = useRouter();
  const look = lookFor(n);
  const orderId = safeOrderId(n.data?.order_id);

  const card = (
    <div className="flex gap-3.5 rounded-2xl border border-neutral-200 bg-white p-3.5">
      {/* Rangli ikon plitkasi. Ranglar `look` dan keladi va ular
          QAT'IY yozilgan sinflar — pastdagi izohga qarang. */}
      <div
        className={`flex h-[52px] w-[52px] shrink-0 items-center justify-center rounded-2xl ${look.tile}`}
      >
        <look.Icon size={24} className={look.icon} />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-3">
          <p className="min-w-0 font-bold leading-snug">{n.title}</p>
          <time
            dateTime={n.created_at}
            className="shrink-0 pt-0.5 text-xs text-neutral-400"
          >
            {relativeTime(n.created_at)}
          </time>
        </div>

        <div className="mt-1 flex items-end justify-between gap-3">
          {n.body ? (
            <p className="min-w-0 text-sm leading-relaxed text-neutral-500">
              {n.body}
            </p>
          ) : (
            <span />
          )}
          {/* O'qilmaganlik belgisi. Ro'yxat ochilganda serverda
              hammasi o'qilgan deb belgilanadi, lekin BU ko'rinish
              o'zgarmaydi — mijoz nimani endi ko'rganini bilishi kerak. */}
          {!n.read_at && (
            <span
              aria-label="O'qilmagan"
              className="mb-1 h-2 w-2 shrink-0 rounded-full bg-brand"
            />
          )}
        </div>
      </div>
    </div>
  );

  // ┌─ BOSILADIGAN KARTA — FAQAT TEKSHIRILGAN ICHKI YO'LGA ───────────┐
  // Server yuborgan URL HECH QACHON ishlatilmaydi. Faqat `order_id`
  // olinadi, u qat'iy shablon bilan tekshiriladi va yo'l BIZ tomonda
  // quriladi.
  //
  // NEGA: bildirishnoma mazmuni ma'lumotlar bazasidan keladi. Agar
  // manzil server maydonidan olinsa, u `javascript:` sxemasi yoki
  // begona domen bo'lishi mumkin edi — ya'ni ochiq qayta yo'naltirish
  // (open redirect) va XSS yuzasi. `Link href` ga tashqi qiymat
  // berilmasligi shu sababli TAMOYIL.
  // └─────────────────────────────────────────────────────────────────┘
  if (!orderId) return <li>{card}</li>;
  return (
    <li>
      <button
        type="button"
        onClick={() => router.push(`/orders/${orderId}`)}
        className="block w-full text-left active:opacity-70"
      >
        {card}
      </button>
    </li>
  );
}

/**
 * `order_id` ni tekshiradi.
 *
 * Qat'iy shablon ATAYLAB: ID — server generatsiya qilgan hex satr.
 * Slash, nuqta yoki foizli kodlash bo'lsa bu bizning ID emas va yo'l
 * qurilmaydi (yo'ldan chiqish — path traversal — urinishlari shu yerda
 * to'xtaydi).
 */
function safeOrderId(raw: string | undefined): string | null {
  if (!raw) return null;
  return /^[A-Za-z0-9_-]{1,64}$/.test(raw) ? raw : null;
}

// ─── Ikon va rang tanlash ─────────────────────────────────────────────

type Look = {
  Icon: typeof Bell;
  /** Plitka foni — QAT'IY yozilgan sinf. */
  tile: string;
  /** Ikon rangi — QAT'IY yozilgan sinf. */
  icon: string;
};

const LOOKS = {
  new: { Icon: ShoppingBag, tile: "bg-orange-50", icon: "text-brand" },
  cooking: { Icon: ChefHat, tile: "bg-amber-50", icon: "text-amber-600" },
  ready: { Icon: UtensilsCrossed, tile: "bg-emerald-50", icon: "text-emerald-600" },
  onWay: { Icon: Bike, tile: "bg-green-50", icon: "text-green-600" },
  done: { Icon: PackageCheck, tile: "bg-blue-50", icon: "text-blue-600" },
  failed: { Icon: XCircle, tile: "bg-red-50", icon: "text-red-600" },
  promo: { Icon: Ticket, tile: "bg-yellow-50", icon: "text-yellow-600" },
  wallet: { Icon: Wallet, tile: "bg-purple-50", icon: "text-purple-600" },
  waiting: { Icon: Clock, tile: "bg-neutral-100", icon: "text-neutral-500" },
  system: { Icon: Bell, tile: "bg-neutral-100", icon: "text-neutral-500" },
} as const satisfies Record<string, Look>;

/**
 * Bildirishnomaga mos ko'rinishni tanlaydi.
 *
 * ┌─ NEGA YOPIQ JADVAL, SINF QURISH EMAS ─────────────────────────────┐
 * `kind` va `status` — SERVERDAN kelgan satrlar. Ular hech qachon
 * sinf nomiga qo'shilmaydi (`bg-${color}-50` KO'RINISHIDA EMAS):
 *   1. Tailwind sinflarni build vaqtida statik tahlil bilan yaratadi —
 *      dinamik qurilgan sinf CSS'da umuman bo'lmaydi va element
 *      rangsiz chiqadi;
 *   2. undan muhimi — server qiymati `class` atributiga tushishi
 *      kerak emas. Yopiq jadval buni tamoyil darajasida bekor qiladi.
 *
 * Noma'lum qiymat — `system` (qo'ng'iroq belgisi). Yangi `kind`
 * qo'shilganda sahifa buziladi emas, shunchaki neytral ko'rinadi.
 * └───────────────────────────────────────────────────────────────────┘
 */
function lookFor(n: Notification): Look {
  if (n.kind === "table_order_ready") return LOOKS.ready;

  if (n.kind === "order_status") {
    // Holatlar — `internal/orders/order.go` dagi `Status` ro'yxati.
    switch (n.data?.status) {
      case "created":
        return LOOKS.waiting;
      case "accepted":
        return LOOKS.new;
      case "preparing":
        return LOOKS.cooking;
      case "ready":
        return LOOKS.ready;
      case "picked_up":
        return LOOKS.onWay;
      case "delivered":
      case "served":
        return LOOKS.done;
      case "rejected":
      case "cancelled":
        return LOOKS.failed;
      default:
        return LOOKS.system;
    }
  }

  // Kelajakdagi turlar (hozir backend ularni yubormaydi, lekin
  // qo'shilganda sahifa tegmasdan to'g'ri ko'rinadi).
  if (n.kind === "promotion") return LOOKS.promo;
  if (n.kind === "wallet") return LOOKS.wallet;

  return LOOKS.system;
}

// ─── Vaqt ─────────────────────────────────────────────────────────────

/**
 * "2 daqiqa oldin", "2 soat oldin", "1 kun oldin" (maketdagidek).
 *
 * `Intl.RelativeTimeFormat` ATAYLAB ishlatilmadi: o'zbek tili uchun
 * natija brauzerga qarab farq qiladi ("2 kun oldin" / "2 days ago"),
 * ba'zi WebView'larda esa lokal umuman yo'q. Matn mahsulot tilida
 * bo'lishi kerak, shuning uchun qo'lda.
 *
 * Bu komponent klientda ishlaydi (`"use client"` + ma'lumot `fetch`
 * bilan olinadi), ya'ni serverda chizilmaydi va hidratsiya mos
 * kelmasligi xavfi yo'q.
 */
function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";

  const sec = Math.floor((Date.now() - t) / 1000);
  // Kelajakdagi sana (server va telefon soati farq qilsa) — "hozir".
  if (sec < 60) return "hozir";

  const min = Math.floor(sec / 60);
  if (min < 60) return `${min} daqiqa oldin`;

  const hour = Math.floor(min / 60);
  if (hour < 24) return `${hour} soat oldin`;

  const day = Math.floor(hour / 24);
  if (day < 7) return `${day} kun oldin`;

  // Bir haftadan oshgach nisbiy vaqt ma'nosini yo'qotadi ("23 kun
  // oldin" hech narsa aytmaydi) — aniq sana ko'rsatiladi.
  const d = new Date(t);
  const p2 = (x: number) => String(x).padStart(2, "0");
  return `${p2(d.getDate())}.${p2(d.getMonth() + 1)}.${d.getFullYear()}`;
}
