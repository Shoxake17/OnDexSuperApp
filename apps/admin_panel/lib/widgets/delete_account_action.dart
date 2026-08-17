import 'package:flutter/material.dart';

import '../api.dart';

/// Akkauntni o'chirish: tasdiq oynasi + so'rov + natija xabari.
///
/// ┌─ NEGA ALOHIDA ────────────────────────────────────────────────────┐
/// Bu amal UCH sahifada bor (mijozlar, affitsiantlar, kuryerlar) va u
/// ORTGA QAYTMAYDI. Har bir sahifa o'z dialogini yozganda ular asta
/// ajralib ketardi — masalan bir joyda ogohlantirish matni yangilanib,
/// boshqasida eski qolib ketishi mumkin edi. Xavfli amal uchun bu
/// qabul qilib bo'lmaydigan holat: admin qanday oqibat kutishini
/// sahifaga qarab boshqacha tushunardi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// `true` qaytsa — akkaunt o'chirildi va chaqiruvchi ro'yxatni
/// yangilashi kerak.
Future<bool> confirmDeleteAccount(
  BuildContext context, {
  required String id,
  required String name,
  String phone = '',
  /// Kuryer/affitsiant uchun qo'shimcha ogohlantirish (masalan
  /// "kuryer ro'yxatdan ham chiqariladi").
  String extraNote = '',
}) async {
  final confirmed = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Akkauntni o\'chirish'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('$name${phone.isEmpty ? '' : ' ($phone)'}',
              style: const TextStyle(fontWeight: FontWeight.bold)),
          const SizedBox(height: 12),
          const Text(
            'Akkaunt va unga bog\'langan shaxsiy ma\'lumotlar '
            '(sevimlilar, bildirishnomalar, push va qurilma yozuvlari) '
            'o\'chiriladi. Foydalanuvchi ilovada ochiq bo\'lsa — DARHOL '
            'chiqarib yuboriladi va qo\'lidagi token shu zahoti '
            'yaroqsiz bo\'ladi.',
          ),
          if (extraNote.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(extraNote),
          ],
          const SizedBox(height: 12),
          const Text(
            'Buyurtmalar tarixi hisobot uchun saqlanib qoladi.',
            style: TextStyle(fontSize: 12, color: Colors.black54),
          ),
          const SizedBox(height: 12),
          const Text('Bu amalni ortga qaytarib bo\'lmaydi.',
              style: TextStyle(fontWeight: FontWeight.bold)),
        ],
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Bekor qilish')),
        FilledButton(
          style: FilledButton.styleFrom(backgroundColor: Colors.red),
          onPressed: () => Navigator.pop(ctx, true),
          child: const Text('O\'chirish'),
        ),
      ],
    ),
  );
  if (confirmed != true || !context.mounted) return false;

  try {
    await api.deleteUser(id);
    if (!context.mounted) return true;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text('$name o\'chirildi')));
    return true;
  } catch (e) {
    if (!context.mounted) return false;
    // Server 409 bilan ANIQ sabab qaytaradi (masalan "bu kuryer hozir
    // buyurtma yetkazyapti") — o'sha matn o'zgarishsiz ko'rsatiladi,
    // umumiy "xatolik yuz berdi" bilan almashtirilmaydi.
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text('O\'chirilmadi: $e'),
      backgroundColor: Colors.red,
    ));
    return false;
  }
}
