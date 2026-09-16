import Image from "next/image";

/** Asl fayl: `image/OnDex.png` (692×749, shaffof fon). */
const LOGO_SRC = "/landing/ondex-logo.png";
const LOGO_RATIO = 692 / 749;

/**
 * OnDex belgisi — rasmiy logotip rasmi.
 *
 * Avval bu yerda qo'lda chizilgan SVG turardi va u haqiqiy logotipga
 * o'xshamasdi. Endi sarlavha ham, futer ham AYNAN brend faylini
 * ko'rsatadi; `next/image` uni ekran zichligiga mos o'lchamda beradi.
 */
export function OndexMark({ size = 32 }: { size?: number }) {
  return (
    <Image
      src={LOGO_SRC}
      alt=""
      aria-hidden="true"
      width={Math.round(size * LOGO_RATIO)}
      height={size}
      className="shrink-0 object-contain"
    />
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
    <span className={`inline-flex items-center gap-2 ${className}`}>
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
