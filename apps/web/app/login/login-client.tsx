"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Bike, MapPin, Send, ShieldCheck, Smartphone } from "lucide-react";
import { getTelegramWebApp } from "@/lib/telegram";

// ─────────────────────────────────────────────────────────────────────
// Kirish sahifasi.
//
// OQIM (mobil ilova bilan aynan bir xil):
//   telefon raqam -> bot raqamni `request_contact` orqali TASDIQLAYDI
//   -> bot 6 xonali kodni Telegram chatiga yuboradi -> kod kiritiladi
//   -> sessiya cookie'si (`/api/auth/verify`).
//
// KO'RINISH:
//   * telefon/planshet (< lg) — mijoz ilovasidagi (Flutter) auth
//     ekranlari bilan BIR XIL: iliq oq fon, orqaga tugmasi,
//     "OnDex / Super App" sarlavhasi, hero rasm, bayroqli telefon
//     maydoni, 6 katakli kod. Qiymatlar `apps/customer_app/lib/widgets/
//     auth_ui.dart` dan bir-bir olingan (rang, radius, balandlik) —
//     mijoz ikkala ilovada bir xil ekranni ko'radi;
//   * kompyuter (lg+) — chapda brend paneli, o'ngda o'sha forma.
//
// Ilovadagi "Google / Apple / Telegram" qatori va "Ro'yxatdan o'tish"
// havolasi bu yerda ATAYLAB YO'Q: web'da OAuth ulanmagan (bosilganda
// hech narsa qilmaydigan tugma bo'lardi), ro'yxatdan o'tish esa
// alohida qadam emas — kod tasdiqlansa akkaunt o'zi yaratiladi.
// ─────────────────────────────────────────────────────────────────────

// Ranglar — `auth_ui.dart` dagi qiymatlar (bitta manba, ikki platforma).
const BRAND = "#F64E03";
const BG = "#FDFBFA";
const BORDER = "#EDE5DF";
const TEXT = "#1A1A1A";
const MUTED = "#7C7671";
const HINT = "#A8A29D";

const CODE_LENGTH = 6; // users.randomCode — "%06d"
const PHONE_DIGITS = 9; // +998 dan keyingi qism

type Step = "phone" | "code";

export default function LoginClient({
  next,
  initialError = null,
}: {
  next: string;
  initialError?: string | null;
}) {
  const router = useRouter();

  const [step, setStep] = useState<Step>("phone");
  const [digits, setDigits] = useState("");
  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(initialError);

  // Telegram Mini App ichida bu sahifa kerak emas — u yerda kirish
  // `initData` orqali avtomatik (`telegram-auth.tsx`).
  useEffect(() => {
    if (getTelegramWebApp()) router.replace("/");
  }, [router]);

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
      // Bot DARHOL ochiladi — ortiqcha bosish bo'lmasin. Brauzer yangi
      // oynani bloklasa ham keyingi qadamda havola tugmasi qoladi.
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
        setBusy(false);
        return false;
      }
      // TO'LIQ SAHIFA YUKLASH ATAYLAB: server komponentlari (root
      // layout'dagi `signedIn`, `requireAuth`) cookie'ni faqat yangi
      // so'rovda ko'radi.
      window.location.replace(next);
      return true;
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
      setBusy(false);
      return false;
    }
  }

  function goBack() {
    if (step === "code") {
      setStep("phone");
      setError(null);
      setDeepLink(null);
      return;
    }
    router.back();
  }

  return (
    <div className="flex min-h-dvh" style={{ background: BG }}>
      <BrandPanel />

      <main className="flex flex-1 flex-col px-5 pb-8 pt-5 sm:px-8 lg:items-center lg:justify-center lg:px-10">
        <div className="w-full lg:max-w-[400px]">
          {/* ── Sarlavha (ilovadagi `AuthHeader` bilan bir xil) ────── */}
          <button
            type="button"
            onClick={goBack}
            aria-label="Orqaga"
            className="flex h-[38px] w-[38px] items-center justify-center rounded-[11px] bg-white"
            style={{ color: TEXT }}
          >
            <ArrowLeft size={20} />
          </button>

          <div className="mt-2.5 flex items-center gap-1.5">
            <div className="min-w-0 flex-1">
              <p
                className="text-[30px] font-extrabold leading-none tracking-[-0.8px]"
                style={{ color: TEXT }}
              >
                On<span style={{ color: BRAND }}>Dex</span>
              </p>
              <p className="text-xs font-medium" style={{ color: MUTED }}>
                Super App
              </p>

              <h1
                className="mt-2.5 truncate text-[21px] font-extrabold"
                style={{ color: TEXT }}
              >
                {step === "phone" ? "Xush kelibsiz!" : "Tasdiqlash kodi"}
              </h1>
              <p className="mt-0.5 text-[12.5px] leading-snug" style={{ color: MUTED }}>
                {step === "phone"
                  ? "OnDex super appga kiring"
                  : "Telegram botdan kelgan 6 xonali kodni kiriting"}
              </p>
            </div>

            {/* Ilovadagi bilan bir xil hero rasm (assets/ondex_hero.png
                dan nusxa). Yuklanmasa joyi shunchaki bo'sh qoladi. */}
            {/* eslint-disable-next-line @next/next/no-img-element -- statik
                fayl, o'lchami qat'iy; `next/image` bu yerda hech narsa
                qo'shmaydi */}
            <img
              src="/ondex_hero.png"
              alt=""
              aria-hidden="true"
              className="w-[126px] shrink-0 object-contain lg:hidden"
            />
          </div>

          <div className="mt-7">
            {step === "phone" ? (
              <PhoneStep
                digits={digits}
                onDigits={setDigits}
                onSubmit={startTelegram}
                busy={busy}
                error={error}
                valid={phoneValid}
              />
            ) : (
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
          </div>
        </div>
      </main>
    </div>
  );
}

// ── Chapdagi brend paneli — FAQAT kompyuterda ────────────────────────
//
// Telefonda ko'rsatilmaydi: u yerda ekran ilovadagidek to'liq formaga
// tegishli bo'lishi kerak (mijoz ikkala ilovada bir xil ekranni
// ko'radi). Kompyuterda esa yolg'iz forma bo'sh oq ekranda "yarim
// tayyor" ko'rinardi.
function BrandPanel() {
  return (
    <aside className="relative hidden w-[46%] max-w-[560px] shrink-0 overflow-hidden bg-[#141414] lg:block">
      <div className="absolute -left-24 -top-24 h-[420px] w-[420px] rounded-full bg-brand/25 blur-[120px]" />
      <div className="absolute -bottom-32 -right-16 h-[380px] w-[380px] rounded-full bg-brand/15 blur-[120px]" />

      <div className="relative flex h-full flex-col justify-between p-12">
        <p className="text-3xl font-extrabold tracking-tight text-white">
          On<span className="text-brand">Dex</span>
        </p>

        <div>
          <h2 className="max-w-[320px] text-[32px] font-bold leading-[1.15] text-white">
            Chust bo&apos;ylab
            <br />
            taom yetkazib berish
          </h2>
          <ul className="mt-9 flex flex-col gap-5">
            <Feature Icon={Bike} title="Tez yetkazib berish">
              Buyurtmani kuryer olgach, yo&apos;lini xaritada real vaqtda
              kuzatasiz.
            </Feature>
            <Feature Icon={MapPin} title="Shahar restoranlari">
              Chustdagi kafe va restoranlar menyusi bitta joyda.
            </Feature>
            <Feature Icon={ShieldCheck} title="Xavfsiz kirish">
              Parol yo&apos;q — raqamingizni Telegram tasdiqlaydi.
            </Feature>
          </ul>
        </div>

        <p className="text-xs text-white/35">© {new Date().getFullYear()} OnDex</p>
      </div>
    </aside>
  );
}

function Feature({
  Icon,
  title,
  children,
}: {
  Icon: typeof Bike;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <li className="flex gap-3.5">
      <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white/10 text-brand">
        <Icon size={18} />
      </span>
      <div className="min-w-0">
        <p className="text-sm font-semibold text-white">{title}</p>
        <p className="mt-0.5 text-[13px] leading-relaxed text-white/45">{children}</p>
      </div>
    </li>
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
      {/* Ilovadagi `AuthTabs` o'rnida — bitta, faol usul. Ikkinchi tab
          ("Email orqali") web'da ulanmagan, ya'ni ko'rsatilsa bosilmas
          tugma bo'lardi. */}
      <div
        className="mb-4 flex h-[50px] items-center gap-2 rounded-xl border bg-white px-3.5"
        style={{ borderColor: BORDER }}
      >
        <Smartphone size={16} style={{ color: BRAND }} />
        <span className="text-[12.5px] font-semibold" style={{ color: BRAND }}>
          Telefon raqami orqali
        </span>
      </div>

      <label
        htmlFor="phone"
        className="mb-1.5 block pl-0.5 text-[12.5px] font-semibold"
        style={{ color: TEXT }}
      >
        Telefon raqam
      </label>

      {/* Bayroq + `+998` + ajratuvchi chiziq + raqam — `AuthPhoneField`
          bilan bir xil tuzilish va o'lchamlar. */}
      <div
        className="flex h-[46px] items-center rounded-[11px] border bg-white"
        style={{ borderColor: BORDER }}
      >
        <div className="flex items-center gap-1.5 px-2.5">
          {/* eslint-disable-next-line @next/next/no-img-element -- kichik statik bayroq */}
          <img src="/uz_flag.png" alt="" aria-hidden="true" className="h-[15px] w-[22px] object-cover" />
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
          onChange={(e) =>
            onDigits(e.target.value.replace(/\D/g, "").slice(0, PHONE_DIGITS))
          }
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

// ── 2-qadam: Telegramdan kelgan kod ──────────────────────────────────
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

  // Xatodan keyin kataklar tozalanadi va fokus boshiga qaytadi.
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
        // Zaxira havola: brauzer avtomatik ochilgan oynani bloklagan
        // bo'lishi mumkin — o'shanda bu yagona yo'l.
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

      {/* 6 ta katak — `auth_ui.dart`dagi `OtpInput` bilan bir xil:
          balandlik 54, radius 11, to'lgan/fokusdagi katak brend
          ramkasi bilan. */}
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
              // Bo'sh katakda Backspace — oldingisiga qaytib tozalaydi.
              if (e.key === "Backspace" && !code[i] && i > 0) {
                const nextCode = [...code];
                nextCode[i - 1] = "";
                setCode(nextCode);
                boxes.current[i - 1]?.focus();
              }
              if (e.key === "ArrowLeft" && i > 0) boxes.current[i - 1]?.focus();
              if (e.key === "ArrowRight" && i < CODE_LENGTH - 1) {
                boxes.current[i + 1]?.focus();
              }
            }}
            onPaste={(e) => {
              // Kodni Telegramdan nusxalash — eng ko'p uchraydigan yo'l.
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
        <AuthButton
          busy={busy}
          disabled={joined.length !== CODE_LENGTH || busy}
          onClick={() => void onVerify(joined)}
        >
          Tasdiqlash
        </AuthButton>
      </div>

      {/* "Qayta yuborish" ATAYLAB yo'q: kodni bot raqamni tasdiqlagandan
          KEYIN yuboradi — tugma bosilsa ham hech narsa o'zgarmasdi.
          To'g'ri amal — oqimni boshidan boshlash. */}
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

// ── Umumiy qismlar (ilovadagi `AuthButton` o'lchamlarida) ────────────

function AuthButton({
  busy,
  disabled,
  onClick,
  children,
}: {
  busy: boolean;
  disabled: boolean;
  onClick?: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type={onClick ? "button" : "submit"}
      onClick={onClick}
      disabled={disabled}
      className="flex h-[50px] w-full items-center justify-center gap-2.5 rounded-xl text-[15.5px] font-bold text-white transition-opacity active:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
      style={{ background: BRAND }}
    >
      {busy && (
        <span className="h-[18px] w-[18px] animate-spin rounded-full border-2 border-white/40 border-t-white" />
      )}
      {children}
    </button>
  );
}

function ErrorNote({ children }: { children: React.ReactNode }) {
  return (
    <p
      role="alert"
      className="mt-3 rounded-xl bg-red-50 px-3.5 py-2.5 text-[13px] font-medium text-red-600"
    >
      {children}
    </p>
  );
}

/** "901234567" -> "90 123 45 67" (o'qish uchun guruhlash). */
function groupPhone(digits: string): string {
  return [
    digits.slice(0, 2),
    digits.slice(2, 5),
    digits.slice(5, 7),
    digits.slice(7, 9),
  ]
    .filter(Boolean)
    .join(" ");
}
