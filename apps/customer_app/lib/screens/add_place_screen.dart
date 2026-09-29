import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:image_picker/image_picker.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';

import '../map/ondexmap_api.dart';
import 'catalog_screen.dart' show kBrand;

/// OB'EKT QO'SHISH — foydalanuvchi xaritaga yangi joy taklif qiladi.
///
/// ┌─ TAKLIF, DARHOL E'LON EMAS ────────────────────────────────────────┐
/// Yuborilgan ob'ekt xaritaga DARHOL chiqmaydi: u karantinga tushadi
/// va moderator ko'rib chiqqandan keyin hammaga ko'rinadi. Foydalanuvchi
/// buni yuborishdan OLDIN bilishi kerak — aks holda «qo'shdim, lekin
/// yo'q» degan tushunmovchilik bo'lardi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ NIMA YUBORILMAYDI ────────────────────────────────────────────────┐
/// Ilova bu so'rovga foydalanuvchi haqida HECH NARSA qo'shmaydi: na
/// akkaunt, na telefon raqami, na qurilma belgisi. Taklif anonim
/// ketadi va OnDexMap tarafida faqat suiste'molga qarshi (soatlik
/// chegara uchun) belgi saqlanadi.
///
/// Formadagi telefon/sayt maydonlari — OB'EKTNING o'zi haqida
/// (masalan do'konning telefoni), foydalanuvchining emas.
/// └────────────────────────────────────────────────────────────────────┘
class AddPlaceScreen extends StatefulWidget {
  const AddPlaceScreen({super.key, required this.lat, required this.lng});

  /// Xaritada tanlangan nuqta.
  final double lat;
  final double lng;

  @override
  State<AddPlaceScreen> createState() => _AddPlaceScreenState();
}

class _AddPlaceScreenState extends State<AddPlaceScreen> {
  final _api = OndexMapApi();
  final _form = GlobalKey<FormState>();

  final _name = TextEditingController();
  final _category = TextEditingController();
  final _description = TextEditingController();
  final _phone = TextEditingController();
  final _hours = TextEditingController();
  final _street = TextEditingController();
  final _house = TextEditingController();
  final _site = TextEditingController();

  String _kind = kOndexPlaceKinds.first.key;
  final _photos = <({String name, Uint8List bytes})>[];
  bool _sending = false;

  @override
  void dispose() {
    for (final c in [
      _name, _category, _description, _phone,
      _hours, _street, _house, _site,
    ]) {
      c.dispose();
    }
    _api.dispose();
    super.dispose();
  }

  /* ── Rasmlar ─────────────────────────────────────────────────────── */

  Future<void> _pickPhoto() async {
    if (_photos.length >= kOndexMaxPhotos) return;
    try {
      final x = await ImagePicker().pickImage(
        source: ImageSource.gallery,
        // Serverda ham chegara bor, lekin kattaroq faylni tarmoqqa
        // chiqarmaslik yaxshiroq: mobil internetda 6 MB sezilarli.
        maxWidth: 2048,
        maxHeight: 2048,
        imageQuality: 85,
      );
      if (x == null) return;
      final bytes = await x.readAsBytes();
      if (bytes.length > kOndexMaxPhotoBytes) {
        _say('Rasm juda katta — 6 MB dan oshmasin');
        return;
      }
      if (!mounted) return;
      setState(() => _photos.add((name: x.name, bytes: bytes)));
    } catch (e) {
      _say('Rasm tanlanmadi');
      debugPrint('[ondexmap] rasm tanlashda xato: $e');
    }
  }

  /* ── Yuborish ────────────────────────────────────────────────────── */

  Future<void> _submit() async {
    if (!_form.currentState!.validate() || _sending) return;
    setState(() => _sending = true);
    try {
      await _api.submitPlace(
        kind: _kind,
        lat: widget.lat,
        lng: widget.lng,
        name: _name.text,
        category: _category.text,
        description: _description.text,
        phone: _phone.text,
        hours: _hours.text,
        street: _street.text,
        house: _house.text,
        site: _site.text,
        photos: [for (final p in _photos) (name: p.name, bytes: p.bytes)],
      );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } on OndexMapException catch (e) {
      _say(e.message);
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  void _say(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), behavior: SnackBarBehavior.floating),
    );
  }

  /* ── Ko'rinish ───────────────────────────────────────────────────── */

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        surfaceTintColor: Colors.white,
        leading: IconButton(
          icon: const Icon(LucideIcons.arrowLeft),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: const Text('Ob\'ekt qo\'shish',
            style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700)),
      ),
      body: Form(
        key: _form,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            _Coords(lat: widget.lat, lng: widget.lng),
            const SizedBox(height: 18),

            const _Label('Ob\'ekt turi'),
            DropdownButtonFormField<String>(
              initialValue: _kind,
              decoration: _dec(),
              items: [
                for (final k in kOndexPlaceKinds)
                  DropdownMenuItem(value: k.key, child: Text(k.label)),
              ],
              onChanged: (v) => setState(() => _kind = v ?? _kind),
            ),
            const SizedBox(height: 14),

            const _Label('Nomi *'),
            TextFormField(
              controller: _name,
              decoration: _dec(hint: 'Masalan: «Chinor» kafesi'),
              textCapitalization: TextCapitalization.sentences,
              maxLength: 120,
              validator: (v) {
                final s = (v ?? '').trim();
                if (s.length < 2) return 'Nom kamida 2 belgi bo\'lsin';
                return null;
              },
            ),

            const _Label('Turkum'),
            TextFormField(
              controller: _category,
              decoration: _dec(hint: 'Kafe, do\'kon, dorixona…'),
              maxLength: 60,
            ),

            const _Label('Manzil'),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  flex: 3,
                  child: TextFormField(
                    controller: _street,
                    decoration: _dec(hint: 'Ko\'cha'),
                    maxLength: 120,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: TextFormField(
                    controller: _house,
                    decoration: _dec(hint: 'Uy'),
                    maxLength: 20,
                  ),
                ),
              ],
            ),

            const _Label('Telefon'),
            TextFormField(
              controller: _phone,
              decoration: _dec(hint: '+998 90 123 45 67'),
              keyboardType: TextInputType.phone,
              // Faqat raqam va telefon belgilari: boshqa matn server
              // tomonidan baribir rad etiladi.
              inputFormatters: [
                FilteringTextInputFormatter.allow(RegExp(r'[0-9+()\-\s]')),
              ],
              maxLength: 32,
            ),

            const _Label('Ish vaqti'),
            TextFormField(
              controller: _hours,
              decoration: _dec(hint: '09:00–21:00, dam olishsiz'),
              maxLength: 80,
            ),

            const _Label('Veb-sayt'),
            TextFormField(
              controller: _site,
              decoration: _dec(hint: 'example.uz'),
              keyboardType: TextInputType.url,
              maxLength: 200,
            ),

            const _Label('Izoh'),
            TextFormField(
              controller: _description,
              decoration: _dec(hint: 'Qo\'shimcha ma\'lumot'),
              maxLines: 3,
              maxLength: 600,
              textCapitalization: TextCapitalization.sentences,
            ),

            const SizedBox(height: 6),
            _Label('Rasmlar (${_photos.length}/$kOndexMaxPhotos)'),
            _PhotoRow(
              photos: _photos,
              onAdd: _photos.length < kOndexMaxPhotos ? _pickPhoto : null,
              onRemove: (i) => setState(() => _photos.removeAt(i)),
            ),

            const SizedBox(height: 20),
            const _ModerationNote(),
            const SizedBox(height: 16),

            FilledButton(
              style: FilledButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
                shape:
                    RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)),
              ),
              onPressed: _sending ? null : _submit,
              child: _sending
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white))
                  : const Text('Yuborish',
                      style: TextStyle(fontWeight: FontWeight.w700)),
            ),
          ],
        ),
      ),
    );
  }

  InputDecoration _dec({String? hint}) => InputDecoration(
        hintText: hint,
        hintStyle: const TextStyle(color: Color(0xFF9E9E9E)),
        filled: true,
        fillColor: const Color(0xFFF4F4F5),
        counterText: '',
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
}

class _Label extends StatelessWidget {
  const _Label(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(top: 12, bottom: 6, left: 2),
        child: Text(text,
            style: const TextStyle(
                fontSize: 13.5,
                fontWeight: FontWeight.w600,
                color: Color(0xFF52525B))),
      );
}

/// Tanlangan nuqtaning koordinatasi — foydalanuvchi to'g'ri joyni
/// belgilaganini tekshira olishi uchun.
class _Coords extends StatelessWidget {
  const _Coords({required this.lat, required this.lng});
  final double lat;
  final double lng;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFFF4F4F5),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          const Icon(LucideIcons.mapPin, size: 18, color: kBrand),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              '${lat.toStringAsFixed(6)}, ${lng.toStringAsFixed(6)}',
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }
}

class _PhotoRow extends StatelessWidget {
  const _PhotoRow({
    required this.photos,
    required this.onAdd,
    required this.onRemove,
  });

  final List<({String name, Uint8List bytes})> photos;
  final VoidCallback? onAdd;
  final ValueChanged<int> onRemove;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 84,
      child: ListView(
        scrollDirection: Axis.horizontal,
        children: [
          for (var i = 0; i < photos.length; i++)
            Padding(
              padding: const EdgeInsets.only(right: 10),
              child: Stack(
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(12),
                    child: Image.memory(photos[i].bytes,
                        width: 84, height: 84, fit: BoxFit.cover),
                  ),
                  Positioned(
                    right: 2,
                    top: 2,
                    child: InkWell(
                      onTap: () => onRemove(i),
                      child: Container(
                        padding: const EdgeInsets.all(3),
                        decoration: const BoxDecoration(
                          color: Colors.black54,
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(LucideIcons.x,
                            size: 13, color: Colors.white),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          if (onAdd != null)
            InkWell(
              onTap: onAdd,
              borderRadius: BorderRadius.circular(12),
              child: Container(
                width: 84,
                height: 84,
                decoration: BoxDecoration(
                  color: const Color(0xFFF4F4F5),
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: const Color(0xFFE4E4E7)),
                ),
                child: const Icon(LucideIcons.imagePlus,
                    color: Color(0xFF71717A)),
              ),
            ),
        ],
      ),
    );
  }
}

/// Moderatsiya haqida ogohlantirish — yuborishdan OLDIN.
class _ModerationNote extends StatelessWidget {
  const _ModerationNote();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF7ED),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFFFED7AA)),
      ),
      child: const Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(LucideIcons.info, size: 18, color: Color(0xFFEA580C)),
          SizedBox(width: 10),
          Expanded(
            child: Text(
              'Ob\'ekt darhol xaritaga chiqmaydi. Moderator tekshirgandan '
              'keyin u hammaga ko\'rinadi.',
              style: TextStyle(fontSize: 13, height: 1.35, color: Color(0xFF7C2D12)),
            ),
          ),
        ],
      ),
    );
  }
}
