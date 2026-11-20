import type { MetadataRoute } from "next";

import { SITE_URL } from "@/lib/site";

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
