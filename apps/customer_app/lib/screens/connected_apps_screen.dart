import 'package:flutter/material.dart';

import '../api.dart';
import '../data/ai_tools.dart';
import '../widgets/common.dart';
import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import 'catalog_screen.dart' show kBrand;

/// Ulangan ilovalar — tashqi AI yordamchilarga berilgan ruxsatlar.
///
/// ┌─ BU EKRAN NIMA UCHUN BOR ──────────────────────────────────────────┐
/// Tashqi yordamchi (masalan Shaddiy AI) foydalanuvchi nomidan
/// buyurtma bera oladi. Bunday huquqning butun xavfsizligi ikki
/// shartga tayanadi:
///
///   1. odam NIMAGA rozi bo'layotganini ko'rgan bo'lishi kerak;
///   2. istalgan vaqtda BIR TUGMA bilan to'xtata olishi kerak.
///
/// Serverdagi tekshiruvlar qanchalik kuchli bo'lmasin, bu ikkisisiz
/// tizim "ishonchli" deb atalmaydi — shuning uchun bu ekran
/// qulaylik emas, himoyaning bir qismi.
///
/// Backend: `internal/httpapi/routes_me_agent.go`
/// └────────────────────────────────────────────────────────────────────┘
class ConnectedAppsScreen extends StatefulWidget {
  /// Deep link (`ondex://agent-link?code=...`) orqali kelinganda kod
  /// darhol beriladi va rozilik oynasi o'zi ochiladi.
  final String? initialCode;

  const ConnectedAppsScreen({super.key, this.initialCode});

  @override
  State<ConnectedAppsScreen> createState() => _ConnectedAppsScreenState();
}

class _ConnectedAppsScreenState extends State<ConnectedAppsScreen> {
  List<Map<String, dynamic>>? _grants;
  List<Map<String, dynamic>> _drafts = const [];
  List<Map<String, dynamic>> _activity = const [];
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _load();
    final code = widget.initialCode;
    if (code != null && code.trim().isNotEmpty) {
      // Ekran chizilib bo'lgach ochiladi — `initState` ichida
      // dialog ko'rsatib bo'lmaydi.
      WidgetsBinding.instance.addPostFrameCallback((_) => _openConsent(code));
    }
  }

  Future<void> _load() async {
    setState(() => _failed = false);
    try {
      // Uchtasi parallel — ketma-ket so'ralganda ekran sezilarli
      // sekin ochilardi.
      final results = await Future.wait([
        api.agentGrants(),
        api.agentDrafts(),
        api.agentActivity(),
      ]);
      if (!mounted) return;
      setState(() {
        _grants = results[0].cast<Map<String, dynamic>>();
        _drafts = results[1].cast<Map<String, dynamic>>();
        _activity = results[2].cast<Map<String, dynamic>>();
      });
    } catch (_) {
      if (mounted) setState(() => _failed = true);
    }
  }

  // ── Rozilik (deep link orqali keladi) ──

  // ESLATMA: kodni QO'LDA kiritish oynasi olib tashlandi — bu ekran
  // endi Shaddiy ruxsatlari haqida. Tashqi ilova baribir deep link
  // (ondex://agent-link?code=...) orqali ulanadi va rozilik oynasi
  // o'sha yo'lda ochiladi (initialCode).

  Future<void> _openConsent(String code) async {
    Map<String, dynamic> info;
    try {
      info = await api.agentLinkInfo(code);
    } catch (e) {
      if (mounted) {
        _snack(context, errorText(e, 'Kod yaroqsiz yoki muddati tugagan'));
      }
      return;
    }
    if (!mounted) return;
    final approved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _ConsentSheet(code: code, info: info),
    );
    if (approved == true) {
      await _load();
      if (mounted) _snack(context, 'Ulandi');
    }
  }

  // ── Uzish ──

  Future<void> _revoke(Map<String, dynamic> g) async {
    final name = (g['partner_name'] ?? 'Ilova').toString();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('$name uzilsinmi?'),
        content: const Text(
          'Ilova endi sizning nomingizdan buyurtma bera olmaydi va '
          'buyurtmalaringizni ko\'ra olmaydi. Bu darhol kuchga kiradi.',
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Bekor')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Uzish'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await api.agentRevoke(g['id'].toString());
      await _load();
      if (mounted) _snack(context, 'Uzildi');
    } catch (e) {
      if (mounted) _snack(context, errorText(e, 'Uzib bo\'lmadi'));
    }
  }

  // ── Tasdiq kutayotgan buyurtma ──

  Future<void> _decideDraft(Map<String, dynamic> d, bool approve) async {
    try {
      if (approve) {
        await api.agentApproveDraft(d['id'].toString());
      } else {
        await api.agentRejectDraft(d['id'].toString());
      }
      await _load();
      if (mounted) {
        _snack(context, approve ? 'Buyurtma berildi' : 'Rad etildi');
      }
    } catch (e) {
      if (mounted) _snack(context, errorText(e, 'Amal bajarilmadi'));
    }
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: const PageAppBar(
          centerTitle: true,
          titleWidget: Text('Ruxsat berilgan',
              style: TextStyle(fontSize: 19, fontWeight: FontWeight.bold)),
        ),
        body: RefreshIndicator(
          color: kBrand,
          onRefresh: _load,
          child: _buildBody(),
        ),
        // ┌─ "KOD BILAN ULASH" OLIB TASHLANDI ────────────────────────┐
        // U TASHQI ilovani ulash uchun edi. Bu ekran endi ilovaning
        // O'Z yordamchisi — Shaddiy — nimalar qila olishini
        // boshqaradi, tashqi integratsiya emas. Ulash oqimi deep
        // link orqali baribir ishlaydi (`initialCode`), ya'ni
        // imkoniyat yo'qolmadi, faqat bu ekrandan olib tashlandi.
        // └───────────────────────────────────────────────────────────┘
      ),
    );
  }

  Widget _buildBody() {
    if (_failed) {
      return ErrorViewList(
        message: 'Ma\'lumotni yuklab bo\'lmadi.',
        onRetry: _load,
      );
    }
    final grants = _grants;
    if (grants == null) {
      return const Center(child: CircularProgressIndicator(color: kBrand));
    }

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 96),
      children: [
        // Tasdiq kutayotganlar ENG TEPADA: bu yagona bo'lim bo'lib,
        // foydalanuvchidan HOZIR harakat kutiladi.
        if (_drafts.isNotEmpty) ...[
          const _SectionTitle('Tasdiqlashingiz kutilmoqda'),
          for (final d in _drafts) _DraftCard(draft: d, onDecide: _decideDraft),
          const SizedBox(height: 20),
        ],

        // ┌─ ILOVA ICHIDAGI YORDAMCHI ──────────────────────────────┐
        // Yuqoridagi bo'limlar TASHQI ilovalar haqida. Bu esa
        // ilovaning O'Z yordamchisi (Shaddiy): u grant talab
        // qilmaydi, lekin nima qila olishini foydalanuvchi
        // boshqarishi kerak.
        // └──────────────────────────────────────────────────────────┘
        const _SectionTitle('Shaddiy nimalar qila oladi'),
        const _AiToolsSection(),

        // ┌─ TASHQI ILOVALAR BO'LIMI ENDI FAQAT KERAK BO'LGANDA ─────┐
        // Bu bo'lim ilgari HAR DOIM turardi va odatda bo'sh edi —
        // ekran esa Shaddiy'ning ruxsatlari haqida, ya'ni ikki xil
        // "ruxsat" bir sahifada chalkash ko'rinardi.
        //
        // BUTUNLAY o'chirilmadi va bu ATAYLAB: ro'yxat yo'qolsa,
        // berilgan ruxsatni BEKOR QILISH yo'li ham yo'qolardi.
        // Ruxsat berish oson, uzish esa imkonsiz bo'lgan tizim
        // xavfsiz emas.
        //
        // Shuning uchun qoida shunday: faol grant BO'LMASA bo'lim
        // umuman chizilmaydi (odatdagi holat), bo'lsa — ko'rinadi va
        // uziladi.
        // └──────────────────────────────────────────────────────────┘
        if (grants.any((g) => g['status'] == 'active')) ...[
          const SizedBox(height: 24),
          const _SectionTitle('Tashqi ilovalar'),
          for (final g in grants.where((g) => g['status'] == 'active'))
            _GrantCard(grant: g, onRevoke: () => _revoke(g)),
        ],

        if (_activity.isNotEmpty) ...[
          const SizedBox(height: 24),
          const _SectionTitle('So\'nggi amallar'),
          // Audit — ishonchning asosi: odam agent nima qilganini
          // KEYIN ham ko'ra olishi kerak.
          for (final a in _activity.take(20)) _ActivityRow(entry: a),
        ],
      ],
    );
  }
}

// ── Rozilik oynasi ──

class _ConsentSheet extends StatefulWidget {
  final String code;
  final Map<String, dynamic> info;

  const _ConsentSheet({required this.code, required this.info});

  @override
  State<_ConsentSheet> createState() => _ConsentSheetState();
}

/// Pul chegarasi variantlari (tiyinda).
///
/// `0` — HAR BIR buyurtma alohida tasdiqlanadi. U ATAYLAB birinchi
/// va standart: sukut bo'yicha eng qattiq rejim tanlangan bo'lishi
/// kerak, yumshatishni odam ONGLI ravishda qilsin.
const _limitOptions = <int, String>{
  0: 'Har safar mendan so\'ralsin',
  5000000: '50 000 so\'mgacha o\'zi bersin',
  10000000: '100 000 so\'mgacha o\'zi bersin',
  20000000: '200 000 so\'mgacha o\'zi bersin',
};

class _ConsentSheetState extends State<_ConsentSheet> {
  int _limit = 0;
  bool _busy = false;
  late final Set<String> _selected = {
    for (final p in _permissions) p['scope'].toString(),
  };

  List<Map<String, dynamic>> get _permissions =>
      (widget.info['permissions'] as List? ?? const [])
          .cast<Map<String, dynamic>>();

  Future<void> _approve() async {
    setState(() => _busy = true);
    try {
      await api.agentApprove(
        widget.code,
        scopes: _selected.toList(),
        perOrderLimitTiyin: _limit,
        // Kunlik chegara — bitta buyurtmanikidan uch barobar.
        // Sabab: bitta chegara yetarli emas edi — agent uni
        // aylanib o'tish uchun ketma-ket kichik buyurtmalar berishi
        // mumkin. "Har safar so'ralsin" rejimida kunlik chegara
        // ma'nosiz (har buyurtma baribir tasdiqlanadi), shuning
        // uchun 0 qoladi.
        dailyLimitTiyin: _limit == 0 ? 0 : _limit * 3,
      );
      if (mounted) Navigator.pop(context, true);
    } catch (e) {
      if (mounted) {
        setState(() => _busy = false);
        _snack(context, errorText(e, 'Ruxsat berib bo\'lmadi'));
      }
    }
  }

  Future<void> _deny() async {
    try {
      await api.agentDeny(widget.code);
    } catch (_) {
      // Rad etish serverda yozilmasa ham foydalanuvchi uchun natija
      // bir xil: ruxsat BERILMADI. So'rov 10 daqiqada o'zi o'chadi.
    }
    if (mounted) Navigator.pop(context, false);
  }

  @override
  Widget build(BuildContext context) {
    final partner = (widget.info['partner'] ?? 'Noma\'lum ilova').toString();
    final isTest = widget.info['environment'] == 'test';

    return Container(
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 20,
        bottom: MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: const Color(0xFFE0E0E0),
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 18),
            Text(
              '$partner ulanmoqchi',
              style:
                  const TextStyle(fontSize: 19, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 6),
            const Text(
              'Bu ilova sizning OnDex akkauntingizdan quyidagilarni '
              'so\'rayapti:',
              style: TextStyle(fontSize: 13, color: Color(0xFF757575)),
            ),
            if (isTest) ...[
              const SizedBox(height: 10),
              // Sinov kaliti bilan HAQIQIY buyurtma yaratilmaydi.
              // Buni aytmaslik foydalanuvchini chalg'itardi.
              const _Hint('Bu — SINOV ulanishi. Haqiqiy buyurtma '
                  'yaratilmaydi va restoranga hech narsa bormaydi.'),
            ],
            const SizedBox(height: 14),

            // Har bir ruxsatni ALOHIDA o'chirish mumkin: "hammasi
            // yoki hech nima" tanlovi aslida tanlov emas.
            for (final p in _permissions)
              CheckboxListTile(
                contentPadding: EdgeInsets.zero,
                dense: true,
                activeColor: kBrand,
                value: _selected.contains(p['scope']),
                title: Text(p['label']?.toString() ?? p['scope'].toString(),
                    style: const TextStyle(fontSize: 14)),
                onChanged: (v) => setState(() {
                  final s = p['scope'].toString();
                  v == true ? _selected.add(s) : _selected.remove(s);
                }),
              ),

            const SizedBox(height: 10),
            const Text('Pul chegarasi',
                style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold)),
            const SizedBox(height: 2),
            const Text(
              'Chegaradan oshgan buyurtma sizga tasdiqlash uchun keladi.',
              style: TextStyle(fontSize: 12, color: Color(0xFF757575)),
            ),
            const SizedBox(height: 6),
            // `RadioListTile` ATAYLAB ishlatilmadi: uning
            // `groupValue`/`onChanged` juftligi Flutter 3.32 dan
            // keyin eskirgan (`RadioGroup` ga o'tilgan). Bu yerda
            // kerak bo'lgani shunchaki "tanlangan qator" — uni qo'lda
            // chizish eskirgan API'ga bog'lanishdan arzonroq.
            for (final e in _limitOptions.entries)
              _LimitOption(
                label: e.value,
                // Kunlik chegara "har safar so'ralsin" rejimida
                // ma'nosiz — har buyurtma baribir tasdiqlanadi.
                sub: e.key == 0
                    ? null
                    : 'Kuniga jami ${formatSum(e.key * 3)} so\'mgacha',
                selected: _limit == e.key,
                onTap: () => setState(() => _limit = e.key),
              ),

            const SizedBox(height: 8),
            const _Hint(
              'Ruxsatni istalgan vaqtda shu bo\'limdan uzishingiz mumkin. '
              'Ilova manzilingizni yoki profilingizni O\'ZGARTIRA OLMAYDI.',
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: _busy ? null : _deny,
                    child: const Text('Rad etish'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton(
                    style: FilledButton.styleFrom(backgroundColor: kBrand),
                    onPressed: _busy || _selected.isEmpty ? null : _approve,
                    child: _busy
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(
                                strokeWidth: 2, color: Colors.white))
                        : const Text('Ruxsat beraman'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ── Kichik qismlar ──

/// Qisqa xabar. Loyihada umumiy `showSnack` yordamchisi yo'q —
/// ekranlar `ScaffoldMessenger` ni to'g'ridan-to'g'ri chaqiradi.
/// Bu yerda u BIR marta o'raladi, chunki shu faylda oltita joyda
/// kerak bo'ladi.
void _snack(BuildContext context, String text) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(content: Text(text)));
}

/// Pul chegarasi tanlovining bitta qatori.
class _LimitOption extends StatelessWidget {
  final String label;
  final String? sub;
  final bool selected;
  final VoidCallback onTap;

  const _LimitOption({
    required this.label,
    required this.sub,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) => InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(10),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Row(
            children: [
              Icon(
                selected
                    ? Icons.radio_button_checked
                    : Icons.radio_button_unchecked,
                size: 20,
                color: selected ? kBrand : const Color(0xFFBDBDBD),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(label, style: const TextStyle(fontSize: 14)),
                    if (sub != null)
                      Text(sub!,
                          style: const TextStyle(
                              fontSize: 12, color: Color(0xFF9E9E9E))),
                  ],
                ),
              ),
            ],
          ),
        ),
      );
}

/// Shaddiy amallariga ruxsat — yoqish/o'chirish.
///
/// ┌─ HAR QATOR NIMA QILISHINI AYTADI ──────────────────────────────────┐
/// "search_food" kabi ichki nomlar ko'rsatilmaydi: odam nimaga ruxsat
/// berayotganini TUSHUNGAN holda hal qilishi kerak. Shuning uchun har
/// qatorda oqibat yozilgan ("o'chirilsa Shaddiy taom topa olmaydi").
/// └────────────────────────────────────────────────────────────────────┘
class _AiToolsSection extends StatefulWidget {
  const _AiToolsSection();

  @override
  State<_AiToolsSection> createState() => _AiToolsSectionState();
}

class _AiToolsSectionState extends State<_AiToolsSection> {
  final _tools = AiTools.instance;

  @override
  void initState() {
    super.initState();
    _tools.addListener(_onChange);
    if (!_tools.loaded) _tools.load();
  }

  void _onChange() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _tools.removeListener(_onChange);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_tools.loaded && _tools.loading) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: CircularProgressIndicator(color: kBrand)),
      );
    }
    if (!_tools.loaded) {
      return _Hint(_tools.error ?? 'Ruxsatlarni yuklab bo\'lmadi.');
    }

    final names = _tools.state.keys.toList();
    if (names.isEmpty) {
      return const _Hint('Yordamchi amallari topilmadi.');
    }

    // Xizmat bo'yicha guruhlar — kelajakda "Do'kon", "Taksi" kabi
    // bo'limlar qo'shilganda ruxsatlar aralashib ketmasligi uchun.
    final groups = AiToolGroup.split(names);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Xato saqlashda chiqadi — o'zgarish qaytarilgan bo'ladi,
        // ya'ni ekranda ko'rinayotgan holat SERVERDAGI holat.
        if (_tools.error != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text(
              _tools.error!,
              style: const TextStyle(fontSize: 12.5, color: Color(0xFFB3261E)),
            ),
          ),
        for (final g in groups) ...[
          Padding(
            padding: const EdgeInsets.fromLTRB(2, 6, 2, 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  g.key.title,
                  style: const TextStyle(
                      fontSize: 14, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 2),
                Text(
                  g.key.subtitle,
                  style: const TextStyle(
                      fontSize: 12, color: Color(0xFF9E9E9E)),
                ),
              ],
            ),
          ),
          Container(
            decoration: BoxDecoration(
              border: Border.all(color: const Color(0xFFE5E5E5)),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Column(
              children: [
                for (var i = 0; i < g.value.length; i++) ...[
                  if (i > 0)
                    const Divider(height: 1, color: Color(0xFFEEEEEE)),
                  _AiToolRow(
                    name: g.value[i],
                    enabled: _tools.state[g.value[i]] ?? true,
                    onChanged: (v) => _tools.setEnabled(g.value[i], v),
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(height: 14),
        ],
      ],
    );
  }
}

class _AiToolRow extends StatelessWidget {
  const _AiToolRow({
    required this.name,
    required this.enabled,
    required this.onChanged,
  });

  final String name;
  final bool enabled;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    final info = AiToolInfo.of(name);
    return SwitchListTile.adaptive(
      value: enabled,
      onChanged: onChanged,
      activeTrackColor: kBrand,
      contentPadding: const EdgeInsets.fromLTRB(14, 4, 8, 4),
      title: Text(
        info.title,
        style: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w600),
      ),
      subtitle: Padding(
        padding: const EdgeInsets.only(top: 2),
        child: Text(
          info.subtitle,
          style: const TextStyle(
              fontSize: 12.5, height: 1.35, color: Color(0xFF757575)),
        ),
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  final String text;
  const _SectionTitle(this.text);

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(top: 4, bottom: 10),
        child: Text(text,
            style:
                const TextStyle(fontSize: 15, fontWeight: FontWeight.bold)),
      );
}

class _Hint extends StatelessWidget {
  final String text;
  const _Hint(this.text);

  @override
  Widget build(BuildContext context) => Container(
        width: double.infinity,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: const Color(0xFFF7F7F7),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Text(text,
            style: const TextStyle(fontSize: 12.5, color: Color(0xFF616161))),
      );
}

class _GrantCard extends StatelessWidget {
  final Map<String, dynamic> grant;
  final VoidCallback onRevoke;

  const _GrantCard({required this.grant, required this.onRevoke});

  @override
  Widget build(BuildContext context) {
    final limit = (grant['per_order_limit_tiyin'] as num?)?.toInt() ?? 0;
    final scopes = (grant['scopes'] as List? ?? const []).length;
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: const Color(0xFFE5E5E5)),
      ),
      child: Row(
        children: [
          const Icon(Icons.smart_toy_outlined, color: Color(0xFF9E9E9E)),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text((grant['partner_name'] ?? 'Ilova').toString(),
                    style: const TextStyle(
                        fontSize: 15, fontWeight: FontWeight.w600)),
                const SizedBox(height: 2),
                Text(
                  limit == 0
                      ? '$scopes ta ruxsat · har buyurtma tasdiqlanadi'
                      : '$scopes ta ruxsat · ${formatSum(limit)} so\'mgacha',
                  style: const TextStyle(
                      fontSize: 12, color: Color(0xFF757575)),
                ),
              ],
            ),
          ),
          TextButton(
            onPressed: onRevoke,
            style: TextButton.styleFrom(foregroundColor: Colors.red),
            child: const Text('Uzish'),
          ),
        ],
      ),
    );
  }
}

class _DraftCard extends StatelessWidget {
  final Map<String, dynamic> draft;
  final Future<void> Function(Map<String, dynamic>, bool) onDecide;

  const _DraftCard({required this.draft, required this.onDecide});

  @override
  Widget build(BuildContext context) {
    final total = (draft['total_tiyin'] as num?)?.toInt() ?? 0;
    final items = (draft['items'] as List? ?? const [])
        .cast<Map<String, dynamic>>();
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF8E1),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: const Color(0xFFFFE082)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('${draft['partner_name'] ?? 'Yordamchi'} buyurtma tayyorladi',
              style:
                  const TextStyle(fontSize: 14, fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          // Tarkib TO'LIQ ko'rsatiladi: odam nimaga pul to'layotganini
          // ko'rmasdan tasdiqlamasligi kerak.
          for (final it in items)
            Text('${it['qty']} × ${it['name']}',
                style: const TextStyle(fontSize: 13)),
          const SizedBox(height: 8),
          Text('Jami: ${formatSum(total)} so\'m',
              style:
                  const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () => onDecide(draft, false),
                  child: const Text('Rad etish'),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: kBrand),
                  onPressed: () => onDecide(draft, true),
                  child: const Text('Tasdiqlash'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ActivityRow extends StatelessWidget {
  final Map<String, dynamic> entry;
  const _ActivityRow({required this.entry});

  /// Audit kodini odam tiliga o'giradi.
  ///
  /// Xom kod (`order.placed`) ko'rsatilsa, bu ro'yxat faqat
  /// dasturchi uchun ma'noli bo'lardi — ya'ni maqsadini yo'qotardi.
  static String _label(String action) {
    switch (action) {
      case 'grant.approved':
        return 'Ruxsat berildi';
      case 'grant.denied':
        return 'Rad etildi';
      case 'grant.revoked':
        return 'Ulanish uzildi';
      case 'link.consumed':
        return 'Ilova ulandi';
      case 'order.draft':
        return 'Buyurtma tayyorlandi';
      case 'order.placed':
        return 'Buyurtma berildi';
      case 'order.rejected':
        return 'Buyurtma rad etildi';
      case 'order.cancelled':
        return 'Buyurtma bekor qilindi';
      case 'scope.denied':
        return 'Ruxsat etilmagan amalga urinildi';
    }
    return action;
  }

  @override
  Widget build(BuildContext context) {
    final ok = entry['ok'] != false;
    final at = DateTime.tryParse(entry['created_at']?.toString() ?? '');
    final when = at == null
        ? ''
        : '${at.day.toString().padLeft(2, '0')}.'
            '${at.month.toString().padLeft(2, '0')} '
            '${at.hour.toString().padLeft(2, '0')}:'
            '${at.minute.toString().padLeft(2, '0')}';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 7),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(ok ? Icons.check_circle_outline : Icons.error_outline,
              size: 16, color: ok ? const Color(0xFF9E9E9E) : Colors.red),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(_label(entry['action']?.toString() ?? ''),
                    style: const TextStyle(fontSize: 13.5)),
                if ((entry['partner'] ?? '').toString().isNotEmpty)
                  Text(entry['partner'].toString(),
                      style: const TextStyle(
                          fontSize: 11.5, color: Color(0xFF9E9E9E))),
              ],
            ),
          ),
          Text(when,
              style:
                  const TextStyle(fontSize: 11.5, color: Color(0xFF9E9E9E))),
        ],
      ),
    );
  }
}
