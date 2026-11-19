import 'package:flutter/material.dart';

import 'qurt_stats_page.dart';
import 'stores_page.dart';

/// Do'kon xizmati -> Do'konlar -> Qurt (egasi 2026-11-19: Qurt alohida xizmat
/// EMAS, do'kon turi). Ikki tab: qurt do'konlari va qurt statistikasi.
class QurtSection extends StatelessWidget {
  const QurtSection({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 2,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Material(
            color: Theme.of(context).colorScheme.surface,
            child: const TabBar(
              key: ValueKey('qurt-tabs'),
              isScrollable: true,
              tabAlignment: TabAlignment.start,
              tabs: [
                Tab(icon: Icon(Icons.storefront_outlined), text: 'Qurt do\'konlari'),
                Tab(icon: Icon(Icons.insights_outlined), text: 'Statistika'),
              ],
            ),
          ),
          const Expanded(
            child: TabBarView(
              children: [
                StoresPage(category: 'qurt', embedded: true),
                QurtStatsPage(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
