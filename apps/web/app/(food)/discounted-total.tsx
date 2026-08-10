import { formatSum } from "@/lib/format";

// product_grid.dart'dagi DiscountedTotal bilan bir xil: chegirma bo'lmasa
// oddiy bitta qator, bo'lsa qizil chegirmali summa + pastida (kichikroq,
// chizilgan) chegirmasiz tan narx.
export default function DiscountedTotal({
  totalTiyin,
  subtotalTiyin,
  className,
  align = "left",
}: {
  totalTiyin: number;
  subtotalTiyin: number;
  className?: string;
  align?: "left" | "right";
}) {
  if (subtotalTiyin <= totalTiyin) {
    return <span className={className}>{formatSum(totalTiyin)}</span>;
  }
  return (
    <div className={`flex flex-col ${align === "right" ? "items-end" : "items-start"}`}>
      <span className={`${className} text-[#E53935]`}>{formatSum(totalTiyin)}</span>
      <span className="text-xs text-neutral-500 line-through">
        {formatSum(subtotalTiyin)}
      </span>
    </div>
  );
}
