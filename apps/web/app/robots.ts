import type { MetadataRoute } from "next";

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "https://app.chustapp.uz";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        // Auth talab qiladigan/shaxsiy sahifalar — indekslashning ma'nosi yo'q.
        disallow: ["/api/", "/cart", "/checkout", "/address", "/orders/"],
      },
    ],
    sitemap: `${SITE_URL}/sitemap.xml`,
  };
}
