import 'package:flutter/material.dart';

import '../api.dart';
import '../services/firebase_phone.dart';
import '../services/otp_delivery.dart';
import '../session.dart';
import '../widgets/auth_flow.dart';
import '../widgets/auth_ui.dart';
import 'pin_screen.dart';

/// PIN kodni TIKLASH — hisobdan chiqmasdan.
///
/// ── OQIM ───────────────────────────────────────────────────────────
///   1. raqam SESSIYADAN olinadi (`GET /me`) — foydalanuvchi uni
///      kiritmaydi;
///   2. kod odatdagi zanjir bo'ylab yuboriladi: Telegram bot ->
///      Firebase -> server SMS (`OtpDelivery`);
///   3. kod tasdiqlanadi (`/auth/verify` yoki `/auth/firebase`);
///   4. yangi PIN yaratiladi.
///
/// ┌─ NEGA RAQAM SO'RALMAYDI ──────────────────────────────────────────┐
/// Raqamni foydalanuvchidan so'rash bu yerda XAVFLI bo'lardi: qulflangan
/// ilovani qo'lga kiritgan odam O'Z raqamini kiritib, o'z Telegramiga
/// kod oldirib, begona hisobga PIN qo'yib olardi. Raqam faqat AMALDAGI
/// SESSIYADAN olinadi va ekranda niqoblangan holda ko'rsatiladi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ── QOLGAN XAVF (ochiq aytilgan) ───────────────────────────────────
/// Telegram O'SHA telefonda bo'lsa, telefonni qo'lga kiritgan odam
/// kodni ham ko'ra oladi. Bu SMS bilan tiklashda ham xuddi shunday va
/// bank ilovalarida ham shu holat. Bunga qarshi yagona haqiqiy to'siq —
/// qurilmaning O'Z qulfi (PIN/barmoq izi), u esa OnDex ixtiyorida
/// emas. Shu sabab PIN'ni ko'p marta xato terish HAMON chiqishga olib
/// keladi (`AppPin.maxAttempts`) — tiklash yo'li uni yumshatmaydi.
class PinResetScreen extends StatefulWidget {
  const PinResetScreen({
    super.key,
    required this.onDone,
    required this.onCancel,
    this.notice,
  });

  /// Yangi PIN o'rnatilgandan keyin.
  final VoidCallback onDone;

  /// Bekor qilindi — chaqiruvchi qulf ekraniga qaytadi.
  final VoidCallback onCancel;

  /// Qo'shimcha tushuntirish (masalan PIN uzunligi o'zgargani).
  final String? notice;

  @override
  State<PinResetScreen> createState() => _PinResetScreenState();
}

class _PinResetScreenState extends State<PinResetScreen>
    with ResendTimerMixin<PinResetScreen> {
  final _otpKey = GlobalKey<OtpInputState>();

  String? _phone;
  String _code = '';
  bool _busy = false;
  bool _codeSent = false;
  bool _verified = false;
  String? _loadError;

  /// Firebase kanalida to'ladi.
  String? _verificationId;
  OtpChannel? _otpChannel;

  @override
  void initState() {
    super.initState();
    _loadPhone();
  }

  /// Raqamni SESSIYADAN olamiz (yuqoridagi izohga qarang).
  Future<void> _loadPhone() async {
    try {
      final me = await api.me();
      final phone = (me['phone'] as String?)?.trim() ?? '';
      if (!mounted) return;
      setState(() {
        _phone = phone.isEmpty ? null : phone;
        _loadError = phone.isEmpty
            ? 'Hisobingizga telefon raqami bog\'lanmagan — PIN kodni '
                'shu yo\'l bilan tiklab bo\'lmaydi.'
            : null;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _loadError = e.message);
    } catch (_) {
      if (!mounted) return;
      setState(() =>
          _loadError = 'Serverga ulanib bo\'lmadi — internetni tekshiring');
    }
  }

  /// "+998901234567" -> "+998 90 *** ** 67".
  ///
  /// To'liq raqamni ko'rsatmaymiz: qulflangan ekranda u begonaga
  /// keraksiz ma'lumot berardi. Oxirgi ikki raqam egasiga qaysi
  /// raqamligini eslatish uchun yetarli.
  String _masked(String p) {
    if (p.length < 9) return p;
    return '${p.substring(0, 4)} ${p.substring(4, 6)} *** ** '
        '${p.substring(p.length - 2)}';
  }

  Future<void> _sendCode() async {
    final phone = _phone;
    if (phone == null || _busy) return;
    setState(() => _busy = true);
    try {
      final ticket = await OtpDelivery.send(phone);
      if (!mounted) return;
      setState(() {
        _codeSent = true;
        _code = '';
        _verified = false;
        _verificationId = ticket.verificationId;
        _otpChannel = ticket.channel;
      });
      startResendTimer();
      _otpKey.currentState?.fill('');
      authSnack(
          context,
          ticket.channel == OtpChannel.telegram
              ? 'Kod Telegram botga yuborildi'
              : 'Kod yuborildi');
    } on OtpDeliveryFailure catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } on PhoneAuthFailure catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } on ApiException catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } catch (_) {
      if (mounted) {
        authSnack(context, 'Serverga ulanib bo\'lmadi — internetni tekshiring',
            error: true);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Kodni tasdiqlaydi. Muvaffaqiyatda yangi PIN yaratish ekraniga
  /// o'tadi.
  Future<void> _verify() async {
    if (_busy || _code.length != 6) return;
    setState(() => _busy = true);
    try {
      if (_otpChannel == OtpChannel.firebase) {
        final vid = _verificationId;
        if (vid == null) {
          if (mounted) {
            authSnack(context, 'Kod muddati tugadi — qayta yuboring',
                error: true);
          }
          return;
        }
        final idToken = await FirebasePhoneAuth.idTokenFor(vid, _code);
        await api.loginWithFirebase(idToken);
      } else {
        await api.verify(_phone!, _code);
      }
      // Tasdiqlash yangi token beradi — uni saqlaymiz, aks holda
      // eskisi bilan qolib ketardik.
      await tokenStore.write(api.token!);
      if (!mounted) return;
      setState(() => _verified = true);
    } on PhoneAuthFailure catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } on ApiException catch (e) {
      if (mounted) authSnack(context, e.message, error: true);
    } catch (_) {
      if (mounted) {
        authSnack(context, 'Serverga ulanib bo\'lmadi — internetni tekshiring',
            error: true);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    // Kod tasdiqlandi — endi yangi PIN.
    if (_verified) {
      return PinSetupScreen(onDone: widget.onDone);
    }

    final phone = _phone;
    return Scaffold(
      backgroundColor: authBg,
      appBar: AppBar(
        backgroundColor: authBg,
        foregroundColor: authText,
        elevation: 0,
        leading: BackButton(onPressed: widget.onCancel),
        title: const Text('PIN kodni tiklash',
            style: TextStyle(fontSize: 17, fontWeight: FontWeight.w600)),
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (_loadError != null) ...[
                Text(_loadError!,
                    style: const TextStyle(
                        color: Color(0xFFC62828), fontSize: 13.5)),
                const SizedBox(height: 16),
                AuthButton(
                  label: 'Orqaga',
                  busy: false,
                  onPressed: widget.onCancel,
                ),
              ] else if (phone == null) ...[
                const Center(child: CircularProgressIndicator()),
              ] else ...[
                if (widget.notice != null) ...[
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: authBrand.withValues(alpha: 0.08),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Text(widget.notice!,
                        style:
                            const TextStyle(fontSize: 13, color: authText)),
                  ),
                  const SizedBox(height: 16),
                ],
                Text(
                  'Tasdiqlash kodi ${_masked(phone)} raqamiga yuboriladi. '
                  'Telegram bot orqali keladi.',
                  style: const TextStyle(fontSize: 13.5, color: authMuted),
                ),
                const SizedBox(height: 20),
                AuthLabelled(
                  label: '6 xonali kodni kiriting',
                  child: OtpInput(
                    key: _otpKey,
                    autofocus: false,
                    onChanged: (v) => setState(() => _code = v),
                    onCompleted: (_) => _verify(),
                  ),
                ),
                const SizedBox(height: 12),
                ResendRow(
                  left: resendLeft,
                  clock: resendClock,
                  onResend: (_busy || resendLeft > 0) ? null : _sendCode,
                  idleLabel: _codeSent
                      ? 'Kodni qayta yuborish mumkin'
                      : 'Kod hali yuborilmadi',
                  actionLabel: _codeSent ? 'Kod qayta yuborish' : 'Kod yuborish',
                ),
                const SizedBox(height: 22),
                AuthButton(
                  label: 'Tasdiqlash',
                  busy: _busy,
                  onPressed: _verify,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
