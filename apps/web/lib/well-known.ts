// ondex.uz/.well-known/* — passkey uchun ilova<->domen bog'lanishi (PK-1).
//
// Fayllarni Go API sozlamadan hosil qiladi (PASSKEY_ANDROID_APPS,
// PASSKEY_APPLE_APP_IDS) — imzo barmoq izi bitta joyda turadi. Bu yerda
// faqat UZATILADI: javob JSON ekanligi va hajmi tekshiriladi, boshqa
// narsa (HTML, xato sahifasi) Android/iOS'ga berilmaydi.
const GO_API_URL = process.env.GO_API_URL ?? "http://localhost:8080";
const MAX_BYTES = 64 * 1024;

export async function proxyWellKnown(path: "/.well-known/assetlinks.json" | "/.well-known/apple-app-site-association") {
  let res: Response;
  try {
    res = await fetch(GO_API_URL + path, {
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(3000),
    });
  } catch {
    return new Response("unavailable", { status: 503, headers: { "Cache-Control": "no-store" } });
  }
  if (res.status === 404) {
    return new Response("not found", { status: 404, headers: { "Cache-Control": "public, max-age=300" } });
  }
  const body = await res.text();
  let valid = res.ok && body.length <= MAX_BYTES;
  if (valid) {
    try {
      JSON.parse(body);
    } catch {
      valid = false;
    }
  }
  if (!valid) {
    return new Response("unavailable", { status: 503, headers: { "Cache-Control": "no-store" } });
  }
  return new Response(body, {
    status: 200,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "public, max-age=3600",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
