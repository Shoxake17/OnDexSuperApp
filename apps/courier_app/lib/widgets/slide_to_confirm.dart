import 'package:flutter/material.dart';

/// "Chapdan o'ngga sudrab" tasdiqlash — Yandex/Uber uslubidagi tasodifiy
/// bosishning oldini oluvchi harakat (foydalanuvchi so'rovi bo'yicha
/// oddiy tugma o'rniga). Tugma bosilishi shart emas — dumaloq tutqichni
/// yo'lakning oxirigacha (`_threshold`) surish kerak, aks holda tutqich
/// avtomatik boshiga qaytadi.
class SlideToConfirm extends StatefulWidget {
  final String label;
  final bool busy;
  final Future<bool> Function() onConfirm;

  const SlideToConfirm({
    super.key,
    required this.label,
    required this.busy,
    required this.onConfirm,
  });

  @override
  State<SlideToConfirm> createState() => _SlideToConfirmState();
}

class _SlideToConfirmState extends State<SlideToConfirm> {
  static const _thumbSize = 48.0;
  static const _threshold = 0.85;

  double _fraction = 0; // 0..1 — tutqichning yo'lak bo'ylab joriy holati
  bool _dragging = false;
  bool _submitting = false;

  double _maxDrag(double trackWidth) => trackWidth - _thumbSize - 8;

  void _reset() {
    _submitting = false;
    _dragging = false;
    _fraction = 0;
  }

  // ┌─ NEGA KERAK (2026-09-15, lokal sinovda topilgan) ───────────────────┐
  // Kuryer ekrani bosqich o'tgach AYNAN o'sha joyga yana `SlideToConfirm`
  // qo'yadi ("Yetib keldim" -> "Buyurtma olindi" -> "Mijozga yetkazdim").
  // Widget turi va joyi bir xil bo'lgani uchun Flutter shu holat obyektini
  // QAYTA ISHLATADI. Holat tozalanmasdi — `_submitting` true qolib, keyingi
  // tugma abadiy aylanardi va surib bo'lmasdi; faqat ilovani qayta ochish
  // (yangi holat) yordam berardi. Yozuv o'zgarsa — bu yangi bosqich.
  // └─────────────────────────────────────────────────────────────────────┘
  @override
  void didUpdateWidget(covariant SlideToConfirm oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.label != widget.label) _reset();
  }

  Future<void> _finishDrag(double trackWidth) async {
    setState(() => _dragging = false);
    if (_fraction < _threshold) {
      setState(() => _fraction = 0);
      return;
    }
    setState(() => _submitting = true);
    await widget.onConfirm();
    if (!mounted) return;
    // Natijadan qat'i nazar holat TOZALANADI: `false` — server rad etdi,
    // kuryer qayta urinadi; `true` — ota-widget keyingi bosqichni ko'rsatadi
    // va u shu holat obyektini qayta ishlatishi mumkin (yuqoridagi izoh).
    // Ota o'zining `busy` holati bilan so'rov davomida bloklab turadi.
    setState(_reset);
  }

  @override
  Widget build(BuildContext context) {
    final busy = widget.busy || _submitting;
    final color = Theme.of(context).colorScheme.primary;
    return LayoutBuilder(
      builder: (context, constraints) {
        final maxDrag = _maxDrag(constraints.maxWidth);
        return Container(
          height: 56,
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(28),
          ),
          child: Stack(
            alignment: Alignment.centerLeft,
            children: [
              Center(
                child: Opacity(
                  opacity: (1 - _fraction * 1.6).clamp(0.0, 1.0),
                  child: Text(
                    '${widget.label}   »',
                    style: TextStyle(
                      color: color,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ),
              AnimatedPositioned(
                duration: _dragging
                    ? Duration.zero
                    : const Duration(milliseconds: 220),
                curve: Curves.easeOut,
                left: 4 + _fraction * maxDrag,
                top: 4,
                child: GestureDetector(
                  onHorizontalDragStart: busy
                      ? null
                      : (_) => setState(() => _dragging = true),
                  onHorizontalDragUpdate: busy
                      ? null
                      : (details) {
                          setState(() {
                            _fraction = (_fraction + details.delta.dx / maxDrag)
                                .clamp(0.0, 1.0);
                          });
                        },
                  onHorizontalDragEnd: busy
                      ? null
                      : (_) => _finishDrag(constraints.maxWidth),
                  child: Container(
                    width: _thumbSize,
                    height: _thumbSize,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: color,
                    ),
                    child: Center(
                      child: busy
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: Colors.white,
                              ),
                            )
                          : const Icon(
                              Icons.arrow_forward,
                              color: Colors.white,
                            ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
