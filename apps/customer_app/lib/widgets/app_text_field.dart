import 'package:flutter/material.dart';

/// Ilovadagi matn kiritish maydonlarining YAGONA ko'rinishi.
///
/// ┌─ NEGA BITTA VIDJET ───────────────────────────────────────────────┐
/// Ilgari har ekran o'z maydonini chizardi: restoran qidiruvi radius 12
/// va `#F7F7F7`, manzil qidiruvi radius 14 va mavzu rangi, "Podyezd" —
/// tagi chizilgan, kupon — konturli. Natijada bir xil vazifadagi
/// maydonlar turlicha ko'rinardi, tozalash (✕) tugmasi esa faqat bitta
/// joyda bor edi.
///
/// Endi o'lcham, rang, radius va xatti-harakat FAQAT shu yerda. Bir
/// joyda o'zgartirilsa — hamma ekranda o'zgaradi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Ko'rinish manbasi — bosh sahifadagi restoran qidiruvi (eng ko'p
/// ishlatiladigan va tasdiqlangan maket).
///
/// MUHIM: bu vidjet faqat KO'RINISHNI birlashtiradi. Har ekran o'z
/// vazifasini bajaraveradi — manzil qidiruvi manzil qidiradi, menyu
/// qidiruvi taom qidiradi. Filtrlash mantig'i bu yerga ko'chirilmagan.
class AppTextField extends StatefulWidget {
  const AppTextField({
    super.key,
    required this.controller,
    required this.hint,
    this.icon,
    this.autofocus = false,
    this.focusNode,
    this.onChanged,
    this.onSubmitted,
    this.onTap,
    this.readOnly = false,
    this.keyboardType,
    this.textInputAction,
    this.textCapitalization = TextCapitalization.none,
    this.showClear = true,
    this.trailing,
  });

  final TextEditingController controller;
  final String hint;

  /// Chapdagi ikon. Qidiruv maydonlarida `Icons.search`, oddiy
  /// maydonlarda (masalan "Podyezd") berilmaydi.
  final IconData? icon;

  final bool autofocus;
  final FocusNode? focusNode;
  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onSubmitted;

  /// Maydon tugma vazifasini bajarganda (`readOnly: true` bilan birga) —
  /// masalan bosilganda alohida qidiruv ekrani ochiladi.
  final VoidCallback? onTap;
  final bool readOnly;

  final TextInputType? keyboardType;
  final TextInputAction? textInputAction;
  final TextCapitalization textCapitalization;

  /// Matn kiritilganda o'ngda tozalash (✕) tugmasi ko'rinadi.
  /// `readOnly` maydonlarda hech qachon ko'rinmaydi.
  final bool showClear;

  /// Tozalash tugmasi o'rniga qo'yiladigan o'z vidjetingiz.
  final Widget? trailing;

  @override
  State<AppTextField> createState() => _AppTextFieldState();
}

class _AppTextFieldState extends State<AppTextField> {
  @override
  void initState() {
    super.initState();
    // Tozalash tugmasi matn bor-yo'qligiga qarab paydo bo'ladi. Ota
    // vidjetning `setState` iga tayanmaymiz — u har ekranda bo'lmasligi
    // mumkin, tugma esa hamma joyda bir xil ishlashi kerak.
    widget.controller.addListener(_onTextChanged);
  }

  @override
  void didUpdateWidget(AppTextField old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller) {
      old.controller.removeListener(_onTextChanged);
      widget.controller.addListener(_onTextChanged);
    }
  }

  @override
  void dispose() {
    // Kontroller EGASI bu vidjet emas — faqat obunani bekor qilamiz.
    widget.controller.removeListener(_onTextChanged);
    super.dispose();
  }

  void _onTextChanged() {
    if (mounted) setState(() {});
  }

  void _clear() {
    widget.controller.clear();
    widget.onChanged?.call('');
  }

  @override
  Widget build(BuildContext context) {
    final showClearButton = widget.showClear &&
        widget.trailing == null &&
        !widget.readOnly &&
        widget.controller.text.isNotEmpty;

    return Container(
      height: 42,
      padding: const EdgeInsets.symmetric(horizontal: 12),
      decoration: BoxDecoration(
        color: const Color(0xFFF7F7F7),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFFE0E0E0)),
      ),
      child: Row(
        children: [
          if (widget.icon != null) ...[
            Icon(widget.icon, size: 20, color: const Color(0xFF9E9E9E)),
            const SizedBox(width: 8),
          ],
          Expanded(
            child: TextField(
              controller: widget.controller,
              focusNode: widget.focusNode,
              autofocus: widget.autofocus,
              readOnly: widget.readOnly,
              onTap: widget.onTap,
              onChanged: widget.onChanged,
              onSubmitted: widget.onSubmitted,
              keyboardType: widget.keyboardType,
              textInputAction: widget.textInputAction,
              textCapitalization: widget.textCapitalization,
              style: const TextStyle(fontSize: 15, color: Color(0xFF1A1A1A)),
              decoration: InputDecoration(
                isDense: true,
                border: InputBorder.none,
                hintText: widget.hint,
                hintStyle: const TextStyle(
                    fontSize: 15, color: Color(0xFF9E9E9E)),
              ),
            ),
          ),
          if (widget.trailing != null) widget.trailing!,
          if (showClearButton)
            // `IconButton` emas: u o'zining kattaligi bilan 42px
            // balandlikni buzadi.
            GestureDetector(
              onTap: _clear,
              behavior: HitTestBehavior.opaque,
              child: const Padding(
                padding: EdgeInsets.only(left: 6),
                child: Icon(Icons.close, size: 18, color: Color(0xFF9E9E9E)),
              ),
            ),
        ],
      ),
    );
  }
}
