// Audit 480 (2026-11-19): Next 16 da `next lint` olib tashlangan va loyihada
// ESLint konfiguratsiyasi umuman yo'q edi — web kodi hech qachon lint
// qilinmagan. Rasmiy flat konfiguratsiya (core-web-vitals + TypeScript).
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const config = [
  ...nextVitals,
  ...nextTs,
  {
    // Mavjud eslint-disable izohlari sababni hujjatlaydi — o'chirilmaydi.
    linterOptions: { reportUnusedDisableDirectives: "off" },
    rules: {
      // O'zbekcha matnda apostrof (o'z, bo'lim) har joyda; React uni xavfsiz
      // chiqaradi — bu qoida faqat shovqin.
      "react/no-unescaped-entities": "off",
      // Rasmlar R2/CDN'dan, o'lchami noma'lum — next/image optimizatori yo'q.
      "@next/next/no-img-element": "warn",
      // localStorage/sessiyadan mount'da o'qish (gidratsiya xavfsiz naqsh)
      // ataylab effect ichida; har biri ko'rib chiqilgan — ogohlantirish.
      "react-hooks/set-state-in-effect": "warn",
    },
  },
  {
    ignores: [
      ".next/**",
      "out/**",
      "node_modules/**",
      "public/**",
      "playwright-report/**",
      "test-results/**",
      "next-env.d.ts",
    ],
  },
];

export default config;
