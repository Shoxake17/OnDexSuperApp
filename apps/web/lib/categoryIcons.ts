// apps/customer_app/lib/category_icons.dart bilan bir xil xarita va
// normalizatsiya mantig'i (backend'dagi catalog.NormalizeForSearch bilan
// mos) — ikkala tomon bir xil turkum nomini bir xil rasmga bog'lashi
// uchun. Rasmlar public/categories/ ichida (Flutter assets'dan nusxa).
const categoryIcons: Record<string, string> = {
  burger: "/categories/burger.png",
  kfc: "/categories/kfc.png",
  pizza: "/categories/pizza.png",
  lavash: "/categories/lavash.png",
  sushi: "/categories/sushi.png",
  kabob: "/categories/kabob.png",
  somsa: "/categories/somsa.png",
  "lag'mon": "/categories/lag'mon.png",
  lagmon: "/categories/lag'mon.png",
  hotdog: "/categories/hotdog.png",
  steyk: "/categories/steyk.png",
  sandvich: "/categories/sandvich.png",
  desert: "/categories/dessert.png",
  pishiriq: "/categories/pishiriq.png",
  nonushta: "/categories/nonushta.png",
  bolalar: "/categories/bolalar.png",
  salat: "/categories/salat.png",
  "milliy taomlar": "/categories/milliy.png",
  "turk taomlari": "/categories/turkcha.png",
  "yevropa taomlar": "/categories/yevropa.png",
  "yapon taomlari": "/categories/yapon.png",
  "italyan taomlari": "/categories/italya.png",
  "fast food": "/categories/fastfood.png",
  ichimlik: "/categories/ichimlik.png",
  shirinlik: "/categories/shirinlik.png",
  norin: "/categories/norin.png",
  gazaklar: "/categories/gazak.png",
  "quyuq ovqatlar": "/categories/quyuq-ovqatlar.png",
  "suyuq ovqatlar": "/categories/suyuq-ovqat.png",
};

function normalizeCategoryKey(s: string): string {
  return s
    .toLowerCase()
    .split("")
    .filter((ch) => /[a-z0-9]/.test(ch))
    .join("");
}

const normalizedCategoryIcons: Record<string, string> = Object.fromEntries(
  Object.entries(categoryIcons).map(([k, v]) => [normalizeCategoryKey(k), v]),
);

export function categoryIconFor(rawLabel: string): string | null {
  const key = normalizeCategoryKey(rawLabel);
  if (!key) return null;
  if (normalizedCategoryIcons[key]) return normalizedCategoryIcons[key];
  for (const [k, v] of Object.entries(normalizedCategoryIcons)) {
    if (key.includes(k) || k.includes(key)) return v;
  }
  return null;
}
