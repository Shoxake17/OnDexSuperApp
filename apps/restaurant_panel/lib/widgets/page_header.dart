import 'package:flutter/material.dart';

import '../theme.dart';

/// Bo'lim sahifalarining UMUMIY karkasi.
///
/// ┌─ NEGA BU FAYL ────────────────────────────────────────────────────┐
/// Sarlavha bloki (nom + tavsif + asosiy amal tugmasi) va sahifa
/// chekinishi HAR BIR bo'limda bir xil bo'lishi kerak. Avval u har
/// sahifada ALOHIDA yozilgan edi va oqibati ko'rinib turardi:
///   * "Menyu" va "Aksiyalar" da bir xil `_Header` ikki nusxada
///     (yagona farq — matn va tugma yozuvi);
///   * "Xodimlar" va "Stollar" da esa umuman boshqacha, KICHIKROQ
///     sarlavha va CHEKINISHSIZ kontent — matn ekran chetiga yopishib
///     turardi.
///
/// Endi o'lchamlar shu yerda BIR MARTA belgilanadi; sahifa faqat matn
/// va kontentini beradi.
/// └───────────────────────────────────────────────────────────────────┘

/// Sahifa kontentining chetdan chekinishi — barcha bo'limlarda bir xil.
const kPagePadding = EdgeInsets.all(28);

/// Sarlavha bloki: nom + tavsif (chapda), amal tugmalari (o'ngda).
/// Tor ekranda tugma sarlavha OSTIGA tushadi.
class PageHeader extends StatelessWidget {
  final String title;
  final String subtitle;
  final Widget? action;

  /// Shu kenglikdan tor bo'lganda tugma pastga tushadi.
  final double narrowBelow;

  const PageHeader({
    super.key,
    required this.title,
    this.subtitle = '',
    this.action,
    this.narrowBelow = 760,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final titleBlock = Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(title,
              style: const TextStyle(
                  fontSize: 27,
                  fontWeight: FontWeight.w800,
                  color: OnDexColors.ink)),
          if (subtitle.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(subtitle,
                style:
                    const TextStyle(fontSize: 14, color: OnDexColors.inkDim)),
          ],
        ],
      );
      if (action == null) return titleBlock;
      if (c.maxWidth < narrowBelow) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [titleBlock, const SizedBox(height: 16), action!],
        );
      }
      return Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [Expanded(child: titleBlock), action!],
      );
    });
  }
}

/// Sahifaning asosiy amal tugmasi — standart FilledButton'dan ataylab
/// kattaroq (bo'limning bosh amali ko'zga yaqqol tashlanishi kerak).
class PageActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback? onPressed;

  const PageActionButton({
    super.key,
    required this.icon,
    required this.label,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) => FilledButton.icon(
        onPressed: onPressed,
        style: FilledButton.styleFrom(
          padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 16),
          textStyle:
              const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w700),
        ),
        icon: Icon(icon, size: 20),
        label: Text(label),
      );
}

/// RO'YXATLI sahifalar uchun to'liq qobiq: bir xil chekinish, sarlavha,
/// xato satri va qolgan balandlikni egallaydigan kontent.
///
/// Skroll qilinadigan (kartochkali) sahifalar buni ishlatmaydi — ular
/// `SingleChildScrollView(padding: kPagePadding)` ichida [PageHeader] ni
/// o'zi chizadi, chunki u yerda kontent balandligi cheksiz.
class PageScaffold extends StatelessWidget {
  final String title;
  final String subtitle;
  final Widget? action;

  /// Sarlavha ostida qizil rangda ko'rsatiladigan xato (null — yo'q).
  final String? error;

  /// Qolgan balandlikni egallaydi (odatda ListView).
  final Widget child;

  const PageScaffold({
    super.key,
    required this.title,
    this.subtitle = '',
    this.action,
    this.error,
    required this.child,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: kPagePadding,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          PageHeader(title: title, subtitle: subtitle, action: action),
          const SizedBox(height: 20),
          if (error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(error!,
                  style: const TextStyle(color: OnDexColors.danger)),
            ),
          Expanded(child: child),
        ],
      ),
    );
  }
}
