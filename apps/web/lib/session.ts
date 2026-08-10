import { cookies } from "next/headers";

// Go backend chiqargan JWT shu httpOnly cookie'da saqlanadi — brauzer JS
// unga umuman kira olmaydi (XSS orqali o'g'irlab bo'lmaydi). Bitta domen
// darajasida (path cheklanmagan) — kelajakdagi har qanday mini-app shu
// domen ostida bo'lsa, avtomatik bir xil sessiyadan foydalanadi.
const COOKIE_NAME = "chust_session";
// Go tomonida token muddati 30 kun (users.NewTokenIssuer, cmd/api/main.go) —
// cookie shu bilan mos keladi.
const COOKIE_MAX_AGE = 60 * 60 * 24 * 30;

export async function setSessionToken(token: string) {
  const store = await cookies();
  store.set(COOKIE_NAME, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: COOKIE_MAX_AGE,
  });
}

export async function getSessionToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(COOKIE_NAME)?.value ?? null;
}

export async function clearSessionToken() {
  const store = await cookies();
  store.delete(COOKIE_NAME);
}
