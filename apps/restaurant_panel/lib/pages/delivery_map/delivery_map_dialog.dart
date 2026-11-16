import 'dart:async';

import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import 'delivery_map_view.dart';

/// RS-1: "Yetkazish xaritasi" — restoranning yo'ldagi kuryerlari jonli
/// (5 soniyada yangilanadi): kuryer joyi, borish nuqtasi, ro'yxat.
Future<void> showDeliveryMapDialog(BuildContext context) => showDialog<void>(
      context: context,
      builder: (_) => const Dialog(
        insetPadding: EdgeInsets.all(24),
        clipBehavior: Clip.antiAlias,
        child: _DeliveryMapBody(),
      ),
    );

extension DeliveryMapApi on RestaurantApi {
  Future<Map<String, dynamic>> deliveryMap() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/delivery-map') as Map);
}

class _DeliveryMapBody extends StatefulWidget {
  const _DeliveryMapBody();

  @override
  State<_DeliveryMapBody> createState() => _DeliveryMapBodyState();
}

class _DeliveryMapBodyState extends State<_DeliveryMapBody> {
  Map<String, dynamic>? _data;
  String? _error;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 5), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final d = await api.deliveryMap();
      if (mounted) {
        setState(() {
          _data = d;
          _error = null;
        });
      }
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    }
  }

  @override
  Widget build(BuildContext context) {
    final couriers = [
      for (final c in (_data?['couriers'] as List? ?? const [])) Map<String, dynamic>.from(c as Map),
    ];
    return SizedBox(
      width: 1100,
      height: 680,
      child: Column(children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 14, 8, 10),
          child: Row(children: [
            const Icon(Icons.delivery_dining_rounded, color: OnDexColors.primary),
            const SizedBox(width: 10),
            const Text('Yetkazish xaritasi', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
            const SizedBox(width: 12),
            Text('${couriers.length} ta kuryer yo\'lda', style: const TextStyle(color: OnDexColors.inkDim)),
            const Spacer(),
            IconButton(tooltip: 'Yopish', onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close_rounded)),
          ]),
        ),
        if (_error != null)
          Padding(padding: const EdgeInsets.all(8), child: Text(_error!, style: const TextStyle(color: OnDexColors.danger))),
        Expanded(
          child: Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Expanded(child: DeliveryMapView(data: _data)),
            SizedBox(
              width: 280,
              child: couriers.isEmpty
                  ? const Center(child: Text('Hozir yo\'lda kuryer yo\'q', style: TextStyle(color: OnDexColors.inkDim)))
                  : ListView.separated(
                      padding: const EdgeInsets.all(12),
                      itemCount: couriers.length,
                      separatorBuilder: (_, __) => const Divider(height: 16),
                      itemBuilder: (_, i) {
                        final c = couriers[i];
                        final num = '${c['order_number'] ?? ''}'.split('-').last;
                        final (label, color, _) = orderStatusStyle('${c['status']}');
                        return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Text('${c['courier_name'] ?? 'Kuryer'}',
                              style: const TextStyle(fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                          Text('#$num · $label', style: TextStyle(fontSize: 12.5, color: color)),
                          if (c['location'] == null)
                            const Text('Joylashuv hali kelmagan', style: TextStyle(fontSize: 12, color: OnDexColors.inkFaint)),
                        ]);
                      },
                    ),
            ),
          ]),
        ),
      ]),
    );
  }
}
