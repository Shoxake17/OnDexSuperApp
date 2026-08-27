import 'package:flutter/material.dart';

/// Buyurtma holati uchun umumiy rang/ikon/bosqich mantiq — "Buyurtmalarim"
/// ro'yxati va "Buyurtma holati" sahifasi AYNAN bir xil ranglar va
/// komponentlardan foydalanadi, shu bilan ilova bir butun, izchil dizaynga
/// ega bo'ladi (bir joyda o'zgartirilsa, hammasi bir xil yangilanadi).
///
/// Ranglar foydalanuvchi so'roviga ko'ra: Qabul qilindi — och yashil,
/// Tayyorlanmoqda va Yo'lda — sabzi (to'q sariq), Yetkazildi — och ko'k.
const Color kStatusNew =
    Color(0xFF90A4AE); // kulrang-ko'k — hali qabul qilinmagan
const Color kStatusAccepted = Color(0xFF81C784); // och yashil
const Color kStatusCarrot =
    Color(0xFFFF9800); // sabzi rang — tayyorlanmoqda/yo'lda
const Color kStatusDelivered = Color(0xFF4FC3F7); // och ko'k
const Color kStatusCancelled = Color(0xFFE57373); // och qizil

const statusStyles = {
  'created': ('Yangi', Icons.receipt_long, kStatusNew),
  'accepted': ('Qabul qilindi', Icons.check_circle_outline, kStatusAccepted),
  'preparing': ('Tayyorlanmoqda', Icons.soup_kitchen, kStatusCarrot),
  'ready': (
    'Tayyor — kuryer kutilmoqda',
    Icons.shopping_bag_outlined,
    kStatusCarrot
  ),
  'picked_up': ('Kuryerda, yo\'lda', Icons.delivery_dining, kStatusCarrot),
  'delivered': ('Yetkazildi', Icons.done_all, kStatusDelivered),
  // STOL buyurtmasining yakuniy holati (`internal/orders`: StatusServed).
  // Ro'yxatda yo'q edi — natijada stolda ovqatlangan mijoz ekranda
  // xom "served" so'zini ko'rardi.
  'served': ('Stolga berildi', Icons.room_service, kStatusDelivered),
  'rejected': ('Rad etildi', Icons.cancel_outlined, kStatusCancelled),
  'cancelled': ('Bekor qilindi', Icons.close, kStatusCancelled),
};

/// STOL (QR) buyurtmasida MA'NOSI boshqacha bo'lgan holatlar.
///
/// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
/// `ready` ikkala turda ham bor, lekin mijoz uchun butunlay boshqa narsa
/// anglatadi:
///   * yetkazishda — taom tayyor, KURYER olib ketishini kutmoqda;
///   * stolda      — taom tayyor, AFFITSIANT stolga olib kelmoqda.
///
/// Ilgari bitta matn ikkalasiga ham ishlatilardi va QR orqali buyurtma
/// bergan mijoz (u restoranning O'ZIDA o'tirgan holda) "Tayyor — kuryer
/// kutilmoqda" degan yozuvni ko'rardi. Kuryer esa bu buyurtmaga umuman
/// chaqirilmaydi: backend stol buyurtmasi uchun dispatch'ni ishga
/// tushirmaydi (`internal/httpapi/routes_orders.go` — `if !o.IsDineIn()`),
/// ya'ni mijoz hech qachon sodir bo'lmaydigan narsani kutardi.
///
/// `picked_up`/`delivered` bu yerda YO'Q — stol buyurtmasi bu holatlarga
/// umuman o'tmaydi (`internal/orders/statemachine.go`: `dine_in` uchun
/// `ready → served`).
/// └───────────────────────────────────────────────────────────────────┘
/// MATN AYNAN backend push va veb bilan bir xil bo'lishi SHART:
///   * `internal/notify/live.go` → `orderStatusText`
///   * `apps/web/lib/order-status.tsx` → `DINE_IN_STATUS_LABELS`
/// Mijoz push'da bir narsa, ekranda boshqa narsa ko'rsa, bu xatoday
/// tuyuladi. Ikkalasi ham "hozir olib kelishadi" deydi.
const dineInStatusStyles = {
  'ready': ('Tayyor — hozir olib kelishadi', Icons.room_service, kStatusCarrot),
};

(String, IconData, Color) statusStyleOf(String status, {bool dineIn = false}) {
  if (dineIn) {
    final override = dineInStatusStyles[status];
    if (override != null) return override;
  }
  return statusStyles[status] ?? (status, Icons.help_outline, Colors.grey);
}

const stageLabels = [
  'Qabul qilindi',
  'Tayyorlanmoqda',
  'Yo\'lda',
  'Yetkazildi'
];

/// Stol buyurtmasida "Yo'lda" bosqichi YO'Q — taomni affitsiant zaldagi
/// stolga olib keladi, ya'ni bosqich uchta.
///
/// Ro'yxat vebdagi `DINE_IN_STAGE_LABELS` bilan AYNAN bir xil
/// (`apps/web/lib/order-status.tsx`) — bitta mijoz ikkala klientda ham
/// bir xil bosqichlarni ko'rishi kerak.
const dineInStageLabels = [
  'Qabul qilindi',
  'Tayyorlanmoqda',
  'Tayyor',
];

/// Kuzatuv sahifasidagi TARIX chizig'i uchun yorliqlar.
///
/// Qisqa chiziqdan (`dineInStageLabels`) farqi — bu yerda YAKUNIY holat
/// ham ko'rsatiladi: tarix chizig'i har bosqichning haqiqiy vaqtini
/// chizadi va "berildi" qadami vaqti bilan ko'rinishi kerak. Qisqa
/// chiziqda esa yakunlangan buyurtma umuman ko'rsatilmaydi
/// (`stageOf` → -1), shuning uchun u yerda oxirgi yorliq "Tayyor".
const dineInTimelineLabels = [
  'Qabul qilindi',
  'Tayyorlanmoqda',
  'Stolga berildi',
];

/// Buyurtma turiga mos bosqich yorliqlari (qisqa chiziq uchun).
///
/// Bitta manba: "Buyurtmalarim" kartochkasi shu yerdan oladi. Ilgari
/// stol bosqichlari faqat `tracking_screen.dart` ichida qo'lda yozilgan
/// edi, kartochka esa "Yo'lda"/"Yetkazildi" chizib turardi.
List<String> stageLabelsFor({required bool dineIn}) =>
    dineIn ? dineInStageLabels : stageLabels;
// Har bir INDEKS o'sha bosqich JORIY bo'lganda butun chiziq oladigan rang
// (0..2 gacha ishlatiladi — "Yetkazildi" alohida, chiziqsiz ko'rsatiladi).
const stageColors = [
  kStatusAccepted,
  kStatusCarrot,
  kStatusCarrot,
  kStatusDelivered
];

/// Haqiqiy backend holatini (6 ta) 4 bosqichli vizual chiziqqa moslaydi —
/// hech qanday holat o'ylab topilmaydi, faqat ko'proq bosqich bitta
/// chiziqda guruhlanadi. -1: chiziq ko'rsatilmaydi — bu YETKAZILGAN
/// (endi alohida, faqat matn bilan ko'rsatiladi — chiziqli holat keraksiz)
/// va rad etilgan/bekor qilingan holatlar uchun.
int stageOf(String status, {bool dineIn = false}) {
  if (dineIn) {
    // Vebdagi `stageOf` bilan AYNAN bir xil: `ready` — UCHINCHI (oxirgi)
    // bosqich, ya'ni taom tayyor bo'lganda chiziq to'ladi. `picked_up`
    // stol buyurtmasida umuman uchramaydi. `served` — terminal, chiziq
    // umuman ko'rsatilmaydi (yetkazishdagi `delivered` kabi).
    switch (status) {
      case 'created':
      case 'accepted':
        return 0;
      case 'preparing':
        return 1;
      case 'ready':
        return 2;
      default:
        return -1;
    }
  }
  switch (status) {
    case 'created':
    case 'accepted':
      return 0;
    case 'preparing':
      return 1;
    case 'ready':
    case 'picked_up':
      return 2;
    default:
      return -1;
  }
}

/// Gorizontal 4 bosqichli progress-chiziq (kartochka ichida, kichikroq).
///
/// MUHIM: barcha O'TIB KETGAN va JORIY bosqichlar BITTA — joriy bosqichga
/// tegishli — rangda ko'rsatiladi (har biri o'zining alohida rangida emas).
/// Masalan holat "Yo'lda" bo'lsa, "Qabul qilindi" va "Tayyorlanmoqda"
/// nuqtalari/chiziqlari ham sabzi (Yo'lda) rangida bo'ladi — "rang-barang"
/// ko'rinish (har bosqich o'z rangida qolib ketishi) ATAYLAB oldini olindi.
class OrderProgressStepper extends StatelessWidget {
  final int stage; // 0..2 — joriy bosqich

  /// Stol (QR) buyurtmasi — bosqichlar UCHTA bo'ladi va oxirgisi
  /// "Stolga berildi". Ilgari bosqichlar soni 4 ga qattiq bog'langan
  /// edi va stol buyurtmasida ham "Yo'lda"/"Yetkazildi" chizilardi.
  final bool dineIn;

  const OrderProgressStepper({
    super.key,
    required this.stage,
    this.dineIn = false,
  });

  @override
  Widget build(BuildContext context) {
    final surface = Theme.of(context).colorScheme.surfaceContainerHighest;
    final color = stageColors[stage.clamp(0, stageColors.length - 1)];
    final labels = stageLabelsFor(dineIn: dineIn);
    final last = labels.length - 1;
    return Column(
      children: [
        Row(
          children: [
            for (var i = 0; i < labels.length; i++) ...[
              _Dot(passed: i <= stage, color: color, surface: surface),
              if (i < last)
                Expanded(
                  child: Container(
                    height: 3,
                    color: i < stage ? color : surface,
                  ),
                ),
            ],
          ],
        ),
        const SizedBox(height: 6),
        Row(
          children: [
            for (var i = 0; i < labels.length; i++)
              Expanded(
                child: Text(
                  labels[i],
                  textAlign: i == 0
                      ? TextAlign.start
                      : (i == last ? TextAlign.end : TextAlign.center),
                  style: TextStyle(
                    fontSize: 10,
                    color: i <= stage ? color : Colors.grey.shade600,
                    fontWeight:
                        i == stage ? FontWeight.bold : FontWeight.normal,
                  ),
                ),
              ),
          ],
        ),
      ],
    );
  }
}

const stageIcons = [
  Icons.check_circle_outline,
  Icons.soup_kitchen,
  Icons.delivery_dining,
  Icons.inventory_2_outlined,
];

/// Stol buyurtmasi bosqich ikonkalari — moped ikonkasi ATAYLAB yo'q
/// (`delivery_dining`): zaldagi mijozga hech kim yetkazib bormaydi.
const dineInStageIcons = [
  Icons.check_circle_outline,
  Icons.soup_kitchen,
  Icons.room_service,
];

List<IconData> stageIconsFor({required bool dineIn}) =>
    dineIn ? dineInStageIcons : stageIcons;

/// "Buyurtma holati" sahifasidagi kengaytirilgan progress — ikonkalar +
/// har bosqich birinchi marta yetilgan HAQIQIY vaqti (buyurtma tarixidan,
/// `orders.Order.History`) bilan. Ranglanish mantig'i xuddi shu — barcha
/// o'tgan/joriy bosqichlar bitta (joriy) rangda.
class DetailedOrderProgress extends StatelessWidget {
  final int stage; // 0..2
  final List<DateTime?> stageTimes; // uzunligi bosqichlar soniga teng
  final bool dineIn;

  const DetailedOrderProgress({
    super.key,
    required this.stage,
    required this.stageTimes,
    this.dineIn = false,
  });

  @override
  Widget build(BuildContext context) {
    final surface = Theme.of(context).colorScheme.surfaceContainerHighest;
    final color = stageColors[stage.clamp(0, stageColors.length - 1)];
    final labels = stageLabelsFor(dineIn: dineIn);
    final icons = stageIconsFor(dineIn: dineIn);
    final last = labels.length - 1;
    return Column(
      children: [
        Row(
          children: [
            for (var i = 0; i < labels.length; i++) ...[
              _IconDot(
                passed: i <= stage,
                current: i == stage,
                icon: icons[i],
                color: color,
                surface: surface,
              ),
              if (i < last)
                Expanded(
                  child: Container(
                    height: 3,
                    color: i < stage ? color : surface,
                  ),
                ),
            ],
          ],
        ),
        const SizedBox(height: 6),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var i = 0; i < labels.length; i++)
              Expanded(
                child: Column(
                  crossAxisAlignment: i == 0
                      ? CrossAxisAlignment.start
                      : (i == last
                          ? CrossAxisAlignment.end
                          : CrossAxisAlignment.center),
                  children: [
                    Text(
                      labels[i],
                      textAlign: i == 0
                          ? TextAlign.start
                          : (i == last ? TextAlign.end : TextAlign.center),
                      style: TextStyle(
                        fontSize: 11,
                        color: i <= stage ? color : Colors.grey.shade600,
                        fontWeight:
                            i == stage ? FontWeight.bold : FontWeight.normal,
                      ),
                    ),
                    if (stageTimes[i] != null)
                      Text(_fmtTime(stageTimes[i]!),
                          style: TextStyle(
                              fontSize: 10, color: Colors.grey.shade600)),
                  ],
                ),
              ),
          ],
        ),
      ],
    );
  }
}

String _fmtTime(DateTime d) {
  final local = d.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.hour)}:${two(local.minute)}';
}

class _IconDot extends StatelessWidget {
  final bool passed;
  final bool current;
  final IconData icon;
  final Color color;
  final Color surface;
  const _IconDot({
    required this.passed,
    required this.current,
    required this.icon,
    required this.color,
    required this.surface,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 36,
      height: 36,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: passed ? color.withValues(alpha: current ? 1 : 0.25) : surface,
        border: passed && !current
            ? null
            : Border.all(color: passed ? color : Colors.grey.shade700),
      ),
      child: Icon(icon,
          size: 18,
          color:
              passed ? (current ? Colors.black : color) : Colors.grey.shade500),
    );
  }
}

class _Dot extends StatelessWidget {
  final bool passed;
  final Color color;
  final Color surface;
  const _Dot(
      {required this.passed, required this.color, required this.surface});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 16,
      height: 16,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: passed ? color : surface,
        border: passed ? null : Border.all(color: Colors.grey.shade700),
      ),
      child: passed
          ? const Icon(Icons.check, size: 11, color: Colors.black)
          : null,
    );
  }
}
