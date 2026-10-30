import Link from "next/link";
import { goFetch } from "@/lib/api";
import {
  AUTH_BG,
  AUTH_BORDER,
  AUTH_BRAND,
  AUTH_MUTED,
  AUTH_TEXT,
} from "@/lib/auth-widgets";
import { requireAuth } from "@/lib/require-auth";
import { getSessionToken } from "@/lib/session";
import ConsentButton from "./consent-button";

export const metadata = { title: "Ruxsat berish" };

// ┌─ YAGONA KIRISH: TASDIQLASH SAHIFASI ─────────────────────────────────┐
// Boshqa xizmat (masalan OnDex Merchant Panel) foydalanuvchini shu yerga
// yuboradi: `/sso/authorize?client_id=...&redirect_uri=...&state=...`.
//
// Sahifa foydalanuvchiga QAYSI ilova, NIMA ma'lumot so'rayotganini va
// qaysi SAYTGA qaytarilishini ko'rsatadi. Bu fishingga qarshi asosiy
// himoya: soxta sahifa orqali kelgan odam qaytish domenini ko'radi.
//
// `client_id` va `redirect_uri` juftligining to'g'riligini Go tekshiradi
// (`GET /sso/client`: manzil ro'yxatdagiga ANIQ teng bo'lishi shart).
// Bu yerda hech narsa o'zgartirilmaydi — kod faqat tugma bosilgach
// beriladi (`/api/sso/authorize`).
// └──────────────────────────────────────────────────────────────────────┘

type Params = Record<string, string | string[] | undefined>;

function one(v: string | string[] | undefined): string {
  return (Array.isArray(v) ? v[0] : v) ?? "";
}

// "+998901234567" -> "+998 90 *** ** 67": noto'g'ri akkauntdan
// ruxsat berib qo'ymaslik uchun kim sifatida kirgani ko'rsatiladi,
// lekin raqam to'liq oshkor qilinmaydi.
function maskPhone(phone: string): string {
  const d = phone.replace(/\D/g, "");
  if (d.length < 9) return "";
  return `+${d.slice(0, 3)} ${d.slice(3, 5)} *** ** ${d.slice(-2)}`;
}

type AppInfo = { name: string; redirect_host: string };

export default async function SSOAuthorizePage({
  searchParams,
}: {
  searchParams: Promise<Params>;
}) {
  const sp = await searchParams;
  // Kirmagan bo'lsa `/login?next=<shu sahifa, to'liq so'rov bilan>`.
  await requireAuth("/sso/authorize", sp);

  const clientId = one(sp.client_id);
  const redirectUri = one(sp.redirect_uri);
  const state = one(sp.state);

  let app: AppInfo | null = null;
  let unavailable = false;
  let phone = "";
  try {
    const res = await goFetch(
      `/sso/client?${new URLSearchParams({ client_id: clientId, redirect_uri: redirectUri })}`,
    );
    if (res.ok) {
      app = (await res.json()) as AppInfo;
    } else {
      unavailable = res.status === 503;
    }
    if (app) {
      const token = await getSessionToken();
      const me = token ? await goFetch("/me", {}, token) : null;
      if (me?.ok) {
        const u = await me.json().catch(() => null);
        if (typeof u?.phone === "string") phone = maskPhone(u.phone);
      }
    }
  } catch {
    unavailable = true;
  }

  return (
    <main
      className="flex min-h-dvh items-center justify-center px-5 py-10"
      style={{ background: AUTH_BG }}
    >
      <div
        className="w-full max-w-[420px] rounded-2xl border bg-white p-6 shadow-sm"
        style={{ borderColor: AUTH_BORDER }}
      >
        <p
          className="text-[26px] font-extrabold leading-none tracking-[-0.6px]"
          style={{ color: AUTH_TEXT }}
        >
          On<span style={{ color: AUTH_BRAND }}>Dex</span>
        </p>

        {app ? (
          <>
            <h1 className="mt-5 text-[20px] font-extrabold leading-snug" style={{ color: AUTH_TEXT }}>
              {app.name} OnDex akkauntingiz orqali kirishni so&apos;ramoqda
            </h1>

            <p className="mt-3 text-[14px] leading-relaxed" style={{ color: AUTH_MUTED }}>
              Unga faqat <b style={{ color: AUTH_TEXT }}>ismingiz va telefon raqamingiz</b>{" "}
              beriladi. Buyurtmalaringiz, wallet va to&apos;lov ma&apos;lumotlaringiz
              berilmaydi.
            </p>

            {phone && (
              <p className="mt-3 text-[13px]" style={{ color: AUTH_MUTED }}>
                Siz <b style={{ color: AUTH_TEXT }}>{phone}</b> sifatida kirgansiz.
              </p>
            )}

            <p
              className="mt-3 rounded-xl px-3.5 py-2.5 text-[13px]"
              style={{ background: "#FDF1E7", color: AUTH_TEXT }}
            >
              Ruxsat bergach sizni <b>{app.redirect_host}</b> saytiga qaytaramiz. Bu
              sayt sizga tanishmi?
            </p>

            <ConsentButton clientId={clientId} redirectUri={redirectUri} state={state} />

            <Link
              href="/"
              className="mt-3 block text-center text-[14px] font-semibold"
              style={{ color: AUTH_MUTED }}
            >
              Bekor qilish
            </Link>
          </>
        ) : (
          <>
            <h1 className="mt-5 text-[20px] font-extrabold" style={{ color: AUTH_TEXT }}>
              {unavailable ? "Yagona kirish hozir ishlamayapti" : "Havola yaroqsiz"}
            </h1>
            <p className="mt-3 text-[14px] leading-relaxed" style={{ color: AUTH_MUTED }}>
              {unavailable
                ? "Birozdan keyin qaytadan urinib ko'ring."
                : "Bu havola noto'g'ri yoki eskirgan. Kirishni ilovaning o'zidan qaytadan boshlang."}
            </p>
            <Link
              href="/"
              className="mt-5 block text-center text-[14px] font-semibold"
              style={{ color: AUTH_BRAND }}
            >
              OnDex bosh sahifasi
            </Link>
          </>
        )}
      </div>
    </main>
  );
}
