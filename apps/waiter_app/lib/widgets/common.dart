import 'package:flutter/material.dart';

import '../theme.dart';

/// Buyurtma holati belgisi.
///
/// Rang `statusStyle()` dan olinadi — ekranlar rangni O'ZI tanlamaydi,
/// aks holda bitta holat turli ekranlarda turlicha ko'rinib ketardi.
class StatusChip extends StatelessWidget {
  const StatusChip({super.key, required this.status, this.dense = false});

  final String status;
  final bool dense;

  @override
  Widget build(BuildContext context) {
    final s = statusStyle(status);
    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: dense ? 8 : 10,
        vertical: dense ? 3 : 5,
      ),
      decoration: BoxDecoration(
        color: s.bg,
        borderRadius: BorderRadius.circular(kRadiusChip),
      ),
      child: Text(
        s.label,
        style: TextStyle(
          color: s.color,
          fontSize: dense ? 11 : 12,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}

/// Bo'sh holat — ikonka, sarlavha, izoh.
///
/// `ListView` ichida ishlatilishi kerak (Center emas): `RefreshIndicator`
/// faqat SURILADIGAN vidjet ustida ishlaydi, aks holda bo'sh ekranda
/// qo'lda yangilash umuman mumkin bo'lmasdi.
class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.icon,
    required this.title,
    this.subtitle,
  });

  final IconData icon;
  final String title;
  final String? subtitle;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 80),
      child: Column(
        children: [
          Icon(icon, size: 56, color: kInkGhost),
          const SizedBox(height: 16),
          Text(
            title,
            textAlign: TextAlign.center,
            style: const TextStyle(
              color: kInkDim,
              fontSize: 16,
              fontWeight: FontWeight.w600,
            ),
          ),
          if (subtitle != null) ...[
            const SizedBox(height: 6),
            Text(
              subtitle!,
              textAlign: TextAlign.center,
              style: const TextStyle(color: kInkFaint, fontSize: 13, height: 1.4),
            ),
          ],
        ],
      ),
    );
  }
}

/// Ro'yxat ichidagi bo'lim sarlavhasi ("Tayyor", "Oshxonada").
class SectionHeader extends StatelessWidget {
  const SectionHeader({
    super.key,
    required this.title,
    this.count,
    this.color,
  });

  final String title;
  final int? count;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 20, 4, 10),
      child: Row(
        children: [
          Text(
            title.toUpperCase(),
            style: TextStyle(
              color: color ?? kInkFaint,
              fontSize: 12,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.8,
            ),
          ),
          if (count != null) ...[
            const SizedBox(width: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
              decoration: BoxDecoration(
                color: (color ?? kInkFaint).withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(20),
              ),
              child: Text(
                '$count',
                style: TextStyle(
                  color: color ?? kInkDim,
                  fontSize: 11,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// Xato + qayta urinish. Tarmoq uzilganda ro'yxat o'rniga ko'rinadi.
class ErrorRetry extends StatelessWidget {
  const ErrorRetry({super.key, required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 80),
      child: Column(
        children: [
          const Icon(Icons.cloud_off, size: 48, color: kInkGhost),
          const SizedBox(height: 16),
          Text(
            message,
            textAlign: TextAlign.center,
            style: const TextStyle(color: kInkDim, height: 1.4),
          ),
          const SizedBox(height: 20),
          OutlinedButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh, size: 18),
            label: const Text('Qaytadan urinish'),
          ),
        ],
      ),
    );
  }
}

/// Ulanish holati belgisi — AppBar'da.
///
/// Affitsiant ro'yxat "tirik"ligini bilishi kerak. Busiz uzilgan
/// ulanish jimgina eskirgan ma'lumot ko'rsatib turardi va odam eng
/// muhim xabarni ko'rmay qolardi.
class ConnectionDot extends StatelessWidget {
  const ConnectionDot({super.key, required this.online});

  final bool online;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: online ? 'Jonli ulanish faol' : 'Ulanish yo\'q — qayta urinilmoqda',
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: online
              ? kReadyColor.withValues(alpha: 0.14)
              : kWaitingColor.withValues(alpha: 0.16),
          borderRadius: BorderRadius.circular(20),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 7,
              height: 7,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: online ? kReadyColor : kWaitingColor,
              ),
            ),
            const SizedBox(width: 6),
            Text(
              online ? 'Jonli' : 'Oflayn',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: online ? kReadyColor : kWaitingColor,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
