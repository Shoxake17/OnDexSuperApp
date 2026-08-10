import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';
import 'firebase_phone.dart';

/// OTP kodni yetkazish ZANJIRI (fallback).
///
/// ── UCH POG'ONA ────────────────────────────────────────────────────
///   1. **Telegram bot** — bepul. Telegram O'RNATILGAN bo'lsa va
///      backend botni sozlagan bo'lsa ishlatiladi.
///   2. **Firebase**     — pullik SMS, ilova tomonida tekshiriladi.
///   3. **Server SMS**   — mahalliy provayder (Eskiz), backend yuboradi.
///
/// Har pog'ona MUSTAQIL: biri sozlanmagan yoki ishlamasa, keyingisiga
/// o'tiladi. Foydalanuvchi buni sezmaydi — u faqat kod kutadi.
///
/// ── NEGA KANAL ESLAB QOLINADI ──────────────────────────────────────
/// Kodni TEKSHIRISH usuli kanalga bog'liq:
///   * Telegram va Server SMS — kod BIZNING `CodeStore` da, ya'ni
///     `POST /auth/verify` bilan tekshiriladi;
///   * Firebase — kod FIREBASE da, ya'ni `verificationId` bilan
///     tekshirilib, natijadagi ID token `POST /auth/firebase` ga
///     yuboriladi.
/// Shu sabab `OtpTicket` qaysi yo'l ishlatilganini olib yuradi —
/// aks holda tekshirish bosqichida noto'g'ri usul tanlanardi.
enum OtpChannel { telegram, firebase, serverSms }

class OtpTicket {
  final OtpChannel channel;

  /// Firebase uchun — kodni tekshirishda kerak.
  final String? verificationId;

  /// Telegram uchun — ochilgan havola (faqat ma'lumot uchun).
  final String? deepLink;

  const OtpTicket({required this.channel, this.verificationId, this.deepLink});

  bool get isFirebase => channel == OtpChannel.firebase;
}

/// Hech bir kanal ishlamaganda.
class OtpDeliveryFailure implements Exception {
  final String message;
  OtpDeliveryFailure(this.message);
  @override
  String toString() => message;
}

class OtpDelivery {
  OtpDelivery._();

  /// Zanjir bo'ylab yuborishga urinadi va ishlagan kanalni qaytaradi.
  static Future<OtpTicket> send(String phone) async {
    final problems = <String>[];

    // ---- 1) Telegram ----
    if (await _telegramInstalled()) {
      try {
        final link = await api.telegramStart(phone);
        final opened = await launchUrl(
          Uri.parse(link),
          mode: LaunchMode.externalApplication,
        );
        if (opened) {
          return OtpTicket(channel: OtpChannel.telegram, deepLink: link);
        }
        problems.add('Telegram ochilmadi');
      } on ApiException catch (e) {
        // Backend botni sozlamagan (503) yoki tarmoq muammosi —
        // keyingi pog'onaga o'tamiz.
        problems.add('Telegram: ${e.message}');
      } catch (e) {
        problems.add('Telegram: $e');
      }
    }

    // ---- 2) Firebase ----
    try {
      final vid = await FirebasePhoneAuth.sendCode(phone);
      return OtpTicket(channel: OtpChannel.firebase, verificationId: vid);
    } on PhoneAuthFailure catch (e) {
      problems.add('Firebase: ${e.message}');
    } catch (e) {
      problems.add('Firebase: $e');
    }

    // ---- 3) Server SMS (Eskiz) ----
    try {
      await api.requestCode(phone);
      return const OtpTicket(channel: OtpChannel.serverSms);
    } on ApiException catch (e) {
      problems.add('SMS: ${e.message}');
    } catch (e) {
      problems.add('SMS: $e');
    }

    // Hammasi qulagan. Foydalanuvchiga OXIRGI sababni ko'rsatamiz —
    // u eng yaqin va odatda eng tushunarli bo'ladi; qolganlari
    // tuzatish uchun log'ga chiqadi.
    debugPrint('OTP zanjiri qulaydi: ${problems.join(" | ")}');
    throw OtpDeliveryFailure(
      problems.isEmpty
          ? 'Kod yuborib bo\'lmadi'
          : problems.last.replaceFirst(RegExp(r'^[^:]+:\s*'), ''),
    );
  }

  /// Telegram qurilmada bormi.
  ///
  /// `tg://` sxemasi bo'yicha tekshiriladi — paket nomi bo'yicha emas,
  /// chunki Telegram'ning bir nechta paketi bor (rasmiy, web, forklar).
  ///
  /// MUHIM: Android 11+ da bu `AndroidManifest.xml` dagi `<queries>`
  /// blokiga bog'liq. Usiz javob HAR DOIM `false` bo'ladi va zanjir
  /// Telegram bor bo'lsa ham uni o'tkazib yuboradi.
  static Future<bool> _telegramInstalled() async {
    try {
      return await canLaunchUrl(Uri.parse('tg://resolve?domain=telegram'));
    } catch (_) {
      return false;
    }
  }
}
