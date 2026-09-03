"use client";

import { ChevronRight, LogOut, Mail, MapPin } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { getTelegramWebApp } from "@/lib/telegram";
import MobileSheet from "../mobile-sheet";
import { AppButton } from "../ui";

// "Profil" — pastki menyudagi beshinchi bo'lim. `GET /me`.
//
// Tahrirlash ATAYLAB yo'q: `POST /me` proksi allowlist'ida yo'q va uni
// ochish alohida qaror (nima tahrirlanishi mumkinligi bilan birga).
// Bu sahifa hozircha ko'rsatadi, manzilni esa mavjud `/address`
// ekranida o'zgartiradi.

type Me = {
  id: string;
  phone: string;
  name?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  phone_verified?: boolean;
  address?: { text?: string };
};

function displayName(me: Me): string {
  const full = [me.first_name, me.last_name].filter(Boolean).join(" ").trim();
  return me.name?.trim() || full || "Mijoz";
}

export default function ProfilePage() {
  const [me, setMe] = useState<Me | null>(null);
  const [failed, setFailed] = useState(false);
  const [inTelegram, setInTelegram] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Telegram ichida ekanini BIR MARTA, ulangandan keyin aniqlaymiz:
    // `getTelegramWebApp()` render paytida chaqirilsa server va klient
    // natijasi farq qilib, hidratsiya ogohlantirishi chiqardi.
    setInTelegram(Boolean(getTelegramWebApp()));

    void (async () => {
      try {
        const res = await fetch("/api/proxy/me");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        setMe(await res.json());
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  async function logout() {
    setBusy(true);
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      // Cookie baribir server tomonda tozalanadi; tarmoq xatosi
      // chiqishga to'sqinlik qilmasin.
    }
    window.location.replace("/");
  }

  return (
    <MobileSheet className="px-4 pb-28 pt-3">
      <h1 className="text-2xl font-bold">Profil</h1>

      {failed ? (
        <div className="py-14 text-center">
          <p className="text-neutral-500">Profilni yuklab bo&apos;lmadi.</p>
          <button
            onClick={() => window.location.reload()}
            className="mt-3 text-sm font-semibold text-brand"
          >
            Qaytadan urinish
          </button>
        </div>
      ) : me === null ? (
        <div className="mt-5 flex flex-col gap-3">
          <div className="h-20 animate-pulse rounded-2xl bg-neutral-100 dark:bg-neutral-800" />
          <div className="h-14 animate-pulse rounded-2xl bg-neutral-100 dark:bg-neutral-800" />
          <div className="h-14 animate-pulse rounded-2xl bg-neutral-100 dark:bg-neutral-800" />
        </div>
      ) : (
        <>
          <div className="tg-surface mt-5 flex items-center gap-3 rounded-2xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-brand text-xl font-bold text-white">
              {displayName(me).charAt(0).toUpperCase()}
            </div>
            <div className="min-w-0">
              <p className="truncate text-lg font-bold">{displayName(me)}</p>
              <p className="tg-muted truncate text-sm text-neutral-500">
                {me.phone}
              </p>
            </div>
          </div>

          {/* Telefon bloki ATAYLAB yo'q: raqam yuqoridagi kartada, ism
              ostida allaqachon turibdi. Ikkinchi marta ko'rsatish
              ro'yxatni cho'zardi va yangi ma'lumot bermasdi. */}
          <div className="mt-4 flex flex-col gap-2">
            {me.email && <Row Icon={Mail} label="Email" value={me.email} />}
            <Link
              href="/address"
              className="tg-surface flex items-center gap-3 rounded-2xl border border-neutral-200 bg-white p-3.5 active:opacity-70 dark:border-neutral-800 dark:bg-neutral-900"
            >
              <MapPin size={19} className="shrink-0 text-neutral-400" />
              <div className="min-w-0 flex-1">
                <p className="tg-muted text-xs text-neutral-500">
                  Yetkazish manzili
                </p>
                <p className="truncate text-[15px]">
                  {me.address?.text?.trim() || "Tanlanmagan"}
                </p>
              </div>
              <ChevronRight size={18} className="shrink-0 text-neutral-400" />
            </Link>
          </div>

          {/* ┌─ CHIQISH FAQAT TELEGRAM TASHQARISIDA ────────────────────┐
              Mini App ichida sessiya har ochilishda `initData` orqali
              QAYTA tiklanadi. "Chiqish" bosilsa, sahifa yangilanishi
              bilan foydalanuvchi darhol qayta kirgan bo'lardi — ya'ni
              tugma hech narsa qilmagandek ko'rinardi. O'lik tugma
              qo'ymaymiz.
              └──────────────────────────────────────────────────────────┘ */}
          {!inTelegram && (
            <div className="mt-8">
              <AppButton variant="outline" onClick={logout} disabled={busy}>
                <LogOut size={18} className="mr-2" />
                {busy ? "Chiqilmoqda…" : "Chiqish"}
              </AppButton>
            </div>
          )}
        </>
      )}
    </MobileSheet>
  );
}

function Row({
  Icon,
  label,
  value,
}: {
  Icon: typeof Mail;
  label: string;
  value: string;
}) {
  return (
    <div className="tg-surface flex items-center gap-3 rounded-2xl border border-neutral-200 bg-white p-3.5 dark:border-neutral-800 dark:bg-neutral-900">
      <Icon size={19} className="shrink-0 text-neutral-400" />
      <div className="min-w-0">
        <p className="tg-muted text-xs text-neutral-500">{label}</p>
        <p className="truncate text-[15px]">{value}</p>
      </div>
    </div>
  );
}
