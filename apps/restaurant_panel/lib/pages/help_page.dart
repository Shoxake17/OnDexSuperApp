import 'package:flutter/material.dart';
import 'package:ondex_support/ondex_support.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/page_header.dart';
import '../widgets/support_actions.dart';

/// Yordam markazi — savol-javob, qo'llanma, OnDex bilan aloqa va onlayn
/// yordam (Chat markazi).
///
/// ┌─ NIMA HAQIQIY ────────────────────────────────────────────────────┐
/// * "Biz bilan bog'laning" — OnDex administratori admin paneldagi
///   "Sozlamalar" da kiritgan telefon, Telegram va pochta. Kodga yozilgan
///   raqam YO'Q: avval bu yerda qattiq yozilgan raqam turardi va u
///   o'zgarsa har bir panelni qayta yig'ish kerak bo'lardi.
/// * Havolalar oq ro'yxat bo'yicha quriladi (`ondex_support/links.dart`).
/// * Savol-javob matnlari panelning HAQIQIY tugma va bo'lim nomlariga
///   tayanadi; "Video darsliklar" hali yo'qligi ochiq aytiladi.
/// └───────────────────────────────────────────────────────────────────┘
class HelpPage extends StatefulWidget {
  const HelpPage({
    super.key,
    this.supportUnread = 0,
    this.onOpenChat,
    this.onOpenOrders,
    this.onOpenMenu,
    this.onOpenTables,
    this.onOpenStatistics,
    this.onOpenSettings,
    this.onOpenStaff,
    this.onOpenUpdates,
  });

  /// O'qilmagan OnDex javoblari ("Onlayn yordam" tugmasida).
  final int supportUnread;
  final VoidCallback? onOpenChat;
  final VoidCallback? onOpenOrders;
  final VoidCallback? onOpenMenu;
  final VoidCallback? onOpenTables;
  final VoidCallback? onOpenStatistics;
  final VoidCallback? onOpenSettings;
  final VoidCallback? onOpenStaff;
  final VoidCallback? onOpenUpdates;

  @override
  State<HelpPage> createState() => _HelpPageState();
}

enum _Target { orders, menu, tables, statistics, settings, staff }

class _Faq {
  const _Faq({
    required this.icon,
    required this.color,
    required this.background,
    required this.question,
    required this.subtitle,
    required this.answer,
    this.target,
    this.actionLabel = '',
  });

  final IconData icon;
  final Color color;
  final Color background;
  final String question;
  final String subtitle;
  final String answer;
  final _Target? target;
  final String actionLabel;
}

const _faqs = <_Faq>[
  _Faq(
    icon: Icons.shopping_cart_outlined,
    color: OnDexColors.info,
    background: OnDexColors.infoBg,
    question: 'Buyurtmani qanday qo\'yish mumkin?',
    subtitle: 'Mijoz uchun buyurtma jarayoni haqida ma\'lumot.',
    answer: 'Mijoz buyurtmani OnDex ilovasi, Telegram mini-ilovasi yoki stol ustidagi QR kod orqali beradi; '
        'affitsiant ham o\'z ilovasidan stol uchun buyurtma yubora oladi. Yangi buyurtma "Buyurtmalar" '
        'bo\'limida ovozli signal bilan paydo bo\'ladi: "Qabul qilish" ni bosib tayyorlash vaqtini kiriting, '
        'so\'ng "Tayyorlashni boshlash" va "Tayyorlandi" tugmalari bilan holatni yangilang. Yetkazib berish '
        'buyurtmasiga kuryer avtomatik qidiriladi.',
    target: _Target.orders,
    actionLabel: 'Buyurtmalarga o\'tish',
  ),
  _Faq(
    icon: Icons.format_list_bulleted_rounded,
    color: OnDexColors.info,
    background: OnDexColors.infoBg,
    question: 'Menyuni qanday qo\'shish yoki o\'zgartirish mumkin?',
    subtitle: 'Menyu bilan ishlash bo\'yicha ko\'rsatmalar.',
    answer: '"Menyu" bo\'limida "Mahsulot qo\'shish" tugmasini bosing: nomi, narxi, turkumi va rasmini kiriting. '
        'Mavjud taomni o\'zgartirish uchun uning kartochkasini oching. Taom vaqtincha tugasa, uni o\'chirmang — '
        '"Nofaol qilish" yetarli: mijozlarga "Tugadi" deb ko\'rinadi va keyin bir bosishda qaytariladi.',
    target: _Target.menu,
    actionLabel: 'Menyuga o\'tish',
  ),
  _Faq(
    icon: Icons.credit_card_rounded,
    color: OnDexColors.info,
    background: OnDexColors.infoBg,
    question: 'Kassa hisobi va to\'lov usullari',
    subtitle: 'Naqd pul, karta va boshqa to\'lov turlari haqida.',
    answer: 'Qabul qilinadigan to\'lov usullarini "Restoran sozlamalari" → "To\'lov usullari" bo\'limida '
        'belgilaysiz ("Naqd pul", "Karta (Terminal)"). Onlayn karta to\'lovi qabul qilinsa, '
        '"Bildirishnomalar" da "To\'lov qabul qilindi" xabari chiqadi. Tushum va buyurtmalar soni '
        '"Statistika" bo\'limida ko\'rinadi.',
    target: _Target.settings,
    actionLabel: 'Sozlamalarga o\'tish',
  ),
  _Faq(
    icon: Icons.groups_rounded,
    color: OnDexColors.info,
    background: OnDexColors.infoBg,
    question: 'Xodimlarni qanday qo\'shish mumkin?',
    subtitle: 'Yangi xodimni tizimga kiritish va ruxsat berish.',
    answer: '"Xodimlar" bo\'limida "Xodim qo\'shish" tugmasini bosing va ism, lavozim hamda telefon raqamini '
        'kiriting. Affitsiant lavozimidagi xodim shu telefon raqami bilan affitsiant ilovasiga kiradi. '
        'Xodim ta\'tilga chiqarilsa yoki ishdan bo\'shatilsa, uning ilovaga kirishi avtomatik yopiladi.',
    target: _Target.staff,
    actionLabel: 'Xodimlarga o\'tish',
  ),
  _Faq(
    icon: Icons.insights_rounded,
    color: OnDexColors.purple,
    background: OnDexColors.purpleBg,
    question: 'Statistika va hisobotlar',
    subtitle: 'Savdo va moliyaviy hisobotlarni ko\'rish va eksport qilish.',
    answer: '"Statistika" bo\'limida davrni tanlang: jami tushum, jami buyurtmalar, o\'rtacha buyurtma va eng '
        'ko\'p sotilgan mahsulotlar ko\'rinadi. Yuqoridagi "Eksport" tugmasi hisobotni yuklab oladi. '
        'Kunlik hisobot har kuni "Bildirishnomalar" ga ham keladi.',
    target: _Target.statistics,
    actionLabel: 'Statistikaga o\'tish',
  ),
  _Faq(
    icon: Icons.print_outlined,
    color: OnDexColors.inkDim,
    background: OnDexColors.pageBg,
    question: 'Printer va QR kod ishlamayapti',
    subtitle: 'Uskunalar bilan bog\'liq muammolar yechimi.',
    answer: '"Stollar (QR)" bo\'limida joyni tanlab "QR kodni yuklab olish" orqali PDF ni saqlang va istalgan '
        'printerda chop eting. Chop etilmasa: printer yoqilganini, qog\'oz borligini va Windows\'da standart '
        'printer qilib tanlanganini tekshiring. QR skanerlanmasa — kodni kattaroq o\'lchamda, yaltiramaydigan '
        'qog\'ozga chop eting. Muammo qolsa, "Onlayn yordam" orqali yozing.',
    target: _Target.tables,
    actionLabel: 'Stollarga o\'tish',
  ),
  _Faq(
    icon: Icons.bolt_rounded,
    color: OnDexColors.amber,
    background: OnDexColors.amberBg,
    question: 'Tizim sekin ishlayapti',
    subtitle: 'Tezlashtirish bo\'yicha tavsiyalar.',
    answer: 'Avval internet ulanishini tekshiring. Panelni yopib qayta oching — jonli ulanish tiklanadi va '
        'uzilish paytidagi buyurtmalar avtomatik yuklanadi. Kompyuterdagi boshqa og\'ir dasturlarni yoping. '
        'Muammo davom etsa, qaysi bo\'limda va qachondan beri sekinlashganini "Onlayn yordam" ga yozing.',
  ),
];

/// Qidiruv: katta-kichik harf va o'zbekcha tutuq belgisining turli
/// shakllari (ʻ ʼ ’ ‘ `) farq qilmasin.
String _normalize(String s) => s.toLowerCase().replaceAll(RegExp('[ʻʼ’‘`]'), '\'').trim();

class _HelpPageState extends State<HelpPage> {
  final _search = TextEditingController();
  String _query = '';
  SupportContacts _contacts = const SupportContacts();
  bool _loadingContacts = true;
  bool _contactsFailed = false;
  int? _expanded;

  @override
  void initState() {
    super.initState();
    _search.addListener(_onSearch);
    _loadContacts();
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  void _onSearch() {
    final q = _normalize(_search.text);
    if (q != _query || (_search.text.isEmpty != _query.isEmpty)) {
      setState(() {
        _query = q;
        _expanded = null;
      });
    }
  }

  Future<void> _loadContacts({bool retry = false}) async {
    if (retry) {
      setState(() {
        _loadingContacts = true;
        _contactsFailed = false;
      });
    }
    try {
      final j = await api.supportContacts();
      if (!mounted) return;
      setState(() {
        _contacts = SupportContacts.fromJson(j);
        _loadingContacts = false;
        _contactsFailed = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _loadingContacts = false;
        _contactsFailed = true;
      });
    }
  }

  bool _matches(_Faq f) =>
      _query.isEmpty || _normalize('${f.question} ${f.subtitle} ${f.answer}').contains(_query);

  VoidCallback? _callbackFor(_Target? t) => switch (t) {
        _Target.orders => widget.onOpenOrders,
        _Target.menu => widget.onOpenMenu,
        _Target.tables => widget.onOpenTables,
        _Target.statistics => widget.onOpenStatistics,
        _Target.settings => widget.onOpenSettings,
        _Target.staff => widget.onOpenStaff,
        null => null,
      };

  Future<void> _showSections(String title, IconData icon, Color color, List<(String, String)> sections,
      {List<Widget> Function(BuildContext ctx)? actions}) {
    return showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        titlePadding: const EdgeInsets.fromLTRB(24, 22, 24, 8),
        title: Row(
          children: [
            Icon(icon, color: color),
            const SizedBox(width: 10),
            Expanded(child: Text(title, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800))),
          ],
        ),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                for (final (heading, body) in sections) ...[
                  if (heading.isNotEmpty)
                    Text(heading,
                        style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                  const SizedBox(height: 4),
                  Text(body, style: const TextStyle(fontSize: 13.5, height: 1.5, color: OnDexColors.inkDim)),
                  const SizedBox(height: 14),
                ],
              ],
            ),
          ),
        ),
        actions: [
          ...?actions?.call(ctx),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Tushunarli')),
        ],
      ),
    );
  }

  void _showGuide() => _showSections('Foydalanuvchi qo\'llanmasi', Icons.menu_book_rounded, const Color(0xFF2F6FED), const [
        (
          'Buyurtmalar',
          'Yangi buyurtma ovozli signal bilan keladi. "Qabul qilish" da tayyorlash vaqtini kiriting, so\'ng '
              '"Tayyorlashni boshlash" va "Tayyorlandi" bilan holatni yangilang. Yetkazib berish buyurtmasiga '
              'kuryer avtomatik qidiriladi; topilmasa, kartada qayta qidirish yoki bekor qilish tugmalari chiqadi.'
        ),
        (
          'Menyu va aksiyalar',
          '"Mahsulot qo\'shish" bilan taom qo\'shing; tugagan taomni "Nofaol qilish" bilan yashiring. '
              '"Aksiyalar" bo\'limida "Yangi aksiya yaratish" orqali mahsulot, turkum yoki butun buyurtmaga '
              'chegirma belgilang.'
        ),
        (
          'Stollar (QR)',
          '"Yangi stol qo\'shish" bilan joy yarating va "QR kodni yuklab olish" orqali PDF ni chop eting. '
              'Mijoz QR kodni skanerlab to\'g\'ridan-to\'g\'ri shu stolga buyurtma beradi.'
        ),
        (
          'Xodimlar',
          '"Xodim qo\'shish" bilan xodim kiriting. Affitsiant o\'z telefon raqami bilan affitsiant ilovasiga kiradi.'
        ),
        (
          'Statistika va bildirishnomalar',
          '"Statistika" da davrni tanlab tushum va buyurtmalarni ko\'ring, "Eksport" bilan yuklab oling. '
              'Buyurtma, to\'lov, xodimlar va kunlik hisobot xabarlari "Bildirishnomalar" da saqlanadi.'
        ),
        (
          'Chat markazi',
          'OnDex administratori bilan yozishma. Javob kelganda yon menyudagi "Chat markazi" yonida raqam chiqadi.'
        ),
      ]);

  void _showVideos() => _showSections(
        'Video darsliklar',
        Icons.play_circle_fill_rounded,
        OnDexColors.success,
        const [
          (
            '',
            'Video darsliklar hali joylanmagan — tayyor bo\'lgach shu yerda paydo bo\'ladi. Hozircha yozma '
                'qo\'llanmadan foydalaning yoki savolingizni onlayn yordamga yozing.'
          ),
        ],
        actions: (ctx) => [
          TextButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              _showGuide();
            },
            child: const Text('Qo\'llanmani ochish'),
          ),
          if (widget.onOpenChat != null)
            TextButton(
              onPressed: () {
                Navigator.of(ctx).pop();
                widget.onOpenChat!();
              },
              child: const Text('Onlayn yordam'),
            ),
        ],
      );

  void _showTips() => _showSections('Tezkor maslahatlar', Icons.tips_and_updates_rounded, OnDexColors.purple, const [
        (
          'Restoranni vaqtincha yopish',
          'Yuqori paneldagi ochiq/yopiq belgisini o\'chiring — yangi buyurtmalar kelmaydi, mavjudlari davom etadi.'
        ),
        ('Tugagan taom', 'Taomni o\'chirmang: "Nofaol qilish" yetarli, keyin bir bosishda qaytarasiz.'),
        (
          'Buyurtma ovozi',
          'Brauzer versiyasida ovoz birinchi bosishdan keyin yoqiladi — ish boshida panelga bir marta bosing.'
        ),
        (
          'Eski buyurtmalar',
          '"Buyurtmalar" dagi "Tarix" yoki "Statistika" ichidagi barcha buyurtmalar ro\'yxatidan davr bo\'yicha toping.'
        ),
        (
          'Tezroq yordam',
          '"Onlayn yordam" ga yozganda buyurtma raqami va nima sodir bo\'lganini qo\'shing.'
        ),
      ]);

  @override
  Widget build(BuildContext context) {
    final visible = [for (var i = 0; i < _faqs.length; i++) if (_matches(_faqs[i])) i];
    return SingleChildScrollView(
      padding: kPagePadding,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          PageHeader(
            title: 'Yordam markazi',
            subtitle: 'Savollaringizga javob toping yoki biz bilan bog\'laning',
            narrowBelow: 900,
            action: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 460),
              child: _SearchField(controller: _search),
            ),
          ),
          const SizedBox(height: 22),
          _ResourceGrid(
            onGuide: _showGuide,
            onVideos: _showVideos,
            onTips: _showTips,
            onUpdates: widget.onOpenUpdates,
          ),
          const SizedBox(height: 20),
          LayoutBuilder(builder: (context, box) {
            final faq = _FaqCard(
              visible: visible,
              expanded: _expanded,
              onToggle: (i) => setState(() => _expanded = _expanded == i ? null : i),
              actionFor: (f) => _callbackFor(f.target),
              onOpenChat: widget.onOpenChat,
            );
            final side = Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _ContactCard(
                  contacts: _contacts,
                  loading: _loadingContacts,
                  failed: _contactsFailed,
                  onRetry: () => _loadContacts(retry: true),
                ),
                const SizedBox(height: 16),
                _OnlineSupportCard(unread: widget.supportUnread, onOpenChat: widget.onOpenChat),
              ],
            );
            if (box.maxWidth >= 1040) {
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(flex: 3, child: faq),
                  const SizedBox(width: 20),
                  Expanded(flex: 2, child: side),
                ],
              );
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [faq, const SizedBox(height: 20), side],
            );
          }),
          const SizedBox(height: 24),
          const _Footer(),
        ],
      ),
    );
  }
}

class _SearchField extends StatelessWidget {
  const _SearchField({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    OutlineInputBorder border(Color c) =>
        OutlineInputBorder(borderRadius: BorderRadius.circular(12), borderSide: BorderSide(color: c));
    return TextField(
      key: const ValueKey('help-search'),
      controller: controller,
      decoration: InputDecoration(
        hintText: 'Savol yoki kalit so\'z kiriting...',
        hintStyle: const TextStyle(color: OnDexColors.inkFaint, fontSize: 14),
        prefixIcon: const Icon(Icons.search_rounded, color: OnDexColors.inkDim),
        suffixIcon: controller.text.isEmpty
            ? null
            : IconButton(tooltip: 'Tozalash', icon: const Icon(Icons.close_rounded), onPressed: controller.clear),
        filled: true,
        fillColor: OnDexColors.cardBg,
        isDense: true,
        contentPadding: const EdgeInsets.symmetric(vertical: 14),
        border: border(OnDexColors.cardBorder),
        enabledBorder: border(OnDexColors.cardBorder),
        focusedBorder: border(OnDexColors.primary),
      ),
    );
  }
}

class _ResourceGrid extends StatelessWidget {
  const _ResourceGrid({required this.onGuide, required this.onVideos, required this.onTips, this.onUpdates});

  final VoidCallback onGuide;
  final VoidCallback onVideos;
  final VoidCallback onTips;
  final VoidCallback? onUpdates;

  @override
  Widget build(BuildContext context) {
    final cards = [
      _ResourceCard(
        icon: Icons.menu_book_rounded,
        color: const Color(0xFF2F6FED),
        background: const Color(0xFFE6EEFD),
        title: 'Foydalanuvchi qo\'llanmasi',
        description: 'Panel bo\'limlari bo\'yicha qisqa va aniq qo\'llanma.',
        onTap: onGuide,
      ),
      _ResourceCard(
        icon: Icons.play_circle_fill_rounded,
        color: OnDexColors.success,
        background: OnDexColors.successBg,
        title: 'Video darsliklar',
        description: 'Tayyorlanmoqda — hozircha yozma qo\'llanmadan foydalaning.',
        onTap: onVideos,
      ),
      _ResourceCard(
        icon: Icons.tips_and_updates_rounded,
        color: OnDexColors.purple,
        background: OnDexColors.purpleBg,
        title: 'Tezkor maslahatlar',
        description: 'Kundalik ishni tezlashtiradigan foydali maslahatlar.',
        onTap: onTips,
      ),
      _ResourceCard(
        icon: Icons.system_update_alt_rounded,
        color: OnDexColors.primary,
        background: OnDexColors.primaryTint,
        title: 'Dastur yangilanishlari',
        description: 'OnDex yangiliklari "Bildirishnomalar" bo\'limida.',
        onTap: onUpdates,
      ),
    ];
    return LayoutBuilder(builder: (context, box) {
      final cols = box.maxWidth >= 1100 ? 4 : (box.maxWidth >= 560 ? 2 : 1);
      final width = (box.maxWidth - 16 * (cols - 1)) / cols;
      return Wrap(
        spacing: 16,
        runSpacing: 16,
        children: [for (final c in cards) SizedBox(width: width, child: c)],
      );
    });
  }
}

class _ResourceCard extends StatelessWidget {
  const _ResourceCard({
    required this.icon,
    required this.color,
    required this.background,
    required this.title,
    required this.description,
    required this.onTap,
  });

  final IconData icon;
  final Color color;
  final Color background;
  final String title;
  final String description;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: OnDexColors.cardBg,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: const BorderSide(color: OnDexColors.cardBorder),
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 104),
          child: Padding(
            padding: const EdgeInsets.all(18),
            child: Row(
              children: [
                Container(
                  width: 52,
                  height: 52,
                  decoration: BoxDecoration(color: background, borderRadius: BorderRadius.circular(14)),
                  child: Icon(icon, color: color, size: 28),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                      const SizedBox(height: 6),
                      Text(description,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim, height: 1.4)),
                    ],
                  ),
                ),
                const SizedBox(width: 6),
                const Icon(Icons.chevron_right_rounded, color: OnDexColors.inkDim),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _FaqCard extends StatelessWidget {
  const _FaqCard({
    required this.visible,
    required this.expanded,
    required this.onToggle,
    required this.actionFor,
    this.onOpenChat,
  });

  final List<int> visible;
  final int? expanded;
  final ValueChanged<int> onToggle;
  final VoidCallback? Function(_Faq) actionFor;
  final VoidCallback? onOpenChat;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      padding: const EdgeInsets.only(bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(20, 18, 20, 8),
            child: Text('Tez-tez so\'raladigan savollar',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          ),
          if (visible.isEmpty)
            Padding(
              key: const ValueKey('help-no-results'),
              padding: const EdgeInsets.fromLTRB(20, 24, 20, 24),
              child: Column(
                children: [
                  const Icon(Icons.search_off_rounded, size: 36, color: OnDexColors.inkFaint),
                  const SizedBox(height: 10),
                  const Text('Hech narsa topilmadi',
                      style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                  const SizedBox(height: 4),
                  const Text('Boshqa so\'z bilan qidiring yoki savolingizni to\'g\'ridan-to\'g\'ri bizga yozing.',
                      textAlign: TextAlign.center, style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
                  if (onOpenChat != null) ...[
                    const SizedBox(height: 12),
                    FilledButton.icon(
                      onPressed: onOpenChat,
                      icon: const Icon(Icons.headset_mic_rounded, size: 18),
                      label: const Text('Onlayn yordam'),
                    ),
                  ],
                ],
              ),
            )
          else
            for (final i in visible)
              _FaqTile(
                index: i,
                faq: _faqs[i],
                expanded: expanded == i,
                onToggle: () => onToggle(i),
                onAction: actionFor(_faqs[i]),
                last: i == visible.last,
              ),
        ],
      ),
    );
  }
}

class _FaqTile extends StatelessWidget {
  const _FaqTile({
    required this.index,
    required this.faq,
    required this.expanded,
    required this.onToggle,
    required this.onAction,
    required this.last,
  });

  final int index;
  final _Faq faq;
  final bool expanded;
  final VoidCallback onToggle;
  final VoidCallback? onAction;
  final bool last;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        InkWell(
          key: ValueKey('faq-$index'),
          onTap: onToggle,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
            child: Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(color: faq.background, borderRadius: BorderRadius.circular(10)),
                  child: Icon(faq.icon, color: faq.color, size: 21),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(faq.question,
                          style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                      const SizedBox(height: 3),
                      Text(faq.subtitle, style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                    ],
                  ),
                ),
                AnimatedRotation(
                  turns: expanded ? 0.25 : 0,
                  duration: const Duration(milliseconds: 180),
                  child: const Icon(Icons.chevron_right_rounded, color: OnDexColors.inkDim),
                ),
              ],
            ),
          ),
        ),
        AnimatedSize(
          duration: const Duration(milliseconds: 180),
          alignment: Alignment.topCenter,
          child: expanded
              ? Padding(
                  key: ValueKey('faq-answer-$index'),
                  padding: const EdgeInsets.fromLTRB(74, 0, 20, 14),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(faq.answer, style: const TextStyle(fontSize: 13, height: 1.55, color: OnDexColors.ink)),
                      if (onAction != null && faq.actionLabel.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: TextButton.icon(
                            onPressed: onAction,
                            icon: const Icon(Icons.arrow_forward_rounded, size: 16),
                            label: Text(faq.actionLabel),
                          ),
                        ),
                    ],
                  ),
                )
              : const SizedBox(width: double.infinity),
        ),
        if (!last) const Divider(height: 1, indent: 20, endIndent: 20, color: OnDexColors.cardBorder),
      ],
    );
  }
}

class _ContactCard extends StatelessWidget {
  const _ContactCard({required this.contacts, required this.loading, required this.failed, required this.onRetry});

  final SupportContacts contacts;
  final bool loading;
  final bool failed;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final rows = <Widget>[
      if (contacts.hasPhone)
        SupportContactRow(
          key: const ValueKey('help-contact-phone'),
          icon: Icons.phone_rounded,
          color: OnDexColors.primary,
          title: formatSupportPhone(contacts.phone),
          subtitle: contacts.phoneHours,
          actionLabel: 'Qo\'ng\'iroq qilish',
          onAction: () =>
              openSupportLink(context, contacts.telUri, copyValue: formatSupportPhone(contacts.phone)),
        ),
      if (contacts.hasTelegram)
        SupportContactRow(
          key: const ValueKey('help-contact-telegram'),
          icon: Icons.send_rounded,
          color: const Color(0xFF229ED9),
          title: 'Telegram',
          subtitle: '@${contacts.telegram}',
          actionLabel: 'O\'tish',
          onAction: () => openSupportLink(context, contacts.telegramUri, copyValue: '@${contacts.telegram}'),
        ),
      if (contacts.hasEmail)
        SupportContactRow(
          key: const ValueKey('help-contact-email'),
          icon: Icons.mail_rounded,
          color: OnDexColors.purple,
          title: contacts.email,
          subtitle: contacts.emailNote,
          actionLabel: 'Xat yuborish',
          onAction: () => openSupportLink(context, contacts.mailUri, copyValue: contacts.email),
        ),
    ];

    Widget body;
    if (loading) {
      body = const Padding(
        padding: EdgeInsets.all(20),
        child: Center(child: SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2.5))),
      );
    } else if (failed) {
      body = Row(
        children: [
          const Expanded(
            child: Text('Aloqa ma\'lumotlarini yuklab bo\'lmadi.',
                style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
          ),
          TextButton(onPressed: onRetry, child: const Text('Qayta urinish')),
        ],
      );
    } else if (rows.isEmpty) {
      body = Container(
        key: const ValueKey('help-contacts-empty'),
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(color: OnDexColors.pageBg, borderRadius: BorderRadius.circular(12)),
        child: const Text(
          'Aloqa ma\'lumotlari hali kiritilmagan. Savolingizni "Onlayn yordam" orqali yozing — javob shu yerda keladi.',
          style: TextStyle(fontSize: 13, color: OnDexColors.inkDim, height: 1.45),
        ),
      );
    } else {
      body = Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var i = 0; i < rows.length; i++) ...[
            if (i > 0) const SizedBox(height: 10),
            rows[i],
          ],
        ],
      );
    }

    return Container(
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            padding: const EdgeInsets.fromLTRB(20, 18, 16, 14),
            decoration: const BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [Color(0xFFFDF1E7), OnDexColors.cardBg],
              ),
            ),
            child: const Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Biz bilan bog\'laning',
                          style: TextStyle(fontSize: 17, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                      SizedBox(height: 6),
                      Text('Agar savollaringiz bo\'lsa, biz har doim yordam berishga tayyormiz.',
                          style: TextStyle(fontSize: 13, color: OnDexColors.inkDim, height: 1.4)),
                    ],
                  ),
                ),
                SizedBox(width: 12),
                Icon(Icons.headset_mic_rounded, size: 52, color: OnDexColors.primary),
              ],
            ),
          ),
          Padding(padding: const EdgeInsets.fromLTRB(16, 4, 16, 16), child: body),
        ],
      ),
    );
  }
}

class _OnlineSupportCard extends StatelessWidget {
  const _OnlineSupportCard({required this.unread, this.onOpenChat});

  final int unread;
  final VoidCallback? onOpenChat;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(color: OnDexColors.infoBg, borderRadius: BorderRadius.circular(12)),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: const BoxDecoration(color: OnDexColors.cardBg, shape: BoxShape.circle),
                  child: const Icon(Icons.lightbulb_rounded, color: OnDexColors.amber, size: 20),
                ),
                const SizedBox(width: 12),
                const Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Maslahat',
                          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                      SizedBox(height: 4),
                      Text(
                        'Muammoni yozayotganda qaysi bo\'lim, buyurtma raqami va nima sodir bo\'lganini aniq '
                        'ko\'rsating — bu yechimni ancha tezlashtiradi.',
                        style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim, height: 1.45),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          Material(
            color: OnDexColors.primary,
            borderRadius: BorderRadius.circular(12),
            child: InkWell(
              key: const ValueKey('help-online-support'),
              borderRadius: BorderRadius.circular(12),
              onTap: onOpenChat,
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                child: Row(
                  children: [
                    const Icon(Icons.headset_mic_rounded, color: Colors.white, size: 26),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Text('Onlayn yordam',
                              style: TextStyle(color: Colors.white, fontSize: 15, fontWeight: FontWeight.w800)),
                          const SizedBox(height: 2),
                          Text(unread > 0 ? '$unread ta yangi javob' : 'Hozir bog\'laning',
                              style: const TextStyle(color: Color(0xE6FFFFFF), fontSize: 12.5)),
                        ],
                      ),
                    ),
                    const Icon(Icons.chevron_right_rounded, color: Colors.white),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Footer extends StatelessWidget {
  const _Footer();

  @override
  Widget build(BuildContext context) {
    // Har reliz +1 (`scripts/version.ps1`): "OnDex Restaurant v0.1.2 (2)".
    final version = 'OnDex Restaurant $appVersionLabel';
    return SizedBox(
      width: double.infinity,
      child: Wrap(
        alignment: WrapAlignment.spaceBetween,
        runSpacing: 8,
        children: [
          Text(version, style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint)),
          const Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Tor oynada matn qatorga bo'linadi, yurakcha chetdan chiqmaydi.
              Flexible(
                child: Text('Biz sizning muvaffaqiyatingiz uchun ishlaymiz',
                    style: TextStyle(fontSize: 12, color: OnDexColors.inkFaint)),
              ),
              SizedBox(width: 4),
              Icon(Icons.favorite_rounded, size: 12, color: OnDexColors.primary),
            ],
          ),
        ],
      ),
    );
  }
}
