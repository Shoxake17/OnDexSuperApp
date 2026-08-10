"use client";

import { MoreHorizontal } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";

// Flutter'dagi _CategoryIconTile bilan bir xil: rasm topilmasa (hali
// public/categories/ ga qo'shilmagan turkum) oddiy belgiga tushadi —
// sahifa hech qachon buzilmaydi (Dart'dagi errorBuilder'ning web ekvivalenti).
export default function CategoryTile({
  label,
  iconSrc,
}: {
  label: string;
  iconSrc: string | null;
}) {
  const [failed, setFailed] = useState(false);

  return (
    <Link
      href={`/search?category=${encodeURIComponent(label)}`}
      className="flex w-[60px] shrink-0 flex-col items-center gap-0.5 text-center"
    >
      <div className="flex h-[50px] w-[50px] items-center justify-center">
        {iconSrc && !failed ? (
          <Image
            src={iconSrc}
            alt={label}
            width={50}
            height={50}
            className="object-contain"
            onError={() => setFailed(true)}
          />
        ) : (
          <div className="flex h-11 w-11 items-center justify-center rounded-full bg-neutral-100 text-neutral-400 dark:bg-neutral-800">
            <MoreHorizontal size={20} />
          </div>
        )}
      </div>
      <span className="line-clamp-2 text-[11px] font-medium leading-tight">{label}</span>
    </Link>
  );
}
