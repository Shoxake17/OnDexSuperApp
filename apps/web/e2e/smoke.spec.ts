import { test, expect } from '@playwright/test';

// ┌─ NEGA AYNAN SHU TESTLAR ────────────────────────────────────────────┐
// `next.config.ts` dagi izohda tasvirlangan HAQIQIY nosozlik: Next.js
// dev-server LAN IP kabi "begona" origin'dan kelgan so'rovlarga
// `_next/static` chunk'larini bermay qo'yardi. Natijada sahifa VIZUAL
// TO'G'RI ko'rinardi (SSR HTML yetib kelgan), lekin React hydration
// umuman ishga tushmasdi va HECH BIR TUGMA ISHLAMASDI.
//
// Bu eng yomon turdagi nosozlik: sahifa ochiladi, skrinshot chiroyli,
// lekin ilova o'lik. Uni faqat HAQIQIY BOSISH aniqlaydi — HTML ni
// tekshirish kifoya qilmaydi.
//
// Shuning uchun testlar quyidagi tartibda:
//   1. sahifa serverdan keldimi (SSR)
//   2. brauzer konsolida xato yo'qmi (hydration buzilganda shu yerda
//      ko'rinadi)
//   3. BOSISH natija berdimi (interaktivlik haqiqatan ishlayaptimi)
// └─────────────────────────────────────────────────────────────────────┘

// Konsol xatolarini yig'ib boruvchi yordamchi.
function collectConsoleErrors(page: import('@playwright/test').Page) {
    const errors: string[] = [];
    page.on('console', (msg) => {
        if (msg.type() === 'error') errors.push(msg.text());
    });
    page.on('pageerror', (err) => errors.push(`pageerror: ${err.message}`));
    return errors;
}

// ┌─ NEGA ANIQ RESTORAN NOMI ISHLATILMAYDI ────────────────────────────┐
// Avval test `Feel Food` nomini qidirardi. Lokalda ishlardi, lekin CI'da
// baza BO'SH ko'tariladi va faqat demo seed (`Chust Osh Markazi`) bo'ladi
// — test ma'lumot farqidan yiqilardi, kod nosozligidan emas.
//
// Shuning uchun tekshiruv MA'LUMOTGA emas, TUZILISHGA qaratilgan:
// katalogda kamida bitta restoran havolasi bo'lishi kerak. Bu ham
// lokalda (4 restoran), ham CI'da (1 demo restoran) ishlaydi.
// └────────────────────────────────────────────────────────────────────┘
const restaurantLink = 'a[href^="/restaurants/"]';

test('bosh sahifa yuklanadi va restoranlar ko\'rinadi', async ({ page }) => {
    const errors = collectConsoleErrors(page);

    const res = await page.goto('/');
    expect(res?.status(), 'bosh sahifa 200 qaytarishi kerak').toBeLessThan(400);

    // Katalog SSR bilan kelishi kerak — kamida bitta restoran.
    await expect(page.locator(restaurantLink).first()).toBeVisible();

    // Hydration buzilsa odatda aynan shu yerda xato chiqadi.
    const fatal = errors.filter((e) => !e.includes('favicon'));
    expect(fatal, `brauzer konsolidagi xatolar:\n${fatal.join('\n')}`).toHaveLength(0);
});

test('restoranga o\'tish ishlaydi (hydration tirik)', async ({ page }) => {
    await page.goto('/');

    // ★ ASOSIY: BOSISH. Agar hydration ishlamasa, havola bosilmaydi
    // yoki navigatsiya bo'lmaydi — test aynan shuni ushlaydi.
    await page.locator(restaurantLink).first().click();

    // ID shakli TURLICHA: haqiqiy restoranlarda hex
    // (`f3e2b323cf5c8752`), demo seed'da esa `r1`. Shuning uchun
    // `[a-f0-9]+` emas — u `r1` dagi "r" ni rad etardi va CI'da
    // navigatsiya ishlagan bo'lsa ham test yiqilardi.
    await expect(page).toHaveURL(/\/restaurants\/[\w-]+/);

    // Menyu sahifasi HAQIQATAN render bo'lganini tekshiramiz.
    // Restoran NOMI bo'yicha tekshirish qilinmadi: kartadagi matn
    // nomdan tashqari reyting va yetkazish vaqtini ham o'z ichiga
    // oladi, ya'ni undan nomni ajratib olish mo'rt bo'lardi va test
    // dizayn o'zgarishidan yiqilaverardi.
    await expect(page.locator('h1, h2').first()).toBeVisible();
});

test('asosiy sahifalar 5xx bermaydi', async ({ page }) => {
    // Kirish talab qiladigan sahifalar ham OCHILISHI kerak — ular
    // login so'rashi mumkin, lekin server xatosi bermasligi shart.
    for (const path of ['/', '/orders', '/favorites', '/profile', '/address']) {
        const res = await page.goto(path);
        expect(res?.status(), `${path} server xatosi berdi`).toBeLessThan(500);
    }
});
