import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../theme.dart';

/// Aloqa havolasini ochadi (`tel:`, `mailto:`, `https://t.me/`).
///
/// Havola `ondex_support` da oq ro'yxat bo'yicha QURILGAN — bu yerga
/// boshqa sxema kelmaydi, lekin baribir tekshiriladi. Windows'da
/// qo'ng'iroq dasturi bo'lmasligi mumkin: ochilmasa qiymat nusxalanadi
/// va foydalanuvchiga aytiladi (jim "hech narsa bo'lmadi" o'rniga).
Future<void> openSupportLink(
  BuildContext context,
  Uri? uri, {
  required String copyValue,
  String missing = 'Bu aloqa ma\'lumoti hali kiritilmagan',
}) async {
  final messenger = ScaffoldMessenger.maybeOf(context);
  if (uri == null || !const {'tel', 'mailto', 'https'}.contains(uri.scheme)) {
    messenger?.showSnackBar(SnackBar(content: Text(missing)));
    return;
  }
  var opened = false;
  try {
    opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
  } catch (_) {
    opened = false;
  }
  if (opened) return;
  await Clipboard.setData(ClipboardData(text: copyValue));
  messenger?.showSnackBar(SnackBar(content: Text('Ochib bo\'lmadi — nusxa olindi: $copyValue')));
}

/// "Biz bilan bog'laning" qatori: rangli doira, matn va o'ngda tugma.
class SupportContactRow extends StatelessWidget {
  const SupportContactRow({
    super.key,
    required this.icon,
    required this.color,
    required this.title,
    required this.subtitle,
    required this.actionLabel,
    required this.onAction,
  });

  final IconData icon;
  final Color color;
  final String title;
  final String subtitle;
  final String actionLabel;
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 12, 12, 12),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            child: Icon(icon, color: Colors.white, size: 20),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                if (subtitle.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                ],
              ],
            ),
          ),
          const SizedBox(width: 8),
          OutlinedButton(
            onPressed: onAction,
            style: OutlinedButton.styleFrom(
              foregroundColor: OnDexColors.ink,
              side: const BorderSide(color: OnDexColors.cardBorder),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              textStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
            ),
            child: Text(actionLabel),
          ),
        ],
      ),
    );
  }
}

/// OnDex qo'llab-quvvatlash avatari (brend logotipi).
class SupportAvatar extends StatelessWidget {
  const SupportAvatar({super.key, this.size = 40});

  final double size;

  @override
  Widget build(BuildContext context) {
    return ClipOval(
      child: Image.asset(
        'assets/ondex.png',
        width: size,
        height: size,
        fit: BoxFit.cover,
        filterQuality: FilterQuality.medium,
        errorBuilder: (_, __, ___) => Container(
          width: size,
          height: size,
          color: OnDexColors.primary,
          child: Icon(Icons.headset_mic_rounded, color: Colors.white, size: size * 0.5),
        ),
      ),
    );
  }
}

/// Yashil/kulrang nuqta + "Online"/"Offline".
class SupportOnlineLabel extends StatelessWidget {
  const SupportOnlineLabel({super.key, required this.online, this.fontSize = 12});

  final bool online;
  final double fontSize;

  @override
  Widget build(BuildContext context) {
    final color = online ? OnDexColors.success : OnDexColors.inkFaint;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(width: 8, height: 8, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
        const SizedBox(width: 5),
        Text(online ? 'Online' : 'Offline',
            style: TextStyle(fontSize: fontSize, color: color, fontWeight: FontWeight.w600)),
      ],
    );
  }
}
