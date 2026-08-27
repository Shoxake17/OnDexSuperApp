import 'dart:async';

import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';

import '../api.dart';
import '../widgets/app_text_field.dart';

/// Manzil qidirish ekrani (Yandex Go uslubi): tepada qidiruv maydoni +
/// "joriy joylashuvim" tugmasi, pastda yozgan matningizga mos manzillar
/// ro'yxati jonli (debounce bilan) yangilanib turadi. Tanlangan manzil
/// `{lat, lng, address}` ko'rinishida chaqiruvchiga qaytariladi.
class AddressSearchScreen extends StatefulWidget {
  const AddressSearchScreen({super.key});

  @override
  State<AddressSearchScreen> createState() => _AddressSearchScreenState();
}

class _AddressSearchScreenState extends State<AddressSearchScreen> {
  final _ctrl = TextEditingController();
  final _focusNode = FocusNode();
  Timer? _debounce;
  int _searchGen = 0;
  List<Map<String, String>> _results = [];
  bool _loading = false;
  bool _locating = false;
  bool _selecting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _focusNode.requestFocus();
    });
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _ctrl.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _onChanged(String value) {
    _debounce?.cancel();
    final q = value.trim();
    if (q.isEmpty) {
      setState(() {
        _results = [];
        _loading = false;
        _error = null;
      });
      return;
    }
    setState(() => _loading = true);
    _debounce = Timer(const Duration(milliseconds: 400), () => _search(q));
  }

  Future<void> _search(String q) async {
    final myGen = ++_searchGen;
    try {
      final r =
          await api.addressAutocomplete(q).timeout(const Duration(seconds: 8));
      if (!mounted || myGen != _searchGen) return;
      setState(() {
        _results = r;
        _loading = false;
        _error = null;
      });
    } catch (_) {
      if (!mounted || myGen != _searchGen) return;
      setState(() {
        _results = [];
        _loading = false;
        _error = 'Qidirishda xatolik — internetni tekshiring';
      });
    }
  }

  Future<void> _selectResult(Map<String, String> r) async {
    final placeId = r['place_id'];
    if (placeId == null || _selecting) return;
    setState(() => _selecting = true);
    try {
      final details =
          await api.placeDetails(placeId).timeout(const Duration(seconds: 8));
      if (!mounted) return;
      Navigator.of(context).pop(details);
    } catch (_) {
      if (!mounted) return;
      setState(() => _selecting = false);
      ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Manzilni aniqlab bo\'lmadi')));
    }
  }

  Future<void> _useCurrentLocation() async {
    setState(() => _locating = true);
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied ||
          perm == LocationPermission.deniedForever) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Joylashuvga ruxsat berilmadi')));
        return;
      }
      final pos = await Geolocator.getCurrentPosition();
      if (!mounted) return;
      Navigator.of(context)
          .pop({'lat': pos.latitude, 'lng': pos.longitude, 'address': null});
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Joylashuvni aniqlab bo\'lmadi')));
      }
    } finally {
      if (mounted) setState(() => _locating = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(8, 8, 12, 8),
              child: Row(
                children: [
                  IconButton(
                    icon: const Icon(Icons.arrow_back),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                  // Ko'rinish ilovadagi boshqa maydonlar bilan bir xil,
                  // vazifasi esa o'zgarmagan: bu MANZIL qidiruvi,
                  // restoran qidirmaydi.
                  Expanded(
                    child: AppTextField(
                      controller: _ctrl,
                      focusNode: _focusNode,
                      hint: 'Yetkazish manzilini kiriting',
                      icon: Icons.search,
                      textInputAction: TextInputAction.search,
                      onChanged: _onChanged,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Material(
                    color: scheme.surfaceContainerHighest,
                    borderRadius: BorderRadius.circular(14),
                    child: InkWell(
                      borderRadius: BorderRadius.circular(14),
                      onTap: _locating ? null : _useCurrentLocation,
                      child: Padding(
                        padding: const EdgeInsets.all(12),
                        child: _locating
                            ? const SizedBox(
                                width: 22,
                                height: 22,
                                child:
                                    CircularProgressIndicator(strokeWidth: 2))
                            : const Icon(Icons.navigation_outlined),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            if (_loading || _selecting)
              const LinearProgressIndicator(minHeight: 2),
            Expanded(
              child: _buildBody(),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBody() {
    if (_error != null) {
      return Center(
          child: Text(_error!, style: const TextStyle(color: Colors.grey)));
    }
    if (_results.isEmpty) {
      if (_loading) return const SizedBox.shrink();
      if (_ctrl.text.trim().isEmpty) return const SizedBox.shrink();
      return const Center(
          child: Text('Hech narsa topilmadi',
              style: TextStyle(color: Colors.grey)));
    }
    return ListView.separated(
      padding: EdgeInsets.zero,
      itemCount: _results.length,
      separatorBuilder: (_, __) => Divider(
          height: 1,
          color: Theme.of(context).colorScheme.surfaceContainerHighest),
      itemBuilder: (context, i) {
        final r = _results[i];
        return ListTile(
          leading: const Icon(Icons.location_on_outlined),
          title: Text(r['description'] ?? ''),
          onTap: _selecting ? null : () => _selectResult(r),
        );
      },
    );
  }
}
