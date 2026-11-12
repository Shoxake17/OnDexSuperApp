// MapLibre GL v6 workerini `public/vendor/` ga ko'chiradi (predev/prebuild).
//
// NEGA (2026-11-12, SEC-ZERO Plane 450): v6 worker manzilini `import.meta.url`
// dan topadi. Next/Turbopack bundle'ida bu manzil chunk'ga ishora qiladi va
// worker o'rniga sahifa HTML'i yuklanib, xarita jimgina o'ladi (2026-10-06 da
// 6.4.1 bilan aynan shunday bo'lgan). Shuning uchun worker o'zimizning
// statik manzilimizdan beriladi va `ondex-view.ts` `setWorkerUrl` bilan
// shu manzilni aytadi. Fayl npm paketidan nusxa — qo'lda tahrirlanmaydi va
// git'ga kirmaydi (.gitignore).
import { copyFileSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const src = join(root, "node_modules", "maplibre-gl", "dist", "maplibre-gl-worker.mjs");
const dstDir = join(root, "public", "vendor");
const dst = join(dstDir, "maplibre-gl-worker.mjs");

if (!existsSync(src)) {
  console.error(`[maplibre] worker topilmadi: ${src} — maplibre-gl versiyasi o'zgarganmi?`);
  process.exit(1);
}
mkdirSync(dstDir, { recursive: true });
copyFileSync(src, dst);
const { version } = JSON.parse(readFileSync(join(root, "node_modules", "maplibre-gl", "package.json"), "utf8"));
console.log(`[maplibre] worker ${version} -> public/vendor/maplibre-gl-worker.mjs`);
