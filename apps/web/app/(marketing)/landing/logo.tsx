/**
 * OnDex belgisi — SVG.
 *
 * ┌─ NEGA RASM EMAS, SVG ──────────────────────────────────────────────┐
 * Logotip sarlavhada (32px) ham, futerda (36px) ham, ijtimoiy
 * kartochkalarda ham ishlatiladi. PNG bo'lsa har o'lcham uchun alohida
 * fayl kerak bo'lardi va Retina ekranlarda xiralashardi.
 *
 * Rang `currentColor` EMAS, ataylab brend to'q sarig'i: logotip
 * matn rangiga ergashmasligi kerak (futerda matn kulrang).
 * └────────────────────────────────────────────────────────────────────┘
 */
export function OndexMark({ size = 32 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      fill="none"
      aria-hidden="true"
    >
      <rect width="48" height="48" rx="14" fill="#F4511E" />
      <path
        d="M24 11.5 32.8 16.6v10.2L24 31.9l-8.8-5.1V16.6L24 11.5Z"
        fill="#fff"
        fillOpacity="0.18"
      />
      <path
        d="M24 14.8 30 18.3v6.9L24 28.7l-6-3.5v-6.9l6-3.5Z"
        stroke="#fff"
        strokeWidth="2.6"
        strokeLinejoin="round"
      />
      <circle cx="24" cy="21.7" r="2.6" fill="#fff" />
    </svg>
  );
}

/** Belgi + "OnDex" yozuvi. */
export function OndexLogo({
  size = 32,
  className = "",
}: {
  size?: number;
  className?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-2.5 ${className}`}>
      <OndexMark size={size} />
      <span
        className="font-extrabold tracking-tight"
        style={{ fontSize: size * 0.78 }}
      >
        <span className="text-neutral-900">On</span>
        <span className="text-brand">Dex</span>
      </span>
    </span>
  );
}
