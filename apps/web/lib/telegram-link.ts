// isTelegramLink — serverdan kelgan Telegram bot havolasi haqiqatan
// Telegram'ga olib boradimi (bug.md 8, audit 480).
//
// Havola foydalanuvchi bosmasdan `window.open` bilan ochiladi. Server
// javobi buzilsa yoki o'zgartirilsa `javascript:`, `data:` yoki fishing
// sahifasi ochilishi mumkin edi. Faqat https://t.me va
// https://telegram.me qabul qilinadi.
export function isTelegramLink(raw: unknown): raw is string {
  if (typeof raw !== "string") return false;
  try {
    const u = new URL(raw);
    return u.protocol === "https:" && (u.hostname === "t.me" || u.hostname === "telegram.me");
  } catch {
    return false;
  }
}
