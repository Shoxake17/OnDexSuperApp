"use client";

import { Minus, Plus, UtensilsCrossed } from "lucide-react";
import { useRef, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import type { ProductDiscount } from "@/lib/promotions";
import type { Product } from "@/lib/types";
import FavoriteButton from "./favorite-button";
import { AppButton, SheetHandle } from "../../ui";

// product_detail_sheet.dart bilan parity + Yandex Eats naqshi:
// pastdan chiquvchi panel, tepasida "tortish" chizig'i (X tugmasi EMAS),
// pastga surib yopiladi. Reyting/ingredient kabi elementlar ATAYLAB yo'q —
// backend'da bunday ma'lumot yo'q (loyihaning "soxta raqam yo'q" qoidasi).
export default function ProductDetailSheet({
  product,
  discount,
  favorited,
  initialQty,
  onClose,
  onConfirm,
  onFavoriteChange,
}: {
  product: Product;
  discount: ProductDiscount | null;
  favorited: boolean;
  initialQty: number;
  onClose: () => void;
  onConfirm: (qty: number) => void;
  onFavoriteChange?: (productId: string, favorited: boolean) => void;
}) {
  const [qty, setQty] = useState(initialQty > 0 ? initialQty : 1);
  const [imgFailed, setImgFailed] = useState(false);
  // Pastga tortib yopish holati — barmoq bilan surilgan masofa (px).
  const [dragY, setDragY] = useState(0);
  const [dragging, setDragging] = useState(false);
  const startY = useRef(0);

  const weight = product.weight ?? 0;
  const weightUnit = product.weight_unit === "l" ? "L" : product.weight_unit;
  const unitPrice = discount?.discountedPriceTiyin ?? product.price_tiyin;
  const description = product.description?.trim();

  // 120px dan ko'p tortilsa — yopiladi, aks holda joyiga qaytadi.
  const CLOSE_THRESHOLD = 120;

  function onTouchStart(e: React.TouchEvent) {
    startY.current = e.touches[0].clientY;
    setDragging(true);
  }
  function onTouchMove(e: React.TouchEvent) {
    if (!dragging) return;
    const dy = e.touches[0].clientY - startY.current;
    // Faqat PASTGA surishga ruxsat — yuqoriga tortilsa panel qimirlamaydi.
    setDragY(dy > 0 ? dy : 0);
  }
  function onTouchEnd() {
    setDragging(false);
    if (dragY > CLOSE_THRESHOLD) onClose();
    else setDragY(0);
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/60"
      onClick={onClose}
    >
      <div
        // Balandlik: qolgan sahifalardan SAL PASTROQ boshlanadi
        // (ular tepadan ~4px, bu esa ~28px) — foydalanuvchi so'raganidek.
        className="flex h-[calc(100dvh-28px)] w-full max-w-2xl flex-col overflow-hidden rounded-t-[20px] bg-white text-neutral-900 dark:bg-[#1A1A1A] dark:text-white"
        style={{
          transform: `translateY(${dragY}px)`,
          transition: dragging ? "none" : "transform 220ms ease-out",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Tortish zonasi — chiziq va rasm ustidan surib yopish mumkin. */}
        <div
          className="shrink-0 touch-none"
          onTouchStart={onTouchStart}
          onTouchMove={onTouchMove}
          onTouchEnd={onTouchEnd}
        >
          {/* Konteyner KVADRAT — mahsulot rasmlari serverda 1:1 qilib
              saqlanadi (21-band), shuning uchun `object-contain` bilan
              rasm konteynerni AYNAN to'ldiradi: na kesiladi, na yon
              tomonlarda boshqa rangli chiziq qoladi. Avval balandlik
              qat'iy 280px edi va rasm kvadrat bo'lgani uchun yonlarda
              konteyner foni ko'rinib, "ikki xil orqa fon" hosil
              bo'lardi — aynan shu tuzatildi. */}
          <div className="relative aspect-square w-full bg-white">
            {/* Tortish chizig'i RASM USTIDA suzadi — avval alohida
                qatorda edi va tepada ortiqcha qora yo'lak hosil
                qilardi (foydalanuvchi aynan shuni ko'rsatdi). */}
            <SheetHandle overlay />
            {product.image_url && !imgFailed ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={fullImageUrl(product.image_url)}
                alt={product.name}
                className="h-full w-full object-contain"
                onError={() => setImgFailed(true)}
              />
            ) : (
              <div className="flex h-full items-center justify-center bg-neutral-100 text-neutral-400">
                <UtensilsCrossed size={56} />
              </div>
            )}
            {discount && (
              <div className="absolute right-3 top-3 rounded-lg bg-[#FFD100] px-2 py-1 text-[11px] font-bold text-black">
                {discount.label}
              </div>
            )}
            <div className="absolute bottom-3 right-3">
              <FavoriteButton
                productId={product.id}
                initialFavorited={favorited}
                onChanged={onFavoriteChange}
              />
            </div>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-5">
          <h2 className="text-xl font-bold">
            {product.name}
            {weight > 0 && (
              <span className="ml-2 text-base font-normal text-neutral-500">
                {weight % 1 === 0 ? weight : weight.toFixed(1)} {weightUnit}
              </span>
            )}
          </h2>
          {discount && (
            <div className="mt-2 flex items-baseline gap-2">
              <span className="text-lg font-bold text-[#E53935]">
                {formatSum(unitPrice)}
              </span>
              <span className="text-sm text-neutral-500 line-through">
                {formatSum(product.price_tiyin)}
              </span>
            </div>
          )}
          <p
            className={`mt-3.5 text-[15px] leading-relaxed ${
              description ? "text-neutral-600 dark:text-neutral-300" : "italic text-neutral-500"
            }`}
          >
            {description || "Tavsif kiritilmagan"}
          </p>
        </div>

        <div className="safe-bottom flex shrink-0 items-center gap-3 border-t border-neutral-200 px-5 pt-3 dark:border-neutral-800">
          {product.available ? (
            <>
              <div className="flex h-[52px] shrink-0 items-center gap-3 rounded-2xl bg-neutral-100 px-2 dark:bg-neutral-800">
                <button
                  onClick={() => setQty((q) => Math.max(1, q - 1))}
                  className="flex h-11 w-9 items-center justify-center"
                  aria-label="Kamaytirish"
                >
                  <Minus size={18} />
                </button>
                <span className="w-4 text-center font-bold">{qty}</span>
                <button
                  onClick={() => setQty((q) => q + 1)}
                  className="flex h-11 w-9 items-center justify-center"
                  aria-label="Ko'paytirish"
                >
                  <Plus size={18} />
                </button>
              </div>
              <AppButton onClick={() => onConfirm(qty)}>
                Qo&apos;shish &nbsp;•&nbsp; {formatSum(unitPrice * qty)}
              </AppButton>
            </>
          ) : (
            <p className="py-4 text-neutral-500">Hozircha mavjud emas</p>
          )}
        </div>
      </div>
    </div>
  );
}
