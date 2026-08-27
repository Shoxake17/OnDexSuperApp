import 'package:flutter/material.dart';

import '../models/waiter_order.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/order_card.dart';
import 'order_detail_screen.dart';

/// Bosh bo'lim — restorandagi barcha faol stol buyurtmalari.
///
/// Tayyor buyurtmalar HAR DOIM tepada: affitsiantning yagona shoshilinch
/// ishi shu. Qolganlari kelish tartibida.
class OrdersScreen extends StatefulWidget {
  const OrdersScreen({super.key, required this.store});

  final WaiterStore store;

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

enum _Filter { all, ready, pending }

class _OrdersScreenState extends State<OrdersScreen> {
  _Filter _filter = _Filter.all;

  @override
  Widget build(BuildContext context) {
    final store = widget.store;
    final ready = store.readyOrders;
    final pending = store.pendingOrders;

    return Column(
      children: [
        // ┌─ FILTR SONLARI ────────────────────────────────────────────┐
        // Sonlar AYNAN quyidagi ro'yxatlardan olinadi (`ready.length`,
        // `pending.length`) — alohida hisoblanmaydi. Maketda "Yangi 1"
        // yozilib, pastda 2 ta element chiqqan edi; sabab aynan ikki
        // xil hisoblash edi.
        // └────────────────────────────────────────────────────────────┘
        _FilterRow(
          filter: _filter,
          allCount: ready.length + pending.length,
          readyCount: ready.length,
          pendingCount: pending.length,
          onChanged: (f) => setState(() => _filter = f),
        ),
        Expanded(child: _buildList(store, ready, pending)),
      ],
    );
  }

  Widget _buildList(
    WaiterStore store,
    List<WaiterOrder> ready,
    List<WaiterOrder> pending,
  ) {
    if (store.loading) {
      return const Center(child: CircularProgressIndicator());
    }

    // Xato FAQAT ro'yxat bo'sh bo'lganda butun ekranni egallaydi.
    // Ma'lumot bor bo'lsa — eskisi ko'rsatilaveradi: tarmoq bir soniya
    // uzilgani uchun affitsiantning ko'z oldidagi ro'yxat yo'qolmasligi
    // kerak.
    if (store.error != null && store.orders.isEmpty) {
      return RefreshIndicator(
        onRefresh: store.refresh,
        color: kBrandColor,
        backgroundColor: kSurface,
        child: ListView(
          children: [ErrorRetry(message: store.error!, onRetry: store.refresh)],
        ),
      );
    }

    final showReady = _filter != _Filter.pending;
    final showPending = _filter != _Filter.ready;
    final visibleReady = showReady ? ready : const <WaiterOrder>[];
    final visiblePending = showPending ? pending : const <WaiterOrder>[];

    return RefreshIndicator(
      onRefresh: store.refresh,
      color: kBrandColor,
      backgroundColor: kSurface,
      // `ListView` ATAYLAB (Center emas): `RefreshIndicator` faqat
      // suriladigan vidjet ustida ishlaydi, aks holda bo'sh ekranda
      // qo'lda yangilash umuman mumkin bo'lmasdi.
      child: ListView(
        padding: const EdgeInsets.fromLTRB(12, 4, 12, 24),
        children: [
          if (visibleReady.isEmpty && visiblePending.isEmpty)
            _emptyFor(_filter, store),
          if (visibleReady.isNotEmpty) ...[
            const SectionHeader(
              title: 'Yetkazish kerak',
              color: kReadyColor,
            ),
            for (final o in visibleReady)
              OrderCard(
                order: o,
                store: store,
                onTap: () => _openDetail(o, store),
              ),
          ],
          if (visiblePending.isNotEmpty) ...[
            SectionHeader(
              title: 'Oshxonada',
              count: visiblePending.length,
            ),
            for (final o in visiblePending)
              OrderCard(
                order: o,
                store: store,
                onTap: () => _openDetail(o, store),
              ),
          ],
        ],
      ),
    );
  }

  Widget _emptyFor(_Filter f, WaiterStore store) {
    if (store.orders.isEmpty) {
      return const EmptyState(
        icon: Icons.check_circle_outline,
        title: 'Faol buyurtma yo\'q',
        subtitle: 'Yangi buyurtma kelganda bu yerda darhol ko\'rinadi '
            'va ovoz chalinadi.',
      );
    }
    return EmptyState(
      icon: f == _Filter.ready ? Icons.room_service_outlined : Icons.soup_kitchen,
      title: f == _Filter.ready
          ? 'Yetkazish kerak bo\'lgan buyurtma yo\'q'
          : 'Oshxonada buyurtma yo\'q',
      subtitle: 'Boshqa filtrlarda buyurtmalar bor.',
    );
  }

  void _openDetail(WaiterOrder order, WaiterStore store) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => OrderDetailScreen(orderId: order.id, store: store),
      ),
    );
  }
}

class _FilterRow extends StatelessWidget {
  const _FilterRow({
    required this.filter,
    required this.allCount,
    required this.readyCount,
    required this.pendingCount,
    required this.onChanged,
  });

  final _Filter filter;
  final int allCount;
  final int readyCount;
  final int pendingCount;
  final ValueChanged<_Filter> onChanged;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 52,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        children: [
          _Chip(
            label: 'Barchasi',
            count: allCount,
            selected: filter == _Filter.all,
            onTap: () => onChanged(_Filter.all),
          ),
          _Chip(
            label: 'Tayyor',
            count: readyCount,
            selected: filter == _Filter.ready,
            accent: kReadyColor,
            onTap: () => onChanged(_Filter.ready),
          ),
          _Chip(
            label: 'Oshxonada',
            count: pendingCount,
            selected: filter == _Filter.pending,
            onTap: () => onChanged(_Filter.pending),
          ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({
    required this.label,
    required this.count,
    required this.selected,
    required this.onTap,
    this.accent,
  });

  final String label;
  final int count;
  final bool selected;
  final VoidCallback onTap;
  final Color? accent;

  @override
  Widget build(BuildContext context) {
    final color = accent ?? kBrandColor;
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: Material(
        color: selected ? color : kSurface,
        borderRadius: BorderRadius.circular(20),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(20),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(20),
              border: Border.all(color: selected ? color : kBorder),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: selected ? Colors.white : kInkDim,
                  ),
                ),
                const SizedBox(width: 6),
                Text(
                  '$count',
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.bold,
                    color: selected
                        ? Colors.white.withValues(alpha: 0.85)
                        : kInkGhost,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
