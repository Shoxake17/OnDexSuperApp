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
