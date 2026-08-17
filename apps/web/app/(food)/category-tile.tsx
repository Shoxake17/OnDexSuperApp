"use client";

import { MoreHorizontal } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";

// Maketdagi (image/restarant.png) dumaloq turkum ikoni: ochiq kulrang
// ramka ichida taom surati, ostida ikki qatorgacha sig'adigan nom.
//
// Rasm topilmasa (hali public/categories/ ga qo'shilmagan turkum) oddiy
// belgiga tushadi — sahifa hech qachon buzilmaydi (Dart'dagi
// errorBuilder'ning web ekvivalenti).
export default function CategoryTile({
  label,
  iconSrc,
}: {
  label: string;
  iconSrc: string | null;
}) {
  const [failed, setFailed] = useState(false);

  return (
    // O'lchamlar ATAYLAB kichik: turkumlar qatori bosh sahifaning
    // yordamchi navigatsiyasi, asosiy kontenti EMAS. Katta bo'lganda u
    // birinchi ekranni to'ldirib, restoranlarni pastga surib yuborardi —
    // haqiqiy qurilmada aynan shu sezilgan.
    <Link
      href={`/search?category=${encodeURIComponent(label)}`}
      className="flex w-[64px] shrink-0 flex-col items-center gap-1 text-center"
    >
      <div className="flex h-[56px] w-[56px] items-center justify-center overflow-hidden rounded-full border border-neutral-200 bg-white dark:border-neutral-700 dark:bg-neutral-800">
        {iconSrc && !failed ? (
          <Image
            src={iconSrc}
            alt={label}
            width={56}
            height={56}
            // `p-1` — surat ramkaga tegib turmasin (maketda ham
            // atrofida biroz oq joy bor).
            className="h-full w-full object-contain p-1"
            onError={() => setFailed(true)}
          />
        ) : (
          <MoreHorizontal size={18} className="text-neutral-400" />
        )}
      </div>
      <span className="line-clamp-2 text-[11px] font-medium leading-tight">
        {label}
      </span>
    </Link>
  );
}
