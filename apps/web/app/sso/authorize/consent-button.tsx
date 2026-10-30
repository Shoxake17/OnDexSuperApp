"use client";

import { useState } from "react";
import { AUTH_BRAND, ErrorNote } from "@/lib/auth-widgets";

// "Ruxsat berish": so'rovni fetch bilan yuboradi va qaytish manziliga
// brauzerni O'ZI olib boradi. Oddiy `<form method="POST">` + server
// yo'naltirishi ishlamaydi: sahifaning CSP'si `form-action 'self'` bo'lgani
// uchun Chrome boshqa domenga (POS paneli) yo'naltirishni to'sadi
// (`app/api/sso/authorize/route.ts` izohiga qarang).
export default function ConsentButton({
  clientId,
  redirectUri,
  state,
}: {
  clientId: string;
  redirectUri: string;
  state: string;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);

  async function allow() {
    if (busy) return;
    setBusy(true);
    setError(false);
    try {
      const res = await fetch("/api/sso/authorize", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ client_id: clientId, redirect_uri: redirectUri, state }),
      });
      if (res.status === 401) {
        // Sessiya tugagan: sahifa qayta yuklanadi va kirish sahifasiga o'tkazadi.
        window.location.reload();
        return;
      }
      const data = await res.json().catch(() => ({}));
      if (!res.ok || typeof data?.redirect_to !== "string") {
        setError(true);
        setBusy(false);
        return;
      }
      window.location.assign(data.redirect_to);
    } catch {
      setError(true);
      setBusy(false);
    }
  }

  return (
    <>
      {error && <ErrorNote>Ruxsat berib bo&apos;lmadi. Qaytadan urinib ko&apos;ring.</ErrorNote>}
      <button
        type="button"
        onClick={allow}
        disabled={busy}
        className="mt-5 flex h-[50px] w-full items-center justify-center rounded-xl text-[15.5px] font-bold text-white active:opacity-90 disabled:opacity-70"
        style={{ background: AUTH_BRAND }}
      >
        {busy ? "Kutilmoqda..." : "Ruxsat berish"}
      </button>
    </>
  );
}
