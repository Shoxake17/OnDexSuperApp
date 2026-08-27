import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum;

import '../format.dart';
import '../models/waiter_order.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';
import 'table_detail_screen.dart';

/// Band stollar — zaldagi holat bir qarashda.
///
/// ┌─ NEGA "BARCHA STOLLAR" EMAS ──────────────────────────────────────┐
/// Backend affitsiantga stollar ro'yxatini BERMAYDI: `/restaurants/{id}/
/// tables` faqat restoran va admin uchun (`routes_tables.go` boshidagi
/// izoh — affitsiant stollarning QR TOKENLARINI ko'ra olmasligi kerak,
/// aks holda u istalgan stol nomidan buyurtma bera olardi).
///
/// Shuning uchun bu ekran FAOL BUYURTMALARDAN yig'iladi: ustida hozir
/// buyurtmasi bor stol — band stol. Bo'sh stollar ko'rinmaydi, chunki
/// ilova ularning borligini bilmaydi. Bu yolg'on ma'lumot ko'rsatishdan
/// ko'ra halolroq: "bo'sh" deb ko'rsatilgan stol aslida boshqa
/// affitsiantning mijozlari o'tirgan stol bo'lishi mumkin edi.
/// └───────────────────────────────────────────────────────────────────┘
class TablesScreen extends StatelessWidget {
  const TablesScreen({super.key, required this.store});

  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    if (store.loading) {
      return const Center(child: CircularProgressIndicator());
    }

    final groups = store.tables;

    return RefreshIndicator(
      onRefresh: store.refresh,
      color: kBrandColor,
      backgroundColor: kSurface,
      child: groups.isEmpty
          ? ListView(
              children: const [
                EmptyState(
                  icon: Icons.table_restaurant_outlined,
                  title: 'Band stol yo\'q',
                  subtitle: 'Mijoz stoldagi QR kodni skanerlab buyurtma '
                      'berganda, stol shu yerda paydo bo\'ladi.',
                ),
              ],
            )
          : GridView.builder(
              padding: const EdgeInsets.fromLTRB(12, 12, 12, 24),
              gridDelegate:
                  const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 2,
                crossAxisSpacing: 12,
                mainAxisSpacing: 12,
                // ┌─ NEGA `childAspectRatio` EMAS ────────────────────┐
                // Nisbat katak balandligini EKRAN KENGLIGIDAN
                // hisoblaydi, ya'ni balandlik telefondan telefonga
                // o'zgaradi. Katak ichidagi matn esa o'zgarmaydi va
                // ustiga "tayyor" holatida BITTA QATOR qo'shiladi
                // (kutish vaqti) — shu sababli tor ekranda "BOTTOM
                // OVERFLOWED" chizig'i chiqqan edi.
                //
                // `mainAxisExtent` — aniq balandlik: eng baland
                // variant (stol nomi + holat + kutish vaqti + sanoq +
                // summa) uchun o'lchangan, qolgan hamma holatda ortiqcha
                // joy qoladi.
                // └───────────────────────────────────────────────────┘
                mainAxisExtent: 150,
              ),
              itemCount: groups.length,
              itemBuilder: (_, i) => _TableTile(
                group: groups[i],
                onTap: () => Navigator.of(context).push(
                  MaterialPageRoute(
                    builder: (_) => TableDetailScreen(
                      label: groups[i].label,
                      store: store,
                    ),
                  ),
                ),
              ),
            ),
    );
  }
}

class _TableTile extends StatelessWidget {
  const _TableTile({required this.group, required this.onTap});

  final TableGroup group;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    // Rang ustuvorligi: TAYYOR (harakat kerak) > tayyorlanmoqda
    // (kutish) > oddiy band. Maketda "berildi" holati eng yorqin
    // yashil edi — ya'ni ALLAQACHON tugagan ish eng ko'p e'tibor
    // tortardi; bu teskari ustuvorlik.
    final ready = group.hasReady;
    final preparing = !ready && group.hasPreparing;
    final accent = ready
        ? kReadyColor
        : preparing
            ? kWaitingColor
            : kInkGhost;
    final wait = group.longestReadyWait;

    return Material(
      color: ready ? const Color(0x142E9E4F) : kSurface,
      borderRadius: BorderRadius.circular(kRadiusCard),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(kRadiusCard),
        child: Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(kRadiusCard),
            border: Border.all(
              color: ready ? kReadyColor : kBorder,
              width: ready ? 1.6 : 1,
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      tableText(group.label),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: kInk,
                      ),
                    ),
                  ),
                  Container(
                    width: 9,
                    height: 9,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: accent,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                ready
                    ? 'Yetkazish kerak'
                    : preparing
                        ? 'Tayyorlanmoqda'
                        : 'Kutilmoqda',
                style: TextStyle(
                  fontSize: 12.5,
                  fontWeight: FontWeight.w600,
                  color: ready ? kReadyColor : kInkFaint,
                ),
              ),
              if (ready && wait != null) ...[
                const SizedBox(height: 3),
                Text(
                  waitBadge(wait),
                  style: TextStyle(
                    fontSize: 11.5,
                    color: wait.inMinutes >= 10 ? kDangerColor : kInkFaint,
                  ),
                ),
              ],
              const Spacer(),
              Row(
                children: [
                  if (group.partySize > 0) ...[
                    const Icon(Icons.people_outline,
                        size: 13, color: kInkGhost),
                    const SizedBox(width: 3),
                    Text(
                      '${group.partySize}',
                      style: const TextStyle(color: kInkGhost, fontSize: 12),
                    ),
                    const SizedBox(width: 10),
                  ],
                  const Icon(Icons.receipt_long, size: 13, color: kInkGhost),
                  const SizedBox(width: 3),
                  Text(
                    '${group.orders.length}',
                    style: const TextStyle(color: kInkGhost, fontSize: 12),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              Text(
                formatSum(group.totalTiyin),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: kInkDim,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
