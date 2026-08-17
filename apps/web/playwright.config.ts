import { defineConfig, devices } from '@playwright/test';

// ┌─ NEGA KATTA TIMEOUT ────────────────────────────────────────────────┐
// `next dev` (Turbopack) marshrutlarni TALAB BO'YICHA kompilyatsiya
// qiladi: har bir sahifaga birinchi murojaat sekundlar oladi, keyingilari
// tez. Standart 30s bilan birinchi ishga tushirish yolg'on "timeout"
// beradi — sahifa aslida ishlayapti, shunchaki hali qurilmagan.
//
// Production build'da bu muammo yo'q (`next build` hammasini oldindan
// tayyorlaydi), lekin dev serverga qarshi ishlaganda shart.
// └─────────────────────────────────────────────────────────────────────┘
export default defineConfig({
    testDir: './e2e',
    timeout: 90_000,
    expect: { timeout: 20_000 },

    // CI'da yiqilsa qayta urinmaydi — flaky test yashirilmasin.
    retries: 0,
    workers: 1,
    reporter: [['list']],

    use: {
        baseURL: process.env.ONDEX_WEB || 'http://localhost:3000',
        // Yiqilganda sabab ko'rinsin.
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
        actionTimeout: 20_000,
        navigationTimeout: 60_000,
    },

    projects: [
        { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    ],
});
