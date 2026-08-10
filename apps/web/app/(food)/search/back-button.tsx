"use client";

import { useRouter } from "next/navigation";
import { goBack } from "@/lib/nav";
import { BackButton } from "../ui";

// `/search` — Server Component (SEO uchun SSR). `BackButton` esa
// `onClick` talab qiladi, shuning uchun shu kichik client o'ram.
export default function SearchBackButton() {
  const router = useRouter();
  return <BackButton onClick={() => goBack(() => router.push("/"))} />;
}
