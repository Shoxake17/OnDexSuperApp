import {
  Bike,
  CheckCircle2,
  ChefHat,
  HelpCircle,
  Package,
  PartyPopper,
  Receipt,
  ShoppingBag,
  Utensils,
  X,
  type LucideIcon,
} from "lucide-react";

// apps/customer_app/lib/widgets/order_status.dart bilan bir xil — ranglar,
// bosqichlar, matnlar. Bitta joyda o'zgartirilsa, "Buyurtma holati" sahifasi
// ham (kelajakda qurilsa) "Buyurtmalarim" ro'yxati ham bir xil yangilanadi.
export const STATUS_COLORS: Record<string, string> = {
  created: "#90A4AE",
  accepted: "#81C784",
  preparing: "#FF9800",
  ready: "#FF9800",
  picked_up: "#FF9800",
  delivered: "#4FC3F7",
  served: "#4FC3F7",
  rejected: "#E57373",
  cancelled: "#E57373",
};

export const STATUS_LABELS: Record<string, string> = {
  created: "Yangi",
  accepted: "Qabul qilindi",
  preparing: "Tayyorlanmoqda",
  ready: "Tayyor — kuryer kutilmoqda",
  picked_up: "Kuryerda, yo'lda",
  delivered: "Yetkazildi",
  served: "Berildi",
  rejected: "Rad etildi",
  cancelled: "Bekor qilindi",
};

// Stolda ovqatlanishda "kuryer" va "yetkazish" atamalari ma'nosiz —
// mijoz restoranning o'zida o'tiribdi. Faqat FARQ QILADIGAN holatlar
// sanaladi, qolganlari umumiy jadvaldan olinadi.
//
// Server ham xuddi shu matnlarni push uchun beradi
// (`internal/notify/live.go` — `orderStatusText`). Ikkalasi bir xil
// bo'lishi kerak: mijoz push'da bir narsa, ekranda boshqa narsa
// ko'rsa, bu xatoday tuyuladi.
export const DINE_IN_STATUS_LABELS: Record<string, string> = {
  ready: "Tayyor — hozir olib kelishadi",
  served: "Yoqimli ishtaha!",
};

export const STATUS_ICONS: Record<string, LucideIcon> = {
  created: Receipt,
  accepted: CheckCircle2,
  preparing: ChefHat,
  ready: ShoppingBag,
  picked_up: Bike,
  delivered: PartyPopper,
  served: PartyPopper,
  rejected: X,
  cancelled: X,
};

export function statusStyleOf(status: string, dineIn = false) {
  return {
    label:
      (dineIn ? DINE_IN_STATUS_LABELS[status] : undefined) ??
      STATUS_LABELS[status] ??
      status,
    Icon: STATUS_ICONS[status] ?? HelpCircle,
    color: STATUS_COLORS[status] ?? "#9E9E9E",
  };
}

export const STAGE_LABELS = ["Qabul qilindi", "Tayyorlanmoqda", "Yo'lda", "Yetkazildi"];
export const STAGE_COLORS = ["#81C784", "#FF9800", "#FF9800", "#4FC3F7"];
export const STAGE_ICONS: LucideIcon[] = [CheckCircle2, ChefHat, Bike, Package];

// Stolda ovqatlanishda UCH bosqich: "Yo'lda" bosqichi umuman yo'q
// (kuryer yo'q), tugash esa "Berildi".
export const DINE_IN_STAGE_LABELS = ["Qabul qilindi", "Tayyorlanmoqda", "Tayyor"];
export const DINE_IN_STAGE_COLORS = ["#81C784", "#FF9800", "#4FC3F7"];
export const DINE_IN_STAGE_ICONS: LucideIcon[] = [CheckCircle2, ChefHat, Utensils];

// -1: chiziq ko'rsatilmaydi (yakunlangan yoki bekor/rad qilingan).
export function stageOf(status: string, dineIn = false): number {
  switch (status) {
    case "created":
    case "accepted":
      return 0;
    case "preparing":
      return 1;
    case "ready":
      return 2;
    case "picked_up":
      // Stol buyurtmasida bu holat umuman uchramaydi.
      return dineIn ? -1 : 2;
    default:
      return -1;
  }
}

export type OrderHistoryEntry = { to?: string; at?: string };

// order.History'dan har bosqich BIRINCHI marta qachon yetilganini chiqaradi
// (haqiqiy backend vaqti, hisoblangan/soxta emas).
export function extractStageTimes(
  createdAt: Date | null,
  history: OrderHistoryEntry[] | null | undefined,
): (Date | null)[] {
  const times: (Date | null)[] = [createdAt, null, null, null];
  for (const h of history ?? []) {
    const at = h.at ? new Date(h.at) : null;
    if (!at || Number.isNaN(at.getTime())) continue;
    switch (h.to) {
      case "preparing":
        times[1] = at;
        break;
      case "ready":
      case "picked_up":
        if (!times[2]) times[2] = at;
        break;
      case "delivered":
        times[3] = at;
        break;
    }
  }
  return times;
}

function fmtTime(d: Date): string {
  const two = (n: number) => n.toString().padStart(2, "0");
  return `${two(d.getHours())}:${two(d.getMinutes())}`;
}

// order_status.dart'dagi DetailedOrderProgress bilan bir xil: barcha
// o'tgan/joriy bosqichlar BITTA (joriy) rangda ko'rsatiladi.
export function DetailedOrderProgress({
  stage,
  stageTimes,
  dineIn = false,
}: {
  stage: number;
  stageTimes: (Date | null)[];
  /** Stolda ovqatlanish — uch bosqich, "Yo'lda" yo'q. */
  dineIn?: boolean;
}) {
  // Bosqichlar soni QATTIQ yozilgan `[0,1,2,3]` emas, jadval
  // uzunligidan olinadi — aks holda dine_in uchun to'rtinchi (bo'sh)
  // doira chizilib qolardi.
  const labels = dineIn ? DINE_IN_STAGE_LABELS : STAGE_LABELS;
  const colors = dineIn ? DINE_IN_STAGE_COLORS : STAGE_COLORS;
  const icons = dineIn ? DINE_IN_STAGE_ICONS : STAGE_ICONS;
  const last = labels.length - 1;
  const indexes = Array.from({ length: labels.length }, (_, i) => i);

  const color = colors[Math.min(Math.max(stage, 0), colors.length - 1)];
  return (
    <div className="w-full">
      <div className="flex items-center">
        {indexes.map((i) => (
          <div key={i} className="flex flex-1 items-center last:flex-none">
            <div
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border"
              style={{
                backgroundColor: i <= stage ? (i === stage ? color : `${color}40`) : "transparent",
                borderColor: i <= stage ? color : "#616161",
                color: i <= stage ? (i === stage ? "#000" : color) : "#9E9E9E",
              }}
            >
              {(() => {
                const StageIcon = icons[i];
                return <StageIcon size={18} />;
              })()}
            </div>
            {i < last && (
              <div
                className="h-[3px] flex-1"
                style={{ backgroundColor: i < stage ? color : "#61616140" }}
              />
            )}
          </div>
        ))}
      </div>
      <div className="mt-1.5 flex">
        {indexes.map((i) => (
          <div
            key={i}
            className={`flex-1 text-[11px] ${i === 0 ? "text-left" : i === last ? "text-right" : "text-center"}`}
            style={{ color: i <= stage ? color : "#757575", fontWeight: i === stage ? 700 : 400 }}
          >
            <div>{labels[i]}</div>
            {stageTimes[i] && <div className="text-neutral-500">{fmtTime(stageTimes[i]!)}</div>}
          </div>
        ))}
      </div>
    </div>
  );
}
