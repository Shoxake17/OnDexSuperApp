import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/date_range_dialog.dart';

part 'staff/staff_models.dart';
part 'staff/staff_widgets.dart';
part 'staff/staff_dialogs.dart';
part 'staff/staff_report.dart';

/// "Xodimlar" — `image/Xodimlar.png` bo'yicha (Eksport tugmasisiz,
/// sarlavha ikonkasisiz).
///
/// ┌─ XODIM ≠ AKKAUNT ─────────────────────────────────────────────────┐
/// Xodim — restoranning ichki yozuvi: oshpaz, kassir, tozalovchi...
/// Kafe, oshxona, choyxona, qahvaxona — hammasi uchun bir xil yopiq
/// lavozimlar ro'yxati serverdan keladi.
///
/// Faqat ilovasi bor lavozimda (ofitsiant) "ilovaga kirish" yoqiladi.
/// Kirish xodim yozuviga ERGASHADI va buni SERVER majburlaydi: ta'til,
/// ishdan bo'shatish yoki lavozim o'zgarishida akkaunt darhol yopiladi,
/// sessiyalar bekor qilinadi. Begona raqamdagi akkauntga tegilmaydi.
///
/// Yozuv o'chirilmaydi — "Ishdan bo'shagan" holati: mehnat tarixi va
/// faoliyat jurnali saqlanadi, kerak bo'lsa qayta ishga olinadi.
/// └───────────────────────────────────────────────────────────────────┘
class StaffPage extends StatefulWidget {
  const StaffPage({super.key});

  @override
  State<StaffPage> createState() => _StaffPageState();
}

enum _Sort { number, name, newest }

class _StaffPageState extends State<StaffPage> {
  _Overview? _data;
  bool _loading = true;
  String? _error;
  final _search = TextEditingController();
  String _position = '';
  String _status = '';
  _Sort _sort = _Sort.number;

  @override
  void initState() {
    super.initState();
    _search.addListener(() {
      if (mounted) setState(() {});
    });
    _load();
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final json = await api.staffOverview();
      if (!mounted) return;
      setState(() {
        _data = _Overview.fromJson(json);
        _loading = false;
        _error = null;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = 'Xodimlarni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';
      });
    }
  }

  List<StaffMember> get _visible {
    final data = _data;
    if (data == null) return const [];
    final q = _search.text.trim().toLowerCase();
    final qDigits = q.replaceAll(RegExp(r'\D'), '');
    final list = data.members.where((m) {
      if (_position.isNotEmpty && m.position != _position) return false;
      if (_status.isNotEmpty && m.status != _status) return false;
      if (q.isEmpty) return true;
      return m.fullName.toLowerCase().contains(q) ||
          m.positionTitle.toLowerCase().contains(q) ||
          m.code.toLowerCase().contains(q) ||
          (qDigits.length >= 3 && m.phone.replaceAll(RegExp(r'\D'), '').contains(qDigits));
    }).toList();
    switch (_sort) {
      case _Sort.number:
        list.sort((a, b) => a.number.compareTo(b.number));
      case _Sort.name:
        list.sort((a, b) => a.fullName.toLowerCase().compareTo(b.fullName.toLowerCase()));
      case _Sort.newest:
        list.sort((a, b) => b.createdAt.compareTo(a.createdAt));
    }
    return list;
  }

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _openForm([StaffMember? member]) async {
    final data = _data;
    if (data == null) return;
    final saved = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (_) => _StaffFormDialog(positions: data.positions, member: member),
    );
    if (saved != true || !mounted) return;
    _toast(member == null ? 'Xodim qo\'shildi' : 'O\'zgarishlar saqlandi');
    await _load();
  }

  Future<void> _openDetails(StaffMember m) async {
    final action = await showDialog<String>(
      context: context,
      builder: (_) => _StaffDetailsDialog(member: m),
    );
    if (action == null || !mounted) return;
    if (action == 'edit') {
      await _openForm(m);
    } else {
      await _changeStatus(m, action);
    }
  }

  Future<void> _changeStatus(StaffMember m, String status) async {
    if (!await _confirmStatus(context, m, status) || !mounted) return;
    try {
      await api.setStaffStatus(m.id, status);
      _toast(switch (status) {
        'dismissed' => '${m.fullName} ishdan bo\'shatildi',
        'on_leave' => '${m.fullName} ta\'tilga chiqarildi',
        _ => '${m.fullName} ishga qaytarildi',
      });
      await _load();
    } on ApiException catch (e) {
      _toast(e.message);
    } catch (_) {
      _toast('Holatni o\'zgartirib bo\'lmadi. Internet aloqasini tekshiring.');
    }
  }

  void _openSchedule() {
    final data = _data;
    if (data == null) return;
    showDialog<void>(
      context: context,
      builder: (_) => _ScheduleDialog(members: data.members.where((m) => m.status == 'active').toList()),
    );
  }

  void _openReport() {
    showDialog<void>(context: context, builder: (_) => const _StaffReportDialog());
  }

  @override
  Widget build(BuildContext context) {
    final data = _data;
    Widget body;
    if (_loading) {
      body = const Center(child: CircularProgressIndicator());
    } else if (data == null) {
      body = Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off_rounded, size: 40, color: OnDexColors.inkFaint),
            const SizedBox(height: 10),
            Text(_error ?? 'Xodimlar topilmadi',
                textAlign: TextAlign.center, style: const TextStyle(fontSize: 14, color: OnDexColors.ink)),
            const SizedBox(height: 14),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    } else {
      body = LayoutBuilder(builder: (context, box) {
        final table = _StaffTableCard(
          members: _visible,
          totalCount: data.members.length,
          search: _search,
          positions: data.positions,
          position: _position,
          status: _status,
          sort: _sort,
          onPosition: (v) => setState(() => _position = v),
          onStatus: (v) => setState(() => _status = v),
          onSort: (v) => setState(() => _sort = v),
          onClearFilters: () => setState(() {
            _search.clear();
            _position = '';
            _status = '';
          }),
          onAdd: () => _openForm(),
          onView: _openDetails,
          onEdit: _openForm,
          onStatusChange: _changeStatus,
        );
        final side = <Widget>[
          _DistributionCard(summary: data.summary, positions: data.positions),
          const SizedBox(height: 16),
          _QuickActionsCard(onSchedule: _openSchedule, onReport: _openReport),
          const SizedBox(height: 16),
          _ActivityCard(events: data.recent),
        ];
        final stats = _StatsRow(summary: data.summary);

        // ┌─ JADVAL QAT'IY O'LCHAMDA ─────────────────────────────────┐
        // Jadval ekranning qolgan balandligini egallaydi va xodimlar
        // ko'paysa CHO'ZILMAYDI — qatorlar uning ICHIDA aylanadi,
        // sarlavha qotib turadi. O'ng ustun jadval yonida qoladi
        // (1000 px dan boshlab — 125% masshtabli noutbukda ham). Yon
        // tomonga aylantirish YO'Q: tor oynada kamroq muhim ustunlar
        // (ish vaqti, telefon, lavozim) yashiriladi.
        // └────────────────────────────────────────────────────────────┘
        if (box.maxWidth >= 1000 && box.maxHeight >= 520) {
          final sideWidth = box.maxWidth >= 1400 ? 360.0 : 320.0;
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              stats,
              const SizedBox(height: 16),
              Expanded(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Expanded(child: table),
                    const SizedBox(width: 16),
                    SizedBox(
                      width: sideWidth,
                      child: SingleChildScrollView(
                        primary: false,
                        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: side),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          );
        }
        return SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              stats,
              const SizedBox(height: 16),
              SizedBox(height: math.max(480.0, box.maxHeight), child: table),
              const SizedBox(height: 16),
              ...side,
            ],
          ),
        );
      });
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _StaffHeader(onAdd: data == null ? null : () => _openForm()),
          const SizedBox(height: 16),
          Expanded(child: body),
        ],
      ),
    );
  }
}
