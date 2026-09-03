import 'package:flutter/material.dart';

import '../data/agent_driver.dart';

/// Shaddiy ilovani boshqarayotganini KO'RSATADIGAN qatlam.
///
/// ┌─ NEGA BU SHART ────────────────────────────────────────────────────┐
/// Ilova o'z-o'zidan ekran ochib, skroll qilib, tugma bosa boshlasa —
/// bu foydalanuvchi uchun nosozlik yoki buzib kirish bo'lib ko'rinadi.
/// Shuning uchun har amalning YONIDA kim qilayotgani, nima
/// qilayotgani va uni QANDAY TO'XTATISH yozilib turadi.
///
/// "Barmoq" ham bezak emas: u qaysi tugma bosilayotganini ko'rsatadi,
/// ya'ni foydalanuvchi keyin o'sha ishni o'zi qanday qilishni ham
/// o'rganadi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// `MaterialApp.builder` orqali BUTUN ilova ustiga qo'yiladi — aks
/// holda u faqat bitta ekranda ko'rinardi, boshqaruv esa ekranlar
/// orasida yuradi.
class AgentOverlay extends StatelessWidget {
  const AgentOverlay({super.key, required this.child});

  final Widget child;

  static const _brand = Color(0xFF5B5BF6);

  @override
  Widget build(BuildContext context) {
    final driver = AgentDriver.instance;
    return AnimatedBuilder(
      animation: driver,
      builder: (context, _) {
        if (!driver.running) return child;
        final media = MediaQuery.of(context);
        return Stack(
          children: [
            // ┌─ TEGISH JARAYONNI TO'XTATADI ──────────────────────────┐
            // `HitTestBehavior.translucent` — teginish OSTIDAGI
            // ekranga ham o'tadi, ya'ni foydalanuvchi boshqaruvni
            // to'xtatib, o'sha zahoti odatdagidek ishlay oladi.
            // Bosishni butunlay to'sish noto'g'ri bo'lardi: odam o'z
            // ilovasida qamalib qolmasligi kerak.
            // └────────────────────────────────────────────────────────┘
            Listener(
              behavior: HitTestBehavior.translucent,
              onPointerDown: (_) => driver.userTouched(),
              child: child,
            ),
            if (driver.pointer != null)
              _Finger(at: driver.pointer!, tapping: driver.tapping),
            Positioned(
              left: 12,
              right: 12,
              top: media.padding.top + 8,
              child: _StatusBar(
                text: driver.status,
                onStop: driver.cancel,
              ),
            ),
          ],
        );
      },
    );
  }
}

/// "Buyurtma rasmiylashtirish" tugmasini boshqaruvchiga ochadi.
///
/// ┌─ NEGA O'RAM VIDJET ────────────────────────────────────────────────┐
/// Tugma savat ekranida, uning amali esa narx hisoblangandagina
/// paydo bo'ladi (`onTap == null` — summa hali yo'q). Boshqaruvchi
/// aynan shuni kutishi kerak: tayyor bo'lmagan tugmani "bosish" hech
/// narsa qilmasdi va jarayon jimgina to'xtardi.
///
/// O'ram vidjet holatni AVTOMATIK kuzatadi: tugma paydo bo'lganda
/// ro'yxatga qo'shiladi, ekran yopilganda o'chiriladi. Qo'lda
/// `register`/`unregister` yozilsa, ulardan biri albatta unutilardi.
/// └────────────────────────────────────────────────────────────────────┘
class AgentCheckoutAnchor extends StatefulWidget {
  const AgentCheckoutAnchor({
    super.key,
    required this.onTap,
    required this.child,
  });

  final VoidCallback? onTap;
  final Widget child;

  @override
  State<AgentCheckoutAnchor> createState() => _AgentCheckoutAnchorState();
}

class _AgentCheckoutAnchorState extends State<AgentCheckoutAnchor> {
  final _key = GlobalKey();

  @override
  void initState() {
    super.initState();
    _sync();
  }

  @override
  void didUpdateWidget(AgentCheckoutAnchor old) {
    super.didUpdateWidget(old);
    if (old.onTap != widget.onTap) _sync();
  }

  void _sync() => AgentStage.instance.registerCartCheckout(widget.onTap, _key);

  @override
  void dispose() {
    AgentStage.instance.unregisterCartCheckout();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      KeyedSubtree(key: _key, child: widget.child);
}

/// Rasmiylashtirish ekranidagi "Buyurtma berish" tugmasini og'zaki
/// tasdiq uchun ochadi.
///
/// ┌─ NEGA ALOHIDA ANCHOR ──────────────────────────────────────────────┐
/// Savatdagi anchor (`AgentCheckoutAnchor`) foydalanuvchini to'lov
/// ekraniga OLIB BORADI — bu pul sarflamaydi. Bu esa PUL
/// SARFLAYDIGAN tugma, shuning uchun ular ataylab aralashtirilmagan:
/// bitta registrga qo'yilsa, boshqaruvchidagi kichik xato savat
/// tugmasi o'rniga buyurtma tugmasini bosib yuborishi mumkin edi.
/// └────────────────────────────────────────────────────────────────────┘
class AgentCheckoutConfirmAnchor extends StatefulWidget {
  const AgentCheckoutConfirmAnchor({
    super.key,
    required this.onCashConfirm,
    required this.child,
  });

  final VoidCallback? onCashConfirm;
  final Widget child;

  @override
  State<AgentCheckoutConfirmAnchor> createState() =>
      _AgentCheckoutConfirmAnchorState();
}

class _AgentCheckoutConfirmAnchorState
    extends State<AgentCheckoutConfirmAnchor> {
  final _key = GlobalKey();

  @override
  void initState() {
    super.initState();
    _sync();
  }

  @override
  void didUpdateWidget(AgentCheckoutConfirmAnchor old) {
    super.didUpdateWidget(old);
    if (old.onCashConfirm != widget.onCashConfirm) _sync();
  }

  void _sync() =>
      AgentStage.instance.registerCashConfirm(widget.onCashConfirm, _key);

  @override
  void dispose() {
    // Ekran yopilgach registr TOZALANADI: aks holda og'zaki "ha"
    // boshqa ekranda turgan foydalanuvchining buyurtmasini
    // berib yuborishi mumkin edi.
    AgentStage.instance.unregisterCashConfirm();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      KeyedSubtree(key: _key, child: widget.child);
}

class _StatusBar extends StatelessWidget {
  const _StatusBar({required this.text, required this.onStop});

  final String text;
  final VoidCallback onStop;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: Container(
        padding: const EdgeInsets.fromLTRB(14, 10, 8, 10),
        decoration: BoxDecoration(
          color: const Color(0xFF111827).withValues(alpha: 0.92),
          borderRadius: BorderRadius.circular(14),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.25),
              blurRadius: 16,
              offset: const Offset(0, 6),
            ),
          ],
        ),
        child: Row(
          children: [
            const CircleAvatar(
              radius: 13,
              backgroundImage: AssetImage('assets/shaddiy/face_idle.jpg'),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text(
                    'Shaddiy buyurtma qilyapti',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 12.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  if (text.isNotEmpty)
                    Text(
                      text,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Color(0xFFD1D5DB),
                        fontSize: 11.5,
                        height: 1.25,
                      ),
                    ),
                ],
              ),
            ),
            TextButton(
              onPressed: onStop,
              style: TextButton.styleFrom(
                foregroundColor: Colors.white,
                backgroundColor: const Color(0xFFEF4444),
                padding: const EdgeInsets.symmetric(horizontal: 12),
                minimumSize: const Size(0, 32),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(9),
                ),
              ),
              child: const Text('To\'xtatish',
                  style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
            ),
          ],
        ),
      ),
    );
  }
}

/// Virtual barmoq — bosilayotgan joyni ko'rsatadi.
class _Finger extends StatelessWidget {
  const _Finger({required this.at, required this.tapping});

  final Offset at;
  final bool tapping;

  static const _size = 44.0;

  @override
  Widget build(BuildContext context) {
    return AnimatedPositioned(
      duration: const Duration(milliseconds: 420),
      curve: Curves.easeOutCubic,
      left: at.dx - _size / 2,
      top: at.dy - _size / 2,
      child: IgnorePointer(
        child: AnimatedScale(
          duration: const Duration(milliseconds: 180),
          scale: tapping ? 0.7 : 1,
          child: Container(
            width: _size,
            height: _size,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: AgentOverlay._brand.withValues(alpha: 0.28),
              border: Border.all(
                color: AgentOverlay._brand.withValues(alpha: 0.85),
                width: 2,
              ),
              boxShadow: [
                BoxShadow(
                  color: AgentOverlay._brand.withValues(alpha: 0.45),
                  blurRadius: tapping ? 22 : 12,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
