"use client";

import { clearStoredCartIfOtherRestaurant } from "@/lib/cart-context";
import { writeTableSession, type TableSession } from "@/lib/table-session";

// Stol QR kodi bo'yicha "shu stolning menyusini och" amali — YAGONA
// joyda.
//
// ┌─ NEGA BIR JOYDA ──────────────────────────────────────────────────┐
// Bu amalning UCHTA kirish nuqtasi bor:
//   1. Telegram Mini App kamerasi (`qr-scan-button.tsx`);
//   2. Telegram `start_param` — QR kod telefon kamerasi bilan
//      o'qilganda (`app/table-init.tsx`);
//   3. Mijoz ilovasining O'Z skaneri (Flutter) — u tokenni native
//      tomonda yechadi va tayyor seansni shu yerga beradi
//      (`app/app-table-bridge.tsx`).
//
// Uchalasida ham AYNI uch qadam bajarilishi shart: seansni yozish,
// savatni tekshirish va sahifani to'liq qayta yuklash. Qadamlardan
// biri biror yo'lda tushib qolsa, xato jimgina bo'lardi — masalan
// savat tozalanmasa, mijoz stolda o'tirib BOSHQA restoran taomlarini
// buyurtma qilib yuborardi.
// └───────────────────────────────────────────────────────────────────┘

/**
 * Skanerlangan matndan stol tokenini ajratadi.
 *
 * Stol QR kodi ichida to'liq havola turadi:
 *   https://t.me/<bot>/<short_name>?startapp=<token>
 * Lekin xom token yozilgan (qo'lda chop etilgan yoki eski) kod ham
 * uchrashi mumkin, shuning uchun ikkala shakl ham qabul qilinadi.
 *
 * `null` — bu bizning QR kodimiz emas.
 */
export function extractTableToken(raw: string): string | null {
  const s = raw.trim();
  if (!s) return null;

  try {
    const u = new URL(s);
    // `start` ham tekshiriladi: bot havolasining eski shakli
    // (`?start=<token>`) hali ham chop etilgan kodlarda uchrashi mumkin.
    const q = u.searchParams.get("startapp") ?? u.searchParams.get("start");
    if (q && q.trim()) return q.trim();
    // Havola, lekin token yo'q — begona sayt QR kodi.
    return null;
  } catch {
    // URL emas — pastda xom token sifatida tekshiriladi.
  }

  // Token — 32 baytlik tasodifiy sirning URL-xavfsiz ko'rinishi.
  // Qat'iy shablon ATAYLAB: har qanday matnni serverga yuborib
  // ko'rishning ma'nosi yo'q va u tokenlarni birma-bir sinashga
  // o'xshab qolardi.
  return /^[A-Za-z0-9_-]{16,128}$/.test(s) ? s : null;
}

/**
 * Tayyor (allaqachon yechilgan) stol seansini o'rnatadi va o'sha
 * restoran menyusini ochadi.
 *
 * ┌─ NEGA TO'LIQ QAYTA YUKLASH (`router.push` EMAS) ──────────────────┐
 * `CartProvider` `localStorage` ni FAQAT bir marta, o'zi ulanganda
 * o'qiydi. Yuqorida savat tozalangan bo'lsa ham, provider xotirasida
 * eski holat turadi va u keyingi o'zgarishda `localStorage` ga QAYTA
 * yozilardi — ya'ni tozalash bekor bo'lardi.
 * └───────────────────────────────────────────────────────────────────┘
 */
export function applyTableSession(session: TableSession): void {
  writeTableSession(session);
  // Savat BOSHQA restoranniki bo'lsa tozalanadi — QR skanerlash aniq
  // niyat: "men SHU restoranning SHU stolidaman".
  clearStoredCartIfOtherRestaurant(session.restaurantId);
  window.location.replace(`/restaurants/${session.restaurantId}`);
}

export type OpenTableResult =
  | { ok: true }
  | { ok: false; error: string };

/**
 * Tokenni serverda yechadi va menyuni ochadi.
 *
 * Xato matnlari FOYDALANUVCHIGA ko'rsatish uchun tayyor: server
 * javobining o'zi emas (u ichki tafsilot bo'lishi mumkin), balki
 * mijoz nima qilishini aytadigan matn.
 */
export async function openTableFromToken(
  token: string,
): Promise<OpenTableResult> {
  try {
    const res = await fetch(
      `/api/proxy/tables/resolve?token=${encodeURIComponent(token)}`,
    );
    if (!res.ok) {
      return {
        ok: false,
        error:
          "Bu QR kod yaroqsiz yoki muddati o'tgan. Ofitsiantga murojaat qiling.",
      };
    }
    const d = (await res.json()) as {
      table_label?: string;
      restaurant_id?: string;
      restaurant_name?: string;
    };
    if (!d.restaurant_id) return { ok: false, error: "Bu QR kod yaroqsiz." };

    applyTableSession({
      token,
      tableLabel: d.table_label ?? "",
      restaurantId: d.restaurant_id,
      restaurantName: d.restaurant_name,
    });
    return { ok: true };
  } catch {
    return {
      ok: false,
      error: "Serverga ulanib bo'lmadi. Internetni tekshiring.",
    };
  }
}
