import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';

/// MARSHRUT PANELI (bottom sheet).
///
/// ┌─ KO'RINISH `image/geologo/route.png` DAN ──────────────────────────┐
/// Yandex'ning marshrut ekrani tepadan pastgacha shunday tuzilgan:
///
///   1. sarlavha — orqaga tugmasi, «Marshrut», o'ngda amal tugmasi;
///   2. transport turlari qatori (tanlangani rangli kvadratda);
///   3. A maydoni — to'ldirilgan doira ichida strelka;
///   4. B maydoni — ichi bo'sh halqa; o'ngda almashtirish tugmasi;
///   5. natija — masofa, vaqt, yetib borish soati.
///
/// ┌─ NIMA CHIZILMAYDI ─────────────────────────────────────────────────┐
/// Rasmda transport turlari yettita: avtomobil, avtobus, taksi,
/// piyoda, velosiped, samokat, karshering. Bizning marshrutimiz —
/// OSRM'ning `driving` profili, ya'ni FAQAT avtomobil.
///
/// Qolganlari ko'rsatiladi, lekin O'CHIRILGAN holatda va bosilganda
/// sababi aytiladi. Ularni butunlay yashirish ham, ishlaydigandek
/// ko'rsatish ham noto'g'ri bo'lardi: birinchisi rasmdan uzoqlashtiradi,
/// ikkinchisi yolg'on va'da beradi.
/// └────────────────────────────────────────────────────────────────────┘

/// A va B belgilarining ranglari — saytdagi `RoutePanel` bilan bir xil.
const Color kRouteDotA = Color(0xFFFF4D3A);
const Color kRouteDotB = Color(0xFF3F3F46);

/// Transport turi.
enum RouteMode { car, bus, taxi, walk, bike, scooter, carshare }

const _modeIcons = {
  RouteMode.car: LucideIcons.carFront,
  RouteMode.bus: LucideIcons.bus,
  RouteMode.taxi: LucideIcons.carTaxiFront,
  RouteMode.walk: LucideIcons.footprints,
  RouteMode.bike: LucideIcons.bike,
  RouteMode.scooter: LucideIcons.bike,
  RouteMode.carshare: LucideIcons.car,
};

const _modeNames = {
  RouteMode.car: 'Avtomobil',
  RouteMode.bus: 'Jamoat transporti',
  RouteMode.taxi: 'Taksi',
  RouteMode.walk: 'Piyoda',
  RouteMode.bike: 'Velosiped',
  RouteMode.scooter: 'Samokat',
  RouteMode.carshare: 'Karshering',
};

class RouteSheet extends StatelessWidget {
  const RouteSheet({
    super.key,
    required this.onClose,
    required this.onClear,
    required this.onSwap,
    required this.onPickA,
    required this.onPickB,
    required this.onUnavailableMode,
    required this.labelA,
    required this.labelB,
    required this.routing,
    required this.summary,
    required this.arrival,
  });

  final VoidCallback onClose;
  final VoidCallback onClear;
  final VoidCallback onSwap;
  final VoidCallback onPickA;
  final VoidCallback onPickB;

  /// Ishlamaydigan transport turi bosilganda.
  final ValueChanged<String> onUnavailableMode;

  final String? labelA;
  final String? labelB;
  final bool routing;

  /// «1,7 km · 4 daq» — tayyor bo'lsa.
  final String? summary;

  /// «14:35 da» — yetib borish soati.
  final String? arrival;

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
      padding: EdgeInsets.only(bottom: pastki + 14),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 36,
            height: 4,
            margin: const EdgeInsets.only(top: 8, bottom: 6),
            decoration: BoxDecoration(
              color: const Color(0xFFD4D4D8),
              borderRadius: BorderRadius.circular(2),
            ),
          ),

          // ── Sarlavha ────────────────────────────────────────────
          Row(
            children: [
              IconButton(
                icon: const Icon(LucideIcons.arrowLeft, size: 21),
                onPressed: onClose,
                tooltip: 'Yopish',
              ),
              const Expanded(
                child: Text(
                  'Marshrut',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
                ),
              ),
              // Sarlavha markazda tursin: o'ngda ham tugma kengligi
              // qoldiriladi.
              SizedBox(
                width: 48,
                child: (labelA != null || labelB != null)
                    ? IconButton(
                        icon: const Icon(LucideIcons.eraser, size: 19),
                        onPressed: onClear,
                        tooltip: 'Tozalash',
                      )
                    : null,
              ),
            ],
          ),

          // ── Transport turlari ───────────────────────────────────
          SizedBox(
            height: 52,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 14),
              itemCount: RouteMode.values.length,
              separatorBuilder: (_, _) => const SizedBox(width: 10),
              itemBuilder: (_, i) {
                final m = RouteMode.values[i];
                final faol = m == RouteMode.car;
                return _ModeButton(
                  icon: _modeIcons[m]!,
                  selected: faol,
                  onTap: faol ? null : () => onUnavailableMode(_modeNames[m]!),
                );
              },
            ),
          ),
          const SizedBox(height: 6),

          // ── A / B ───────────────────────────────────────────────
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 4, 8, 0),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      _Field(
                        start: true,
                        color: kRouteDotA,
                        label: labelA,
                        hint: 'Qayerdan',
                        onTap: onPickA,
                      ),
                      const Divider(height: 1, indent: 30),
                      _Field(
                        start: false,
                        color: kRouteDotB,
                        label: labelB,
                        hint: 'Qayerga',
                        onTap: onPickB,
                      ),
                    ],
                  ),
                ),
                IconButton(
                  icon: const Icon(LucideIcons.arrowUpDown, size: 20),
                  color: const Color(0xFF71717A),
                  onPressed: (labelA != null || labelB != null) ? onSwap : null,
                  tooltip: 'Almashtirish',
                ),
              ],
            ),
          ),

          // ── Natija ──────────────────────────────────────────────
          if (routing)
            const Padding(
              padding: EdgeInsets.fromLTRB(16, 14, 16, 0),
              child: Row(
                children: [
                  SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2)),
                  SizedBox(width: 10),
                  Text('Marshrut hisoblanmoqda…',
                      style: TextStyle(color: Color(0xFF71717A))),
                ],
              ),
            )
          else if (summary != null)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 14, 16, 0),
              child: Row(
                children: [
                  const Icon(LucideIcons.carFront, size: 20, color: kRouteDotA),
                  const SizedBox(width: 10),
                  Text(summary!,
                      style: const TextStyle(
                          fontSize: 18, fontWeight: FontWeight.w800)),
                  const Spacer(),
                  if (arrival != null)
                    Text(arrival!,
                        style: const TextStyle(
                            fontSize: 13.5, color: Color(0xFF71717A))),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

/// Transport turi tugmasi — rasmda tanlangani ko'k kvadratda.
class _ModeButton extends StatelessWidget {
  const _ModeButton({
    required this.icon,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final bool selected;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected ? const Color(0xFF2563EB) : const Color(0xFFF4F4F5),
      borderRadius: BorderRadius.circular(13),
      child: InkWell(
        borderRadius: BorderRadius.circular(13),
        onTap: onTap,
        child: SizedBox(
          width: 52,
          height: 44,
          child: Icon(
            icon,
            size: 21,
            color: selected
                ? Colors.white
                // Ishlamaydigan turlar OCHIQ kulrang: ular bor,
                // lekin hozir tanlanmaydi.
                : const Color(0xFFA1A1AA),
          ),
        ),
      ),
    );
  }
}

/// A yoki B maydoni.
class _Field extends StatelessWidget {
  const _Field({
    required this.start,
    required this.color,
    required this.label,
    required this.hint,
    required this.onTap,
  });

  final bool start;
  final Color color;
  final String? label;
  final String hint;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final bor = label != null && label!.isNotEmpty;
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 14),
        child: Row(
          children: [
            if (start)
              Container(
                width: 20,
                height: 20,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
                child: const Icon(LucideIcons.navigation,
                    size: 11, color: Colors.white),
              )
            else
              Container(
                width: 20,
                height: 20,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  border: Border.all(color: color, width: 3.5),
                ),
              ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                bor ? label! : hint,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 16,
                  color: bor ? const Color(0xFF111827) : const Color(0xFF9E9E9E),
                  fontWeight: bor ? FontWeight.w600 : FontWeight.w400,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
