"use client";

import {
  Bell,
  ChevronRight,
  Heart,
  LogOut,
  MapPin,
  Search,
  ShoppingBag,
  User,
  X,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import DesktopAddressDialog from "./desktop-address-dialog";
import DesktopCartMenu from "./desktop-cart-menu";
import {
  FavoritesPanel,
  NotificationsPanel,
  OrdersPanel,
} from "./desktop-panels";
import DesktopProfileDialog from "./desktop-profile-dialog";

// Kompyuter ko'rinishidagi YAGONA navbar — bosh sahifa ham, restoran
// menyusi ham shuni ishlatadi.
//
// ┌─ NEGA ALOHIDA FAYL ────────────────────────────────────────────────┐
// Avval u `desktop-home.tsx` ichida edi. Menyu sahifasiga ham navbar
// kerak bo'lgach, uni ko'chirib yozish — o'sha "API klienti 4 marta
// ko'chirilgan" xatosini takrorlash bo'lardi: keyingi har bir tuzatish
// (savat, profil menyusi, qidiruv) ikki joyda qilinishi kerak edi.
// └────────────────────────────────────────────────────────────────────┘
export default function DesktopNavbar({
  signedIn,
  /** Qidiruv maydonidagi kontekst yorlig'i (masalan restoran nomi). */
  scopeLabel,
  /** Yorliqdagi ✕ bosilganda (odatda bosh sahifaga qaytish). */
  onClearScope,
  placeholder = "OnDex'dan qidirish",
  initialQuery = "",
}: {
  signedIn: boolean;
  scopeLabel?: string;
  onClearScope?: () => void;
  placeholder?: string;
  /** Qidiruv sahifasida — maydonda joriy so'rov turadi. */
  initialQuery?: string;
}) {
  const router = useRouter();
  const [query, setQuery] = useState(initialQuery);
  // Qidiruv sahifasida yangi so'rov bilan qayta chizilganda komponent
  // saqlanib qoladi — maydon ham yangi so'rovni ko'rsatsin (effekt emas,
  // render paytida moslash: bir kadr eski qiymat ko'rinmaydi).
  const [syncedQuery, setSyncedQuery] = useState(initialQuery);
  if (syncedQuery !== initialQuery) {
    setSyncedQuery(initialQuery);
    setQuery(initialQuery);
  }
  const [addressOpen, setAddressOpen] = useState(false);
  // Saqlangan manzil nomi — yo'q bo'lsa shahar nomi ko'rsatiladi
  // (platforma faqat Chust uchun, ya'ni bu doim to'g'ri zaxira).
  const [addressLabel, setAddressLabel] = useState("Chust");

  useEffect(() => {
    if (!signedIn) return;
    let cancelled = false;
    fetch("/api/proxy/me/address")
      .then((r) => (r.ok ? r.json() : null))
      .then((a: { text?: string } | null) => {
        const text = a?.text?.trim();
        if (text && !cancelled) setAddressLabel(text);
      })
      .catch(() => {
        // Manzil ko'rsatilmaydi, xolos — "Chust" zaxira qiymati qoladi.
      });
    return () => {
      cancelled = true;
    };
  }, [signedIn]);

  function submitSearch(e: React.FormEvent) {
    e.preventDefault();
    const q = query.trim();
    if (q) router.push(`/search?category=${encodeURIComponent(q)}`);
  }

  return (
    // ┌─ NAVBAR ALOHIDA QATLAM SIFATIDA ──────────────────────────────┐
    // Foni sahifa fonidan (`#141414`) ajralib turadi (`#302F2D`),
    // pastki burchaklari aylantirilgan va pastdan chegara chizig'i bor
    // (namuna: image/eats.png). Uchalasi birga bo'lgandagina u "ustida
    // turgan panel" bo'lib ko'rinadi.
    // └───────────────────────────────────────────────────────────────┘
    <header className="sticky top-0 z-30 rounded-b-[22px] border-b border-white/10 bg-[#302F2D]">
      {/* Navbar chetlari ATAYLAB kontentdan tor: namunadagidek logotip
          ekran chetiga yaqinroq turadi, kontent ichkariroqdan
          boshlanadi. */}
      <div className="mx-auto flex max-w-[1600px] items-center gap-4 px-6 py-3.5">
        <Link href="/" className="shrink-0 text-2xl font-extrabold tracking-tight">
          On<span className="text-brand">Dex</span>
        </Link>

        <form
          onSubmit={submitSearch}
          className="flex h-11 min-w-0 flex-1 items-center gap-2 rounded-full bg-white/10 px-3 transition-colors focus-within:bg-white/[0.14]"
        >
          {/* Kontekst yorlig'i — namunadagi "KFC ✕" chipi: qidiruv
              qayerda ketayotgani ko'rinib turadi. */}
          {scopeLabel ? (
            <span className="flex h-7 shrink-0 items-center gap-1.5 rounded-full bg-white/15 pl-3 pr-2 text-[13px] font-semibold">
              <span className="max-w-[160px] truncate">{scopeLabel}</span>
              {onClearScope && (
                <button
                  type="button"
                  onClick={onClearScope}
                  aria-label="Restorandan chiqish"
                  className="flex h-4 w-4 items-center justify-center rounded-full bg-white/20 text-white/80 transition-colors hover:bg-white/30 hover:text-white"
                >
                  <X size={11} />
                </button>
              )}
            </span>
          ) : (
            <Search size={18} className="ml-1 shrink-0 text-white/50" />
          )}
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={placeholder}
            className="w-full min-w-0 bg-transparent text-sm text-white placeholder:text-white/50 outline-none"
          />
        </form>

        {/* ┌─ MANZIL ──────────────────────────────────────────────────┐
            Avval bu shunchaki "Chust" yozuvi edi — bosilmaydigan yorliq.
            Endi u yetkazish manzilini tanlash oynasini ochadi
            (`desktop-address-dialog.tsx`, namuna: image/maps.png).

            Kirmagan foydalanuvchi uchun manzilni saqlab bo'lmaydi
            (`POST /me/address` auth talab qiladi), shuning uchun u
            avval kirish sahifasiga yuboriladi — oyna ochilib, keyin
            "saqlab bo'lmadi" xatosini ko'rsatish yomon oqim bo'lardi.
            └───────────────────────────────────────────────────────────┘ */}
        <button
          type="button"
          onClick={() => {
            if (!signedIn) {
              router.push(
                `/login?next=${encodeURIComponent(window.location.pathname)}`,
              );
              return;
            }
            setAddressOpen(true);
          }}
          className="flex shrink-0 items-center gap-2 rounded-full bg-white/10 px-4 py-2.5 text-sm font-medium transition-colors hover:bg-white/[0.16]"
        >
          <MapPin size={16} className="text-white/60" />
          <span className="max-w-[160px] truncate">{addressLabel}</span>
        </button>

        {/* Savat — FAQAT bo'sh bo'lmaganda ko'rinadi va sahifaga
            o'tmasdan, shu yerda ochiladi (`desktop-cart-menu.tsx`). */}
        <DesktopCartMenu />

        {/* Kirmagan bo'lsa "Kirish" tugmasi; kirgan bo'lsa —
            image/profile.png namunasidagi avatar + ochiladigan menyu. */}
        {signedIn ? (
          <UserMenu />
        ) : (
          <Link
            href="/login"
            className="shrink-0 rounded-full bg-white px-5 py-2.5 text-sm font-bold text-[#141414] transition-colors hover:bg-white/90"
          >
            Kirish
          </Link>
        )}
      </div>

      {addressOpen && (
        <DesktopAddressDialog
          onClose={() => setAddressOpen(false)}
          onSaved={(text) => setAddressLabel(text)}
        />
      )}
    </header>
  );
}

type Me = {
  name?: string;
  first_name?: string;
  phone?: string;
};

function displayName(me: Me | null): string {
  if (!me) return "Foydalanuvchi";
  return me.name?.trim() || me.first_name?.trim() || me.phone || "Foydalanuvchi";
}

/** Navbardan ochiladigan panellar (`desktop-panels.tsx`). */
type PanelKind = "notifications" | "orders" | "favorites";

/**
 * Bell + avatar va ulardan ochiladigan panellar.
 *
 * ┌─ NEGA PANEL, SAHIFAGA O'TISH EMAS ────────────────────────────────┐
 * Savat bilan bir xil mantiq: bildirishnoma, buyurtmalar va
 * sevimlilar — kompyuterda tez ko'z tashlanadigan ro'yxatlar.
 * Har biri uchun butun sahifani almashtirish mijozni menyudan
 * uzoqlashtiradi va orqaga qaytishga majbur qiladi.
 *
 * Sahifalarning o'zi qoladi (mobil ko'rinish + to'g'ridan-to'g'ri
 * havola) — panel pastida "Hammasini ko'rish" havolasi bor.
 * └───────────────────────────────────────────────────────────────────┘
 */
function UserMenu() {
  const [me, setMe] = useState<Me | null>(null);
  const [open, setOpen] = useState(false);
  const [panel, setPanel] = useState<PanelKind | null>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [unread, setUnread] = useState(0);
  const [busy, setBusy] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // Bir vaqtda BITTA qatlam ochiq bo'ladi — panel ochilsa menyu
  // yopiladi va aksincha. Ikkalasi birga ochiq qolsa ular ustma-ust
  // tushib, qaysi biri faol ekani bilinmasdi.
  function openPanel(kind: PanelKind) {
    setOpen(false);
    setPanel((prev) => (prev === kind ? null : kind));
  }

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const res = await fetch("/api/proxy/me");
        if (res.ok && !cancelled) setMe(await res.json());
      } catch {
        // Nom ko'rsatilmaydi, xolos — menyu baribir ishlaydi.
      }
    })();
    void (async () => {
      try {
        const res = await fetch("/api/proxy/notifications/unread-count");
        if (!res.ok) return;
        const d = (await res.json()) as { count?: number };
        if (!cancelled && typeof d.count === "number") setUnread(d.count);
      } catch {
        // Badge'siz qoladi.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!open && !panel) return;
    function onClick(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
        setPanel(null);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setOpen(false);
        setPanel(null);
      }
    }
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open, panel]);

  async function logout() {
    setBusy(true);
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      // Cookie baribir server tomonda tozalanadi.
    }
    window.location.replace("/");
  }

  return (
    <div ref={rootRef} className="relative flex shrink-0 items-center gap-2">
      <button
        type="button"
        onClick={() => {
          openPanel("notifications");
          // Panel ochilishi bilan ro'yxat o'qilgan deb belgilanadi,
          // ya'ni nuqta ham darhol so'nishi kerak.
          setUnread(0);
        }}
        aria-label="Bildirishnomalar"
        aria-expanded={panel === "notifications"}
        className="relative flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-white/10 text-white/80 transition-colors hover:bg-white/[0.16]"
      >
        <Bell size={18} />
        {unread > 0 && (
          <span
            aria-hidden="true"
            className="absolute right-1 top-1 h-2 w-2 rounded-full bg-brand"
          />
        )}
      </button>

      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-label="Profil menyusi"
        aria-expanded={open}
        className="flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/[0.16]"
      >
        <User size={18} />
      </button>

      {open && (
        <div className="absolute right-0 top-[calc(100%+10px)] w-72 overflow-hidden rounded-2xl bg-[#1e1e1e] py-2 text-white shadow-2xl ring-1 ring-white/10">
          {/* Profil ham sahifaga emas, OYNAGA ochiladi (namuna:
              image/profiles.png) — savat va manzil bilan bir xil
              naqsh. `/profile` sahifasi mobil uchun qoladi. */}
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              setProfileOpen(true);
            }}
            className="flex w-full items-center justify-between px-4 py-3.5 text-left hover:bg-white/5"
          >
            <div className="min-w-0">
              <p className="truncate text-base font-bold">{displayName(me)}</p>
              <p className="mt-0.5 text-xs text-white/50">
                Ma&apos;lumotlarim
              </p>
            </div>
            <ChevronRight size={18} className="shrink-0 text-white/40" />
          </button>

          <div className="my-1 border-t border-white/10" />

          {/* Buyurtmalar va sevimlilar — SAHIFAGA emas, panelga
              ochiladi (yuqoridagi izoh). Manzil esa oyna ochadi. */}
          <button
            type="button"
            onClick={() => openPanel("orders")}
            className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-white/85 hover:bg-white/5"
          >
            <ShoppingBag size={17} className="shrink-0 text-white/50" />
            Buyurtmalarim
          </button>
          <button
            type="button"
            onClick={() => openPanel("favorites")}
            className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-white/85 hover:bg-white/5"
          >
            <Heart size={17} className="shrink-0 text-white/50" />
            Sevimlilar
          </button>
          <button
            type="button"
            onClick={() => openPanel("notifications")}
            className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-white/85 hover:bg-white/5"
          >
            <Bell size={17} className="shrink-0 text-white/50" />
            Bildirishnomalar
          </button>

          <div className="my-1 border-t border-white/10" />

          <button
            type="button"
            onClick={logout}
            disabled={busy}
            className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-white/85 hover:bg-white/5 disabled:opacity-50"
          >
            <LogOut size={17} className="shrink-0 text-white/50" />
            {busy ? "Chiqilmoqda…" : "Chiqish"}
          </button>
        </div>
      )}

      {panel === "notifications" && (
        <NotificationsPanel onNavigate={() => setPanel(null)} />
      )}
      {panel === "orders" && <OrdersPanel onNavigate={() => setPanel(null)} />}
      {panel === "favorites" && (
        <FavoritesPanel onNavigate={() => setPanel(null)} />
      )}

      {profileOpen && (
        <DesktopProfileDialog
          onClose={() => setProfileOpen(false)}
          // Nom o'zgarsa menyudagi sarlavha ham darhol yangilanadi —
          // aks holda mijoz saqlagan nomini ko'rish uchun sahifani
          // yangilashi kerak bo'lardi.
          onSaved={(updated) => setMe(updated)}
        />
      )}
    </div>
  );
}
