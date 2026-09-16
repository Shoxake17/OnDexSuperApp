/**
 * Ijtimoiy tarmoq belgilari — har biri O'Z brend rangida.
 *
 * ┌─ NEGA QO'LDA ──────────────────────────────────────────────────────┐
 * `lucide-react` ning bu versiyasida brend ikonalari (Facebook,
 * Instagram, YouTube) YO'Q. Shuning uchun to'rttasi shu yerda chizilgan.
 *
 * Avval ular `currentColor` bilan kulrang edi; endi foydalanuvchi
 * belgini birinchi qarashda tanishi uchun rasmiy ranglarda:
 * Telegram #26A5E4, Instagram gradient, Facebook #1877F2, YouTube #FF0000.
 * └────────────────────────────────────────────────────────────────────┘
 */
type IconProps = { className?: string };

export function TelegramIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <circle cx="12" cy="12" r="12" fill="#26A5E4" />
      <path
        fill="#fff"
        d="M5.3 11.7 16.8 7.3c.6-.2 1.1.1.9 1l-1.9 9.1c-.1.7-.6.8-1.1.5l-3-2.2-1.4 1.4c-.2.2-.4.3-.7.3l.2-3.1 5.7-5.1c.2-.2-.1-.3-.4-.1l-7 4.4-3-.9c-.7-.2-.7-.6.2-.9Z"
      />
    </svg>
  );
}

/** `gradientId` — sahifada belgi bir necha marta chiziladi, id takrorlanmasin. */
export function InstagramIcon({
  className,
  gradientId = "ondex-instagram-gradient",
}: IconProps & { gradientId?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <defs>
        <linearGradient id={gradientId} x1="0" y1="24" x2="24" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#FEDA75" />
          <stop offset="0.3" stopColor="#FA7E1E" />
          <stop offset="0.55" stopColor="#D62976" />
          <stop offset="0.8" stopColor="#962FBF" />
          <stop offset="1" stopColor="#4F5BD5" />
        </linearGradient>
      </defs>
      <rect width="24" height="24" rx="6.5" fill={`url(#${gradientId})`} />
      <rect x="5.5" y="5.5" width="13" height="13" rx="4" fill="none" stroke="#fff" strokeWidth="1.8" />
      <circle cx="12" cy="12" r="3.1" fill="none" stroke="#fff" strokeWidth="1.8" />
      <circle cx="15.9" cy="8.1" r="1" fill="#fff" />
    </svg>
  );
}

export function FacebookIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <circle cx="12" cy="12" r="12" fill="#1877F2" />
      <path
        fill="#fff"
        d="M13.4 24v-8.4h2.8l.4-3.3h-3.2v-2.1c0-.9.3-1.6 1.6-1.6h1.7V5.7c-.3 0-1.3-.1-2.5-.1-2.5 0-4.1 1.5-4.1 4.2v2.5H7.3v3.3h2.8V24h3.3Z"
      />
    </svg>
  );
}

export function YoutubeIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <rect x="0.5" y="4" width="23" height="16" rx="4.5" fill="#FF0000" />
      <path fill="#fff" d="M9.7 8.4v7.2l6.2-3.6-6.2-3.6Z" />
    </svg>
  );
}
