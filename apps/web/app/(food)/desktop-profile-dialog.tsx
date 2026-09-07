"use client";

import { LogOut, ShieldCheck, X } from "lucide-react";
import { useEffect, useState } from "react";

// "Mening ma'lumotlarim" — kompyuter uchun modal oyna
// (namuna: image/profiles.png).
//
// ┌─ NEGA MODAL, SAHIFA EMAS ──────────────────────────────────────────┐
// Xarita oynasi bilan bir xil sabab: profilga qarash — qisqa, oraliq
// amal. Sahifa almashtirilsa mijoz yig'ayotgan savat/menyu konteksti
// yo'qoladi. `/profile` sahifasi mobil ko'rinish uchun o'z joyida
// qoladi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ NIMA TAHRIRLANADI VA NEGA ─────────────────────────────────────────┐
// FAQAT ism va familiya — Go tomondagi `POST /me` shundan boshqasini
// qabul qilmaydi (`internal/httpapi/routes_me.go`).
//
// TELEFON o'zgartirilmaydi: u kimlikning O'ZI va Telegram tomonidan
// tasdiqlangan. Uni sahifadan almashtirish mumkin bo'lsa, butun
// tasdiqlash oqimi ma'nosiz bo'lardi.
//
// EMAIL ham shu yerda tahrirlanmaydi: uning o'z tasdiqlash oqimi bor
// (`/auth/email/request-code`), tasdiqsiz o'zgartirish esa "boshqa
// odamning emaili"ni yozib qo'yish imkonini berardi.
//
// Namunadagi "Reklama va aksiyalar" bloki ATAYLAB olinmadi —
// so'ralmagan va unga mos backend sozlamasi ham yo'q.
// └────────────────────────────────────────────────────────────────────┘

type Me = {
  id?: string;
  phone?: string;
  name?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
};

export default function DesktopProfileDialog({
  onClose,
  onSaved,
}: {
  onClose: () => void;
  onSaved?: (me: Me) => void;
}) {
  const [me, setMe] = useState<Me | null>(null);
  const [failed, setFailed] = useState(false);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [saving, setSaving] = useState(false);
  const [savedOk, setSavedOk] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loggingOut, setLoggingOut] = useState(false);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [onClose]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/me");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        const data = (await res.json()) as Me;
        setMe(data);
        setFirstName(data.first_name ?? "");
        setLastName(data.last_name ?? "");
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  const changed =
    me !== null &&
    (firstName.trim() !== (me.first_name ?? "").trim() ||
      lastName.trim() !== (me.last_name ?? "").trim());

  async function save() {
    if (!changed || saving) return;
    setSaving(true);
    setError(null);
    setSavedOk(false);
    try {
      const res = await fetch("/api/proxy/me", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          first_name: firstName.trim(),
          last_name: lastName.trim(),
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        setError(data?.error ?? "Saqlab bo'lmadi.");
        return;
      }
      setMe(data as Me);
      setSavedOk(true);
      onSaved?.(data as Me);
    } catch {
      setError("Tarmoq xatosi. Internet aloqasini tekshiring.");
    } finally {
      setSaving(false);
    }
  }

  async function logout() {
    setLoggingOut(true);
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      // Cookie baribir server tomonda tozalanadi.
    }
    window.location.replace("/");
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Mening ma'lumotlarim"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="flex max-h-[88vh] w-full max-w-[440px] flex-col overflow-hidden rounded-3xl border border-white/10 bg-[#1e1e1e] text-white shadow-2xl">
        <div className="flex items-center justify-between gap-4 px-6 pb-4 pt-5">
          <h2 className="text-[22px] font-extrabold">Mening ma&apos;lumotlarim</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Yopish"
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white/60 transition-colors hover:bg-white/10 hover:text-white"
          >
            <X size={20} />
          </button>
        </div>

        <div className="ondex-scroll min-h-0 flex-1 overflow-y-auto px-6 pb-6">
          {failed ? (
            <p className="py-10 text-center text-[14px] text-white/45">
              Ma&apos;lumotlarni yuklab bo&apos;lmadi.
            </p>
          ) : me === null ? (
            <div className="space-y-3">
              {[0, 1, 2].map((i) => (
                <div key={i} className="h-[68px] animate-pulse rounded-2xl bg-white/5" />
              ))}
            </div>
          ) : (
            <>
              <div className="space-y-3">
                <Field label="Ism" value={firstName} onChange={setFirstName} />
                <Field label="Familiya" value={lastName} onChange={setLastName} />

                {/* Telefon — kimlikning o'zi, tahrirlanmaydi. */}
                <Field label="Telefon" value={me.phone ?? ""} readOnly />
                <p className="flex items-center gap-1.5 px-1 text-[12px] text-white/35">
                  <ShieldCheck size={13} />
                  Raqam Telegram orqali tasdiqlangan — o&apos;zgartirib
                  bo&apos;lmaydi
                </p>

                {me.email && (
                  <Field label="Email" value={me.email} readOnly />
                )}
              </div>

              {error && (
                <p className="mt-4 rounded-xl bg-red-500/10 px-3.5 py-2.5 text-[13px] font-medium text-red-400">
                  {error}
                </p>
              )}
              {savedOk && !changed && (
                <p className="mt-4 rounded-xl bg-green-500/10 px-3.5 py-2.5 text-[13px] font-medium text-green-400">
                  Saqlandi
                </p>
              )}

              <button
                type="button"
                onClick={save}
                disabled={!changed || saving}
                className="mt-5 h-12 w-full rounded-2xl bg-brand text-[15px] font-bold text-white transition-colors hover:bg-brand-light disabled:cursor-not-allowed disabled:bg-white/10 disabled:text-white/40"
              >
                {saving ? "Saqlanmoqda…" : "O'zgarishlarni saqlash"}
              </button>

              <button
                type="button"
                onClick={logout}
                disabled={loggingOut}
                className="mt-3 flex h-12 w-full items-center justify-center gap-2 rounded-2xl border border-white/10 text-[15px] font-semibold text-white/80 transition-colors hover:bg-white/5 disabled:opacity-50"
              >
                <LogOut size={17} />
                {loggingOut ? "Chiqilmoqda…" : "Chiqish"}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

/** Namunadagi kabi: yorliq maydon ICHIDA, ustki qatorda. */
function Field({
  label,
  value,
  onChange,
  readOnly = false,
}: {
  label: string;
  value: string;
  onChange?: (v: string) => void;
  readOnly?: boolean;
}) {
  return (
    <label
      className={`block rounded-2xl px-4 py-2.5 transition-colors ${
        readOnly ? "bg-white/[0.04]" : "bg-white/[0.07] focus-within:bg-white/10"
      }`}
    >
      <span className="block text-[12px] text-white/40">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        readOnly={readOnly}
        maxLength={64}
        className={`w-full bg-transparent text-[16px] font-medium outline-none ${
          readOnly ? "text-white/55" : "text-white"
        }`}
      />
    </label>
  );
}
