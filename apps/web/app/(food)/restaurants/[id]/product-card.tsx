"use client";

import { Minus, Plus, UtensilsCrossed } from "lucide-react";
import { useState } from "react";
import { fullImageUrl } from "@/lib/images";
import { formatSum } from "@/lib/format";
import type { ProductDiscount } from "@/lib/promotions";
import type { Product } from "@/lib/types";
import FavoriteButton from "./favorite-button";
import ProductDetailSheet from "./product-detail-sheet";

// product_grid.dart'dagi ProductCard bilan bir xil: kvadrat rasm, chap
// yuqorida yurak, o'ng yuqorida aksiya lentasi, pastda "+"/miqdor
// tugmasi, nom+narx (chegirmali bo'lsa qizil narx+chizilgan tan narx).
export default function ProductCard({
  product,
  qty,
  promoted,
  discount,
  favorited,
  onAdd,
  onRemove,
  onQtyChange,
  onFavoriteChange,
}: {
  product: Product;
  qty: number;
  // promoted — HOZIR faol biror aksiya shu mahsulotni/turkumni yoki butun
  // buyurtmani qamrab olganini bildiradi (ribbon uchun). `discount` esa
  // ANIQ narx hisoblanganda TO'LDIRILADI (faqat mahsulot/turkum darajasidagi
  // percent/fixed_amount) — order-wide/BOGO holatida promoted=true bo'lsa
  // ham discount null bo'lishi mumkin (narx o'zgarmaydi, faqat umumiy
  // "Aksiya" ribbon'i ko'rsatiladi) — product_grid.dart bilan bir xil.
  promoted: boolean;
  discount: ProductDiscount | null;
  favorited: boolean;
  onAdd: () => void;
  onRemove: () => void;
  // onQtyChange — tafsilotlar oynasidagi "Qo'shish" tugmasi ANIQ miqdorga
  // (masalan +1 emas, to'g'ridan-to'g'ri 3 taga) o'rnatish uchun.
  onQtyChange?: (qty: number) => void;
  // onFavoriteChange — yurak bosilib serverga SAQLANGANDAN keyin
  // chaqiriladi. Chaqiruvchi (menyu/savat sahifasi) o'zining
  // `favoriteIds` to'plamini yangilashi SHART — aks holda kartochkada
  // bosilgan yurak tafsilotlar oynasida ESKI holatda ko'rinadi
  // (aynan shu bug topilgan edi).
  onFavoriteChange?: (productId: string, favorited: boolean) => void;
}) {
  const [imgFailed, setImgFailed] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const available = product.available;
  const weight = product.weight ?? 0;
  const weightUnit = product.weight_unit === "l" ? "L" : product.weight_unit;

  return (
    <div className={available ? "" : "opacity-40"}>
      <div className="relative aspect-square w-full overflow-hidden rounded-2xl bg-neutral-100 dark:bg-neutral-800">
        <button
          type="button"
          onClick={() => setDetailOpen(true)}
          className="absolute inset-0 z-0 h-full w-full text-left"
          aria-label={product.name}
        >
          {product.image_url && !imgFailed ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={fullImageUrl(product.image_url)}
              alt={product.name}
              className="h-full w-full object-cover"
              onError={() => setImgFailed(true)}
            />
          ) : (
            <div className="flex h-full items-center justify-center text-neutral-400">
              <UtensilsCrossed size={28} />
            </div>
          )}
        </button>

        <div className="absolute left-2 top-2">
          <FavoriteButton
            productId={product.id}
            initialFavorited={favorited}
            onChanged={onFavoriteChange}
          />
        </div>

        {promoted && available && (
          <div className="absolute right-2 top-2 rounded-lg bg-[#FFD100] px-2 py-1 text-[11px] font-bold text-black">
            {discount?.label ?? "Aksiya"}
          </div>
        )}

        {!available && (
          <div className="absolute bottom-2 left-2 rounded-lg bg-black/85 px-2 py-1 text-[11px] text-white">
            Tugadi
          </div>
        )}

        {available && qty === 0 && (
          <button
            onClick={onAdd}
            className="absolute bottom-2 right-2 flex h-9 w-9 items-center justify-center rounded-full bg-white text-black shadow-md active:scale-95"
          >
            <Plus size={20} />
          </button>
        )}
        {available && qty > 0 && (
          <div className="absolute bottom-2 left-2 right-2 flex items-center justify-between">
            <button
              onClick={onRemove}
              className="flex h-9 w-9 items-center justify-center rounded-full bg-white text-black shadow-md active:scale-95"
            >
              <Minus size={20} />
            </button>
            <span className="rounded-full bg-white px-2.5 py-1 text-sm font-bold text-black shadow-md">
              {qty}
            </span>
            <button
              onClick={onAdd}
              className="flex h-9 w-9 items-center justify-center rounded-full bg-white text-black shadow-md active:scale-95"
            >
              <Plus size={20} />
            </button>
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={() => setDetailOpen(true)}
        className="mt-2 block w-full text-left"
      >
        {discount ? (
          <div className="flex items-baseline gap-1.5">
            <span className="font-bold text-[#E53935]">
              {formatSum(discount.discountedPriceTiyin)}
            </span>
            <span className="text-xs text-neutral-500 line-through">
              {formatSum(product.price_tiyin)}
            </span>
          </div>
        ) : (
          <span className="font-bold">{formatSum(product.price_tiyin)}</span>
        )}
        <p className="mt-0.5 line-clamp-2 text-sm">{product.name}</p>
        {weight > 0 && (
          <p className="mt-0.5 text-xs text-neutral-500">
            {weight % 1 === 0 ? weight : weight.toFixed(1)} {weightUnit}
          </p>
        )}
      </button>

      {detailOpen && (
        <ProductDetailSheet
          product={product}
          discount={discount}
          favorited={favorited}
          initialQty={qty}
          onFavoriteChange={onFavoriteChange}
          onClose={() => setDetailOpen(false)}
          onConfirm={(newQty) => {
            if (onQtyChange) onQtyChange(newQty);
            else for (let i = 0; i < newQty; i++) onAdd();
            setDetailOpen(false);
          }}
        />
      )}
    </div>
  );
}
