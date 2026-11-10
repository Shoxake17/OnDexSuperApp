import 'package:flutter/material.dart';

import '../api.dart';

/// KYC — tashkilotlarni tasdiqlash (A6, AUTH-PHASE-A: "tasdiqlashni FAQAT
/// egasi qiladi"). Har qaror — Authenticator kodi (server talab qiladi) +
/// rad/to'xtatishda sabab; hammasi auditga yoziladi.
///
/// Faqat OnDex ID (yangi) admin sessiyasida ishlaydi — eski token bilan
/// server 401 qaytaradi, sahifa qayta kirishni so'raydi.
class KycPage extends StatefulWidget {
  const KycPage({super.key});

  @override
  State<KycPage> createState() => _KycPageState();
}

class _KycPageState extends State<KycPage> {
  static const _filters = {
    'pending': 'Kutilmoqda',
    'active': 'Tasdiqlangan',
    'rejected': 'Rad etilgan',
    'suspended': 'To\'xtatilgan',
  };

  String _status = 'pending';
  List<Map<String, dynamic>>? _items;
  String? _error;
  String? _busyId;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _error = null;
      _items = null;
    });
    if (!api.isCentralSession) {
      setState(() => _error = 'KYC faqat OnDex ID orqali kirgan admin uchun. '
          'Paneldan chiqib, qaytadan kiring (kod + parol + Authenticator).');
      return;
    }
    try {
      final list = await api.organizations(status: _status);
      if (mounted) setState(() => _items = list);
    } on ApiException catch (e) {
      if (mounted) {
        setState(() => _error = e.isUnauthorized
            ? 'Admin sessiyasi tugagan — paneldan chiqib, qaytadan kiring.'
            : e.message);
      }
    } catch (_) {
      if (mounted) setState(() => _error = 'Serverga ulanib bo\'lmadi');
    }
  }

  Future<void> _decide(Map<String, dynamic> org, String decision) async {
    final input = await showDialog<({String code, String reason})>(
      context: context,
      builder: (_) => _DecisionDialog(org: org, decision: decision),
    );
    if (input == null) return;
    setState(() => _busyId = org['id'] as String?);
    try {
      await api.decideOrganization(org['id'] as String, decision,
          reason: input.reason, stepUpCode: input.code);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text('«${org['name']}»: ${_decisionDone[decision]}')));
      await _load();
    } on ApiException catch (e) {
      if (!mounted) return;
      // Noto'g'ri kod — server 403 + aniq xabar (sessiya yopilmaydi).
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) setState(() => _busyId = null);
    }
  }

  static const _decisionDone = {
    'approve': 'tasdiqlandi',
    'reject': 'rad etildi',
    'suspend': 'to\'xtatildi',
  };

  @override
  Widget build(BuildContext context) {
    final items = _items;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            Text('Tashkilotlar (KYC)',
                style: Theme.of(context).textTheme.headlineSmall),
            const Spacer(),
            IconButton(
                tooltip: 'Yangilash',
                onPressed: _load,
                icon: const Icon(Icons.refresh)),
          ]),
          const SizedBox(height: 12),
          SegmentedButton<String>(
            key: const ValueKey('kyc-filter'),
            showSelectedIcon: false,
            segments: [
              for (final e in _filters.entries)
                ButtonSegment(value: e.key, label: Text(e.value)),
            ],
            selected: {_status},
            onSelectionChanged: (s) {
              setState(() => _status = s.first);
              _load();
            },
          ),
          const SizedBox(height: 16),
          Expanded(
            child: _error != null
                ? Center(
                    child: Text(_error!,
                        key: const ValueKey('kyc-error'),
                        textAlign: TextAlign.center))
                : items == null
                    ? const Center(child: CircularProgressIndicator())
                    : items.isEmpty
                        ? const Center(
                            child: Text('Bu holatda tashkilot yo\'q'))
                        : ListView.separated(
                            itemCount: items.length,
                            separatorBuilder: (_, __) =>
                                const SizedBox(height: 8),
                            itemBuilder: (_, i) => _orgCard(items[i]),
                          ),
          ),
        ],
      ),
    );
  }

  Widget _orgCard(Map<String, dynamic> o) {
    final status = o['status'] as String? ?? '';
    final busy = _busyId == o['id'];
    final tin = (o['tin'] as String?) ?? '';
    final reason = (o['status_reason'] as String?) ?? '';
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('${o['name'] ?? ''}',
                      style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 4),
                  Text('STIR: ${tin.isEmpty ? '—' : tin} · '
                      'holat: ${_filters[status] ?? status}'),
                  if (reason.isNotEmpty) Text('Sabab: $reason'),
                ],
              ),
            ),
            if (busy)
              const SizedBox(
                  width: 24,
                  height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2))
            else
              Wrap(spacing: 8, children: [
                if (status != 'active')
                  FilledButton(
                    key: ValueKey('kyc-approve-${o['id']}'),
                    onPressed: () => _decide(o, 'approve'),
                    child: const Text('Tasdiqlash'),
                  ),
                if (status == 'pending')
                  OutlinedButton(
                    onPressed: () => _decide(o, 'reject'),
                    child: const Text('Rad etish'),
                  ),
                if (status == 'active')
                  OutlinedButton(
                    onPressed: () => _decide(o, 'suspend'),
                    child: const Text('To\'xtatish'),
                  ),
              ]),
          ],
        ),
      ),
    );
  }
}

/// Qaror oynasi: Authenticator kodi (har doim) + sabab (rad/to'xtatishda).
class _DecisionDialog extends StatefulWidget {
  final Map<String, dynamic> org;
  final String decision;

  const _DecisionDialog({required this.org, required this.decision});

  @override
  State<_DecisionDialog> createState() => _DecisionDialogState();
}

class _DecisionDialogState extends State<_DecisionDialog> {
  final _code = TextEditingController();
  final _reason = TextEditingController();
  String? _error;

  bool get _needsReason => widget.decision != 'approve';

  @override
  void dispose() {
    _code.dispose();
    _reason.dispose();
    super.dispose();
  }

  void _submit() {
    final code = _code.text.replaceAll(' ', '');
    if (code.isEmpty) {
      setState(() => _error = 'Authenticator kodini kiriting');
      return;
    }
    if (_needsReason && _reason.text.trim().isEmpty) {
      setState(() => _error = 'Sababni yozing — egasiga ko\'rsatiladi');
      return;
    }
    Navigator.pop(context, (code: code, reason: _reason.text.trim()));
  }

  @override
  Widget build(BuildContext context) {
    final title = switch (widget.decision) {
      'approve' => 'Tasdiqlash',
      'reject' => 'Rad etish',
      _ => 'To\'xtatish',
    };
    return AlertDialog(
      title: Text('$title: ${widget.org['name']}'),
      content: SizedBox(
        width: 380,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (_needsReason) ...[
              TextField(
                key: const ValueKey('kyc-reason'),
                controller: _reason,
                maxLength: 500,
                decoration: const InputDecoration(
                    labelText: 'Sabab', border: OutlineInputBorder()),
              ),
              const SizedBox(height: 8),
            ],
            TextField(
              key: const ValueKey('kyc-code'),
              controller: _code,
              autofocus: !_needsReason,
              keyboardType: TextInputType.number,
              onSubmitted: (_) => _submit(),
              decoration: const InputDecoration(
                  labelText: 'Authenticator kodi (yoki tiklash kodi)',
                  border: OutlineInputBorder()),
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Bekor')),
        FilledButton(
            key: const ValueKey('kyc-confirm'),
            onPressed: _submit,
            child: Text(title)),
      ],
    );
  }
}
