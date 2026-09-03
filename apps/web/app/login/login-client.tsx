"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, MessageCircle, Phone, ShieldCheck } from "lucide-react";
import { getTelegramWebApp } from "@/lib/telegram";
import { AppButton } from "../(food)/ui";

// Kirish sahifasi — ikki kanal:
//
//   1. Telegram bot orqali (STANDART, hozircha YAGONA ISHLAYDIGAN
//      yo'l — SMS provayder hali ULANMAGAN, `internal/users/service.go`
//      dagi `s.sms.Send` production'da xato qaytaradi). Foydalanuvchi
//      hech narsa yozmaydi: bot raqamini so'raydi, tasdiqlangач
//      "OnDex'ga qaytish" tugmasi bosiladi va
//      `/api/auth/telegram-return` sessiyani yakunlaydi
//      (`internal/telegram/verifier.go`dagi `StartLoginWeb`/
//      `Pending.Web`).
//   2. Telefon + SMS kod (ZAXIRA — SMS ulangach standart bo'lishi
//      mumkin). Mavjud `/api/auth/request-code`/`verify`ni ishlatadi.
//
// Ikkalasi ham backend'da akkauntni avtomatik topadi/yaratadi —
// alohida "ro'yxatdan o'tish" qadami yo'q.

const RESEND_COOLDOWN = 60; // users.resendCooldown bilan mos (soniya)
const CODE_LENGTH = 6; // users.randomCode — "%06d"

type Mode = "choose" | "phone" | "code";

export default function LoginClient({
  next,
  initialError = null,
}: {
  next: string;
  initialError?: string | null;
}) {
  const router = useRouter();

  const [mode, setMode] = useState<Mode>("choose");
  const [tgBusy, setTgBusy] = useState(false);
  const [tgWaiting, setTgWaiting] = useState(false);
  const [tgError, setTgError] = useState<string | null>(initialError);

  const [digits, setDigits] = useState(""); // 998'dan keyingi 9 ta raqam
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [devCode, setDevCode] = useState<string | null>(null);
  const codeInputRef = useRef<HTMLInputElement>(null);

  // Telegram ichida bu sahifa umuman ko'rinmasin — o'sha yerda kirish
  // avtomatik (`telegram-auth.tsx`). Amalda bu yerga hech qachon
  // Telegram'dan kelinmaydi, lekin himoya qatlami sifatida qoldiriladi.
  useEffect(() => {
    if (getTelegramWebApp()) router.replace("/");
  }, [router]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(t);
  }, [cooldown]);

  useEffect(() => {
    if (mode === "code") codeInputRef.current?.focus();
  }, [mode]);

  async function startTelegramLogin() {
    if (tgBusy) return;
    setTgBusy(true);
    setTgError(null);
    try {
      const res = await fetch("/api/auth/telegram-login/start", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ next }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok || typeof data?.deep_link !== "string") {
        setTgError(data?.error ?? "Telegram bilan bog'lanib bo'lmadi.");
        return;
      }
      // YANGI OYNADA: shu sahifa ochiq qoladi va "kutilmoqda" holatini
      // ko'rsatadi — botdagi "OnDex'ga qaytish" tugmasi qaytadan shu
      // brauzerni (istalgan oynasini) ochadi va yakunlaydi.
      window.open(data.deep_link, "_blank", "noopener,noreferrer");
      setTgWaiting(true);
    } catch {
      setTgError("Tarmoq xatosi. Internet aloqasini tekshiring.");
    } finally {
      setTgBusy(false);
    }
  }

  const phone = `+998${digits}`;
  const phoneValid = digits.length === 9;

  async function requestCode() {
    if (!phoneValid || busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/auth/request-code", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        if (typeof data?.retry_after === "number") setCooldown(data.retry_after);
        setError(data?.error ?? "Kod yuborib bo'lmadi. Qaytadan urinib ko'ring.");
        return;
      }
      setDevCode(typeof data?.dev_code === "string" ? data.dev_code : null);
      setCode("");
      setMode("code");
      setCooldown(RESEND_COOLDOWN);
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
    } finally {
      setBusy(false);
    }
  }

  async function verifyCode(candidate: string) {
    if (candidate.length !== CODE_LENGTH || busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/auth/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone, code: candidate }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        setError(data?.error ?? "Kod noto'g'ri. Qaytadan urinib ko'ring.");
        setCode("");
        setBusy(false);
        return;
      }
      // TO'LIQ SAHIFA YUKLASH ATAYLAB: server komponentlari (root
      // layout'dagi `signedIn`, himoyalangan sahifalardagi
      // `requireAuth`) cookie'ni FAQAT yangi so'rovda ko'radi.
      window.location.replace(next);
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
      setBusy(false);
    }
  }

  function onDigitsChange(raw: string) {
    setDigits(raw.replace(/\D/g, "").slice(0, 9));
  }

  function onCodeChange(raw: string) {
    const next = raw.replace(/\D/g, "").slice(0, CODE_LENGTH);
    setCode(next);
    if (next.length === CODE_LENGTH) void verifyCode(next);
  }

  return (
    <div className="mx-auto flex min-h-dvh max-w-sm flex-col justify-center px-6 py-10">
      {mode !== "choose" && (
        <button
          type="button"
          onClick={() => {
            setMode("choose");
            setError(null);
            setCode("");
          }}
          aria-label="Orqaga"
          className="-ml-1.5 mb-4 flex h-10 w-10 items-center justify-center rounded-full text-neutral-600 active:bg-neutral-200 dark:text-neutral-300 dark:active:bg-neutral-700"
        >
          <ArrowLeft size={24} />
        </button>
      )}

      {mode === "choose" && (
        <>
          <div className="mb-6 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand/10 text-brand">
            <MessageCircle size={26} />
          </div>
          <h1 className="text-2xl font-bold">Kirish</h1>
          <p className="mt-1.5 text-sm text-neutral-500">
            Buyurtma berish uchun Telegram orqali tasdiqlang — kod yozish
            shart emas, bot o'zi so'raydi.
          </p>

          <div className="mt-6">
            <AppButton onClick={startTelegramLogin} disabled={tgBusy}>
              {tgBusy ? "Ochilmoqda…" : "Telegram orqali kirish"}
            </AppButton>
          </div>

          {tgWaiting && (
            <p className="mt-4 text-center text-sm text-neutral-500">
              Yangi oynada botni tasdiqlang — u yerdagi{" "}
              <span className="font-medium text-neutral-700 dark:text-neutral-300">
                &quot;OnDex&apos;ga qaytish&quot;
              </span>{" "}
              tugmasi bosilgach, shu sahifa avtomatik davom etadi.
            </p>
          )}
          {tgError && <p className="mt-4 text-sm text-red-500">{tgError}</p>}

          <button
            type="button"
            onClick={() => setMode("phone")}
            className="mt-6 text-center text-sm font-semibold text-neutral-500"
          >
            yoki telefon raqam bilan
          </button>
        </>
      )}

      {mode === "phone" && (
        <>
          <div className="mb-6 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand/10 text-brand">
            <Phone size={26} />
          </div>
          <h1 className="text-2xl font-bold">Telefon raqam</h1>
          <p className="mt-1.5 text-sm text-neutral-500">
            SMS orqali tasdiqlash kodi yuboramiz.
          </p>

          <form
            className="mt-6"
            onSubmit={(e) => {
              e.preventDefault();
              void requestCode();
            }}
          >
            <label className="block text-xs font-medium text-neutral-500">
              Telefon raqam
            </label>
            <div className="mt-1.5 flex h-[52px] items-center rounded-2xl border border-neutral-300 px-4 focus-within:border-brand dark:border-neutral-700">
              <span className="mr-1 text-base font-medium text-neutral-500">
                +998
              </span>
              <input
                type="tel"
                inputMode="numeric"
                autoComplete="tel"
                placeholder="90 123 45 67"
                value={digits}
                onChange={(e) => onDigitsChange(e.target.value)}
                className="min-w-0 flex-1 bg-transparent text-base outline-none"
                autoFocus
              />
            </div>

            {error && <p className="mt-3 text-sm text-red-500">{error}</p>}

            <div className="mt-6">
              <AppButton type="submit" disabled={!phoneValid || busy}>
                {busy ? "Yuborilmoqda…" : "Kod yuborish"}
              </AppButton>
            </div>
          </form>
        </>
      )}

      {mode === "code" && (
        <>
          <div className="mb-6 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand/10 text-brand">
            <ShieldCheck size={26} />
          </div>
          <h1 className="text-2xl font-bold">Kod kiriting</h1>
          <p className="mt-1.5 text-sm text-neutral-500">
            <span className="font-medium text-neutral-700 dark:text-neutral-300">
              {phone}
            </span>{" "}
            raqamiga {CODE_LENGTH} xonali kod yubordik.
          </p>
          {devCode && (
            <p className="mt-2 text-sm font-mono text-brand">
              DEV kod: {devCode}
            </p>
          )}

          <div className="mt-6">
            <input
              ref={codeInputRef}
              type="tel"
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => onCodeChange(e.target.value)}
              maxLength={CODE_LENGTH}
              className="h-[52px] w-full rounded-2xl border border-neutral-300 px-4 text-center text-2xl font-bold tracking-[0.5em] outline-none focus:border-brand dark:border-neutral-700"
              placeholder="——————"
            />

            {error && <p className="mt-3 text-sm text-red-500">{error}</p>}

            <div className="mt-6">
              <AppButton
                onClick={() => void verifyCode(code)}
                disabled={code.length !== CODE_LENGTH || busy}
              >
                {busy ? "Tekshirilmoqda…" : "Tasdiqlash"}
              </AppButton>
            </div>

            <button
              type="button"
              onClick={() => void requestCode()}
              disabled={cooldown > 0 || busy}
              className="mt-4 w-full text-center text-sm font-semibold text-brand disabled:text-neutral-400"
            >
              {cooldown > 0 ? `Qayta yuborish (${cooldown})` : "Kodni qayta yuborish"}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
