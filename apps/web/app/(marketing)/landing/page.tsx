import type { Metadata } from "next";
import Image from "next/image";
import {
  ArrowRight,
  Briefcase,
  Download,
  Home,
  Mail,
  MapPin,
  Phone,
  PlayCircle,
  ShieldCheck,
  ShoppingBag,
  UtensilsCrossed,
  Wrench,
} from "lucide-react";

import { OndexLogo, OndexMark } from "./logo";
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
 * Qolgan `apps/web` — Telegram Mini App va mijoz ilovasining WebView
 * qobig'i, ya'ni FAQAT tizimga kirgan foydalanuvchi uchun. Bu sahifa
 * esa aksincha: hech kim tanimaydigan mehmon uchun, sessiya ham,
 * cookie ham talab qilmaydi.
 *
 * Shu sabab u alohida `(marketing)` guruhida: kelajakda mini-ilovaga
 * qo'shiladigan qobiq (pastki menyu, avtorizatsiya devori) bu yerga
 * tasodifan tushib qolmasin.
 *
 * Manzil: `ondex.uz/` -> `/landing` (qarang: `apps/web/proxy.ts`).
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ MAKETDAN FARQLAR (ATAYLAB) ───────────────────────────────────────┐
 * Maketdagi uchta bo'lim TUSHIRIB QOLDIRILDI (buyurtmachi talabi):
 * "Требования к вашему сайту", "Как подключиться к OnDex оплате",
 * "Почему выбирают OnDex?" — ular savdogarlar uchun edi, bu sahifa
 * esa oddiy foydalanuvchi uchun.
 *
 * Shu sabab pastdagi to'q sariq chaqiruv lentasi ham o'zgardi:
 * maketda u "to'lovga ulaning" deb savdogarni chaqirardi va endi
 * hech qayerga olib bormasdi. Uning o'rniga ilovani yuklab olish
 * chaqiruvi turadi — ya'ni lenta o'z vazifasini bajaradi.
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

/** Ilova do'koniga havola — bitta joyda. */
const DOWNLOAD_URL = "#yuklab-olish";

const NAV = [
  { label: "Bosh sahifa", href: "#bosh" },
  { label: "Xizmatlar", href: "#xizmatlar" },
  { label: "Biz haqimizda", href: "#biz-haqimizda" },
  { label: "Hujjatlar", href: "#hujjatlar" },
  { label: "Yordam", href: "#yordam" },
];

const FEATURES = [
  {
    icon: UtensilsCrossed,
    title: "Restoranlar",
    text: "Sevimli taomlaringizni toping va onlayn buyurtma bering.",
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
 * To'lov tizimlari — rasmiy logotiplar (`public/landing/`).
 *
 * ┌─ HAR BIRI BIR XIL QUTIDA ──────────────────────────────────────────┐
 * Fayllarning nisbatlari juda har xil: `mir` — 738x222 (uzun),
 * `uzcard` — 447x447 (kvadrat). Bir xil BALANDLIK berilsa kvadrat
 * logotip qo'shnilaridan ikki barobar katta ko'rinardi.
 *
 * Shuning uchun har biri bir xil o'lchamdagi qutiga solinadi va
 * `object-contain` bilan ichiga sig'diriladi — qator optik jihatdan
 * tekis chiqadi.
 * └────────────────────────────────────────────────────────────────────┘
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
  { icon: TelegramIcon, label: "Telegram", href: "https://t.me/ondex_uz" },
  {
    icon: InstagramIcon,
    label: "Instagram",
    href: "https://instagram.com/ondex.uz",
  },
  { icon: FacebookIcon, label: "Facebook", href: "https://facebook.com/ondex.uz" },
  { icon: YoutubeIcon, label: "YouTube", href: "https://youtube.com/@ondex_uz" },
];

const FOOTER_COLUMNS = [
  {
    title: "Xizmatlar",
    links: [
      "Restoranlar",
      "Do'konlar",
      "Xizmatlar",
      "Uy-joy",
      "Ish joylari",
      "OnDex Xarita",
    ],
  },
  {
    title: "Kompaniya",
    links: ["Biz haqimizda", "Hujjatlar", "Tariflar", "Yangiliklar", "Karyera"],
  },
  {
    title: "Yordam",
    links: [
      "Yordam markazi",
      "Savol-javob",
      "Foydalanish shartlari",
      "Maxfiylik siyosati",
    ],
  },
];

export default function LandingPage() {
  return (
    <main className="bg-white text-neutral-900">
      <Header />
      <Hero />
      <Features />
      <Payments />
      <DownloadBanner />
      <Footer />
    </main>
  );
}

// ── Sarlavha ────────────────────────────────────────────────────────

function Header() {
  return (
    <header className="sticky top-0 z-40 border-b border-neutral-100 bg-white/90 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-6 px-4 sm:px-6">
        <a href="#bosh" aria-label="OnDex — bosh sahifa">
          <OndexLogo size={30} />
        </a>

        {/* Navigatsiya faqat kengroq ekranlarda: telefonda u yig'ilib
            ketardi va asosiy tugmani (yuklab olish) siqib qo'yardi.
            Ro'yxatning o'zi futerda to'liq takrorlanadi, ya'ni
            hech qanday havola yo'qolmaydi. */}
        <nav className="ml-4 hidden items-center gap-6 lg:flex">
          {NAV.map((n, i) => (
            <a
              key={n.href}
              href={n.href}
              className={
                i === 0
                  ? "border-b-2 border-brand pb-1 text-sm font-semibold text-brand"
                  : "text-sm font-medium text-neutral-600 transition-colors hover:text-neutral-900"
              }
            >
              {n.label}
            </a>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-2 sm:gap-3">
          <a
            href="/"
            className="hidden rounded-xl border border-neutral-200 px-4 py-2 text-sm font-semibold transition-colors hover:bg-neutral-50 sm:inline-block"
          >
            Kirish
          </a>
          <a
            href={DOWNLOAD_URL}
            className="inline-flex items-center gap-2 rounded-xl bg-brand px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-brand-light"
          >
            Ilovani yuklab olish
            <Download className="h-4 w-4" />
          </a>
        </div>
      </div>
    </header>
  );
}

// ── Bosh ekran ──────────────────────────────────────────────────────

function Hero() {
  return (
    <section id="bosh" className="relative overflow-hidden">
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
              href={DOWNLOAD_URL}
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
            <SocialRow />
          </div>
        </div>

        <div className="lg:justify-self-end">
          <PhoneMock />
        </div>
      </div>
    </section>
  );
}

function SocialRow() {
  return (
    <div className="flex items-center gap-3">
      {SOCIALS.map(({ icon: Icon, label, href }) => (
        <a
          key={label}
          href={href}
          aria-label={label}
          target="_blank"
          rel="noreferrer"
          className="text-neutral-400 transition-colors hover:text-brand"
        >
          <Icon className="h-5 w-5" />
        </a>
      ))}
    </div>
  );
}

// ── Imkoniyatlar ────────────────────────────────────────────────────

function Features() {
  return (
    <section id="xizmatlar" className="mx-auto max-w-6xl px-4 pb-4 sm:px-6">
      <h2 className="text-center text-2xl font-bold tracking-tight sm:text-[28px]">
        OnDex&apos;da nimalar bor?
      </h2>

      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        {FEATURES.map(({ icon: Icon, title, text }) => (
          <div
            key={title}
            className="rounded-2xl border border-neutral-200 bg-white p-5 text-center transition-shadow hover:shadow-md"
          >
            <Icon className="mx-auto h-7 w-7 text-brand" />
            <h3 className="mt-3 text-[15px] font-bold">{title}</h3>
            <p className="mt-2 text-[13px] leading-relaxed text-neutral-500">
              {text}
            </p>
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
            <div
              key={p.file}
              className="relative h-9 w-24 shrink-0 sm:h-11 sm:w-28"
            >
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

// ── Yuklab olishga chaqiruv ─────────────────────────────────────────

function DownloadBanner() {
  return (
    <section
      id="yuklab-olish"
      className="mx-auto max-w-6xl px-4 pb-14 sm:px-6"
    >
      <div className="flex flex-col items-start gap-6 rounded-3xl bg-brand px-7 py-8 text-white sm:flex-row sm:items-center sm:justify-between sm:px-10">
        <div>
          <h2 className="text-xl font-extrabold tracking-tight sm:text-[26px]">
            OnDex&apos;ni bugun yuklab oling!
          </h2>
          <p className="mt-2 max-w-xl text-sm text-white/85">
            Restoran, do&apos;kon, xizmatlar va yetkazib berish — barchasi
            bitta ilovada, bir necha bosishda.
          </p>
        </div>

        <a
          href={DOWNLOAD_URL}
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
  // Yil QO'LDA yozilmaydi: maketda "© 2024" turgan va u har yanvarda
  // eskirib, saytni tashlab qo'yilgandek ko'rsatardi.
  const year = new Date().getFullYear();

  return (
    <footer
      id="yordam"
      className="border-t border-neutral-100 bg-white pb-10 pt-12"
    >
      <div className="mx-auto grid max-w-6xl gap-10 px-4 sm:px-6 lg:grid-cols-[1.3fr_repeat(3,1fr)_1.4fr]">
        <div>
          <OndexLogo size={34} />
          <p className="mt-4 text-sm text-neutral-500">
            OnDex — barchasi bir ilovada!
          </p>
        </div>

        {FOOTER_COLUMNS.map((col) => (
          <div key={col.title} id={col.title === "Kompaniya" ? "biz-haqimizda" : undefined}>
            <h3 className="text-sm font-bold">{col.title}</h3>
            <ul className="mt-4 space-y-2.5">
              {col.links.map((l) => (
                <li key={l}>
                  <a
                    href="#"
                    className="text-[13px] text-neutral-500 transition-colors hover:text-brand"
                  >
                    {l}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}

        <div id="hujjatlar">
          <h3 className="text-sm font-bold">Biz bilan bog&apos;laning</h3>
          <ul className="mt-4 space-y-3 text-[13px] text-neutral-500">
            <li className="flex items-center gap-2.5">
              <Phone className="h-4 w-4 shrink-0 text-brand" />
              <a href="tel:+998902784207" className="hover:text-brand">
                +998 90 278 42 07
              </a>
            </li>
            <li className="flex items-center gap-2.5">
              <Mail className="h-4 w-4 shrink-0 text-brand" />
              <a href="mailto:info@ondex.uz" className="hover:text-brand">
                info@ondex.uz
              </a>
            </li>
            <li className="flex items-center gap-2.5">
              <MapPin className="h-4 w-4 shrink-0 text-brand" />
              Namangan viloyat Chust Shaxar
            </li>
          </ul>

          <h3 className="mt-7 text-sm font-bold">Bizga qo&apos;shiling</h3>
          <div className="mt-3">
            <SocialRow />
          </div>
        </div>
      </div>

      {/* Kichik ekranlarda belgi takrorlanmaydi — yuqoridagi ustun
          allaqachon logotip bilan boshlanadi. */}
      <div className="sr-only">
        <OndexMark size={16} />
      </div>
    </footer>
  );
}
