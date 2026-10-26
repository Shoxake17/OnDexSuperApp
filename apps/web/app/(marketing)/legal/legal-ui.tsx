import Link from "next/link";
import type { ReactNode } from "react";

/**
 * Huquqiy hujjatlar uchun umumiy ko'rinish (maxfiylik siyosati,
 * ommaviy oferta).
 *
 * ┌─ NEGA ALOHIDA KOMPONENT ───────────────────────────────────────────┐
 * Ikkala hujjat ham bir xil talablarga bo'ysunadi:
 *
 *   * uzoq matn — o'qilishi qulay bo'lishi kerak (o'lcham, qator
 *     oralig'i, sarlavhalar ierarxiyasi);
 *   * bo'lim raqamlari HAVOLA bo'lishi kerak — hujjatga murojaat
 *     qilganda "4.2-bandga qarang" deb aniq ko'rsatish mumkin
 *     bo'lsin (`#4.2` — anchor);
 *   * versiya va kuch kirish sanasi KO'RINIB turishi shart — bu
 *     huquqiy hujjatning majburiy elementi;
 *   * chop etishga (print) yaroqli bo'lsin — nizoli holatda hujjat
 *     qog'ozda so'raladi.
 *
 * Ikki joyda ikki nusxa tipografiya bo'lsa, ular ajralib ketardi va
 * bittasida sana yangilanmay qolardi.
 * └────────────────────────────────────────────────────────────────────┘
 */

/** Hujjatning amaldagi tahriri. Har o'zgarishda YANGILANADI. */
export const LEGAL_VERSION = "1.0";

/** Kuch kirish sanasi (hujjat ushbu tahrirda amal qila boshlagan kun). */
export const LEGAL_EFFECTIVE_DATE = "2026-09-07";

/** Operator rekvizitlari — IKKALA hujjat uchun YAGONA manba. */
export const OPERATOR = {
  brand: "OnDex",
  /** To'liq yuridik nom. */
  legalName: "OnDex",
  /** STIR (INN). */
  taxId: "070977329",
  /** Davlat ro'yxatidan o'tganlik guvohnomasi raqami va sanasi. */
  registration: "10.08.2026",
  /** Yuridik manzil. */
  address: "Namangan viloyati, Chust tumani, Chust, Mustaqillik mahalla",
  /** Bank rekvizitlari (hisob raqami, bank, MFO). */
  bank: "20218000907519062001",
  phone: "+998 90 278 42 07",
  email: "info@ondex.uz",
  /** Shaxsga doir ma'lumotlar bo'yicha mas'ul shaxs bilan aloqa. */
  privacyEmail: "privacy@ondex.uz",
  site: "https://ondex.uz",
} as const;

/** Rekvizit qiymati yoki to'ldirilmaganini bildiruvchi belgi. */
export function req(value: string): ReactNode {
  if (value) return value;
  return (
    <span className="rounded bg-amber-100 px-1.5 py-0.5 text-amber-900">
      to‘ldirilishi kerak
    </span>
  );
}

export function LegalPage({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <main className="bg-white text-neutral-900">
      <div className="mx-auto max-w-3xl px-5 py-10 sm:px-8 sm:py-16">
        <h1 className="text-2xl font-bold leading-tight sm:text-3xl">
          {title}
        </h1>
        <p className="mt-2 text-neutral-600">{subtitle}</p>

        <dl className="mt-6 grid grid-cols-1 gap-x-8 gap-y-2 rounded-xl bg-neutral-50 p-4 text-sm sm:grid-cols-2">
          <div className="flex gap-2">
            <dt className="text-neutral-500">Tahrir:</dt>
            <dd className="font-medium">{LEGAL_VERSION}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-neutral-500">Kuchga kirgan:</dt>
            <dd className="font-medium">{LEGAL_EFFECTIVE_DATE}</dd>
          </div>
        </dl>

        <article className="legal mt-8">{children}</article>

        <hr className="my-10 border-neutral-200" />

        <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-neutral-500">
          <Link href="/maxfiylik" className="hover:text-neutral-900">
            Maxfiylik siyosati
          </Link>
          <Link href="/oferta" className="hover:text-neutral-900">
            Ommaviy oferta
          </Link>
          <a href={`mailto:${OPERATOR.email}`} className="hover:text-neutral-900">
            {OPERATOR.email}
          </a>
        </div>
      </div>
    </main>
  );
}

/**
 * Raqamlangan bo'lim.
 *
 * `id` — anchor: hujjatga murojaat qilganda `#4` deb aniq bandga
 * havola berish mumkin bo'lsin.
 */
export function Section({
  n,
  title,
  children,
}: {
  n: string;
  title: string;
  children: ReactNode;
}) {
  return (
    <section id={n} className="scroll-mt-6">
      <h2 className="mt-9 text-lg font-bold leading-snug sm:text-xl">
        <a href={`#${n}`} className="text-neutral-400 hover:text-brand">
          {n}.
        </a>{" "}
        {title}
      </h2>
      <div className="mt-3 space-y-3 leading-relaxed text-neutral-800">
        {children}
      </div>
    </section>
  );
}

/** Ichki band — `4.2` ko'rinishidagi raqam bilan. */
export function Clause({ n, children }: { n: string; children: ReactNode }) {
  return (
    <p id={n} className="scroll-mt-6">
      <a href={`#${n}`} className="mr-1 font-semibold text-neutral-400 hover:text-brand">
        {n}.
      </a>
      {children}
    </p>
  );
}

/** Ma'lumot turlari jadvali — maxfiylik siyosatining o'zagi. */
export function DataTable({
  rows,
}: {
  rows: { what: string; why: string; basis: string; keep: string }[];
}) {
  return (
    <div className="mt-4 overflow-x-auto">
      <table className="w-full min-w-[42rem] border-collapse text-sm">
        <thead>
          <tr className="border-b border-neutral-300 text-left align-bottom">
            <th className="py-2 pr-4 font-semibold">Qanday ma’lumot</th>
            <th className="py-2 pr-4 font-semibold">Nima uchun</th>
            <th className="py-2 pr-4 font-semibold">Huquqiy asos</th>
            <th className="py-2 font-semibold">Saqlanish muddati</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.what} className="border-b border-neutral-100 align-top">
              <td className="py-2.5 pr-4">{r.what}</td>
              <td className="py-2.5 pr-4 text-neutral-700">{r.why}</td>
              <td className="py-2.5 pr-4 text-neutral-700">{r.basis}</td>
              <td className="py-2.5 text-neutral-700">{r.keep}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
