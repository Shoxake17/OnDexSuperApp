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
