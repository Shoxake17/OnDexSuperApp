# OnDex — Next.js mini-app'lari (BFF).

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
# `npm ci` — `npm install` EMAS: u package-lock.json ga QAT'IY amal
# qiladi. `install` esa lock faylni jimgina yangilashi mumkin, ya'ni
# CI'da va mahalliy mashinada boshqa-boshqa versiyalar yig'ilardi.
RUN npm ci

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# ┌─ NEXT_PUBLIC_* BUILD VAQTIDA SINGDIRILADI ────────────────────────┐
# Go API'da barcha sozlamalar ISHLASH vaqtida o'qiladi, shuning uchun
# bitta image har qanday muhitda ishlaydi. Next.js'da bunday EMAS:
# `NEXT_PUBLIC_*` qiymatlari kompilyatsiya paytida klient paketiga
# MATN sifatida yoziladi.
#
# Ya'ni bu qiymatlarni `docker run -e ...` bilan berish MUMKIN EMAS —
# ular allaqachon paket ichida. Shuning uchun `ARG` orqali build
# vaqtida beriladi (CI: `build-args`).
#
# Berilmasa kodadagi zaxira qiymat ishlatiladi (`lib/images.ts`,
# `app/robots.ts`) — u esa eski `chustapp.uz` domeniga ishora qiladi
# va sayt hech qayerga ulanmasdi.
#
# Server tomondagi `GO_API_URL` bunga KIRMAYDI: u `NEXT_PUBLIC_`
# prefiksisiz, ya'ni ishlash vaqtida o'qiladi va compose'da beriladi.
# └───────────────────────────────────────────────────────────────────┘
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_SITE_URL
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_SITE_URL=${NEXT_PUBLIC_SITE_URL}

RUN npm run build

# ---------- Ishlash bosqichi ----------
FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

# ┌─ BAZANI YAMOQLASH ────────────────────────────────────────────────┐
# `node:20-alpine` Alpine'ning xavfsizlik yangilanishlaridan ORQADA
# qoladi: image haftada bir marta qayta yig'iladi, Alpine esa
# yamoqlarni kunlik chiqaradi.
#
# Trivy shu sababdan `libcrypto3`/`libssl3` da HIGH darajali CVE topdi
# (CVE-2026-45447, OpenSSL PKCS7_verify da use-after-free):
#     o'rnatilgan 3.5.6-r0  ->  tuzatilgan 3.5.7-r0
#
# `apk upgrade` yamoqni build vaqtida oladi. Bu har qurilishda
# bajariladi, ya'ni image har doim eng so'nggi yamoqlar bilan chiqadi
# va yangi baza chiqishini kutish shart emas.
# └───────────────────────────────────────────────────────────────────┘
RUN apk upgrade --no-cache

# ┌─ NPM, YARN, COREPACK OLIB TASHLANADI ─────────────────────────────┐
# Next.js `standalone` chiqishi FAQAT `node` ni talab qiladi — paket
# menejerlari ishlash vaqtida umuman kerak emas.
#
# Ularni qoldirish ikki narsani beradi, ikkalasi ham yomon:
#
#  1. ZAIFLIKLAR. npm o'z ichida `tar`, `glob`, `cross-spawn`,
#     `ip-address` kabi paketlarni olib yuradi. Trivy ularda 9 ta
#     CVE topdi (node-tar'da yo'l bo'ylab chiqish, ixtiyoriy fayl
#     yozish) — ILOVA kodida emas, aynan shu paket menejerida.
#     Biz ularni ishlatmaymiz, lekin skaner (haqli ravishda) image
#     ichida borligini ko'radi.
#
#  2. HUJUM YUZASI. Konteynerga kirgan hujumchi uchun `npm install`
#     — tashqaridan kod tortib olishning tayyor yo'li.
#
# Natijada image ham kichrayadi.
# └───────────────────────────────────────────────────────────────────┘
RUN rm -rf \
      /usr/local/lib/node_modules/npm \
      /usr/local/lib/node_modules/corepack \
      /usr/local/bin/npm /usr/local/bin/npx \
      /usr/local/bin/corepack \
      /opt/yarn-* /usr/local/bin/yarn /usr/local/bin/yarnpkg

# ┌─ NEGA `USER node` ────────────────────────────────────────────────┐
# Avval bu qator YO'Q edi va konteyner ROOT ostida ishlardi. Buni
# Trivy konfiguratsiya skaneri topdi (DS-0002, HIGH).
#
# Xavf: Next.js'da masofaviy kod bajarish zaifligi topilsa,
# hujumchi darhol konteyner ichida root bo'ladi. Root'dan konteyner
# chegarasidan chiqib ketish (container escape) yo'llari sezilarli
# darajada ko'p — yadro zaifliklari, noto'g'ri ulangan volume'lar,
# ortiqcha capability'lar.
#
# `node` — rasmiy node image'idagi tayyor foydalanuvchi (uid 1000).
# Alohida yaratish shart emas.
#
# `--chown` MAJBURIY: usiz fayllar root'ga tegishli bo'lib qoladi va
# `node` ularni o'qiy olmasligi mumkin.
# └───────────────────────────────────────────────────────────────────┘
COPY --from=builder --chown=node:node /app/public ./public
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static

# Next.js ish vaqtida rasm optimizatsiyasi keshini shu yerga yozadi.
# Papka oldindan `node` egaligida yaratilmasa, birinchi so'rovda
# "EACCES: permission denied" chiqadi — ayniqsa `read_only: true`
# bilan ishlaganda (production compose shunday ishlatadi).
RUN mkdir -p .next/cache && chown -R node:node .next

USER node

EXPOSE 3000
CMD ["node", "server.js"]
