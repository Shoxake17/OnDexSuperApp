"use client";

// Telefon+kod bilan tasdiqlash ekranlarining UMUMIY bo'laklari —
// `app/login/login-client.tsx` va `app/delete-account/delete-account-client.tsx`
// da AYNAN bir xil qiymatlar (rang, o'lcham, radius) ishlatiladi.
//
// NEGA AJRATILDI: ilgari bu atomlar login-client.tsx ichida edi.
// "Akkauntni o'chirish" sahifasi ham xuddi shu SMS/Telegram-kod
// oqimidan foydalanadi (bir xil `/api/auth/telegram-start` va
// `/api/auth/verify`) — vizual qismni ikkinchi marta yozish o'rniga
// bitta manbadan import qilinadi.

// Ranglar — `apps/customer_app/lib/widgets/auth_ui.dart` dagi qiymatlar
// (bitta manba, uchta platforma: mobil, veb login, veb o'chirish).
export const AUTH_BRAND = "#F64E03";
export const AUTH_BG = "#FDFBFA";
export const AUTH_BORDER = "#EDE5DF";
export const AUTH_TEXT = "#1A1A1A";
export const AUTH_MUTED = "#7C7671";
export const AUTH_HINT = "#A8A29D";

export const CODE_LENGTH = 6; // users.randomCode — "%06d"
export const PHONE_DIGITS = 9; // +998 dan keyingi qism

/** "901234567" -> "90 123 45 67" (o'qish uchun guruhlash). */
export function groupPhone(digits: string): string {
  return [
    digits.slice(0, 2),
    digits.slice(2, 5),
    digits.slice(5, 7),
    digits.slice(7, 9),
  ]
    .filter(Boolean)
    .join(" ");
}

export function AuthButton({
  busy,
  disabled,
  onClick,
  children,
  danger = false,
}: {
  busy: boolean;
  disabled: boolean;
  onClick?: () => void;
  children: React.ReactNode;
  /** Halokatli amal (masalan "Akkauntni o'chirish") — qizil fon. */
  danger?: boolean;
}) {
  return (
    <button
      type={onClick ? "button" : "submit"}
      onClick={onClick}
      disabled={disabled}
      className="flex h-[50px] w-full items-center justify-center gap-2.5 rounded-xl text-[15.5px] font-bold text-white transition-opacity active:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
      style={{ background: danger ? "#DC2626" : AUTH_BRAND }}
    >
      {busy && (
        <span className="h-[18px] w-[18px] animate-spin rounded-full border-2 border-white/40 border-t-white" />
      )}
      {children}
    </button>
  );
}

export function ErrorNote({ children }: { children: React.ReactNode }) {
  return (
    <p
      role="alert"
      className="mt-3 rounded-xl bg-red-50 px-3.5 py-2.5 text-[13px] font-medium text-red-600"
    >
      {children}
    </p>
  );
}
