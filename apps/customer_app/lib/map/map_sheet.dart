import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';

/// XARITA PASTKI PANELI (bottom sheet).
///
/// ┌─ KO'RINISH `image/geologo/map.png` DAN ────────────────────────────┐
/// Yandex'ning xarita ilovasida qidiruv EKRAN TEPASIDA emas, pastki
/// panelda turadi. Sabab ergonomik: telefonni bir qo'lda ushlab
/// turgan odam ekran tepasiga yeta olmaydi, xaritaning o'zi esa
/// to'liq ochiq qoladi.
///
/// Panel tuzilishi rasmdagi bilan bir xil:
///
///   1. tortish belgisi (grabber) — panel ochilishini bildiradi;
///   2. qidiruv maydoni — chap tomonda belgi, o'ngda qo'shimcha tugma;
///   3. tezkor chiplar qatori («Qayerda ovqatlanish», «Dorixona»…).
///
/// Panel ILOVA mavzusida: oq fon, to'q matn. Rasm qorong'i rejimda
/// olingan, lekin ilovaning qolgan qismi yorug' — ikkisi bir xil
/// bo'lishi kerak.
/// └────────────────────────────────────────────────────────────────────┘

/// Panelning yopiq holatdagi balandligi (chiplar bilan).
const double kMapSheetHeight = 132;

/// Xarita pastki paneli — qidiruv va tezkor chiplar.
class MapSearchSheet extends StatelessWidget {
  const MapSearchSheet({
    super.key,
    required this.controller,
    required this.focusNode,
    required this.onChanged,
    required this.onSubmitted,
    required this.onClear,
    required this.onChip,
    required this.searching,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final ValueChanged<String> onChanged;
  final ValueChanged<String> onSubmitted;
  final VoidCallback onClear;

  /// Tezkor chip bosilganda — uning qidiruv so'zi bilan.
  final ValueChanged<String> onChip;

  final bool searching;

  /// Tezkor qidiruvlar — rasmda «Где поесть», «Заправки» va hokazo.
  /// Bu yerda ular OnDexMap topa oladigan turkumlarga moslangan.
  static const _chips = <({String label, IconData icon, String query})>[
    (label: 'Ovqatlanish', icon: LucideIcons.utensils, query: 'restoran'),
    (label: 'Dorixona', icon: LucideIcons.cross, query: 'dorixona'),
    (label: 'Shifoxona', icon: LucideIcons.hospital, query: 'shifoxona'),
    (label: 'Yoqilg\'i', icon: LucideIcons.fuel, query: 'zapravka'),
    (label: 'Bank', icon: LucideIcons.creditCard, query: 'bank'),
    (label: 'Maktab', icon: LucideIcons.graduationCap, query: 'maktab'),
  ];

  @override
  Widget build(BuildContext context) {
    final pastki = MediaQuery.paddingOf(context).bottom;
    return Container(
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(18)),
        boxShadow: [
          BoxShadow(color: Colors.black26, blurRadius: 14, offset: Offset(0, -2)),
        ],
      ),
      padding: EdgeInsets.only(bottom: pastki),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Tortish belgisi — rasmda ham shunday.
          Container(
            width: 36,
            height: 4,
            margin: const EdgeInsets.only(top: 8, bottom: 10),
            decoration: BoxDecoration(
              color: const Color(0xFFD4D4D8),
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: _SearchField(
              controller: controller,
              focusNode: focusNode,
              onChanged: onChanged,
              onSubmitted: onSubmitted,
              onClear: onClear,
              searching: searching,
            ),
          ),
          const SizedBox(height: 10),
          SizedBox(
            height: 36,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              itemCount: _chips.length,
              separatorBuilder: (_, __) => const SizedBox(width: 8),
              itemBuilder: (_, i) {
                final c = _chips[i];
                return _Chip(
                  label: c.label,
                  icon: c.icon,
                  onTap: () => onChip(c.query),
                );
              },
            ),
          ),
          const SizedBox(height: 12),
        ],
      ),
    );
  }
}

class _SearchField extends StatelessWidget {
  const _SearchField({
    required this.controller,
    required this.focusNode,
    required this.onChanged,
    required this.onSubmitted,
    required this.onClear,
    required this.searching,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final ValueChanged<String> onChanged;
  final ValueChanged<String> onSubmitted;
  final VoidCallback onClear;
  final bool searching;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: const Color(0xFFF4F4F5),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          const SizedBox(width: 14),
          const Icon(LucideIcons.search, size: 19, color: Color(0xFF71717A)),
          const SizedBox(width: 10),
          Expanded(
            child: TextField(
              controller: controller,
              focusNode: focusNode,
              textInputAction: TextInputAction.search,
              onChanged: onChanged,
              onSubmitted: onSubmitted,
              style: const TextStyle(fontSize: 16),
              decoration: const InputDecoration(
                hintText: 'Joy va manzil qidirish',
                hintStyle: TextStyle(color: Color(0xFF9E9E9E)),
                border: InputBorder.none,
                isDense: true,
                contentPadding: EdgeInsets.symmetric(vertical: 14),
              ),
            ),
          ),
          if (searching)
            const Padding(
              padding: EdgeInsets.only(right: 14),
              child: SizedBox(
                width: 17,
                height: 17,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            )
          else
            // ⚠️ `controller.text` ni TINGLASH kerak: matn
            // o'zgarganda Flutter bu vidjetni o'zi qayta qurmaydi.
            // Busiz tozalash tugmasi faqat tashqi `setState` bilan
            // (ya'ni 300 ms kechikish bilan) paydo bo'lardi.
            ValueListenableBuilder<TextEditingValue>(
              valueListenable: controller,
              builder: (_, value, __) => value.text.isEmpty
                  ? const SizedBox(width: 14)
                  : IconButton(
                      icon: const Icon(LucideIcons.x, size: 18),
                      color: const Color(0xFF71717A),
                      onPressed: onClear,
                      tooltip: 'Tozalash',
                    ),
            ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({required this.label, required this.icon, required this.onTap});

  final String label;
  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFFF4F4F5),
      borderRadius: BorderRadius.circular(18),
      child: InkWell(
        borderRadius: BorderRadius.circular(18),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 16, color: const Color(0xFF3F3F46)),
              const SizedBox(width: 6),
              Text(label,
                  style: const TextStyle(
                      fontSize: 14, fontWeight: FontWeight.w500)),
            ],
          ),
        ),
      ),
    );
  }
}
