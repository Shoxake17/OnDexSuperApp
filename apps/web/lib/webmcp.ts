"use client";

/**
 * WebMCP — sahifaning agentga ochadigan amallari.
 *
 * ┌─ WEBMCP NIMA ──────────────────────────────────────────────────────┐
 * Brauzer agenti (ChatGPT ichki brauzeri yoki Chrome 149+) sahifadan
 * "sen nima qila olasan?" deb so'raydi. Sahifa `document.modelContext`
 * orqali amallarini e'lon qiladi va agent ularni chaqiradi.
 *
 * MUHIM: bu yerda til modeli YO'Q. Miya — foydalanuvchining agenti,
 * biz faqat amallarni beramiz. Shuning uchun sahifada chat oynasi ham,
 * ovoz ham qurilmaydi.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ XAVFSIZLIK: AGENT YANGI HUQUQ OLMAYDI ────────────────────────────┐
 * Har bir amal BFF proksi orqali ketadi (`/api/proxy/...`), ya'ni:
 *   * sessiya tokeni brauzerga umuman chiqmaydi (httpOnly cookie);
 *   * ruxsat etilgan yo'llar ro'yxati serverda
 *     (`app/api/proxy/[...path]/route.ts`);
 *   * narx, egalik va holat tekshiruvlari Go tomonda o'zgarishsiz.
 *
 * Ya'ni agentga ochilgan yuza — foydalanuvchining O'ZI sahifadan qila
 * oladigan ishlar bilan bir xil, undan katta emas.
 * └────────────────────────────────────────────────────────────────────┘
 */

// ── Standart tiplari (Chrome origin trial; hali `lib.dom` da yo'q) ──

type ToolResult = string;

type ToolDefinition = {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  execute: (
    input: Record<string, never> | Record<string, unknown>,
    options: { signal: AbortSignal },
  ) => Promise<ToolResult> | ToolResult;
};

type ModelContext = {
  registerTool: (
    tool: ToolDefinition,
    options?: { signal?: AbortSignal },
  ) => Promise<unknown>;
};

/** Brauzer WebMCP'ni qo'llaydimi. */
export function webmcpAvailable(): boolean {
  return (
    typeof document !== "undefined" &&
    "modelContext" in document &&
    typeof (document as unknown as { modelContext?: ModelContext })
      .modelContext?.registerTool === "function"
  );
}

function modelContext(): ModelContext | null {
  if (!webmcpAvailable()) return null;
  return (document as unknown as { modelContext: ModelContext }).modelContext;
}

/**
 * Amallarni ro'yxatdan o'tkazadi va tozalash funksiyasini qaytaradi.
 *
 * `AbortController` — hujjatdagi rasmiy usul: `abort()` chaqirilganda
 * amallar olib tashlanadi. React'ning `useEffect` tozalashiga aynan
 * mos tushadi.
 */
export function registerTools(tools: ToolDefinition[]): () => void {
  const ctx = modelContext();
  if (!ctx) return () => {};

  const controller = new AbortController();
  for (const tool of tools) {
    // `await` qilinmaydi: ro'yxatga olish sahifani to'smasligi kerak.
    // Xato bo'lsa (masalan nom takrorlansa) — konsolga, sahifa
    // ishlayveradi.
    void Promise.resolve(
      ctx.registerTool(tool, { signal: controller.signal }),
    ).catch((e) => {
      console.warn(`[webmcp] "${tool.name}" ro'yxatga olinmadi:`, e);
    });
  }
  return () => controller.abort();
}

// ── BFF chaqiruvlari ────────────────────────────────────────────────

/**
 * Proksi orqali so'rov.
 *
 * Xato holatida ISTISNO TASHLAMAYDI: agentga o'qiladigan matn
 * qaytariladi. Istisno tashlansa agent "amal ishlamadi" deb tushunmay,
 * shunchaki to'xtab qolardi.
 */
async function api<T>(
  path: string,
  init?: RequestInit,
): Promise<{ ok: true; data: T } | { ok: false; error: string }> {
  try {
    const res = await fetch(`/api/proxy/${path}`, {
      ...init,
      headers: init?.body
        ? { "Content-Type": "application/json", ...(init?.headers ?? {}) }
        : init?.headers,
    });
    const text = await res.text();
    if (!res.ok) {
      let msg = `so'rov ${res.status} bilan tugadi`;
      try {
        const j = JSON.parse(text) as { error?: string };
        if (j.error) msg = j.error;
      } catch {
        // JSON emas — standart matn qoladi
      }
      if (res.status === 401) {
        msg = "tizimga kirilmagan — foydalanuvchi avval kirishi kerak";
      }
      return { ok: false, error: msg };
    }
    return { ok: true, data: (text ? JSON.parse(text) : null) as T };
  } catch (e) {
    return { ok: false, error: `tarmoq xatosi: ${(e as Error).message}` };
  }
}

export const proxy = api;

// ── Matn yordamchilari ──────────────────────────────────────────────

/** Tiyinni o'qiladigan summaga aylantiradi. */
export function sum(tiyin: number): string {
  return `${Math.round(tiyin / 100).toLocaleString("uz-UZ")} so'm`;
}

/**
 * Agentga qaytariladigan matn.
 *
 * ┌─ NEGA MATN, JSON EMAS ─────────────────────────────────────────────┐
 * Spetsifikatsiya `execute` dan satr kutadi. Xom JSON qaytarilsa agent
 * uni foydalanuvchiga o'qib berishga urinadi va natija tushunarsiz
 * bo'ladi. Shuning uchun har bir amal QISQA, o'qiladigan javob beradi
 * va ID larni ataylab ko'rsatadi — keyingi amal uchun ular kerak.
 * └────────────────────────────────────────────────────────────────────┘
 */
export function lines(...parts: (string | false | null | undefined)[]): string {
  return parts.filter(Boolean).join("\n");
}
