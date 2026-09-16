import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight,
  Briefcase,
  ChevronRight,
  Download,
  FileText,
  Headphones,
  Home,
  Mail,
  MapPin,
  MessageCircle,
  Navigation,
  Phone,
  PlayCircle,
  ShieldCheck,
  ShoppingBag,
  UtensilsCrossed,
  Wrench,
} from "lucide-react";

import { LandingHeader, type LandingNavItem } from "./landing-header";
import {
  ANDROID_DOWNLOAD_PATH,
  EATS_URL,
  FACEBOOK_URL,
  INSTAGRAM_URL,
  SUPPORT_EMAIL,
  SUPPORT_PHONE,
  SUPPORT_PHONE_LABEL,
  TELEGRAM_URL,
  YOUTUBE_URL,
} from "./links";
import { OndexLogo } from "./logo";
import { PhoneMock } from "./phone";
import {
  FacebookIcon,
  InstagramIcon,
  TelegramIcon,
  YoutubeIcon,
} from "./social";

/**
 * ondex.uz — ommaviy tanishtiruv sahifasi (landing).
 *
 * ┌─ BU SAHIFA ILOVA EMAS ─────────────────────────────────────────────┐
 * Qolgan `apps/web` — mijoz veb-ilovasi (eats.ondex.uz), Telegram Mini App
 * va mijoz ilovasining WebView qobig'i. Bu sahifa esa hech kim tanimaydigan
 * mehmon uchun: sessiya ham, cookie ham talab qilmaydi.
 *
 * Manzil: `ondex.uz/` -> `/landing` (qarang: `apps/web/proxy.ts`).
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ HAVOLALAR (2026-09-16) ───────────────────────────────────────────┐
 * Barcha manzillar `links.ts` da. "Kirish" va "Restoranlar" veb-ilovaga
 * (eats.ondex.uz), "Ilovani yuklab olish" eng so'nggi APK relizga
 * (`/download/android`), yordam — telefon va Telegram. Menyudagi har bir
 * band endi o'z bo'limiga olib boradi (`landing-header.tsx`).
 * └────────────────────────────────────────────────────────────────────┘
 */

export const metadata: Metadata = {
  title: "OnDex — Hayotingizni osonlashtiruvchi Super App",
  description:
    "Restoran, do'kon, xizmatlar, uy-joy, ish joylari va yetkazib berish — " +
    "barchasi bitta ilovada. OnDex'ni yuklab oling.",
  openGraph: {
    title: "OnDex — Hayotingizni osonlashtiruvchi Super App",
    description:
      "Restoran, do'kon, xizmatlar, uy-joy va yetkazib berish — barchasi bir joyda.",
    type: "website",
  },
};

/** Tartib sahifadagi bo'limlar tartibi bilan BIR XIL (faol bo'limni aniqlash shunga tayanadi). */
const NAV: LandingNavItem[] = [
  { label: "Bosh sahifa", id: "bosh" },
  { label: "Xizmatlar", id: "xizmatlar" },
  { label: "Biz haqimizda", id: "biz-haqimizda" },
  { label: "Hujjatlar", id: "hujjatlar" },
  { label: "Yordam", id: "yordam" },
];

/** `href` — faqat HAQIQATAN ishlaydigan bo'lim; qolganlari tez orada. */
const FEATURES: {
  icon: typeof UtensilsCrossed;
  title: string;
  text: string;
  href?: string;
}[] = [
  {
    icon: UtensilsCrossed,
    title: "Restoranlar",
    text: "Sevimli taomlaringizni toping va onlayn buyurtma bering.",
    href: EATS_URL,
  },
  {
    icon: ShoppingBag,
    title: "Do'konlar",
    text: "Mahsulotlarni tanlang va tez yetkazib berish xizmatidan foydalaning.",
  },
  {
    icon: Wrench,
    title: "Xizmatlar",
    text: "Turli xizmatlarni toping va onlayn buyurtma qiling.",
  },
  {
    icon: Home,
    title: "Uy-joy",
    text: "Sotish yoki ijaraga olish uchun eng yaxshi uy va kvartiralar.",
  },
  {
    icon: MapPin,
    title: "OnDex Xarita",
    text: "Yaqin atrofdagi joylarni xaritada toping va manzilga osongina yetib boring.",
  },
  {
    icon: Briefcase,
    title: "Ish joylari",
    text: "Yangi ish imkoniyatlarini toping va o'z karyerangizni rivojlantiring.",
  },
];

/**
 * To'lov tizimlari — rasmiy logotiplar (`public/landing/`). Har biri bir
 * xil o'lchamdagi qutiga `object-contain` bilan sig'diriladi: nisbatlari
 * juda har xil (`mir` 738x222, `uzcard` 447x447).
 */
const PAYMENTS = [
  { file: "visa", label: "Visa" },
  { file: "mastercard", label: "Mastercard" },
  { file: "humo", label: "HUMO" },
  { file: "uzcard", label: "UzCard" },
  { file: "mir", label: "МИР" },
  { file: "jcb", label: "JCB" },
];

const SOCIALS = [
  { icon: TelegramIcon, label: "Telegram", href: TELEGRAM_URL },
  { icon: InstagramIcon, label: "Instagram", href: INSTAGRAM_URL },
  { icon: FacebookIcon, label: "Facebook", href: FACEBOOK_URL },
  { icon: YoutubeIcon, label: "YouTube", href: YOUTUBE_URL },
];

const ABOUT = [
  {
    icon: MapPin,
    title: "Chust uchun, Chustda",
    text: "Mahalliy restoranlar, kuryerlar va xizmatlar bitta ilovada — shahar bo'ylab tez yetkazib berish.",
  },
  {
    icon: Navigation,
    title: "Jonli kuzatuv",
    text: "Buyurtmangiz qayerdaligini, kuryer yo'lda qancha vaqtda yetib kelishini xaritada ko'rasiz.",
  },
  {
    icon: ShieldCheck,
    title: "Xavfsiz to'lov",
    text: "Karta ma'lumotlari bank sahifasida kiritiladi — OnDex ularni ko'rmaydi va saqlamaydi.",
  },
];

/**
 * Futer ustunlari. `href` bo'lmagan yozuv — hali tayyor bo'lmagan bo'lim:
 * u KO'RINADI, lekin bosilmaydi ("bosildi-yu hech narsa bo'lmadi" — eng
 * yomon variant).
 */
const FOOTER_COLUMNS: {
  title: string;
  links: { label: string; href?: string }[];
}[] = [
  {
    title: "Xizmatlar",
    links: [
      { label: "Restoranlar", href: EATS_URL },
      { label: "Do'konlar" },
      { label: "Xizmatlar" },
      { label: "Uy-joy" },
      { label: "Ish joylari" },
      { label: "OnDex Xarita" },
    ],
  },
  {
    title: "Kompaniya",
    links: [
      { label: "Biz haqimizda", href: "#biz-haqimizda" },
      { label: "Ommaviy oferta", href: "/oferta" },
      { label: "Maxfiylik siyosati", href: "/maxfiylik" },
      { label: "Tariflar" },
      { label: "Karyera" },
    ],
  },
  {
    title: "Yordam",
    links: [
      { label: "Yordam markazi", href: `tel:${SUPPORT_PHONE}` },
      { label: "Savol-javob", href: TELEGRAM_URL },
      { label: "Foydalanish shartlari", href: "/oferta" },
      { label: "Maxfiylik siyosati", href: "/maxfiylik" },
    ],
  },
];

/** Veb versiyasi — `package.json` dan (`next.config.ts`, `scripts/version.ps1`). */
const WEB_VERSION = process.env.NEXT_PUBLIC_APP_VERSION ?? "";

export default function LandingPage() {
  return (
    <main className="bg-white text-neutral-900">
      <LandingHeader items={NAV} />
      <Hero />
      <Features />
      <About />
      <Payments />
      <Documents />
      <Help />
      <DownloadBanner />
      <Footer />
    </main>
  );
}

// ── Umumiy ──────────────────────────────────────────────────────────

const isExternal = (href: string) => /^https?:\/\//.test(href);

/** Havola turi bo'yicha to'g'ri element: ichki sahifa, tashqi sayt, tel/anchor. */
function SmartLink({
  href,
  className,
  children,
  ariaLabel,
}: {
  href: string;
  className?: string;
  children: React.ReactNode;
  ariaLabel?: string;
}) {
  if (href.startsWith("/")) {
    return (
      <Link href={href} className={className} aria-label={ariaLabel}>
        {children}
      </Link>
    );
  }
  if (isExternal(href) && !href.startsWith(EATS_URL)) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className={className}
        aria-label={ariaLabel}
      >
        {children}
      </a>
    );
  }
  return (
    <a href={href} className={className} aria-label={ariaLabel}>
      {children}
    </a>
  );
}

function SocialRow({ idPrefix }: { idPrefix: string }) {
  return (
    <div className="flex items-center gap-3">
      {SOCIALS.map(({ icon: Icon, label, href }) => (
        <a
          key={label}
          href={href}
          aria-label={label}
          title={label}
          target="_blank"
          rel="noopener noreferrer"
          className="rounded-full transition-transform hover:scale-110 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand"
        >
          {Icon === InstagramIcon ? (
            <InstagramIcon className="h-7 w-7" gradientId={`${idPrefix}-instagram`} />
          ) : (
            <Icon className="h-7 w-7" />
          )}
        </a>
      ))}
    </div>
  );
}

function SectionTitle({ children, sub }: { children: React.ReactNode; sub?: string }) {
  return (
    <div className="text-center">
      <h2 className="text-2xl font-bold tracking-tight sm:text-[28px]">{children}</h2>
      {sub && <p className="mx-auto mt-2 max-w-2xl text-sm text-neutral-500">{sub}</p>}
    </div>
  );
}

// ── Bosh ekran ──────────────────────────────────────────────────────

function Hero() {
  return (
    <section id="bosh" className="relative scroll-mt-20 overflow-hidden">
      <div className="mx-auto grid max-w-6xl items-center gap-12 px-4 py-14 sm:px-6 lg:grid-cols-2 lg:py-20">
        <div>
          <h1 className="text-4xl font-extrabold leading-[1.1] tracking-tight sm:text-5xl">
            OnDex — Hayotingizni osonlashtiruvchi{" "}
            <span className="text-brand">Super App!</span>
          </h1>

          <p className="mt-5 max-w-lg text-base leading-relaxed text-neutral-500">
            Restoran, Do&apos;kon, Xizmatlar, Buyurtma, Yetkazib berish,
            Rezervatsiya va ko&apos;plab imkoniyatlar — barchasi bir joyda!
          </p>

          <div className="mt-8 flex flex-wrap items-center gap-3">
            <a
              href={ANDROID_DOWNLOAD_PATH}
              className="inline-flex items-center gap-2 rounded-2xl bg-brand px-6 py-3.5 text-[15px] font-semibold text-white shadow-lg shadow-brand/25 transition-colors hover:bg-brand-light"
            >
              Ilovani yuklab olish
              <Download className="h-[18px] w-[18px]" />
            </a>
            <a
              href="#xizmatlar"
              className="inline-flex items-center gap-2 rounded-2xl border border-neutral-200 px-6 py-3.5 text-[15px] font-semibold transition-colors hover:bg-neutral-50"
            >
              OnDex haqida
              <PlayCircle className="h-[18px] w-[18px]" />
            </a>
          </div>

          <div className="mt-9 flex flex-wrap items-center gap-4">
            <span className="text-sm text-neutral-500">
              Biz bilan bog&apos;laning:
            </span>
            <SocialRow idPrefix="hero" />
          </div>
        </div>

        <div className="lg:justify-self-end">
          <PhoneMock />
        </div>
      </div>
    </section>
  );
}

// ── Imkoniyatlar ────────────────────────────────────────────────────

function Features() {
  return (
    <section id="xizmatlar" className="mx-auto max-w-6xl scroll-mt-20 px-4 pb-4 sm:px-6">
      <SectionTitle>OnDex&apos;da nimalar bor?</SectionTitle>

      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        {FEATURES.map(({ icon: Icon, title, text, href }) => {
          const body = (
            <>
              <Icon className="mx-auto h-7 w-7 text-brand" />
              <h3 className="mt-3 text-[15px] font-bold">{title}</h3>
              <p className="mt-2 text-[13px] leading-relaxed text-neutral-500">{text}</p>
              {href ? (
                <span className="mt-3 inline-flex items-center gap-1 text-[13px] font-semibold text-brand">
                  Ochish <ChevronRight className="h-3.5 w-3.5" />
                </span>
              ) : (
                <span className="mt-3 inline-block text-[12px] text-neutral-400">
                  Tez orada
                </span>
              )}
            </>
          );
          const cls =
            "block rounded-2xl border border-neutral-200 bg-white p-5 text-center transition-shadow";
          return href ? (
            <SmartLink key={title} href={href} className={`${cls} hover:border-brand/40 hover:shadow-md`}>
              {body}
            </SmartLink>
          ) : (
            <div key={title} className={cls}>
              {body}
            </div>
          );
        })}
      </div>
    </section>
  );
}

// ── Biz haqimizda ───────────────────────────────────────────────────

function About() {
  return (
    <section id="biz-haqimizda" className="mx-auto max-w-6xl scroll-mt-20 px-4 pt-14 sm:px-6">
      <SectionTitle sub="OnDex — Chust shahrida ishlab chiqilgan super ilova: kundalik ehtiyojlarni bitta joyda, tez va ishonchli hal qilish uchun.">
        Biz haqimizda
      </SectionTitle>
      <div className="mt-8 grid gap-4 md:grid-cols-3">
        {ABOUT.map(({ icon: Icon, title, text }) => (
          <div key={title} className="rounded-2xl bg-neutral-50 p-6">
            <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-brand/10">
              <Icon className="h-5 w-5 text-brand" />
            </span>
            <h3 className="mt-4 text-base font-bold">{title}</h3>
            <p className="mt-1.5 text-sm leading-relaxed text-neutral-500">{text}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

// ── To'lov tizimlari ────────────────────────────────────────────────

function Payments() {
  return (
    <section className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
      <div className="rounded-3xl bg-orange-50/70 px-6 py-8">
        <h2 className="text-center text-xl font-bold tracking-tight sm:text-2xl">
          To&apos;lovni qabul qilamiz
        </h2>

        <div className="mt-7 flex flex-wrap items-center justify-center gap-x-8 gap-y-6 sm:gap-x-12">
          {PAYMENTS.map((p) => (
            <div key={p.file} className="relative h-9 w-24 shrink-0 sm:h-11 sm:w-28">
              <Image
                src={`/landing/${p.file}.png`}
                alt={p.label}
                fill
                sizes="112px"
                className="object-contain"
              />
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

// ── Hujjatlar ───────────────────────────────────────────────────────

function Documents() {
  const docs = [
    {
      title: "Ommaviy oferta",
      text: "Xizmatdan foydalanish shartlari, buyurtma va to'lov qoidalari.",
      href: "/oferta",
    },
    {
      title: "Maxfiylik siyosati",
      text: "Qanday ma'lumot yig'iladi, nima uchun va qanday himoyalanadi.",
      href: "/maxfiylik",
    },
  ];
  return (
    <section id="hujjatlar" className="mx-auto max-w-6xl scroll-mt-20 px-4 pb-4 sm:px-6">
      <SectionTitle>Hujjatlar</SectionTitle>
      <div className="mx-auto mt-8 grid max-w-3xl gap-4 sm:grid-cols-2">
        {docs.map((d) => (
          <Link
            key={d.href}
            href={d.href}
            className="group flex items-start gap-4 rounded-2xl border border-neutral-200 p-5 transition-colors hover:border-brand/40 hover:bg-orange-50/40"
          >
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-brand/10">
              <FileText className="h-5 w-5 text-brand" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-base font-bold">{d.title}</span>
              <span className="mt-1 block text-sm text-neutral-500">{d.text}</span>
            </span>
            <ChevronRight className="mt-3 h-5 w-5 shrink-0 text-neutral-300 transition-colors group-hover:text-brand" />
          </Link>
        ))}
      </div>
    </section>
  );
}

// ── Yordam ──────────────────────────────────────────────────────────

function Help() {
  const items = [
    {
      icon: Headphones,
      title: "Yordam markazi",
      text: `Qo'ng'iroq qiling: ${SUPPORT_PHONE_LABEL}`,
      href: `tel:${SUPPORT_PHONE}`,
    },
    {
      icon: MessageCircle,
      title: "Savol-javob",
      text: "Telegram orqali yozing — tez javob beramiz.",
      href: TELEGRAM_URL,
    },
    {
      icon: Mail,
      title: "Elektron pochta",
      text: SUPPORT_EMAIL,
      href: `mailto:${SUPPORT_EMAIL}`,
    },
  ];
  return (
    <section id="yordam" className="mx-auto max-w-6xl scroll-mt-20 px-4 py-14 sm:px-6">
      <SectionTitle sub="Savolingiz bormi yoki buyurtmada muammo chiqdimi — biz bilan bog'laning.">
        Yordam
      </SectionTitle>
      <div className="mt-8 grid gap-4 md:grid-cols-3">
        {items.map(({ icon: Icon, title, text, href }) => (
          <SmartLink
            key={title}
            href={href}
            className="group flex items-center gap-4 rounded-2xl border border-neutral-200 p-5 transition-colors hover:border-brand/40 hover:bg-orange-50/40"
          >
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-brand text-white">
              <Icon className="h-5 w-5" />
            </span>
            <span className="min-w-0">
              <span className="block text-base font-bold">{title}</span>
              <span className="mt-0.5 block truncate text-sm text-neutral-500">{text}</span>
            </span>
          </SmartLink>
        ))}
      </div>
    </section>
  );
}

// ── Yuklab olishga chaqiruv ─────────────────────────────────────────

function DownloadBanner() {
  return (
    <section id="yuklab-olish" className="mx-auto max-w-6xl scroll-mt-20 px-4 pb-14 sm:px-6">
      <div className="flex flex-col items-start gap-6 rounded-3xl bg-brand px-7 py-8 text-white sm:flex-row sm:items-center sm:justify-between sm:px-10">
        <div>
          <h2 className="text-xl font-extrabold tracking-tight sm:text-[26px]">
            OnDex&apos;ni bugun yuklab oling!
          </h2>
          <p className="mt-2 max-w-xl text-sm text-white/85">
            Restoran, do&apos;kon, xizmatlar va yetkazib berish — barchasi
            bitta ilovada, bir necha bosishda. Android uchun.
          </p>
        </div>

        <a
          href={ANDROID_DOWNLOAD_PATH}
          className="inline-flex shrink-0 items-center gap-2 rounded-2xl bg-white px-6 py-3.5 text-[15px] font-bold text-brand shadow-sm transition-transform hover:scale-[1.02]"
        >
          Ilovani yuklab olish
          <ArrowRight className="h-[18px] w-[18px]" />
        </a>
      </div>
    </section>
  );
}

// ── Futer ───────────────────────────────────────────────────────────

function Footer() {
  // Yil QO'LDA yozilmaydi: aks holda har yanvarda eskirib qolardi.
  const year = new Date().getFullYear();

  return (
    <footer className="border-t border-neutral-100 bg-white pb-8 pt-12">
      <div className="mx-auto grid max-w-6xl gap-10 px-4 sm:px-6 lg:grid-cols-[1.3fr_repeat(3,1fr)_1.4fr]">
        <div>
          <OndexLogo size={34} />
          <p className="mt-4 text-sm text-neutral-500">OnDex — barchasi bir ilovada!</p>
        </div>

        {FOOTER_COLUMNS.map((col) => (
          <div key={col.title}>
            <h3 className="text-sm font-bold">{col.title}</h3>
            <ul className="mt-4 space-y-2.5">
              {col.links.map((l) => (
                <li key={`${col.title}-${l.label}`}>
                  {l.href ? (
                    <SmartLink
                      href={l.href}
                      className="text-[13px] text-neutral-500 transition-colors hover:text-brand"
                    >
                      {l.label}
                    </SmartLink>
                  ) : (
                    <span className="text-[13px] text-neutral-400">{l.label}</span>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ))}

        <div>
          <h3 className="text-sm font-bold">Biz bilan bog&apos;laning</h3>
          <ul className="mt-4 space-y-3 text-[13px] text-neutral-500">
            <li className="flex items-center gap-2.5">
              <Phone className="h-4 w-4 shrink-0 text-brand" />
              <a href={`tel:${SUPPORT_PHONE}`} className="hover:text-brand">
                {SUPPORT_PHONE_LABEL}
              </a>
            </li>
            <li className="flex items-center gap-2.5">
              <Mail className="h-4 w-4 shrink-0 text-brand" />
              <a href={`mailto:${SUPPORT_EMAIL}`} className="hover:text-brand">
                {SUPPORT_EMAIL}
              </a>
            </li>
            <li className="flex items-center gap-2.5">
              <MapPin className="h-4 w-4 shrink-0 text-brand" />
              Namangan viloyat Chust Shaxar
            </li>
          </ul>

          <h3 className="mt-7 text-sm font-bold">Bizga qo&apos;shiling</h3>
          <div className="mt-3">
            <SocialRow idPrefix="footer" />
          </div>
        </div>
      </div>

      <div className="mx-auto mt-10 flex max-w-6xl flex-wrap items-center justify-between gap-2 border-t border-neutral-100 px-4 pt-6 text-xs text-neutral-400 sm:px-6">
        <span>© {year} OnDex. Barcha huquqlar himoyalangan.</span>
        {WEB_VERSION && <span>Versiya v{WEB_VERSION}</span>}
      </div>
    </footer>
  );
}
