-- Tashqi AI agentlar (integratsiya sheriklari) — "ikki kalitli" model.
--
-- ┌─ NEGA IKKI KALIT ──────────────────────────────────────────────────┐
-- Bitta API kalit BILAN foydalanuvchi nomidan ish qilish MUMKIN EMAS.
-- Har bir so'rov ikkita mustaqil sirni talab qiladi:
--
--   1. SHERIK KALITI (`ondex_live_...`)  — QAYSI ILOVA so'rayapti.
--      Sherikning SERVERIDA turadi, hech qachon mijoz qurilmasiga
--      tushmaydi.
--   2. GRANT TOKENI (`ondexg_...`)       — QAYSI FOYDALANUVCHI ruxsat
--      bergan. Faqat o'sha odam ilovadan "Ruxsat beraman" deganda
--      yaratiladi va istalgan vaqtda bekor qilinadi.
--
-- Natija: sherikning kaliti butunlay o'g'irlansa ham, hujumchi hech
-- kimning akkauntiga kira olmaydi — faqat ochiq menyuni o'qiydi.
-- Aksincha, bitta grant o'g'irlansa faqat BITTA foydalanuvchi zarar
-- ko'radi va u bitta tugma bilan uziladi.
-- └────────────────────────────────────────────────────────────────────┘
--
-- SIRLARNING HECH BIRI OCHIQ SAQLANMAYDI: jadvallarda faqat
-- `sha256(sir)` hex ko'rinishida yotadi. Bazaning zaxira nusxasi
-- oshkor bo'lsa ham undan ishlaydigan kalit tiklanmaydi.
--
-- Nega sha256, Argon2 emas: bular PAROL emas, 32 baytlik TASODIFIY
-- sirlar. Ularni lug'at bilan topib bo'lmaydi, sekin hash esa har
-- so'rovda protsessorni yoqib, DoS yuzasi ochardi (parollarda
-- `internal/users/password.go` ataylab Argon2id ishlatadi).

-- ─────────────── SHERIKLAR (integratsiya mijozlari) ───────────────
CREATE TABLE IF NOT EXISTS agent_partners (
    id          TEXT PRIMARY KEY,
    -- Foydalanuvchiga rozilik ekranida KO'RSATILADIGAN nom
    -- ("Shaddiy AI"). Shu sabab u ishonchli manbadan — CLI orqali
    -- admin kiritadi, sherik o'zi o'zgartira olmaydi.
    name        TEXT NOT NULL,
    -- Kalitning ochiq (sir bo'lmagan) boshlanishi: "ondex_live_a1b2c3d4".
    -- Loglarda va admin ro'yxatida qaysi kalit ekanini ko'rsatish
    -- uchun — to'liq kalitni saqlashga hojat qolmaydi.
    key_prefix  TEXT NOT NULL,
    key_hash    TEXT NOT NULL,
    -- live | test. `test` sherigi HAQIQIY buyurtma yarata olmaydi
    -- (kuryer chaqirilmaydi, restoranga hech narsa bormaydi).
    environment TEXT NOT NULL DEFAULT 'live',
    -- Sherik SO'RASHI mumkin bo'lgan ruxsatlar TAVANI (vergul bilan).
    -- Foydalanuvchi bundan ko'proq ruxsat bera olmaydi — hatto o'zi
    -- xohlasa ham. Ya'ni tavan ikki tomondan qo'yiladi.
    scopes      TEXT NOT NULL DEFAULT '',
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    contact     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ
);

-- Kalit bo'yicha qidiruv har so'rovda bo'ladi — indeks SHART.
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_partners_key
    ON agent_partners(key_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_partners_prefix
    ON agent_partners(key_prefix);

-- ─────────────── ULANISH SO'ROVI (rozilik oqimi) ───────────────
--
-- OAuth'ning "authorization code" bosqichiga mos. Sherik so'rov
-- ochadi, FOYDALANUVCHI OnDex ilovasida tasdiqlaydi, keyin sherik
-- tokenni BIR MARTA olib ketadi.
CREATE TABLE IF NOT EXISTS agent_link_requests (
    id           TEXT PRIMARY KEY,
    partner_id   TEXT NOT NULL REFERENCES agent_partners(id) ON DELETE CASCADE,
    -- Sherik pollingda ko'rsatadigan sir. Busiz istalgan odam
    -- `link_id` ni taxmin qilib (yoki loglardan ko'rib) begona
    -- ulanishning tokenini o'g'irlab ketardi.
    secret_hash  TEXT NOT NULL,
    -- Foydalanuvchi ilovada TERADIGAN qisqa kod ("K7P2-9QMX").
    -- Deep link ishlamagan holat uchun zaxira yo'l. Hash saqlanadi:
    -- baza oshkor bo'lsa ham kutayotgan ulanishni tasdiqlab
    -- bo'lmaydi.
    code_hash    TEXT NOT NULL,
    -- Sherikdagi foydalanuvchi identifikatori (biz uchun ma'nosiz
    -- satr). Sherik "bu ulanish kimga tegishli" ekanini o'zi
    -- bilishi uchun. HECH QACHON boshqa foydalanuvchiga
    -- ko'rsatilmaydi.
    external_ref TEXT NOT NULL DEFAULT '',
    scopes       TEXT NOT NULL DEFAULT '',
    -- pending | approved | denied | consumed | expired
    status       TEXT NOT NULL DEFAULT 'pending',
    user_id      TEXT REFERENCES users(id) ON DELETE CASCADE,
    grant_id     TEXT,
    created_at   TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    decided_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_links_code
    ON agent_link_requests(code_hash);
CREATE INDEX IF NOT EXISTS idx_agent_links_partner
    ON agent_link_requests(partner_id, created_at DESC);

-- ─────────────── GRANT (foydalanuvchi bergan ruxsat) ───────────────
CREATE TABLE IF NOT EXISTS agent_grants (
    id         TEXT PRIMARY KEY,
    partner_id TEXT NOT NULL REFERENCES agent_partners(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Token grant YARATILGANDA emas, sherik uni BIRINCHI marta olib
    -- ketganda hosil qilinadi. Shu sababli bu ustun qisqa vaqt bo'sh
    -- turadi va OCHIQ token bazada hech qachon yotmaydi.
    token_hash TEXT NOT NULL DEFAULT '',
    scopes     TEXT NOT NULL DEFAULT '',
    -- ┌─ PUL CHEGARASI ────────────────────────────────────────────┐
    -- Bu ikki qiymatni FOYDALANUVCHI rozilik ekranida o'zi qo'yadi.
    --
    --   per_order_limit_tiyin = 0  → HAR BIR buyurtma ilovada
    --     alohida tasdiqlanadi (eng qattiq rejim, standart).
    --   > 0 → shu summagacha agent o'zi yakunlaydi; undan oshsa
    --     ilovaga tasdiq so'rovi boradi.
    --
    -- daily_limit_tiyin — kunlik tavan. Agent takror-takror kichik
    -- buyurtma berib chegarani aylanib o'tolmasligi uchun.
    -- └────────────────────────────────────────────────────────────┘
    per_order_limit_tiyin BIGINT NOT NULL DEFAULT 0,
    daily_limit_tiyin     BIGINT NOT NULL DEFAULT 0,
    -- active | revoked
    status       TEXT NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    -- NULL = muddatsiz (foydalanuvchi o'zi uzguncha).
    expires_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_grants_token
    ON agent_grants(token_hash) WHERE token_hash <> '';
CREATE INDEX IF NOT EXISTS idx_agent_grants_user
    ON agent_grants(user_id, created_at DESC);

-- Bitta sherik + bitta foydalanuvchi = bitta FAOL grant. Aks holda
-- foydalanuvchi "Ulangan ilovalar" ro'yxatida bitta Shaddiy'ni
-- uzganda, ko'rinmaydigan ikkinchi grant ishlab turaverardi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_grants_partner_user
    ON agent_grants(partner_id, user_id) WHERE status = 'active';

-- ─────────────── BUYURTMA QORALAMASI (ikki bosqichli tasdiq) ───────────────
--
-- Agent buyurtmani BIR SO'ROV bilan yarata olmaydi. Avval qoralama
-- (narxi bilan), keyin tasdiq. Ikki sabab:
--   * LLM taomni/miqdorni noto'g'ri tushunishi mumkin — tasdiq
--     bosqichida agent AYNAN summani qaytarishi shart;
--   * chegaradan oshgan buyurtma foydalanuvchining O'ZIGA boradi.
CREATE TABLE IF NOT EXISTS agent_order_drafts (
    id             TEXT PRIMARY KEY,
    grant_id       TEXT NOT NULL REFERENCES agent_grants(id) ON DELETE CASCADE,
    user_id        TEXT NOT NULL,
    partner_id     TEXT NOT NULL DEFAULT '',
    restaurant_id  TEXT NOT NULL,
    -- Narxlangan tarkib SURATI. Katalog o'zgarsa ham foydalanuvchi
    -- KO'RGAN narx bilan tasdiqlashi uchun (tasdiqda qayta narxlanadi
    -- va farq chiqsa qoralama bekor qilinadi).
    items          JSONB NOT NULL,
    subtotal_tiyin BIGINT NOT NULL DEFAULT 0,
    discount_tiyin BIGINT NOT NULL DEFAULT 0,
    total_tiyin    BIGINT NOT NULL,
    payment_method TEXT NOT NULL DEFAULT '',
    -- draft | awaiting_user | placed | rejected | expired
    status         TEXT NOT NULL DEFAULT 'draft',
    -- Chegaradan oshgani uchun foydalanuvchi tasdig'i kerakmi.
    requires_user  BOOLEAN NOT NULL DEFAULT TRUE,
    order_id       TEXT,
    created_at     TIMESTAMPTZ NOT NULL,
    expires_at     TIMESTAMPTZ NOT NULL,
    decided_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_drafts_user
    ON agent_order_drafts(user_id, created_at DESC);
-- Kunlik sarfni hisoblash uchun (grant bo'yicha, joylashtirilganlar).
CREATE INDEX IF NOT EXISTS idx_agent_drafts_spend
    ON agent_order_drafts(grant_id, created_at) WHERE status = 'placed';

-- ─────────────── AUDIT ───────────────
--
-- Agent nomidan qilingan HAR BIR amal yoziladi. Bu foydalanuvchiga
-- ko'rsatiladi ("Shaddiy AI 12:03 da buyurtma yaratdi") — ya'ni bu
-- shunchaki texnik log emas, mahsulotning bir qismi. Odam nima
-- bo'layotganini ko'rmasa, unga ishonish uchun asos yo'q.
CREATE TABLE IF NOT EXISTS agent_audit (
    id         BIGSERIAL PRIMARY KEY,
    partner_id TEXT NOT NULL DEFAULT '',
    grant_id   TEXT NOT NULL DEFAULT '',
    user_id    TEXT NOT NULL DEFAULT '',
    action     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    ok         BOOLEAN NOT NULL DEFAULT TRUE,
    ip         TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_audit_user
    ON agent_audit(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_audit_partner
    ON agent_audit(partner_id, created_at DESC);
