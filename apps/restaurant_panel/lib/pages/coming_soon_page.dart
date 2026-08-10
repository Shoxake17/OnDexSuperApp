import 'package:flutter/material.dart';

import '../theme.dart';

/// Hali qurilmagan bo'limlar (Statistika, Sozlamalar) uchun — sidebar
/// navigatsiyasi TO'LIQ ko'rinishi uchun kerak, lekin ICHIDA soxta
/// ma'lumot/funksiya ko'rsatilmaydi, faqat halol "hali tayyor emas" holati.
class ComingSoonPage extends StatelessWidget {
  final String title;
  final IconData icon;
  final String description;

  const ComingSoonPage({
    super.key,
    required this.title,
    required this.icon,
    required this.description,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              color: OnDexColors.primaryTint,
              borderRadius: BorderRadius.circular(20),
            ),
            child: Icon(icon, size: 32, color: OnDexColors.primary),
          ),
          const SizedBox(height: 20),
          Text(title,
              style: const TextStyle(
                  fontSize: 20, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          const SizedBox(height: 8),
          SizedBox(
            width: 340,
            child: Text(
              description,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 13.5, color: OnDexColors.inkDim, height: 1.5),
            ),
          ),
        ],
      ),
    );
  }
}
