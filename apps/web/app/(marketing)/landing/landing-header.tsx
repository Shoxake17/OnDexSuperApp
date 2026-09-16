"use client";

import { Download, Menu, X } from "lucide-react";
import { useEffect, useState } from "react";
import { ANDROID_DOWNLOAD_PATH, EATS_LOGIN_URL } from "./links";
import { OndexLogo } from "./logo";

export type LandingNavItem = { label: string; id: string };

/**
 * Landing sarlavhasi — navigatsiya bilan.
 *
 * ┌─ NEGA KLIENT KOMPONENT (2026-09-16) ───────────────────────────────┐
 * Avval menyu oddiy `#anchor` havolalar edi va tagiga chiziq DOIM
 * "Bosh sahifa" da turardi: bosilganda chiziq ko'chmasdi, uchta bo'lim
 * (Biz haqimizda, Hujjatlar, Yordam) esa futer ustunlariga ishora
 * qilgani uchun sahifa deyarli qimirlamasdi — "bosildi-yu hech narsa
 * bo'lmadi" taassuroti. Telefonda esa menyu umuman yo'q edi.
 *
 * Endi: har bir band o'z bo'limiga SILLIQ suriladi (sarlavha balandligi
 * `scroll-mt` bilan hisobga olinadi), faol bo'lim skroll bo'yicha
 * aniqlanib tagiga chiziq ko'chadi, telefonda ochiladigan menyu bor.
 * JavaScript o'chiq bo'lsa ham oddiy havola sifatida ishlaydi.
 * └────────────────────────────────────────────────────────────────────┘
 */
export function LandingHeader({ items }: { items: LandingNavItem[] }) {
  const [active, setActive] = useState(items[0]?.id ?? "");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const sections = items
      .map((i) => document.getElementById(i.id))
      .filter((el): el is HTMLElement => el !== null);
    if (sections.length === 0) return;

    // Sarlavha ostidagi chiziqdan YUQORIDA boshlangan eng oxirgi bo'lim —
    // joriy bo'lim. Sahifa oxiriga yetganda oxirgisi (qisqa bo'lim tepasi
    // chiziqqa hech qachon yetib bormaydi).
    const LINE = 120;
    let raf = 0;
    const update = () => {
      raf = 0;
      let current = sections[0].id;
      for (const s of sections) {
        if (s.getBoundingClientRect().top <= LINE) current = s.id;
      }
      const atBottom =
        window.innerHeight + window.scrollY >=
        document.documentElement.scrollHeight - 4;
      if (atBottom) current = sections[sections.length - 1].id;
      setActive(current);
    };
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, [items]);

  function go(e: React.MouseEvent<HTMLAnchorElement>, id: string) {
    const el = document.getElementById(id);
    if (!el) return; // oddiy havola sifatida ishlasin
    e.preventDefault();
    setOpen(false);
    setActive(id);
    el.scrollIntoView({ behavior: "smooth", block: "start" });
    window.history.replaceState(null, "", `#${id}`);
  }

  return (
    <header className="sticky top-0 z-40 border-b border-neutral-100 bg-white/90 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-6 px-4 sm:px-6">
        <a href="#bosh" onClick={(e) => go(e, "bosh")} aria-label="OnDex — bosh sahifa">
          <OndexLogo size={30} />
        </a>

        <nav aria-label="Asosiy bo'limlar" className="ml-4 hidden items-center gap-6 lg:flex">
          {items.map((n) => {
            const isActive = n.id === active;
            return (
              <a
                key={n.id}
                href={`#${n.id}`}
                onClick={(e) => go(e, n.id)}
                aria-current={isActive ? "true" : undefined}
                className={`relative pb-1 text-sm transition-colors after:absolute after:inset-x-0 after:-bottom-0.5 after:h-0.5 after:origin-left after:rounded-full after:bg-brand after:transition-transform after:duration-200 ${
                  isActive
                    ? "font-semibold text-brand after:scale-x-100"
                    : "font-medium text-neutral-600 after:scale-x-0 hover:text-neutral-900 hover:after:scale-x-100"
                }`}
              >
                {n.label}
              </a>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2 sm:gap-3">
          <a
            href={EATS_LOGIN_URL}
            className="hidden rounded-xl border border-neutral-200 px-4 py-2 text-sm font-semibold transition-colors hover:bg-neutral-50 sm:inline-block"
          >
            Kirish
          </a>
          <a
            href={ANDROID_DOWNLOAD_PATH}
            className="inline-flex items-center gap-2 rounded-xl bg-brand px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-brand-light"
          >
            <span className="hidden sm:inline">Ilovani yuklab olish</span>
            <span className="sm:hidden">Yuklab olish</span>
            <Download className="h-4 w-4" />
          </a>
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
            aria-controls="landing-mobile-nav"
            aria-label={open ? "Menyuni yopish" : "Menyuni ochish"}
            className="flex h-10 w-10 items-center justify-center rounded-xl border border-neutral-200 text-neutral-700 lg:hidden"
          >
            {open ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>
      </div>

      {open && (
        <nav
          id="landing-mobile-nav"
          aria-label="Asosiy bo'limlar"
          className="border-t border-neutral-100 bg-white px-4 pb-4 pt-2 lg:hidden"
        >
          {items.map((n) => (
            <a
              key={n.id}
              href={`#${n.id}`}
              onClick={(e) => go(e, n.id)}
              className={`flex items-center rounded-xl px-3 py-3 text-[15px] ${
                n.id === active
                  ? "bg-orange-50 font-semibold text-brand"
                  : "font-medium text-neutral-700"
              }`}
            >
              {n.label}
            </a>
          ))}
          <a
            href={EATS_LOGIN_URL}
            className="mt-2 flex items-center justify-center rounded-xl border border-neutral-200 px-3 py-3 text-[15px] font-semibold sm:hidden"
          >
            Kirish
          </a>
        </nav>
      )}
    </header>
  );
}
