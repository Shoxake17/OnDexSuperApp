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
  'rejected': ('Rad etildi', Icons.cancel_outlined, kStatusCancelled),
  'cancelled': ('Bekor qilindi', Icons.close, kStatusCancelled),
};

(String, IconData, Color) statusStyleOf(String status) =>
    statusStyles[status] ?? (status, Icons.help_outline, Colors.grey);

const stageLabels = [
  'Qabul qilindi',
  'Tayyorlanmoqda',
  'Yo\'lda',
  'Yetkazildi'
];
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
int stageOf(String status) {
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

  const OrderProgressStepper({super.key, required this.stage});

  @override
  Widget build(BuildContext context) {
    final surface = Theme.of(context).colorScheme.surfaceContainerHighest;
    final color = stageColors[stage.clamp(0, stageColors.length - 1)];
    return Column(
      children: [
        Row(
          children: [
            for (var i = 0; i < 4; i++) ...[
              _Dot(passed: i <= stage, color: color, surface: surface),
              if (i < 3)
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
            for (var i = 0; i < 4; i++)
              Expanded(
                child: Text(
                  stageLabels[i],
                  textAlign: i == 0
                      ? TextAlign.start
                      : (i == 3 ? TextAlign.end : TextAlign.center),
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

/// "Buyurtma holati" sahifasidagi kengaytirilgan progress — ikonkalar +
/// har bosqich birinchi marta yetilgan HAQIQIY vaqti (buyurtma tarixidan,
/// `orders.Order.History`) bilan. Ranglanish mantig'i xuddi shu — barcha
/// o'tgan/joriy bosqichlar bitta (joriy) rangda.
class DetailedOrderProgress extends StatelessWidget {
  final int stage; // 0..2
  final List<DateTime?> stageTimes; // uzunligi 4

  const DetailedOrderProgress(
      {super.key, required this.stage, required this.stageTimes});

  @override
  Widget build(BuildContext context) {
    final surface = Theme.of(context).colorScheme.surfaceContainerHighest;
    final color = stageColors[stage.clamp(0, stageColors.length - 1)];
    return Column(
      children: [
        Row(
          children: [
            for (var i = 0; i < 4; i++) ...[
              _IconDot(
                passed: i <= stage,
                current: i == stage,
                icon: stageIcons[i],
                color: color,
                surface: surface,
              ),
              if (i < 3)
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
            for (var i = 0; i < 4; i++)
              Expanded(
                child: Column(
                  crossAxisAlignment: i == 0
                      ? CrossAxisAlignment.start
                      : (i == 3
                          ? CrossAxisAlignment.end
                          : CrossAxisAlignment.center),
                  children: [
                    Text(
                      stageLabels[i],
                      textAlign: i == 0
                          ? TextAlign.start
                          : (i == 3 ? TextAlign.end : TextAlign.center),
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
