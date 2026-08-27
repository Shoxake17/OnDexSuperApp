import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../widgets/empty_state.dart';
import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import 'catalog_screen.dart' show kBrand;
import 'tracking_screen.dart';

/// Bildirishnomalar — NATIVE.
///
/// Veb bilan parity: `apps/web/app/(food)/notifications/page.tsx`.
///
/// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
/// WebSocket xabari FAQAT ilova ochiq bo'lganda yetadi. Ilova yopiq,
/// tarmoq uzilgan yoki soket o'lik bo'lsa xabar yo'qolardi. Server
/// ularni bazaga yozadi — bu ekran esa o'sha tarixni ko'rsatadi.
/// └───────────────────────────────────────────────────────────────────┘
class NotificationsScreen extends StatefulWidget {
  const NotificationsScreen({super.key});

  @override
  State<NotificationsScreen> createState() => _NotificationsScreenState();
}

class _NotificationsScreenState extends State<NotificationsScreen> {
  List<Map<String, dynamic>>? _items;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _failed = false);
    try {
      final list = await api.notifications(limit: 50);
      if (!mounted) return;
      final items = list.cast<Map<String, dynamic>>();
      setState(() => _items = items);

      // O'qilgan deb belgilash FAQAT ro'yxat MUVAFFAQIYATLI
      // ko'rsatilgandan keyin. Aks holda so'rov yiqilgan holatda ham
      // hisoblagich nolga tushib, mijoz xabarlarni ko'rmay qolardi.
      if (items.any((n) => n['read_at'] == null)) {
        try {
          await api.markAllNotificationsRead();
        } catch (_) {
          // Belgilay olmasak ham ro'yxat ko'rinaveradi.
        }
      }
    } catch (_) {
      // 401 ham shu yerga tushadi — bu ekranga faqat kirgan holatda
      // kelinadi, shuning uchun "yuklab bo'lmadi" to'g'ri.
      if (mounted) setState(() => _failed = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
        child: Scaffold(
      backgroundColor: Colors.white,
      appBar: const PageAppBar(
        centerTitle: true,
        titleWidget: Text('Bildirishnomalar',
            style: TextStyle(fontSize: 19, fontWeight: FontWeight.bold)),
      ),
      body: RefreshIndicator(
        color: kBrand,
        onRefresh: _load,
        child: _buildBody(),
      ),
    ));
  }

  Widget _buildBody() {
    if (_failed) {
      return ErrorViewList(
        message: 'Bildirishnomalarni yuklab bo\'lmadi.',
        onRetry: _load,
      );
    }

    final items = _items;
    if (items == null) {
      // Skelet — vebdagi `animate-pulse` bloklari bilan bir xil o'lchov.
      return ListView.separated(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
        itemCount: 4,
        separatorBuilder: (_, __) => const SizedBox(height: 12),
        itemBuilder: (_, __) => Container(
          height: 104,
          decoration: BoxDecoration(
            color: const Color(0xFFF5F5F5),
            borderRadius: BorderRadius.circular(16),
          ),
        ),
      );
    }

    if (items.isEmpty) {
      return const EmptyStateList(
        image: 'assets/empty/notifications.png',
        title: 'Hozircha bu yer jimjit...',
        subtitle: 'Xafa bo\'lishga shoshilmang! Tez orada bu yerni ajoyib '
            'chegirmalar, qaynoq promokodlar va xushxabarlar bilan '
            'to\'ldiramiz.',
      );
    }

    return ListView.separated(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      itemCount: items.length,
      separatorBuilder: (_, __) => const SizedBox(height: 12),
      itemBuilder: (_, i) => _NotificationCard(n: items[i]),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// BITTA KARTA
// ═══════════════════════════════════════════════════════════════════

class _NotificationCard extends StatelessWidget {
  final Map<String, dynamic> n;
  const _NotificationCard({required this.n});

  @override
  Widget build(BuildContext context) {
    final look = _lookFor(n);
    final title = (n['title'] as String?) ?? '';
    final body = ((n['body'] as String?) ?? '').trim();
    final unread = n['read_at'] == null;
    final orderId = _safeOrderId(
        (n['data'] as Map?)?['order_id']?.toString());

    final card = Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: const Color(0xFFE5E5E5)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 52,
            height: 52,
            decoration: BoxDecoration(
              color: look.tile,
              borderRadius: BorderRadius.circular(16),
            ),
            child: Icon(look.icon, size: 24, color: look.iconColor),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Text(title,
                          style: const TextStyle(
                              fontWeight: FontWeight.bold, height: 1.3)),
                    ),
                    const SizedBox(width: 12),
                    Padding(
                      padding: const EdgeInsets.only(top: 1),
                      child: Text(
                        _relativeTime((n['created_at'] as String?) ?? ''),
                        style: const TextStyle(
                            fontSize: 12, color: Color(0xFF9E9E9E)),
                      ),
                    ),
                  ],
                ),
                if (body.isNotEmpty || unread)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.end,
                      children: [
                        Expanded(
                          child: Text(
                            body,
                            style: const TextStyle(
                                fontSize: 13.5,
                                height: 1.45,
                                color: Color(0xFF757575)),
                          ),
                        ),
                        // O'qilmaganlik belgisi. Ro'yxat ochilganda
                        // serverda hammasi o'qilgan deb belgilanadi,
                        // lekin BU ko'rinish o'zgarmaydi — mijoz nimani
                        // endi ko'rganini bilishi kerak.
                        if (unread)
                          Container(
                            margin: const EdgeInsets.only(left: 10, bottom: 4),
                            width: 8,
                            height: 8,
                            decoration: const BoxDecoration(
                              shape: BoxShape.circle,
                              color: kBrand,
                            ),
                          ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );

    // ┌─ BOSILADIGAN KARTA — FAQAT TEKSHIRILGAN ICHKI EKRANGA ────────┐
    // Server yuborgan URL HECH QACHON ishlatilmaydi. Faqat `order_id`
    // olinadi, u qat'iy shablon bilan tekshiriladi va ekran BIZ
    // tomonda ochiladi.
    //
    // NEGA: bildirishnoma mazmuni ma'lumotlar bazasidan keladi. Agar
    // manzil server maydonidan olinsa, u begona sxema yoki domen
    // bo'lishi mumkin edi.
    // └───────────────────────────────────────────────────────────────┘
    if (orderId == null) return card;
    return InkWell(
      borderRadius: BorderRadius.circular(16),
      onTap: () => Navigator.of(context).push(
        sheetRoute(TrackingScreen(orderId: orderId)),
      ),
      child: card,
    );
  }
}

/// `order_id` ni tekshiradi.
///
/// Qat'iy shablon ATAYLAB: ID — server generatsiya qilgan satr. Slash,
/// nuqta yoki foizli kodlash bo'lsa bu bizning ID emas.
String? _safeOrderId(String? raw) {
  if (raw == null || raw.isEmpty) return null;
  return RegExp(r'^[A-Za-z0-9_-]{1,64}$').hasMatch(raw) ? raw : null;
}

// ═══════════════════════════════════════════════════════════════════
// IKON VA RANG
// ═══════════════════════════════════════════════════════════════════

class _Look {
  final IconData icon;
  final Color tile;
  final Color iconColor;
  const _Look(this.icon, this.tile, this.iconColor);
}

// Ranglar YOPIQ jadvalda. Server qiymati (`kind`, `status`) hech qachon
// rangni O'ZI belgilamaydi — noma'lum qiymat neytral ko'rinishga
// tushadi va ekran buzilmaydi.
const _lookNew = _Look(Icons.shopping_bag_outlined, Color(0xFFFFF3E0), kBrand);
const _lookCooking =
    _Look(Icons.soup_kitchen_outlined, Color(0xFFFFF8E1), Color(0xFFD97706));
const _lookReady =
    _Look(Icons.restaurant, Color(0xFFE8F5E9), Color(0xFF059669));
const _lookOnWay =
    _Look(Icons.delivery_dining, Color(0xFFE8F5E9), Color(0xFF16A34A));
const _lookDone =
    _Look(Icons.inventory_2_outlined, Color(0xFFE3F2FD), Color(0xFF2563EB));
const _lookFailed =
    _Look(Icons.cancel_outlined, Color(0xFFFFEBEE), Color(0xFFDC2626));
const _lookPromo =
    _Look(Icons.local_activity_outlined, Color(0xFFFFFDE7), Color(0xFFCA8A04));
const _lookWallet = _Look(Icons.account_balance_wallet_outlined,
    Color(0xFFF3E5F5), Color(0xFF9333EA));
const _lookWaiting =
    _Look(Icons.schedule, Color(0xFFF5F5F5), Color(0xFF757575));
const _lookSystem =
    _Look(Icons.notifications_none, Color(0xFFF5F5F5), Color(0xFF757575));

_Look _lookFor(Map<String, dynamic> n) {
  final kind = (n['kind'] as String?) ?? '';
  if (kind == 'table_order_ready') return _lookReady;

  if (kind == 'order_status') {
    // Holatlar — `internal/orders/order.go` dagi `Status` ro'yxati.
    switch ((n['data'] as Map?)?['status']) {
      case 'created':
        return _lookWaiting;
      case 'accepted':
        return _lookNew;
      case 'preparing':
        return _lookCooking;
      case 'ready':
        return _lookReady;
      case 'picked_up':
        return _lookOnWay;
      case 'delivered':
      case 'served':
        return _lookDone;
      case 'rejected':
      case 'cancelled':
        return _lookFailed;
      default:
        return _lookSystem;
    }
  }

  // Kelajakdagi turlar (hozir backend ularni yubormaydi, lekin
  // qo'shilganda ekran tegmasdan to'g'ri ko'rinadi).
  if (kind == 'promotion') return _lookPromo;
  if (kind == 'wallet') return _lookWallet;

  return _lookSystem;
}

// ═══════════════════════════════════════════════════════════════════
// VAQT
// ═══════════════════════════════════════════════════════════════════

/// "2 daqiqa oldin", "2 soat oldin", "1 kun oldin".
///
/// Tayyor lokalizatsiya kutubxonasi ATAYLAB ishlatilmaydi: o'zbek tili
/// uchun natija platformaga qarab farq qiladi, ba'zilarida esa lokal
/// umuman yo'q. Matn mahsulot tilida bo'lishi kerak.
///
/// Veb tomondagi `relativeTime()` bilan bir xil — bu STEKLAR
/// ORASIDAGI dublikat (ruxsat etilgan), stek ichida emas.
String _relativeTime(String iso) {
  final t = DateTime.tryParse(iso);
  if (t == null) return '';

  final sec = DateTime.now().difference(t.toLocal()).inSeconds;
  // Kelajakdagi sana (server va telefon soati farq qilsa) — "hozir".
  if (sec < 60) return 'hozir';

  final min = sec ~/ 60;
  if (min < 60) return '$min daqiqa oldin';

  final hour = min ~/ 60;
  if (hour < 24) return '$hour soat oldin';

  final day = hour ~/ 24;
  if (day < 7) return '$day kun oldin';

  // Bir haftadan oshgach nisbiy vaqt ma'nosini yo'qotadi ("23 kun
  // oldin" hech narsa aytmaydi) — aniq sana ko'rsatiladi.
  final d = t.toLocal();
  String p2(int x) => x.toString().padLeft(2, '0');
  return '${p2(d.day)}.${p2(d.month)}.${d.year}';
}
