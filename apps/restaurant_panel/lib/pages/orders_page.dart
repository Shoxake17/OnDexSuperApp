import 'dart:async';

import 'package:flutter/material.dart';

// `isTerminalStatus` — `ondex_core` dan, `api.dart` orqali
// (u butun paketni qayta eksport qiladi).
import '../api.dart';
import '../live.dart';
import '../panel_prefs.dart';
import '../sound.dart';
import '../widgets/courier_status_box.dart';
import '../widgets/order_card_header.dart';
import '../widgets/page_header.dart';
import '../theme.dart';

const _preparingStatuses = {'accepted', 'preparing'};

// ┌─ TUZATILGAN NOSOZLIK (bug.md 78-band) ─────────────────────────────┐
// Bu yerda AVVAL o'z nusxasi turardi:
//
//	const _terminalStatuses = {'delivered', 'rejected', 'cancelled'};
//
// `served` — STOL buyurtmasining yakuniy holati — ro'yxatda YO'Q edi.
// Natijada berilgan stol buyurtmasi:
//   1. "faol" deb hisoblanardi va "Jami N ta buyurtma" ga qo'shilardi;
//   2. hech qaysi ustunga tushmasdi (`_byStatus` ning to'rt to'plamiga
//      mos kelmaydi);
//   3. tarixga ham tushmasdi.
// Ya'ni hisoblagich har stol buyurtmasidan keyin ekrandagi kartalar
// sonidan uzoqlashib borardi va restoran zal savdosini tarixdan
// umuman topa olmasdi.
//
// Endi ro'yxat `packages/ondex_core` dagi YAGONA manbadan
// (`isTerminalStatus`) olinadi — u backend'ning
// `orders.TerminalStatusStrings()` ro'yxatiga mos.
// └────────────────────────────────────────────────────────────────────┘

enum _TabFilter { all, yangi, tayyorlanmoqda, tayyor, kuryerda }

enum _ViewMode { kanban, list }

/// Buyurtmalar sahifasi — image/buyurtma.png namunasiga mos, Kanban-uslubidagi
/// to'rt ustunli taxta (Yangi/Tayyorlanmoqda/Tayyor/Kuryerda), qidiruv,
/// "Tarix" tugmasi va grid/ro'yxat almashtirgichi bilan.
/// Yangi buyurtma WebSocket orqali JONLI tushadi.
class OrdersPage extends StatefulWidget {
  /// "Tarix" — to'liq buyurtmalar tarixi (Statistika → "Barcha
  /// buyurtmalar", davr filtri bilan). `null` bo'lsa tugma ko'rinmaydi.
  final VoidCallback? onOpenHistory;

  const OrdersPage({super.key, this.onOpenHistory});

  @override
  State<OrdersPage> createState() => _OrdersPageState();
}

class _OrdersPageState extends State<OrdersPage> {
  List<Map<String, dynamic>> _orders = [];
  bool _loading = true;
  late final LiveRefresher _live;
  StreamSubscription<Map<String, dynamic>>? _uiSub;
  Timer? _tickTimer;

  final _searchCtrl = TextEditingController();
  String _search = '';
  _TabFilter _tab = _TabFilter.all;
  _ViewMode _view = _ViewMode.kanban;

  @override
  void initState() {
    super.initState();
    _load();
    // Jonli kanal — butun panelga UMUMIY (`lib/live.dart`). Avval bu
    // sahifa o'z soketini ochardi va yon paneldagi hisoblagich alohida
    // so'rov sikliga tayanardi; endi ikkalasi bitta ulanishdan.
    //
    // So'rov sikli o'chirilmadi, ZAXIRA bo'lib qoldi: soket ulangan
    // bo'lsa siyrak (60s), uzilgan bo'lsa tez-tez (15s).
    _live = LiveRefresher(
      bus: restaurantLive,
      onRefresh: _load,
      types: const {
        'new_order',
        'order_status',
        'courier_assigned',
        'dispatch_failed',
        // Kuryer qidiruvi holati: "Kuryer topilmadi" tugmalari va qayta
        // qidirish boshqa xodim kompyuterida ham DARHOL ko'rinsin.
        'courier_not_found',
        'dispatch_state',
      },
      offlineInterval: const Duration(seconds: 15),
    )..start();
    _uiSub = restaurantLive.events.listen(_showEventNotice);
    // Faqat VIZUAL yangilanish (tayyorlash/yetkazish hisoblagichlari) —
    // tarmoq so'rovi yubormaydi, shunchaki matnni qayta chizadi. Courier
    // ilovasidagi countdown bilan bir xil naqsh (1s emas, 10s — bu yerda
    // sekundiga qadar aniqlik shart emas, ortiqcha qayta chizishdan qochish
    // uchun).
    _tickTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      if (mounted) setState(() {});
    });
    _searchCtrl.addListener(() {
      setState(() => _search = _searchCtrl.text.trim().toLowerCase());
    });
  }

  @override
  void dispose() {
    _live.dispose();
    _uiSub?.cancel();
    _tickTimer?.cancel();
    _searchCtrl.dispose();
    super.dispose();
  }

  /// Yandex Eats uslubida: yangi (hali qabul/rad qilinmagan) buyurtma bo'lsa
  /// ovoz TO'XTOVSIZ jiringlayveradi, restoran qabul yoki rad qilishi bilan
  /// darhol o'chadi.
  ///
  /// Holat bu yerda SAQLANMAYDI — u `RingSound` ichida
  /// (`sound.dart` dagi `setPending` izohiga qarang). Bu chaqiruv
  /// shunchaki TEZLIK uchun: restoran tugmani bosishi bilan ovoz
  /// o'chadi, soket xabari qaytishi kutilmaydi. Sahifa yopiq bo'lganda
  /// ayni ishni qobiq (`screens/shell.dart`) bajaradi.
  Future<void> _updateRinging() =>
      RingSound.setPending(_orders.any((o) => o['status'] == 'created'));

  /// Hodisa bo'yicha XABAR ko'rsatish (ro'yxatni yangilash bilan
  /// `LiveRefresher` shug'ullanadi).
  ///
  /// Qayta ulanish mantiqi bu yerda YO'Q — u `LiveBus` ichida, bitta
  /// joyda (eksponensial backoff + jitter bilan). Avval har bir ekran
  /// o'z qayta ulanishini yozardi va ular bir xil emasdi.
  void _showEventNotice(Map<String, dynamic> e) {
    if (!mounted) return;
    switch (e['type']) {
      case 'new_order':
        // "Restoran sozlamalari" → "Bildirishnomalar" (shu kompyuter uchun).
        if (!PanelPrefs.newOrderBanner.value) return;
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(
              'YANGI BUYURTMA! ${formatSum((e['total_tiyin'] ?? 0) as int)}'),
          backgroundColor: Colors.green,
        ));
      case 'courier_not_found':
        // Taom tayyor, lekin belgilangan muddatda kuryer topilmadi —
        // restoran qaror qilishi kerak. Tugmalar kartochkada; bu xabar
        // faqat e'tiborni tortadi (xodim boshqa ustunga qarab turgan
        // bo'lishi mumkin).
        if (!PanelPrefs.courierAlerts.value) return;
        final number = shortOrderNumber(e);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(
              '$number buyurtmaga kuryer topilmadi — bekor qiling yoki kuryerni qayta qidiring.'),
          backgroundColor: OnDexColors.danger,
          duration: const Duration(seconds: 8),
        ));
      case 'dispatch_failed':
        // Bu xabar faqat haqiqiy infratuzilma xatosida keladi (masalan
        // server ichki xatosi), oddiy "hozircha kuryer yo'q" holatida
        // EMAS — u uchun `courier_not_found` bor. To'xtab qolgan
        // qidiruvni server nazoratchisi o'zi qayta boshlaydi.
        if (!PanelPrefs.courierAlerts.value) return;
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text(
              'Kuryer qidirishda kutilmagan xato yuz berdi — server jurnalini tekshiring.'),
          backgroundColor: Colors.red,
          duration: Duration(seconds: 6),
        ));
    }
  }

  Future<void> _load() async {
    try {
      final l = await api.orders();
      if (!mounted) return;
      setState(() {
        _orders = l.cast<Map<String, dynamic>>();
        _loading = false;
      });
      _updateRinging();
    } catch (_) {
      if (!mounted) return;
      setState(() => _loading = false);
    }
  }

  /// Buyurtma holatini o'zgartiradigan YAGONA nuqta.
  ///
  /// `label` faqat tahlil uchun: superadmin PostHog'da "bu restoran
  /// buyurtmani qabul qildimi, rad etdimi, qancha vaqtdan keyin tayyor
  /// dedi" degan savolga javob topa olishi kerak. Amalning O'ZIGA
  /// ta'sir qilmaydi.
  Future<void> _do(String orderId, Future<void> Function() action,
      {String? label}) async {
    if (_busy.contains(orderId)) return;
    setState(() => _busy.add(orderId));
    try {
      await action();
      if (label != null) {
        Analytics.instance.capture('buyurtma_amali', {
          'amal': label,
          // Buyurtma ID si — PostHog'da hodisani haqiqiy buyurtma bilan
          // solishtirish uchun. Shaxsiy ma'lumot emas.
          'buyurtma': orderId,
        });
      }
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Xato: ${e.message}')));
      }
    } finally {
      if (mounted) setState(() => _busy.remove(orderId));
    }
    // Xatoda ham yangilanadi: 409 — holat shu oraliqda o'zgargan (boshqa
    // xodim bosdi, kuryer topildi), kartochka eskirgan holatda qolmasin.
    await _load();
  }

  /// Hozir serverga amal yuborilayotgan buyurtmalar — kartochka
  /// tugmalari shu vaqtda o'chiq (ikki marta bosish ikkita so'rov
  /// yubormasin).
  final Set<String> _busy = {};

  /// "Kuryer topilmadi" → "Bekor qilish". Tasdiq so'raladi: amalni
  /// qaytarib bo'lmaydi, mijozga xabar boradi, karta to'lovi qaytariladi.
  ///
  /// Server bekor qilishni FAQAT "kuryer topilmadi" holatida qabul
  /// qiladi — oraliqda kuryer topilgan bo'lsa 409 keladi va ro'yxat
  /// yangilanadi.
  Future<void> _cancelNoCourier(Map<String, dynamic> order) async {
    final id = order['id'] as String;
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('${shortOrderNumber(order)} bekor qilinsinmi?'),
        content: const Text(
            'Kuryer topilmadi. Buyurtma bekor qilinadi, mijozga xabar boradi, '
            'karta orqali to\'langan bo\'lsa pul qaytariladi.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Yo\'q')),
          FilledButton(
              style: FilledButton.styleFrom(backgroundColor: OnDexColors.danger),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Ha, bekor qilish')),
        ],
      ),
    );
    if (yes != true) return;
    await _do(id, () => api.transition(id, 'cancelled'),
        label: 'kuryer_topilmadi_bekor');
  }

  /// "Kuryer topilmadi" → "Kuryer qidirish": server qidiruvni yangi
  /// muddat bilan qaytadan boshlaydi.
  Future<void> _retryCourier(String id) => _do(
      id, () => api.retryCourierSearch(id),
      label: 'kuryer_qayta_qidirish');

  /// Qabul qilish — taxminiy tayyorlash vaqtini so'raydi va shu ZAHOTI
  /// kuryer qidirishni AVTOMATIK boshlaydi (qo'lda "Kuryer chaqirish"
  /// tugmasisiz). Backend ETA-asoslangan matching engine orqali kuryerning
  /// restoranga yetib borish vaqtini AYNAN shu tayyorlash vaqtiga
  /// moslashtiradi — na erta (bekorga kutmasin), na kech (ovqat sovimasin).
  Future<void> _accept(String id) async {
    final minutes = await _askPreparationMinutes();
    if (minutes == null) return; // xodim bekor qildi
    await _do(
        id, () => api.transition(id, 'accepted', preparationMinutes: minutes),
        label: 'qabul_qildi');
  }

  /// Restoran "Qabul qilish" bosganda taxminiy tayyorlash vaqtini (daqiqada)
  /// so'raydi — bekor qilinsa `null` qaytadi.
  Future<int?> _askPreparationMinutes() {
    final controller = TextEditingController(text: '20');
    return showDialog<int>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Taxminiy tayyorlash vaqti'),
        content: TextField(
          controller: controller,
          keyboardType: TextInputType.number,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: 'Necha daqiqada tayyor bo\'ladi?',
            suffixText: 'daqiqa',
            border: OutlineInputBorder(),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('Bekor qilish'),
          ),
          FilledButton(
            onPressed: () {
              final v = int.tryParse(controller.text.trim());
              if (v == null || v <= 0) return;
              Navigator.of(ctx).pop(v);
            },
            child: const Text('Qabul qilish'),
          ),
        ],
      ),
    );
  }

  Future<void> _reject(String id) async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Buyurtma rad etilsinmi?'),
        content:
            const Text('Mijozga buyurtma rad etilgani haqida xabar boradi.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Yo\'q')),
          FilledButton(
              style: FilledButton.styleFrom(backgroundColor: Colors.red),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Ha, rad etish')),
        ],
      ),
    );
    if (yes == true) {
      await _do(id, () => api.transition(id, 'rejected'), label: 'rad_etdi');
    }
  }

  // ---------------------------------------------------------------------
  // Filtrlash
  // ---------------------------------------------------------------------

  bool _matchesSearch(Map<String, dynamic> o) {
    if (_search.isEmpty) return true;
    final number = (o['order_number']?.toString() ?? '').toLowerCase();
    return number.contains(_search);
  }

  // MUHIM: faol buyurtmalar sana bo'yicha FILTRLANMAYDI — ular haqiqatan
  // ham "joriy" (masalan kecha qabul qilingan-u hali yetkazilmagan
  // buyurtma bo'lishi mumkin).
  //
  // Yakunlangan buyurtmalar "Tarix" da — Statistika → "Barcha
  // buyurtmalar" (`onOpenHistory`). Avval tarix shu sahifada
  // `api.orders()` dan ko'rsatilardi: u faqat oxirgi 100 ta buyurtmani
  // beradi, ya'ni eski kunlar jimgina BO'SH chiqardi.
  List<Map<String, dynamic>> get _activeOrders => _orders
      .where((o) => !isTerminalStatus((o['status'] ?? '').toString()))
      .toList();

  List<Map<String, dynamic>> _byStatus(Set<String> statuses) => _activeOrders
      .where((o) => statuses.contains(o['status']))
      .where(_matchesSearch)
      .toList()
    ..sort((a, b) {
      final da = _parseAt(a['created_at']) ?? DateTime(0);
      final db = _parseAt(b['created_at']) ?? DateTime(0);
      return db.compareTo(da);
    });

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    final yangi = _byStatus({'created'});
    final tayyorlanmoqda = _byStatus(_preparingStatuses);
    final tayyor = _byStatus({'ready'});
    final kuryerda = _byStatus({'picked_up'});
    final totalActive = _activeOrders.length;

    return RefreshIndicator(
      onRefresh: _load,
      child: SingleChildScrollView(
        padding: kPagePadding,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _Header(
              totalCount: totalActive,
              searchCtrl: _searchCtrl,
              onOpenHistory: widget.onOpenHistory,
            ),
            const SizedBox(height: 20),
              _TabRow(
                selected: _tab,
                onSelect: (t) => setState(() => _tab = t),
                allCount: totalActive,
                yangiCount: yangi.length,
                tayyorlanmoqdaCount: tayyorlanmoqda.length,
                tayyorCount: tayyor.length,
                kuryerdaCount: kuryerda.length,
                view: _view,
                onViewChanged: (v) => setState(() => _view = v),
              ),
              const SizedBox(height: 20),
              if (_view == _ViewMode.kanban)
                _KanbanBoard(
                  tab: _tab,
                  yangi: yangi,
                  tayyorlanmoqda: tayyorlanmoqda,
                  tayyor: tayyor,
                  kuryerda: kuryerda,
                  onAccept: _accept,
                  onReject: _reject,
                  onStartPreparing: (id) =>
                      _do(id, () => api.transition(id, 'preparing'),
                    label: 'tayyorlashni_boshladi'),
                  onReady: (id) =>
              _do(id, () => api.transition(id, 'ready'), label: 'tayyor'),
                  onCancelNoCourier: _cancelNoCourier,
                  onRetryCourier: _retryCourier,
                  busy: _busy,
                )
              else
                _ActiveOrdersList(orders: [
                  ...yangi,
                  ...tayyorlanmoqda,
                  ...tayyor,
                  ...kuryerda,
                ]..sort((a, b) {
                    final da = _parseAt(a['created_at']) ?? DateTime(0);
                    final db = _parseAt(b['created_at']) ?? DateTime(0);
                    return db.compareTo(da);
                  })),
          ],
        ),
      ),
    );
  }
}

// Quyidagi uchta yordamchi `widgets/order_card_header.dart` da
// (ochiq ko'rinishda) yashaydi — kartochka sarlavhasi shu yerdan
// chiqarilganda ular ham ko'chgan. Bu yerda faqat qisqa nomlar
// qoladi: sahifada o'nlab chaqiruv bor va ularning hammasini
// o'zgartirish diffni kattalashtirardi, ikki NUSXA qilish esa
// vaqt formati ikki joyda ajralib ketishiga olib kelardi.
DateTime? _parseAt(dynamic iso) => parseOrderAt(iso);

DateTime? _statusChangedAt(Map<String, dynamic> order, String status) {
  final history = (order['history'] as List?) ?? [];
  for (final h in history.reversed) {
    if (h is Map && h['to'] == status) return _parseAt(h['at']);
  }
  return null;
}

String _itemsSummary(Map<String, dynamic> o) {
  final items = (o['items'] as List?) ?? [];
  return items.map((i) => '${i['qty']}x ${i['name']}').join(', ');
}

String _shortOrderNumber(Map<String, dynamic> o) => shortOrderNumber(o);

String _timeOfDay(DateTime? d) => orderTimeOfDay(d);

/// "X daqiqa oldin"/"X soat Y daqiqa oldin" — ANIQ createdAt/statusAt
/// vaqtidan hisoblanadi, soxta emas.
String _elapsedSince(DateTime since) {
  final diff = DateTime.now().difference(since);
  if (diff.inMinutes < 1) return 'hozirgina';
  if (diff.inMinutes < 60) return '${diff.inMinutes} daqiqa oldin';
  final h = diff.inHours;
  final m = diff.inMinutes % 60;
  return m == 0 ? '$h soat oldin' : '$h soat $m daqiqa oldin';
}

/// `ready_at`gacha qolgan vaqt — HAQIQIY backend maydonidan (restoran
/// "Qabul qilindi" bosganda kiritgan tayyorlash vaqtiga asoslanib
/// hisoblangan), MM:SS formatida. Muddat o'tib ketgan bo'lsa alohida
/// (kechikkan) holat qaytadi — soxta manfiy vaqt ko'rsatilmaydi.
(String, bool) _readyCountdown(Map<String, dynamic> order) {
  final readyAtRaw = order['ready_at'];
  final readyAt = _parseAt(readyAtRaw);
  if (readyAt == null) return ('—', false);
  final remaining = readyAt.difference(DateTime.now());
  if (remaining.isNegative) {
    final overdue = -remaining;
    final m = overdue.inMinutes;
    return ('$m daqiqa kechikdi', true);
  }
  final m = remaining.inMinutes;
  final s = remaining.inSeconds % 60;
  return ('$m:${s.toString().padLeft(2, '0')} qoldi', false);
}

// ---------------------------------------------------------------------------
// Sarlavha: qidiruv, sana, tarix
// ---------------------------------------------------------------------------

class _Header extends StatelessWidget {
  final int totalCount;
  final TextEditingController searchCtrl;
  final VoidCallback? onOpenHistory;

  const _Header({
    required this.totalCount,
    required this.searchCtrl,
    required this.onOpenHistory,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final narrow = c.maxWidth < 900;
      final titleBlock = Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          const Text('Buyurtmalar',
              style: TextStyle(
                  fontSize: 27, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          const SizedBox(height: 4),
          Text(
            'Jami $totalCount ta buyurtma',
            style: const TextStyle(fontSize: 14, color: OnDexColors.inkDim),
          ),
        ],
      );
      final controls = Wrap(
        spacing: 12,
        runSpacing: 12,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          SizedBox(
            width: 240,
            child: TextField(
              controller: searchCtrl,
              decoration: InputDecoration(
                isDense: true,
                hintText: 'Buyurtma raqamini kiriting...',
                hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
                prefixIcon: const Icon(Icons.search_rounded, size: 19, color: OnDexColors.inkFaint),
                filled: true,
                fillColor: OnDexColors.cardBg,
                contentPadding: const EdgeInsets.symmetric(vertical: 12),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: const BorderSide(color: OnDexColors.cardBorder),
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: const BorderSide(color: OnDexColors.primary),
                ),
                border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
              ),
            ),
          ),
          // Faol buyurtmalar ATAYLAB sana bo'yicha filtrlanmaydi. Sana
          // filtri "Tarix" sahifasining yuqori panelida (davr bilan).
          if (onOpenHistory != null)
            OutlinedButton.icon(
              onPressed: onOpenHistory,
              style: OutlinedButton.styleFrom(
                foregroundColor: OnDexColors.primary,
                side: const BorderSide(color: OnDexColors.primary),
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              icon: const Icon(Icons.history_rounded, size: 17),
              label: const Text('Tarix'),
            ),
        ],
      );
      if (narrow) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [titleBlock, const SizedBox(height: 16), controls],
        );
      }
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(child: titleBlock),
          controls,
        ],
      );
    });
  }
}

// ---------------------------------------------------------------------------
// Filtr tablari + grid/ro'yxat almashtirgich
// ---------------------------------------------------------------------------

class _TabRow extends StatelessWidget {
  final _TabFilter selected;
  final ValueChanged<_TabFilter> onSelect;
  final int allCount, yangiCount, tayyorlanmoqdaCount, tayyorCount, kuryerdaCount;
  final _ViewMode view;
  final ValueChanged<_ViewMode> onViewChanged;

  const _TabRow({
    required this.selected,
    required this.onSelect,
    required this.allCount,
    required this.yangiCount,
    required this.tayyorlanmoqdaCount,
    required this.tayyorCount,
    required this.kuryerdaCount,
    required this.view,
    required this.onViewChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              _TabChip(
                  label: 'Barchasi',
                  count: allCount,
                  selected: selected == _TabFilter.all,
                  onTap: () => onSelect(_TabFilter.all)),
              _TabChip(
                  label: 'Yangi',
                  count: yangiCount,
                  selected: selected == _TabFilter.yangi,
                  onTap: () => onSelect(_TabFilter.yangi)),
              _TabChip(
                  label: 'Tayyorlanmoqda',
                  count: tayyorlanmoqdaCount,
                  selected: selected == _TabFilter.tayyorlanmoqda,
                  onTap: () => onSelect(_TabFilter.tayyorlanmoqda)),
              _TabChip(
                  label: 'Tayyor',
                  count: tayyorCount,
                  selected: selected == _TabFilter.tayyor,
                  onTap: () => onSelect(_TabFilter.tayyor)),
              _TabChip(
                  label: 'Kuryerda',
                  count: kuryerdaCount,
                  selected: selected == _TabFilter.kuryerda,
                  onTap: () => onSelect(_TabFilter.kuryerda)),
            ],
          ),
        ),
        const SizedBox(width: 12),
        Container(
          padding: const EdgeInsets.all(3),
          decoration: BoxDecoration(
            color: OnDexColors.cardBg,
            border: Border.all(color: OnDexColors.cardBorder),
            borderRadius: BorderRadius.circular(10),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _ViewIconButton(
                icon: Icons.grid_view_rounded,
                selected: view == _ViewMode.kanban,
                onTap: () => onViewChanged(_ViewMode.kanban),
              ),
              _ViewIconButton(
                icon: Icons.view_list_rounded,
                selected: view == _ViewMode.list,
                onTap: () => onViewChanged(_ViewMode.list),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _TabChip extends StatelessWidget {
  final String label;
  final int count;
  final bool selected;
  final VoidCallback onTap;
  const _TabChip(
      {required this.label, required this.count, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(999),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: selected ? OnDexColors.primaryTint : OnDexColors.cardBg,
          border: Border.all(color: selected ? OnDexColors.primary : OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(999),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(label,
                style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: selected ? OnDexColors.primaryPressed : OnDexColors.ink)),
            const SizedBox(width: 6),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
              decoration: BoxDecoration(
                color: selected ? OnDexColors.primary : OnDexColors.pageBg,
                borderRadius: BorderRadius.circular(999),
              ),
              child: Text('$count',
                  style: TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w800,
                      color: selected ? Colors.white : OnDexColors.inkDim)),
            ),
          ],
        ),
      ),
    );
  }
}

class _ViewIconButton extends StatelessWidget {
  final IconData icon;
  final bool selected;
  final VoidCallback onTap;
  const _ViewIconButton({required this.icon, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          color: selected ? OnDexColors.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Icon(icon, size: 19, color: selected ? Colors.white : OnDexColors.inkDim),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Kanban taxtasi
// ---------------------------------------------------------------------------

class _KanbanBoard extends StatelessWidget {
  final _TabFilter tab;
  final List<Map<String, dynamic>> yangi, tayyorlanmoqda, tayyor, kuryerda;
  final void Function(String id) onAccept;
  final void Function(String id) onReject;
  final void Function(String id) onStartPreparing;
  final void Function(String id) onReady;
  final void Function(Map<String, dynamic> order) onCancelNoCourier;
  final void Function(String id) onRetryCourier;

  /// Hozir serverga amal yuborilayotgan buyurtmalar.
  final Set<String> busy;

  const _KanbanBoard({
    required this.tab,
    required this.yangi,
    required this.tayyorlanmoqda,
    required this.tayyor,
    required this.kuryerda,
    required this.onAccept,
    required this.onReject,
    required this.onStartPreparing,
    required this.onReady,
    required this.onCancelNoCourier,
    required this.onRetryCourier,
    required this.busy,
  });

  @override
  Widget build(BuildContext context) {
    final columns = <Widget>[];
    void addColumn(_TabFilter forTab, Widget column) {
      if (tab == _TabFilter.all || tab == forTab) columns.add(column);
    }

    addColumn(
      _TabFilter.yangi,
      _KanbanColumn(
        title: 'Yangi',
        dotColor: OnDexColors.primary,
        count: yangi.length,
        children: [
          for (final o in yangi)
            _YangiCard(order: o, onAccept: () => onAccept(o['id']), onReject: () => onReject(o['id'])),
        ],
      ),
    );
    addColumn(
      _TabFilter.tayyorlanmoqda,
      _KanbanColumn(
        title: 'Tayyorlanmoqda',
        dotColor: OnDexColors.amber,
        count: tayyorlanmoqda.length,
        children: [
          for (final o in tayyorlanmoqda)
            _PreparingCard(
              order: o,
              onStartPreparing: () => onStartPreparing(o['id']),
              onReady: () => onReady(o['id']),
            ),
        ],
      ),
    );
    addColumn(
      _TabFilter.tayyor,
      _KanbanColumn(
        title: 'Tayyor',
        dotColor: OnDexColors.success,
        count: tayyor.length,
        children: [
          for (final o in tayyor)
            _ReadyCard(
              order: o,
              busy: busy.contains(o['id']),
              onCancel: () => onCancelNoCourier(o),
              onRetry: () => onRetryCourier(o['id'] as String),
            ),
        ],
      ),
    );
    addColumn(
      _TabFilter.kuryerda,
      _KanbanColumn(
        title: 'Kuryerda',
        dotColor: OnDexColors.info,
        count: kuryerda.length,
        children: [for (final o in kuryerda) _KuryerdaCard(order: o)],
      ),
    );

    // ┌─ RESPONSIV TARTIB ───────────────────────────────────────────────┐
    // Avval ustunlar `IntrinsicHeight` ichida QAT'IY 4 tadan yonma-yon
    // turardi. Ikki muammo bor edi:
    //   1. tor oynada (kichik noutbuk, yarim ekran) har ustun ~200 px
    //      gacha torayib, kartochka matnlari sig'masdi;
    //   2. `IntrinsicHeight` bolalar balandligini OLDINDAN taxmin
    //      qiladi, qatorga o'raladigan matnni esa noto'g'ri hisoblashi
    //      mumkin — bu ham toshishga olib keladi (ustiga har kadrda
    //      ikki marta o'lchash — sekin).
    // Endi ustunlar soni kenglikdan: keng oynada 4 ta, o'rtada 2 ta,
    // torda 1 ta; har ustun o'z mazmunicha balandlikda.
    // └───────────────────────────────────────────────────────────────────┘
    if (columns.isEmpty) return const SizedBox.shrink();
    return LayoutBuilder(builder: (context, constraints) {
      const gap = 16.0;
      const minColumnWidth = 250.0;
      var perRow = ((constraints.maxWidth + gap) / (minColumnWidth + gap)).floor();
      if (perRow < 1) perRow = 1;
      if (perRow > columns.length) perRow = columns.length;
      // 3 ta ustun 4 tadan birini yolg'iz pastki qatorga tushirardi —
      // ikkitadan juft qatorlar tartibliroq ko'rinadi.
      if (perRow == 3 && columns.length == 4) perRow = 2;

      final rows = <Widget>[];
      for (var start = 0; start < columns.length; start += perRow) {
        rows.add(Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var i = start; i < start + perRow; i++) ...[
              if (i > start) const SizedBox(width: gap),
              // Oxirgi qator to'lmasa bo'sh joy qoladi — ustunlar
              // kengligi qatordan qatorga o'zgarmasin.
              Expanded(
                  child: i < columns.length ? columns[i] : const SizedBox.shrink()),
            ],
          ],
        ));
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var r = 0; r < rows.length; r++) ...[
            if (r > 0) const SizedBox(height: gap),
            rows[r],
          ],
        ],
      );
    });
  }
}

class _KanbanColumn extends StatelessWidget {
  final String title;
  final Color dotColor;
  final int count;
  final List<Widget> children;
  const _KanbanColumn(
      {required this.title, required this.dotColor, required this.count, required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                  width: 8, height: 8, decoration: BoxDecoration(color: dotColor, shape: BoxShape.circle)),
              const SizedBox(width: 8),
              Text(title,
                  style: const TextStyle(
                      fontSize: 14.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
              const Spacer(),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                decoration:
                    BoxDecoration(color: OnDexColors.pageBg, borderRadius: BorderRadius.circular(999)),
                child: Text('$count',
                    style: const TextStyle(
                        fontSize: 11.5, fontWeight: FontWeight.w800, color: OnDexColors.inkDim)),
              ),
            ],
          ),
          const SizedBox(height: 14),
          if (children.isEmpty)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 24),
              child: Center(
                child: Text('Bo\'sh', style: TextStyle(color: OnDexColors.inkFaint, fontSize: 12.5)),
              ),
            )
          else
            for (final c in children) Padding(padding: const EdgeInsets.only(bottom: 12), child: c),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Kartochkalar (ustunlar ichida)
// ---------------------------------------------------------------------------

class _CardShell extends StatelessWidget {
  final Widget child;
  const _CardShell({required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: OnDexColors.pageBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: child,
    );
  }
}


class _ItemsList extends StatelessWidget {
  final Map<String, dynamic> order;
  const _ItemsList({required this.order});

  @override
  Widget build(BuildContext context) {
    final items = (order['items'] as List?) ?? [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final raw in items)
          Padding(
            padding: const EdgeInsets.only(bottom: 2),
            child: Text('${raw['qty']}× ${raw['name']}',
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
      ],
    );
  }
}

class _PhoneRow extends StatelessWidget {
  final String? phone;
  const _PhoneRow({required this.phone});

  @override
  Widget build(BuildContext context) {
    final p = phone ?? '';
    if (p.isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Row(
        children: [
          const Icon(Icons.call_rounded, size: 14, color: OnDexColors.inkFaint),
          const SizedBox(width: 7),
          // `Flexible` — tor ustunda raqam kartochkadan chiqib
          // ketmasin (Flutter'ning sariq-qora "overflow" chizig'i).
          Flexible(
            child: Text(p,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                    fontSize: 12.5, color: OnDexColors.inkDim)),
          ),
        ],
      ),
    );
  }
}

class _TotalRow extends StatelessWidget {
  final int totalTiyin;
  const _TotalRow({required this.totalTiyin});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 10),
      child: Row(
        children: [
          // Yorliq qisqaradi, SUMMA esa hech qachon qisqarmaydi —
          // restoran uchun eng muhim raqam shu.
          const Flexible(
            child: Text('Jami summa',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
          ),
          const SizedBox(width: 8),
          const Spacer(),
          Text(formatSum(totalTiyin),
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        ],
      ),
    );
  }
}

class _YangiCard extends StatelessWidget {
  final Map<String, dynamic> order;
  final VoidCallback onAccept;
  final VoidCallback onReject;
  const _YangiCard({required this.order, required this.onAccept, required this.onReject});

  @override
  Widget build(BuildContext context) {
    return _CardShell(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          OrderCardHeader(order: order),
          const SizedBox(height: 8),
          _ItemsList(order: order),
          _PhoneRow(phone: order['customer_phone'] as String?),
          _TotalRow(totalTiyin: (order['total_tiyin'] ?? 0) as int),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: onReject,
                  style: OutlinedButton.styleFrom(
                    foregroundColor: OnDexColors.danger,
                    side: const BorderSide(color: OnDexColors.danger),
                    padding: const EdgeInsets.symmetric(vertical: 11),
                  ),
                  icon: const Icon(Icons.close_rounded, size: 16),
                  // Juda tor ustunda ham chiziq chiqmasligi uchun.
                  label: const Text('Rad etish',
                      maxLines: 1, overflow: TextOverflow.ellipsis),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: FilledButton.icon(
                  onPressed: onAccept,
                  icon: const Icon(Icons.check_rounded, size: 16),
                  label: const Text('Qabul qilish',
                      maxLines: 1, overflow: TextOverflow.ellipsis),
                  style: FilledButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 11)),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _PreparingCard extends StatelessWidget {
  final Map<String, dynamic> order;
  final VoidCallback onStartPreparing;
  final VoidCallback onReady;
  const _PreparingCard(
      {required this.order, required this.onStartPreparing, required this.onReady});

  @override
  Widget build(BuildContext context) {
    final isPreparing = order['status'] == 'preparing';
    final (countdownText, overdue) = _readyCountdown(order);
    return _CardShell(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          OrderCardHeader(order: order),
          const SizedBox(height: 8),
          _ItemsList(order: order),
          _PhoneRow(phone: order['customer_phone'] as String?),
          if (isPreparing) ...[
            const SizedBox(height: 8),
            // ┌─ TUZATILGAN NOSOZLIK: "RIGHT OVERFLOWED BY 26 PIXELS" ────┐
            // Matn qatorga `Expanded`siz qo'yilgan edi. Kechikish
            // qo'shilganda ("49 daqiqa kechikdi") yozuv uzayib, tor Kanban
            // ustuniga sig'masdi va kartochka ustida sariq-qora chiziq
            // chiqardi. Endi matn qolgan kenglikni oladi va kerak bo'lsa
            // ikkinchi qatorga o'tadi — kenglik qanchalik tor bo'lmasin.
            // └───────────────────────────────────────────────────────────┘
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Padding(
                  padding: const EdgeInsets.only(top: 1),
                  child: Icon(Icons.timer_outlined,
                      size: 14, color: overdue ? OnDexColors.danger : OnDexColors.inkFaint),
                ),
                const SizedBox(width: 7),
                Expanded(
                  child: Text('Tayyor bo\'lish vaqti: $countdownText',
                      key: const ValueKey('ready-countdown'),
                      style: TextStyle(
                          fontSize: 12.5,
                          color: overdue ? OnDexColors.danger : OnDexColors.inkDim,
                          fontWeight: overdue ? FontWeight.w700 : FontWeight.w400)),
                ),
              ],
            ),
          ],
          _TotalRow(totalTiyin: (order['total_tiyin'] ?? 0) as int),
          const SizedBox(height: 10),
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              onPressed: isPreparing ? onReady : onStartPreparing,
              style: FilledButton.styleFrom(
                backgroundColor: OnDexColors.ink,
                padding: const EdgeInsets.symmetric(vertical: 11, horizontal: 8),
              ),
              icon: Icon(isPreparing ? Icons.check_circle_rounded : Icons.soup_kitchen_rounded, size: 16),
              // Tor ustunda tugma yozuvi ham chiqib ketmasin.
              label: Text(isPreparing ? 'Tayyorlandi' : 'Tayyorlashni boshlash',
                  maxLines: 1, overflow: TextOverflow.ellipsis),
            ),
          ),
        ],
      ),
    );
  }
}

class _ReadyCard extends StatelessWidget {
  final Map<String, dynamic> order;
  final VoidCallback onCancel;
  final VoidCallback onRetry;
  final bool busy;
  const _ReadyCard({
    required this.order,
    required this.onCancel,
    required this.onRetry,
    required this.busy,
  });

  @override
  Widget build(BuildContext context) {
    return _CardShell(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Tartib: raqam → stol (stol buyurtmasida) → taomlar → mijoz
          // raqami → jami summa → kuryer holati.
          OrderCardHeader(order: order),
          const SizedBox(height: 8),
          _ItemsList(order: order),
          // Raqam bo'lmasa qator umuman chizilmaydi (`_PhoneRow`), bor
          // bo'lsa stol buyurtmasida ham ko'rsatiladi.
          _PhoneRow(phone: order['customer_phone'] as String?),
          _TotalRow(totalTiyin: (order['total_tiyin'] ?? 0) as int),
          const SizedBox(height: 10),
          // MUHIM: bu yerda "Kuryerga topshirish" tugmasi ATAYLAB YO'Q —
          // buyurtmani "picked_up" holatiga FAQAT KURYER o'zi, o'z
          // ilovasida o'tkazadi (restoran raqamning oxirgi 4 xonasini
          // og'zaki aytadi).
          //
          // Kuryer topilmasa esa abadiy "Kuryer qidirilmoqda..." emas,
          // "Bekor qilish" / "Kuryer qidirish" tugmalari chiqadi
          // (`CourierStatusBox`, qaror serverda).
          CourierStatusBox(
            order: order,
            busy: busy,
            onCancel: onCancel,
            onRetry: onRetry,
          ),
        ],
      ),
    );
  }
}

class _KuryerdaCard extends StatelessWidget {
  final Map<String, dynamic> order;
  const _KuryerdaCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final courierName = order['courier_name'] as String? ?? '';
    final pickedUpAt = _statusChangedAt(order, 'picked_up');
    return _CardShell(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          OrderCardHeader(order: order),
          const SizedBox(height: 8),
          Row(
            children: [
              const Icon(Icons.pedal_bike_rounded, size: 16, color: OnDexColors.info),
              const SizedBox(width: 8),
              Expanded(
                child: Text(courierName.isEmpty ? 'Kuryer' : courierName,
                    style: const TextStyle(
                        fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
              ),
            ],
          ),
          if (pickedUpAt != null)
            Padding(
              padding: const EdgeInsets.only(top: 3, left: 24),
              child: Text('Yo\'lda: ${_elapsedSince(pickedUpAt)}',
                  style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint)),
            ),
          _PhoneRow(phone: order['customer_phone'] as String?),
          _TotalRow(totalTiyin: (order['total_tiyin'] ?? 0) as int),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Ro'yxat ko'rinishi (grid/list almashtirgichning "list" holati)
// ---------------------------------------------------------------------------

class _ActiveOrdersList extends StatelessWidget {
  final List<Map<String, dynamic>> orders;
  const _ActiveOrdersList({required this.orders});

  @override
  Widget build(BuildContext context) {
    if (orders.isEmpty) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 40),
        child: Center(
            child: Text('Hozircha faol buyurtma yo\'q', style: TextStyle(color: OnDexColors.inkDim))),
      );
    }
    return Container(
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        children: [
          for (var i = 0; i < orders.length; i++) ...[
            if (i > 0) const Divider(height: 1, color: OnDexColors.cardBorder),
            _ActiveListRow(order: orders[i]),
          ],
        ],
      ),
    );
  }
}

class _ActiveListRow extends StatelessWidget {
  final Map<String, dynamic> order;
  const _ActiveListRow({required this.order});

  @override
  Widget build(BuildContext context) {
    var (label, color, bg) = orderStatusStyle(order['status'] as String? ?? '');
    // Kuryer topilmagan tayyor buyurtma ro'yxatda ham ajralib tursin —
    // qaror tugmalari Kanban kartochkasida.
    if (isCourierNotFound(order)) {
      (label, color, bg) = ('Kuryer topilmadi', OnDexColors.danger, OnDexColors.dangerBg);
    }
    // Stol buyurtmasida stol raqami taomlardan OLDIN — xodim uchun eng
    // muhim ma'lumot.
    final tableLabel = order['table_label'] as String? ?? '';
    final summary = isDineInOrder(order)
        ? '${tableText(tableLabel)} · ${_itemsSummary(order)}'
        : _itemsSummary(order);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 14),
      child: Row(
        children: [
          SizedBox(
              width: 70,
              child: Text(_shortOrderNumber(order),
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink))),
          SizedBox(
              width: 50,
              child: Text(_timeOfDay(_parseAt(order['created_at'])),
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkFaint))),
          Expanded(
            child: Text(summary,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: 10),
            padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
            decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
            child: Text(label, style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w700)),
          ),
          SizedBox(
            width: 90,
            child: Text(formatSum((order['total_tiyin'] ?? 0) as int),
                textAlign: TextAlign.right,
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
          ),
        ],
      ),
    );
  }
}

