import type { Metadata } from "next";

import { Clause, LegalPage, Section } from "../legal/legal-ui";

/**
 * Zaiflik haqida xabar berish siyosati (VDP) — Plane 192/196/197.
 *
 * ┌─ ASOS ────────────────────────────────────────────────────────────────┐
 * disclose.io "core terms" va ISO/IEC 29147 tuzilishi: doira, qoidalar,
 * xavfsiz hudud (safe harbor), javob muddatlari, kelishilgan oshkor qilish.
 * Pul mukofoti YO'Q (egasi qarori) — minnatdorchilik Hall of Fame'da.
 * `/.well-known/security.txt` (RFC 9116) shu sahifaga ishora qiladi;
 * uning `Expires` muddati smoke tekshiruvida kuzatiladi (cmd/smoke).
 * └───────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ HUQUQIY KO'RIB CHIQISH ──────────────────────────────────────────────┐
 * Xavfsiz hudud (6-bo'lim) matni yurist ko'rigidan o'tishi kerak — bu
 * texnik jihatdan to'liq loyiha, yakuniy huquqiy xulosa emas.
 * └───────────────────────────────────────────────────────────────────────┘
 */

const CONTACT = "security@ondex.uz";

export const metadata: Metadata = {
  title: "Xavfsizlik va zaiflik haqida xabar berish — OnDex",
  description:
    "OnDex ekotizimida zaiflik topdingizmi? Qanday xabar berish, nima doirada, " +
    "biz nimaga majburmiz va vijdonan tadqiqotchilar uchun xavfsiz hudud.",
  alternates: { canonical: "https://ondex.uz/security" },
};

export default function SecurityPage() {
  return (
    <LegalPage
      title="Zaiflik haqida xabar berish siyosati"
      subtitle="OnDex foydalanuvchilar, hamkorlar va ularning pullari xavfsizligini birinchi o'ringa qo'yadi. Zaiflik topgan bo'lsangiz — bizga xabar bering, birga tuzatamiz."
    >
      <nav className="mb-6 flex gap-4 text-sm">
        <a href="#uz" className="text-brand hover:underline">O‘zbekcha</a>
        <a href="#ru" className="text-brand hover:underline">Русский</a>
        <a href="#en" className="text-brand hover:underline">English</a>
      </nav>

      <div id="uz" />
      <Section n="1" title="Qanday xabar beriladi">
        <Clause n="1.1">
          Xabarni <a href={`mailto:${CONTACT}`} className="font-semibold text-brand">{CONTACT}</a>{" "}
          manziliga yuboring. Iltimos, quyidagilarni yozing: zaif joy (manzil, ilova va
          versiya), qadamma-qadam takrorlash, ta’sir (nima qilish mumkin bo‘ladi) va
          imkon bo‘lsa — isbot (skrinshot, so‘rov/javob). Bitta xabarda bitta zaiflik.
        </Clause>
        <Clause n="1.2">
          Zaiflik haqida ochiq joyda (ijtimoiy tarmoq, guruh, GitHub issue) yozmang —
          avval bizga xabar bering (7-bo‘lim).
        </Clause>
      </Section>

      <Section n="2" title="Doira">
        <Clause n="2.1">
          Doira ichida: <b>ondex.uz</b> va uning subdomenlari (api, passport, merchant,
          maps va boshqalar), OnDex mobil ilovalari (mijoz ilovasi, OnDexPro, OnDexGO),
          OnDex Merchant paneli va OnDex ID (yagona kirish).
        </Clause>
        <Clause n="2.2">
          Doiradan tashqari: uchinchi tomon xizmatlari (Cloudflare, Telegram, Google,
          to‘lov tizimlari) — ularga o‘z dasturlari orqali xabar bering; xizmatni
          to‘xtatishga urinish (DoS/DDoS, ko‘p so‘rov bilan yuklash); ijtimoiy muhandislik,
          fishing va jismoniy hujumlar; spam; isbotsiz avtomatik skaner natijalari;
          real ta’siri ko‘rsatilmagan holatlar (masalan ta’sirsiz sarlavha yo‘qligi,
          kirish sahifasidagi clickjacking, SPF/DMARC sozlamalari, versiya oshkor bo‘lishi).
        </Clause>
      </Section>

      <Section n="3" title="Tadqiqot qoidalari">
        <Clause n="3.1">
          Faqat o‘z akkauntingiz yoki aniq ruxsat berilgan test akkauntlari bilan sinang.
          Boshqa odamning ma’lumotiga tegsangiz — darhol to‘xtang, ma’lumotni saqlamang,
          tarqatmang va bizga xabar bering.
        </Clause>
        <Clause n="3.2">
          Zaiflikni isbotlash uchun kerak bo‘lgan eng kam harakatni qiling: ma’lumotni
          o‘zgartirmang va o‘chirmang, pul o‘tkazmang, buyurtma va to‘lov yaratmang,
          tizimga orqa eshik qoldirmang, xizmat ishiga xalal bermang.
        </Clause>
        <Clause n="3.3">
          So‘rovlar tezligini cheklang. Avtomatik vositalarni ishlab turgan foydalanuvchilarga
          ta’sir qilmaydigan darajada ishlating.
        </Clause>
      </Section>

      <Section n="4" title="Bizning majburiyatlarimiz">
        <Clause n="4.1">
          Xabaringizni <b>3 ish kunida</b> qabul qilganimizni tasdiqlaymiz, <b>10 ish kunida</b>{" "}
          dastlabki baho beramiz va tuzatish jarayoni haqida xabardor qilib turamiz.
        </Clause>
        <Clause n="4.2">
          Tuzatish muddati xavf darajasiga bog‘liq: kritik — imkon qadar tez (odatda
          7 kungacha), yuqori — 30 kungacha, o‘rta — 60 kungacha, past — 90 kungacha.
        </Clause>
        <Clause n="4.3">
          Rozilingiz bilan ismingizni (yoki taxallusingizni) Hall of Fame bo‘limida
          minnatdorchilik bilan e’lon qilamiz.
        </Clause>
      </Section>

      <Section n="5" title="Mukofot">
        <Clause n="5.1">
          Hozircha pul mukofoti dasturi yo‘q. Minnatdorchilik — Hall of Fame va rasmiy
          tashakkurnoma. Xabar uchun to‘lov talab qilish (shantaj) xabarni yaxshi niyatli
          deb hisoblamaslikka asos bo‘ladi.
        </Clause>
      </Section>

      <Section n="6" title="Xavfsiz hudud (safe harbor)">
        <Clause n="6.1">
          Ushbu siyosatga vijdonan amal qilgan tadqiqotni biz ruxsat etilgan deb
          hisoblaymiz: bunday tadqiqot uchun tadqiqotchiga nisbatan huquqiy da’vo
          qo‘zg‘atmaymiz va uchinchi tomon bu borada da’vo qilsa, siyosatga amal
          qilganingizni ma’lum qilamiz.
        </Clause>
        <Clause n="6.2">
          Ushbu kafolat 3-bo‘lim qoidalari buzilgan, ma’lumot o‘g‘irlangan yoki
          tarqatilgan, foydalanuvchilarga zarar yetkazilgan yoki tovlamachilik
          qilingan holatlarga taalluqli emas.
        </Clause>
      </Section>

      <Section n="7" title="Kelishilgan oshkor qilish">
        <Clause n="7.1">
          Zaiflik tuzatilgach, uni birga e’lon qilishimiz mumkin. Tuzatilmagan zaiflikni
          oshkor qilishdan oldin biz bilan kelishing; standart muddat — xabar berilgan
          kundan <b>90 kun</b> (zarurat bo‘lsa birga o‘zgartiramiz).
        </Clause>
      </Section>

      <section id="hall-of-fame" className="scroll-mt-6">
        <h2 className="mt-9 text-lg font-bold leading-snug sm:text-xl">Hall of Fame</h2>
        <p className="mt-3 leading-relaxed text-neutral-800">
          OnDex’ni xavfsizroq qilishga hissa qo‘shganlar. Hozircha ro‘yxat bo‘sh — birinchi
          bo‘ling.
        </p>
      </section>

      <hr className="my-10 border-neutral-200" />

      <section id="ru" className="scroll-mt-6">
        <h2 className="text-lg font-bold sm:text-xl">Политика раскрытия уязвимостей (кратко)</h2>
        <div className="mt-3 space-y-3 leading-relaxed text-neutral-800">
          <p>
            Сообщайте об уязвимостях на <a href={`mailto:${CONTACT}`} className="font-semibold text-brand">{CONTACT}</a>:
            где найдено, шаги воспроизведения, влияние и доказательство. Область: ondex.uz и
            поддомены, мобильные приложения OnDex, OnDex Merchant и OnDex ID. Вне области:
            сторонние сервисы, DoS, социальная инженерия, спам, результаты сканеров без
            доказательства.
          </p>
          <p>
            Правила: тестируйте только на своих аккаунтах, минимально необходимые действия,
            не изменяйте и не удаляйте данные, не проводите платежи и заказы. Мы подтверждаем
            получение за 3 рабочих дня, первичная оценка — 10 рабочих дней. Денежного
            вознаграждения пока нет — благодарность в Hall of Fame. Добросовестное
            исследование в рамках политики считаем разрешённым и не преследуем. Согласованное
            раскрытие — по умолчанию 90 дней.
          </p>
        </div>
      </section>

      <section id="en" className="scroll-mt-6">
        <h2 className="mt-9 text-lg font-bold sm:text-xl">Vulnerability Disclosure Policy (summary)</h2>
        <div className="mt-3 space-y-3 leading-relaxed text-neutral-800">
          <p>
            Report vulnerabilities to <a href={`mailto:${CONTACT}`} className="font-semibold text-brand">{CONTACT}</a>{" "}
            with the affected asset, reproduction steps, impact and proof. In scope: ondex.uz and
            its subdomains, OnDex mobile apps, OnDex Merchant and OnDex ID (SSO). Out of scope:
            third-party services, denial of service, social engineering, spam, and unverified
            scanner output.
          </p>
          <p>
            Rules: test only with your own accounts, take the minimum action needed to
            demonstrate the issue, never modify or delete data, and do not create payments or
            orders. We acknowledge reports within 3 business days and triage within 10. There is
            no monetary bounty at this time; we credit researchers in our Hall of Fame. We consider
            good-faith research under this policy authorized and will not pursue legal action.
            Coordinated disclosure defaults to 90 days.
          </p>
        </div>
      </section>
    </LegalPage>
  );
}
