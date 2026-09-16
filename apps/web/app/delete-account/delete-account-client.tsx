"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Send, Smartphone, TriangleAlert } from "lucide-react";
import {
  AUTH_BORDER as BORDER,
  AUTH_BRAND as BRAND,
  AUTH_MUTED as MUTED,
  AUTH_TEXT as TEXT,
  AuthButton,
  CODE_LENGTH,
  ErrorNote,
  PHONE_DIGITS,
  groupPhone,
} from "@/lib/auth-widgets";

// ─────────────────────────────────────────────────────────────────────
// "Akkauntni o'chirish" — https://ondex.uz/delete-account.
//
// Google Play / App Store talabi: hisobni o'chirish so'rovi ILOVA
// O'RNATILMAGAN holatda ham, brauzerdan bajarila olishi kerak. Mobil
// ilovada ham xuddi shu amal bor (Profil -> "Akkauntni o'chirish"),
// ikkalasi bir xil backend'ga (`POST /me/delete-account`) murojaat
// qiladi.
//
// OQIM — kirish sahifasi bilan AYNAN bir xil identifikatsiya
// (`/api/auth/telegram-start` + `/api/auth/verify`, SMS EMAS — sabab
// `otp-delivery-channels-prod` xotirasida: Eskiz o'chirilgan), keyin
// UCHINCHI qadam qo'shiladi — tasdiqlash va o'chirish.
//
// MA'LUMOT O'CHIRILMAYDI. Faqat kirish yopiladi (backend: `SoftDelete`).
// O'sha telefon bilan qaytadan tasdiqlansa — akkaunt AVTOMATIK tiklanadi.
// ─────────────────────────────────────────────────────────────────────

type Step = "phone" | "code" | "confirm" | "done";

type Me = { name?: string; phone?: string };

const CONFIRM_WORD = "O'CHIRISH";

export default function DeleteAccountClient() {
  const [step, setStep] = useState<Step>("phone");
  const [digits, setDigits] = useState("");
  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [me, setMe] = useState<Me | null>(null);

  const phone = `+998${digits}`;
  const phoneValid = digits.length === PHONE_DIGITS;

  async function startTelegram() {
    if (!phoneValid || busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/auth/telegram-start", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok || typeof data?.deep_link !== "string") {
        setError(data?.error ?? "Telegram bilan bog'lanib bo'lmadi.");
        return;
      }
      setDeepLink(data.deep_link);
      setStep("code");
      window.open(data.deep_link, "_blank", "noopener,noreferrer");
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
    } finally {
      setBusy(false);
    }
  }

  async function verifyCode(code: string): Promise<boolean> {
    if (code.length !== CODE_LENGTH || busy) return false;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/auth/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone, code }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        setError(data?.error ?? "Kod noto'g'ri. Qaytadan urinib ko'ring.");
        return false;
      }
      // Kim ekanini SAHIFADA ko'rsatish uchun — odam "bu haqiqatan
      // MENING akkauntimmi?" deb ishonch hosil qilsin.
      try {
        const meRes = await fetch("/api/proxy/me", { cache: "no-store" });
        const meData = await meRes.json().catch(() => ({}));
        if (meRes.ok) setMe({ name: meData?.name, phone: meData?.phone });
      } catch {
        // Ko'rsatish ixtiyoriy — bo'lmasa ham keyingi qadam ochiladi.
      }
      setStep("confirm");
      return true;
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function deleteAccount() {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/proxy/me/delete-account", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        setError(data?.error ?? "O'chirishning iloji bo'lmadi. Qaytadan urinib ko'ring.");
        return;
      }
      // Sessiya allaqachon serverda bekor qilindi (`Revoke`) — cookie'ni
      // ham tozalaymiz, aks holda brauzer "kirgan" holatda qolib ketardi.
      await fetch("/api/auth/logout", { method: "POST" }).catch(() => {});
      setStep("done");
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
    } finally {
      setBusy(false);
    }
  }

  function goBack() {
    if (step === "code") {
      setStep("phone");
      setError(null);
      setDeepLink(null);
      return;
    }
    if (step === "confirm") {
      setStep("code");
      setError(null);
      return;
    }
  }

  return (
    <div className="flex min-h-dvh justify-center" style={{ background: "#FDFBFA" }}>
      <main className="flex w-full max-w-[440px] flex-col px-5 pb-10 pt-5 sm:px-8">
        {step !== "done" && (
          <button
            type="button"
            onClick={goBack}
            aria-label="Orqaga"
            disabled={step === "phone"}
            className="flex h-[38px] w-[38px] items-center justify-center rounded-[11px] bg-white disabled:opacity-0"
            style={{ color: TEXT }}
          >
            <ArrowLeft size={20} />
          </button>
        )}

        <div className="mt-3">
          <p className="text-[28px] font-extrabold leading-none tracking-[-0.8px]" style={{ color: TEXT }}>
            On<span style={{ color: BRAND }}>Dex</span>
          </p>
          <p className="mt-2.5 text-[21px] font-extrabold" style={{ color: TEXT }}>
            Akkauntni o&apos;chirish
          </p>
        </div>

        <div className="mt-7">
          {step === "phone" && (
            <PhoneStep
              digits={digits}
              onDigits={setDigits}
              onSubmit={startTelegram}
              busy={busy}
              error={error}
              valid={phoneValid}
            />
          )}
          {step === "code" && (
            <CodeStep
              phone={phone}
              deepLink={deepLink}
              onVerify={verifyCode}
              onRestart={goBack}
              busy={busy}
              error={error}
              clearError={() => setError(null)}
            />
          )}
          {step === "confirm" && (
            <ConfirmStep me={me} onConfirm={deleteAccount} busy={busy} error={error} />
          )}
          {step === "done" && <DoneStep />}
        </div>
      </main>
    </div>
  );
}

// ── 1-qadam: telefon raqam ───────────────────────────────────────────
function PhoneStep({
  digits,
  onDigits,
  onSubmit,
  busy,
  error,
  valid,
}: {
  digits: string;
  onDigits: (v: string) => void;
  onSubmit: () => void;
  busy: boolean;
  error: string | null;
  valid: boolean;
}) {
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <p className="mb-5 text-[13px] leading-relaxed" style={{ color: MUTED }}>
        O&apos;chirmoqchi bo&apos;lgan OnDex akkauntingizga bog&apos;langan telefon
        raqamini kiriting. Egaligingizni tasdiqlash uchun Telegram orqali kod
        yuboramiz.
      </p>

      <label htmlFor="phone" className="mb-1.5 block pl-0.5 text-[12.5px] font-semibold" style={{ color: TEXT }}>
        Telefon raqam
      </label>
      <div className="flex h-[46px] items-center rounded-[11px] border bg-white" style={{ borderColor: BORDER }}>
        <div className="flex items-center gap-1.5 px-2.5">
          <Smartphone size={16} style={{ color: BRAND }} />
          <span className="text-[14.5px] font-bold" style={{ color: TEXT }}>
            +998
          </span>
        </div>
        <span className="h-6 w-px" style={{ background: BORDER }} />
        <input
          id="phone"
          type="tel"
          inputMode="numeric"
          autoComplete="tel"
          placeholder="90 123 45 67"
          value={groupPhone(digits)}
          onChange={(e) => onDigits(e.target.value.replace(/\D/g, "").slice(0, PHONE_DIGITS))}
          className="min-w-0 flex-1 bg-transparent px-2.5 text-[14.5px] outline-none"
          style={{ color: TEXT }}
          autoFocus
        />
      </div>

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="mt-4">
        <AuthButton busy={busy} disabled={!valid || busy}>
          Davom etish
        </AuthButton>
      </div>
    </form>
  );
}

// ── 2-qadam: Telegramdan kelgan kod (login-client.tsx bilan bir xil) ──
function CodeStep({
  phone,
  deepLink,
  onVerify,
  onRestart,
  busy,
  error,
  clearError,
}: {
  phone: string;
  deepLink: string | null;
  onVerify: (code: string) => Promise<boolean>;
  onRestart: () => void;
  busy: boolean;
  error: string | null;
  clearError: () => void;
}) {
  const [code, setCode] = useState<string[]>(Array(CODE_LENGTH).fill(""));
  const boxes = useRef<Array<HTMLInputElement | null>>([]);
  const joined = code.join("");

  useEffect(() => {
    boxes.current[0]?.focus();
  }, []);

  useEffect(() => {
    if (!error) return;
    setCode(Array(CODE_LENGTH).fill(""));
    boxes.current[0]?.focus();
  }, [error]);

  function fill(value: string, from: number) {
    const chars = value.replace(/\D/g, "").split("");
    if (chars.length === 0) return;
    const nextCode = [...code];
    let i = from;
    for (const ch of chars) {
      if (i >= CODE_LENGTH) break;
      nextCode[i] = ch;
      i += 1;
    }
    setCode(nextCode);
    boxes.current[Math.min(i, CODE_LENGTH - 1)]?.focus();
    const complete = nextCode.join("");
    if (complete.length === CODE_LENGTH && !nextCode.includes("")) {
      void onVerify(complete);
    }
  }

  return (
    <>
      <p className="text-[13px] leading-relaxed" style={{ color: MUTED }}>
        Telegramda bot ochildi. U yerda{" "}
        <span className="font-semibold" style={{ color: TEXT }}>
          «Raqamni ulashish»
        </span>{" "}
        tugmasini bosing — kod{" "}
        <span className="font-semibold" style={{ color: TEXT }}>
          {phone}
        </span>{" "}
        tasdiqlangach o&apos;sha chatga keladi.
      </p>

      {deepLink && (
        <a
          href={deepLink}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-3.5 flex h-[46px] items-center justify-center gap-2 rounded-xl border bg-white text-[13px] font-semibold"
          style={{ borderColor: BORDER, color: TEXT }}
        >
          <Send size={16} />
          Telegramda botni ochish
        </a>
      )}

      <div className="mt-5 flex gap-2">
        {code.map((digit, i) => (
          <input
            key={i}
            ref={(el) => {
              boxes.current[i] = el;
            }}
            value={digit}
            inputMode="numeric"
            autoComplete={i === 0 ? "one-time-code" : "off"}
            aria-label={`Kodning ${i + 1}-raqami`}
            maxLength={1}
            placeholder="–"
            onChange={(e) => {
              clearError();
              fill(e.target.value, i);
            }}
            onKeyDown={(e) => {
              if (e.key === "Backspace" && !code[i] && i > 0) {
                const nextCode = [...code];
                nextCode[i - 1] = "";
                setCode(nextCode);
                boxes.current[i - 1]?.focus();
              }
              if (e.key === "ArrowLeft" && i > 0) boxes.current[i - 1]?.focus();
              if (e.key === "ArrowRight" && i < CODE_LENGTH - 1) boxes.current[i + 1]?.focus();
            }}
            onPaste={(e) => {
              e.preventDefault();
              clearError();
              fill(e.clipboardData.getData("text"), 0);
            }}
            className="h-[54px] w-full min-w-0 rounded-[11px] border bg-white text-center text-[21px] font-bold outline-none transition-colors placeholder:font-normal placeholder:text-[18px]"
            style={{
              color: TEXT,
              borderColor: error ? "#F87171" : digit ? BRAND : BORDER,
              borderWidth: digit ? 1.6 : 1,
            }}
          />
        ))}
      </div>

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="mt-4">
        <AuthButton busy={busy} disabled={joined.length !== CODE_LENGTH || busy} onClick={() => void onVerify(joined)}>
          Tasdiqlash
        </AuthButton>
      </div>

      <button
        type="button"
        onClick={onRestart}
        className="mt-4 w-full text-center text-[13.5px] font-bold"
        style={{ color: BRAND }}
      >
        Kod kelmadimi? Qaytadan urinish
      </button>
    </>
  );
}

// ── 3-qadam: oxirgi tasdiq ───────────────────────────────────────────
function ConfirmStep({
  me,
  onConfirm,
  busy,
  error,
}: {
  me: Me | null;
  onConfirm: () => void;
  busy: boolean;
  error: string | null;
}) {
  const [typed, setTyped] = useState("");
  const ready = typed.trim().toUpperCase() === CONFIRM_WORD;

  return (
    <>
      {me?.name && (
        <div className="mb-4 rounded-xl border bg-white p-3.5" style={{ borderColor: BORDER }}>
          <p className="text-[12px]" style={{ color: MUTED }}>
            Tasdiqlangan akkaunt
          </p>
          <p className="mt-0.5 text-[15px] font-semibold" style={{ color: TEXT }}>
            {me.name}
          </p>
          {me.phone && (
            <p className="text-[13px]" style={{ color: MUTED }}>
              {me.phone}
            </p>
          )}
        </div>
      )}

      <div className="flex gap-2.5 rounded-xl bg-red-50 p-3.5">
        <TriangleAlert size={18} className="mt-0.5 shrink-0 text-red-600" />
        <div className="text-[13px] leading-relaxed text-red-700">
          <p className="font-semibold">Akkaunt o&apos;chirilgach:</p>
          <ul className="mt-1 list-disc pl-4">
            <li>ushbu raqam bilan ilovaga KIRA OLMAYSIZ;</li>
            <li>
              buyurtmalar tarixi, sevimlilar va manzil ma&apos;lumotlari{" "}
              <b>saqlanadi</b> — ular o&apos;chirilmaydi;
            </li>
            <li>
              O&apos;SHA telefon raqami bilan qaytadan tasdiqlansangiz (SMS/
              Telegram kod), akkaunt avtomatik tiklanadi va barcha ma&apos;lumot
              qaytadan ko&apos;rinadi.
            </li>
          </ul>
        </div>
      </div>

      <label htmlFor="confirm" className="mb-1.5 mt-5 block pl-0.5 text-[12.5px] font-semibold" style={{ color: TEXT }}>
        Tasdiqlash uchun <span className="font-mono">{CONFIRM_WORD}</span> deb yozing
      </label>
      <input
        id="confirm"
        value={typed}
        onChange={(e) => setTyped(e.target.value)}
        placeholder={CONFIRM_WORD}
        autoComplete="off"
        className="h-[46px] w-full rounded-[11px] border bg-white px-3.5 text-[14.5px] outline-none"
        style={{ borderColor: BORDER, color: TEXT }}
      />

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="mt-4">
        <AuthButton busy={busy} disabled={!ready || busy} onClick={onConfirm} danger>
          Ha, akkauntni o&apos;chirish
        </AuthButton>
      </div>
    </>
  );
}

// ── 4-qadam: tayyor ──────────────────────────────────────────────────
function DoneStep() {
  return (
    <div className="rounded-xl border bg-white p-5 text-center" style={{ borderColor: BORDER }}>
      <p className="text-[16px] font-bold" style={{ color: TEXT }}>
        Akkauntingiz o&apos;chirildi
      </p>
      <p className="mt-2 text-[13.5px] leading-relaxed" style={{ color: MUTED }}>
        Ma&apos;lumotlaringiz saqlanadi. O&apos;sha telefon raqami bilan qaytadan
        ro&apos;yxatdan o&apos;tsangiz, akkauntingiz avvalgi holida tiklanadi.
      </p>
    </div>
  );
}
