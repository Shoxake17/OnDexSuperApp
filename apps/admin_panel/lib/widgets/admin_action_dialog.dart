import 'package:flutter/material.dart';

import '../api.dart';

/// Xavfli admin amali tasdig'i (CC-0.4): sabab (auditga yoziladi) va
/// Authenticator yoki tiklash kodi. Server nimani so'ragan bo'lsa shu
/// maydonlar ko'rsatiladi.
Future<AdminActionConfirm?> showAdminActionDialog(
  BuildContext context, {
  required bool needReason,
  required bool needCode,
  String? error,
}) =>
    showDialog<AdminActionConfirm>(
      context: context,
      barrierDismissible: false,
      builder: (_) => _AdminActionDialog(
          needReason: needReason, needCode: needCode, error: error),
    );

class _AdminActionDialog extends StatefulWidget {
  final bool needReason;
  final bool needCode;
  final String? error;

  const _AdminActionDialog(
      {required this.needReason, required this.needCode, this.error});

  @override
  State<_AdminActionDialog> createState() => _AdminActionDialogState();
}

class _AdminActionDialogState extends State<_AdminActionDialog> {
  final _reason = TextEditingController();
  final _code = TextEditingController();
  late String? _error = widget.error;

  @override
  void dispose() {
    _reason.dispose();
    _code.dispose();
    super.dispose();
  }

  void _submit() {
    final reason = _reason.text.trim();
    final code = _code.text.replaceAll(' ', '');
    if (widget.needReason && reason.length < 3) {
      setState(() => _error = 'Sababni yozing (auditga yoziladi)');
      return;
    }
    if (widget.needCode && code.isEmpty) {
      setState(() => _error = 'Authenticator kodini kiriting');
      return;
    }
    Navigator.pop(context, (reason: reason, code: code));
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Xavfli amalni tasdiqlang'),
      content: SizedBox(
        width: 380,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (widget.needReason) ...[
              TextField(
                key: const ValueKey('action-reason'),
                controller: _reason,
                autofocus: true,
                maxLength: 500,
                decoration: const InputDecoration(
                    labelText: 'Sabab', border: OutlineInputBorder()),
              ),
              const SizedBox(height: 8),
            ],
            if (widget.needCode)
              TextField(
                key: const ValueKey('action-code'),
                controller: _code,
                autofocus: !widget.needReason,
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
            key: const ValueKey('action-confirm'),
            onPressed: _submit,
            child: const Text('Tasdiqlash')),
      ],
    );
  }
}
