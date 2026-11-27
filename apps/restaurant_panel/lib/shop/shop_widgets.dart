import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api.dart';
import '../theme.dart';

/// ShoxPos `Category.tsx` CATEGORY_META — kategoriya emojisi (o'zbekcha nom
/// qiymatning o'zi).
const Map<String, String> kCategoryEmoji = {
  'Don va un': '🌾',
  "Go'sht": '🥩',
  'Sabzavot': '🥬',
  'Mevalar': '🍎',
  'Ichimliklar': '🧃',
  "Yog'": '🧈',
  'Sut mahsulotlari': '🥛',
  'Qandolat mahsulotlari': '🍫',
  'Kiyim': '🧥',
  'Texnika': '🛠️',
  'Avtotransport vositalari': '🚗',
  'Gigiyena vositalari': '🧼',
  'Oshxona jihozlari': '🍽️',
  'Bolalar mahsulotlari': '🧸',
  'Kantselyariya mahsulotlari': '✏️',
  'Kanselyariya maxsulotlari': '✏️',
  'Uy hayvonlari uchun tovarlar': '🐶',
  'Dorilar': '💊',
  'Dori-darmonlar': '💊',
  'Tamaki mahsulotlari': '🚬',
  'Moy': '🫙',
  "Go'zallik va Parvarish": '💄',
  'Sport oziqlanishi': '💪',
  'Nonvoylik mahsulotlari': '🍞',
  'Oziq-ovqat mahsulotlari': '🥗',
  'Alkogol ichimliklar': '🍷',
  'Shirinliklar': '🍬',
  "Go'sht mahsulotlari": '🥩',
  'Baliq mahsulotlari': '🐟',
  'Konservalar': '🥫',
  'Ziravorlar': '🌶️',
  'Uy kimyosi': '🧹',
  'Kiyim-kechak': '👕',
  'Poyabzal': '👟',
  'Elektronika': '📱',
  'Maishiy texnika': '🏠',
  "Qo'shimcha oziqlanish": '🌿',
  'Auto mahsulotlari': '🚗',
  'Qurilish materiallari': '🧱',
  'Kitoblar': '📚',
  'Muz qaymoq': '🍦',
  'general': '📦',
  'Boshqa': '🗂️',
};

String categoryEmoji(String c) => kCategoryEmoji[c] ?? '📦';

/// Kategoriya yorlig'i: "🥛 Sut mahsulotlari"; bo'sh — "Barcha kategoriyalar".
String categoryLabel(String c) => c.isEmpty ? 'Barcha kategoriyalar' : '${categoryEmoji(c)} $c';

double numOf(Object? v) => (v as num?)?.toDouble() ?? 0;
int intOf(Object? v) => (v as num?)?.toInt() ?? 0;

/// Xato matni: server xabari yoki umumiy.
String errText(Object e) => e is ApiException ? e.message : 'Xatolik yuz berdi — internetni tekshiring';

void toast(BuildContext context, String text, {bool error = false}) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(
      content: Text(text),
      backgroundColor: error ? OnDexColors.danger : null,
      behavior: SnackBarBehavior.floating,
      width: 420,
    ));
}

Future<bool> confirmDialog(BuildContext context, String text, {String ok = 'O\'chirish', bool danger = true}) async {
  final r = await showDialog<bool>(
    context: context,
    builder: (c) => AlertDialog(
      content: Text(text),
      actions: [
        TextButton(onPressed: () => Navigator.pop(c, false), child: const Text('Bekor qilish')),
        FilledButton(
          style: danger ? FilledButton.styleFrom(backgroundColor: OnDexColors.danger) : null,
          onPressed: () => Navigator.pop(c, true),
          child: Text(ok),
        ),
      ],
    ),
  );
  return r == true;
}

/// Sahifa ichki qobig'i: chekka bo'shliq + aylantirish.
class ShopPage extends StatelessWidget {
  const ShopPage({super.key, required this.children, this.maxWidth});
  final List<Widget> children;
  final double? maxWidth;

  @override
  Widget build(BuildContext context) {
    final narrow = MediaQuery.sizeOf(context).width < 700;
    final body = Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: children);
    return SingleChildScrollView(
      padding: EdgeInsets.all(narrow ? 14 : 24),
      child: maxWidth == null
          ? body
          : Center(child: ConstrainedBox(constraints: BoxConstraints(maxWidth: maxWidth!), child: body)),
    );
  }
}

class ShopCard extends StatelessWidget {
  const ShopCard({super.key, required this.child, this.padding = const EdgeInsets.all(16), this.color});
  final Widget child;
  final EdgeInsets padding;
  final Color? color;

  @override
  Widget build(BuildContext context) => Container(
        padding: padding,
        decoration: BoxDecoration(
          color: color ?? OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: child,
      );
}

/// Statistika kartochkasi (ShoxPos stat tile).
class StatTile extends StatelessWidget {
  const StatTile({super.key, required this.label, required this.value, required this.icon, this.color = OnDexColors.primary});
  final String label;
  final String value;
  final IconData icon;
  final Color color;

  @override
  Widget build(BuildContext context) => ShopCard(
        child: Row(children: [
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(color: color.withValues(alpha: 0.13), borderRadius: BorderRadius.circular(10)),
            child: Icon(icon, size: 19, color: color),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(label.toUpperCase(),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 10.5, color: OnDexColors.inkDim, fontWeight: FontWeight.w700, letterSpacing: .4)),
              const SizedBox(height: 3),
              FittedBox(
                fit: BoxFit.scaleDown,
                alignment: Alignment.centerLeft,
                child: Text(value, style: TextStyle(fontSize: 19, fontWeight: FontWeight.w800, color: color)),
              ),
            ]),
          ),
        ]),
      );
}

/// Statistika kartochkalari to'ri (kenglikka moslashadi).
class StatGrid extends StatelessWidget {
  const StatGrid({super.key, required this.tiles});
  final List<Widget> tiles;

  @override
  Widget build(BuildContext context) => LayoutBuilder(builder: (context, c) {
        final cols = c.maxWidth >= 1000 ? 4 : c.maxWidth >= 560 ? 2 : 1;
        final w = (c.maxWidth - (cols - 1) * 12) / cols;
        return Wrap(spacing: 12, runSpacing: 12, children: [for (final t in tiles) SizedBox(width: w, child: t)]);
      });
}

class SearchBox extends StatelessWidget {
  const SearchBox({super.key, required this.onChanged, this.hint = 'Shtrix-kod yoki nom...', this.controller, this.onSubmitted, this.autofocus = false, this.focusNode});
  final ValueChanged<String> onChanged;
  final ValueChanged<String>? onSubmitted;
  final String hint;
  final TextEditingController? controller;
  final bool autofocus;
  final FocusNode? focusNode;

  @override
  Widget build(BuildContext context) => TextField(
        controller: controller,
        focusNode: focusNode,
        autofocus: autofocus,
        onChanged: onChanged,
        onSubmitted: onSubmitted,
        decoration: shopInput(hint).copyWith(prefixIcon: const Icon(Icons.search_rounded, size: 20)),
      );
}

InputDecoration shopInput(String hint, {String? suffix}) => InputDecoration(
      hintText: hint,
      isDense: true,
      filled: true,
      fillColor: OnDexColors.cardBg,
      suffixText: suffix,
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12), borderSide: const BorderSide(color: OnDexColors.cardBorder)),
      enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12), borderSide: const BorderSide(color: OnDexColors.cardBorder)),
      focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12), borderSide: const BorderSide(color: OnDexColors.primary, width: 1.5)),
    );

/// Faqat raqam (va bitta nuqta) — narx/miqdor maydonlari.
final List<TextInputFormatter> kNumberInput = [FilteringTextInputFormatter.allow(RegExp(r'[0-9.]'))];

class FieldLabel extends StatelessWidget {
  const FieldLabel(this.text, {super.key, this.hint});
  final String text;
  final String? hint;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(text, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 13.5, color: OnDexColors.ink)),
          if (hint != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Text(hint!, style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
            ),
        ]),
      );
}

/// Bo'sh / yuklanmoqda holati.
class EmptyState extends StatelessWidget {
  const EmptyState({super.key, required this.loading, this.text = 'Ma\'lumot topilmadi'});
  final bool loading;
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 60),
        child: Center(
          child: loading
              ? const CircularProgressIndicator()
              : Text(text, style: const TextStyle(color: OnDexColors.inkDim)),
        ),
      );
}

/// Kategoriya chipi (ShoxPos binafsha badge o'rniga OnDex ohangi).
class CategoryChip extends StatelessWidget {
  const CategoryChip(this.category, {super.key});
  final String category;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
        decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(20)),
        child: Text(category.isEmpty ? '—' : '${categoryEmoji(category)} $category',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: OnDexColors.primaryPressed)),
      );
}

/// Qoldiq rangi: 0 — qizil, 1..5 — to'q sariq, aks holda yashil (ShoxPos).
Color qtyColor(num q) => q <= 0
    ? OnDexColors.danger
    : q <= 5
        ? const Color(0xFFF97316)
        : OnDexColors.success;

/// Kategoriya tanlash oynasi (ShoxPos CategoryModal).
Future<String?> pickCategory(BuildContext context, List<String> cats, String selected, {bool withAll = false}) {
  final options = [if (withAll) '', ...cats];
  return showDialog<String>(
    context: context,
    builder: (c) => Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 440, maxHeight: 560),
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          ListTile(
            title: const Text('Kategoriya tanlang', style: TextStyle(fontWeight: FontWeight.w800)),
            subtitle: Text('${cats.length} ta kategoriya mavjud'),
            trailing: IconButton(onPressed: () => Navigator.pop(c), icon: const Icon(Icons.close)),
          ),
          const Divider(height: 1),
          Flexible(
            child: options.isEmpty
                ? const Padding(padding: EdgeInsets.all(24), child: Text('Kategoriya yo\'q'))
                : ListView(shrinkWrap: true, padding: const EdgeInsets.all(10), children: [
                    for (final o in options)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 6),
                        child: ListTile(
                          key: ValueKey('cat-$o'),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                            side: BorderSide(color: o == selected ? OnDexColors.primary : OnDexColors.cardBorder),
                          ),
                          tileColor: o == selected ? OnDexColors.primaryTint : null,
                          leading: Text(o.isEmpty ? '🗂️' : categoryEmoji(o), style: const TextStyle(fontSize: 22)),
                          title: Text(o.isEmpty ? 'Barcha kategoriyalar' : o),
                          trailing: o == selected
                              ? const Icon(Icons.check_circle, color: OnDexColors.primary)
                              : const Icon(Icons.radio_button_unchecked, color: OnDexColors.inkFaint),
                          onTap: () => Navigator.pop(c, o),
                        ),
                      ),
                  ]),
          ),
        ]),
      ),
    ),
  );
}
