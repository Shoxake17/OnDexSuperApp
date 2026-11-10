import 'package:flutter/material.dart';

/// Parol maydoni "ko'z" tugmasi bilan (ko'rsatish/yashirish).
///
/// Xavfsizlik: imlo tekshiruvi, avto-to'g'rilash va taklif O'CHIQ (matn
/// klaviatura/bulut lug'atiga tushmaydi); yuborilganda va ilova fonga
/// o'tganda parol yana yashiriladi — ekranda "unutilib" qolmaydi.
class PasswordField extends StatefulWidget {
  final TextEditingController controller;
  final String label;
  final bool autofocus;
  final ValueChanged<String>? onSubmitted;
  final Iterable<String>? autofillHints;
  final String? helperText;
  final bool outlined;

  const PasswordField({
    super.key,
    required this.controller,
    required this.label,
    this.autofocus = false,
    this.onSubmitted,
    this.autofillHints = const [AutofillHints.password],
    this.helperText,
    this.outlined = true,
  });

  @override
  State<PasswordField> createState() => _PasswordFieldState();
}

class _PasswordFieldState extends State<PasswordField>
    with WidgetsBindingObserver {
  bool _visible = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed && _visible && mounted) {
      setState(() => _visible = false);
    }
  }

  @override
  Widget build(BuildContext context) => TextField(
        controller: widget.controller,
        obscureText: !_visible,
        enableSuggestions: false,
        autocorrect: false,
        spellCheckConfiguration: const SpellCheckConfiguration.disabled(),
        keyboardType: TextInputType.visiblePassword,
        autofillHints: widget.autofillHints,
        autofocus: widget.autofocus,
        onSubmitted: (v) {
          if (_visible) setState(() => _visible = false);
          widget.onSubmitted?.call(v);
        },
        decoration: InputDecoration(
          labelText: widget.label,
          helperText: widget.helperText,
          helperMaxLines: 3,
          border: widget.outlined ? const OutlineInputBorder() : null,
          suffixIcon: IconButton(
            tooltip: _visible ? 'Parolni yashirish' : 'Parolni ko\'rsatish',
            icon: Icon(_visible
                ? Icons.visibility_off_outlined
                : Icons.visibility_outlined),
            onPressed: () => setState(() => _visible = !_visible),
          ),
        ),
      );
}
