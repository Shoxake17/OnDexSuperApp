import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Boshqaruv paneli" — ShoxPos `Menu.tsx` (do'kon) va `PharmMenu.tsx`
/// (dorixona).
///
/// Do'kon: "Umumiy mahsulotlar bazasi" (global baza soni → Mahsulotlar),
/// "Mahsulotlarim bazasi" (o'z mahsulotlari soni → Kirim bazasi), "Sotuvlar",
/// "Hisobot". Dorixona: "Umumiy dorixona bazasi" (→ Dorilar bazasi),
/// "Mening dorilarim" (→ Kirim bazasi), "Sotuv" (→ Savdo terminali),
/// "Hisobotlar" (→ Kirim bazasi — ShoxPos'da ham shunday).
class DashboardPage extends StatefulWidget {
  const DashboardPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  int? _base;
  int? _own;

  bool get _pharm => widget.ctx.isPharmacy;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final base = _pharm ? await api.pharmBaseCount() : await api.shopBaseCount();
      final own = _pharm ? (await api.pharmProducts()).length : (await api.shopProducts()).length;
      if (mounted) {
        setState(() {
          _base = base;
          _own = own;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _base ??= 0;
          _own ??= 0;
        });
        toast(context, errText(e), error: true);
      }
    }
  }

  Widget _big(String key, String title, int? count, IconData icon, Color color, StorePage to) => Material(
        color: color,
        borderRadius: BorderRadius.circular(18),
        child: InkWell(
          key: ValueKey(key),
          borderRadius: BorderRadius.circular(18),
          onTap: () => widget.ctx.go(to),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Row(children: [
              Expanded(
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(count == null ? '...' : '${formatCount(count)} ta',
                      style: const TextStyle(color: Colors.white, fontSize: 30, fontWeight: FontWeight.w900)),
                  const SizedBox(height: 6),
                  Text(title, style: TextStyle(color: Colors.white.withValues(alpha: .9), fontSize: 14)),
                ]),
              ),
              Icon(icon, size: 48, color: Colors.white.withValues(alpha: .85)),
            ]),
          ),
        ),
      );

  Widget _small(String key, String title, IconData icon, Color color, StorePage to) => Material(
        color: OnDexColors.cardBg,
        shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16), side: const BorderSide(color: OnDexColors.cardBorder)),
        child: InkWell(
          key: ValueKey(key),
          borderRadius: BorderRadius.circular(16),
          onTap: () => widget.ctx.go(to),
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 26, horizontal: 16),
            child: Column(children: [
              Icon(icon, color: color, size: 32),
              const SizedBox(height: 12),
              Text(title, style: const TextStyle(fontWeight: FontWeight.w700)),
            ]),
          ),
        ),
      );

  @override
  Widget build(BuildContext context) {
    final narrow = MediaQuery.sizeOf(context).width < 760;
    final big = _pharm
        ? [
            _big('dash-base', 'Umumiy dorixona bazasi', _base, Icons.medication_rounded, OnDexColors.primary,
                StorePage.pharmBase),
            _big('dash-own', 'Mening dorilarim', _own, Icons.inventory_rounded, OnDexColors.primaryPressed,
                StorePage.pharmIncoming),
          ]
        : [
            _big('dash-base', 'Umumiy mahsulotlar bazasi', _base, Icons.storage_rounded, OnDexColors.primary,
                StorePage.products),
            _big('dash-own', 'Mahsulotlarim bazasi', _own, Icons.inventory_rounded, OnDexColors.primaryPressed,
                StorePage.incoming),
          ];
    final small = _pharm
        ? [
            _small('dash-sales', 'Sotuv', Icons.shopping_cart_rounded, OnDexColors.success, StorePage.pharmScanner),
            _small('dash-reports', 'Hisobotlar', Icons.trending_up_rounded, OnDexColors.info, StorePage.pharmIncoming),
          ]
        : [
            _small('dash-sales', 'Sotuvlar', Icons.payments_rounded, OnDexColors.success, StorePage.sales),
            _small('dash-reports', 'Hisobot', Icons.bar_chart_rounded, OnDexColors.info, StorePage.reports),
          ];
    return ShopPage(children: [
      if (narrow) ...[big[0], const SizedBox(height: 16), big[1]]
      else
        Row(children: [Expanded(child: big[0]), const SizedBox(width: 20), Expanded(child: big[1])]),
      const SizedBox(height: 24),
      Row(children: [
        Expanded(child: small[0]),
        const SizedBox(width: 16),
        Expanded(child: small[1]),
        if (!narrow) const Spacer(flex: 2),
      ]),
    ]);
  }
}

/// 178123 -> "178 123".
String formatCount(int n) => n.toString().replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (m) => '${m[1]} ');
