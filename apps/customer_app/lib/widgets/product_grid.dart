import 'package:flutter/material.dart';

import '../api.dart';

/// "Istaklarim" yurak belgisi — oq doira, elevatsiyali; o'z holatini o'zi
/// boshqaradi: bosilganda DARHOL (optimistik) qizarib/oqarib, fon rejimida
/// serverga so'rov yuboradi. Xato bo'lsa (masalan tarmoq uzilsa) holat
/// ORQAGA qaytariladi — foydalanuvchi hech qachon soxta/yolg'on holat
/// ko'rmaydi.
class FavoriteButton extends StatefulWidget {
  final String productId;
  final bool initialFavorited;
  // onChanged — muvaffaqiyatli o'zgarishdan KEYIN chaqiriladi (masalan
  // "Istaklarim" sahifasi shu orqali mahsulotni ro'yxatdan olib tashlaydi).
  final ValueChanged<bool>? onChanged;
  final double size;

  const FavoriteButton({
    super.key,
    required this.productId,
    required this.initialFavorited,
    this.onChanged,
    this.size = 20,
  });

  @override
  State<FavoriteButton> createState() => _FavoriteButtonState();
}

class _FavoriteButtonState extends State<FavoriteButton> {
  late bool _favorited = widget.initialFavorited;
  bool _busy = false;

  @override
  void didUpdateWidget(covariant FavoriteButton old) {
    super.didUpdateWidget(old);
    if (old.productId != widget.productId ||
        old.initialFavorited != widget.initialFavorited) {
      _favorited = widget.initialFavorited;
    }
  }

  Future<void> _toggle() async {
    if (_busy) return;
    final next = !_favorited;
    setState(() {
      _favorited = next;
      _busy = true;
    });
    try {
      if (next) {
        await api.addFavorite(widget.productId);
      } else {
        await api.removeFavorite(widget.productId);
      }
      widget.onChanged?.call(next);
    } catch (_) {
      if (mounted) setState(() => _favorited = !next);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white,
      shape: const CircleBorder(),
      elevation: 3,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: _toggle,
        child: Padding(
          padding: const EdgeInsets.all(8),
          child: Icon(
            _favorited ? Icons.favorite : Icons.favorite_border,
            color: _favorited ? const Color(0xFFE53935) : Colors.black,
            size: widget.size,
          ),
        ),
      ),
    );
  }
}
