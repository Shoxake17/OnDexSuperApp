import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';

import '../api.dart';

/// Kafe kutubxonasi — 3D maketdagi javondan olib o'qiladigan kitoblar.
///
/// ┌─ NEGA ALOHIDA BO'LIM ──────────────────────────────────────────────┐
/// Kitob mahsulot emas: narxi yo'q, savatga tushmaydi, buyurtmaga
/// kirmaydi. Uni "Restoranlar" bo'limidagi menyu bilan aralashtirish
/// "kitobga chegirma" kabi ma'nosiz holatlarga yo'l ochardi.
///
/// Shuning uchun alohida sahifa: bu yerda narx maydoni UMUMAN yo'q.
/// └────────────────────────────────────────────────────────────────────┘
class BooksPage extends StatefulWidget {
  /// [restaurantId] berilsa — restoran moduli ichidagi ko'rinish: faqat shu
  /// restoranning kitoblari, restoran tanlash ro'yxati yo'q.
  const BooksPage({super.key, this.restaurantId});

  final String? restaurantId;

  @override
  State<BooksPage> createState() => _BooksPageState();
}

class _BooksPageState extends State<BooksPage> {
  List<dynamic> _restaurants = [];
  List<dynamic> _books = [];
  String? _restaurantId;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    final fixed = widget.restaurantId;
    if (fixed != null && fixed.isNotEmpty) {
      // Restoran modulida restoran allaqachon tanlangan: ro'yxatni
      // yuklash ham, almashtirish ham kerak emas.
      _restaurantId = fixed;
      _loadBooks();
    } else {
      _loadRestaurants();
    }
  }

  Future<void> _loadRestaurants() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final list = await api.restaurants();
      if (!mounted) return;
      setState(() {
        _restaurants = list;
        _restaurantId = list.isEmpty ? null : '${list.first['id']}';
        _loading = false;
      });
      if (_restaurantId != null) await _loadBooks();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _loading = false;
      });
    }
  }

  Future<void> _loadBooks() async {
    final id = _restaurantId;
    if (id == null) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      // Boshqaruv ro'yxati: o'chirilgan kitoblar ham ko'rinadi, aks
      // holda ularni qayta yoqishning yo'li qolmasdi.
      final list = await api.booksAll(id);
      if (!mounted) return;
      setState(() {
        _books = list;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        // Kutubxona ombori ulanmagan bo'lsa server 503 qaytaradi —
        // bu xato emas, sozlama holati. Xabar shuni ochiq aytadi.
        _error = '$e';
        _books = [];
        _loading = false;
      });
    }
  }

  Future<void> _edit({Map<String, dynamic>? book}) async {
    final id = _restaurantId;
    if (id == null) return;

    // Tahrirlashda matn ALOHIDA so'raladi: ro'yxat javobida u yo'q.
    Map<String, dynamic>? full = book;
    if (book != null) {
      try {
        full = await api.book('${book['id']}');
      } catch (_) {
        // Matn olinmasa ham qolgan maydonlarni tahrirlash mumkin.
      }
    }
    if (!mounted) return;

    final saved = await showDialog<bool>(
      context: context,
      builder: (_) => _BookDialog(restaurantId: id, book: full),
    );
    if (saved == true) await _loadBooks();
  }

  Future<void> _delete(Map<String, dynamic> book) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Kitobni o\'chirish'),
        content: Text('"${book['title']}" o\'chirilsinmi?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Bekor qilish'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('O\'chirish'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await api.deleteBook('${book['id']}');
      await _loadBooks();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('O\'chirilmadi: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('Kutubxona',
                  style:
                      TextStyle(fontSize: 26, fontWeight: FontWeight.bold)),
              const SizedBox(width: 20),
              if (widget.restaurantId == null && _restaurants.isNotEmpty)
                DropdownButton<String>(
                  value: _restaurantId,
                  items: [
                    for (final r in _restaurants)
                      DropdownMenuItem(
                        value: '${r['id']}',
                        child: Text('${r['name']}'),
                      ),
                  ],
                  onChanged: (v) {
                    setState(() => _restaurantId = v);
                    _loadBooks();
                  },
                ),
              const Spacer(),
              FilledButton.icon(
                onPressed: _restaurantId == null ? null : () => _edit(),
                icon: const Icon(Icons.add),
                label: const Text('Kitob qo\'shish'),
              ),
            ],
          ),
          const SizedBox(height: 8),
          const Text(
            'Kitoblar 3D maketdagi javonga yaqinlashganda ko\'rinadi. '
            'Ular sotilmaydi — narx maydoni yo\'q.',
            style: TextStyle(color: Colors.black54),
          ),
          const SizedBox(height: 16),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  Widget _buildBody() {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Kitoblar yuklanmadi:\n$_error',
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.red)),
            const SizedBox(height: 12),
            OutlinedButton(
                onPressed: _loadBooks, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }
    if (_books.isEmpty) {
      return const Center(
        child: Text('Bu kafeda hozircha kitob yo\'q',
            style: TextStyle(color: Colors.black54)),
      );
    }
    return ListView.separated(
      itemCount: _books.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (_, i) {
        final b = Map<String, dynamic>.from(_books[i] as Map);
        final active = b['active'] == true;
        final cover = '${b['cover_url'] ?? ''}';
        final pageCount = (b['pages'] as List?)?.length ?? 0;
        // Skanerlangan kitob: PDF bor, matn ham, sahifa rasmi ham yo'q.
        // Bunday kitob maketda ochiladi-yu, ichi bo'sh chiqadi.
        final needsPages = '${b['pdf_url'] ?? ''}'.isNotEmpty &&
            pageCount == 0 &&
            '${b['text'] ?? ''}'.trim().isEmpty;
        return ListTile(
          // Muqova javondagi ko'rinish bilan bir xil nisbatda (2:3) —
          // admin ro'yxatda ham maketdagidek ko'radi.
          leading: SizedBox(
            width: 40,
            height: 60,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: cover.isEmpty
                  ? Container(
                      color: active ? Colors.brown.shade100 : Colors.black12,
                      child: Icon(Icons.menu_book,
                          size: 20,
                          color: active ? Colors.brown : Colors.black38),
                    )
                  : Image.network(
                      imageUrl(cover),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => Container(
                        color: Colors.black12,
                        child: const Icon(Icons.broken_image_outlined,
                            size: 18, color: Colors.black26),
                      ),
                    ),
            ),
          ),
          title: Text('${b['title']}'),
          subtitle: Text([
            if ('${b['author']}'.isNotEmpty) '${b['author']}',
            if ('${b['pdf_url'] ?? ''}'.isNotEmpty) 'PDF',
            // Sahifa rasmlari — skanerlangan kitob maketda AYNAN shular
            // orqali ko'rinadi. Soni ko'rsatiladi, chunki "PDF bor,
            // lekin sahifalar tayyorlanmagan" holati ko'zga
            // tashlanmasa kitob maketda bo'sh chiqadi.
            if (pageCount > 0) '$pageCount sahifa',
            if (!active) 'o\'chirilgan',
          ].join(' · ')),
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (needsPages)
                Tooltip(
                  message: 'Skanerlangan kitob: sahifalari hali '
                      'tayyorlanmagan.\nMaketda ichi bo\'sh ko\'rinadi.\n\n'
                      'Tayyorlash:  go run ./cmd/bookpages -book ${b['id']}',
                  child: const Padding(
                    padding: EdgeInsets.only(right: 8),
                    child: Icon(Icons.warning_amber_rounded,
                        color: Colors.orange),
                  ),
                ),
              // Ko'rinishni tez yoqib-o'chirish: kitobni o'chirmasdan
              // maketdan olib qo'yish uchun.
              Switch(
                value: active,
                onChanged: (v) async {
                  try {
                    await api.updateBook('${b['id']}', active: v);
                    await _loadBooks();
                  } catch (e) {
                    if (!mounted) return;
                    ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text('Saqlanmadi: $e')));
                  }
                },
              ),
              IconButton(
                tooltip: 'Tahrirlash',
                icon: const Icon(Icons.edit_outlined),
                onPressed: () => _edit(book: b),
              ),
              IconButton(
                tooltip: 'O\'chirish',
                icon: const Icon(Icons.delete_outline),
                onPressed: () => _delete(b),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// Kitob qo'shish va tahrirlash oynasi.
class _BookDialog extends StatefulWidget {
  const _BookDialog({required this.restaurantId, this.book});

  final String restaurantId;
  final Map<String, dynamic>? book;

  @override
  State<_BookDialog> createState() => _BookDialogState();
}

class _BookDialogState extends State<_BookDialog> {
  late final _title =
      TextEditingController(text: '${widget.book?['title'] ?? ''}');
  late final _author =
      TextEditingController(text: '${widget.book?['author'] ?? ''}');

  /// Saqlangan muqova manzili. Yangi rasm tanlansa yuklashdan keyin
  /// almashadi.
  late String _coverUrl = '${widget.book?['cover_url'] ?? ''}';
  late String _pdfUrl = '${widget.book?['pdf_url'] ?? ''}';

  /// PDF dan ajratilgan matn. Formada KO'RSATILMAYDI — u serverning
  /// ishi, admin uni tahrirlamaydi.
  late String _text = '${widget.book?['text'] ?? ''}';

  /// Tanlangan, lekin hali yuklanmagan fayllar.
  Uint8List? _coverBytes;
  String? _coverName;
  String? _pdfName;
  int? _pdfSize;

  bool _uploadingCover = false;
  bool _uploadingPdf = false;
  bool _saving = false;
  String? _error;
  String? _notice;

  @override
  void dispose() {
    _title.dispose();
    _author.dispose();
    super.dispose();
  }

  /// Muqova tanlanadi va DARROV yuklanadi.
  ///
  /// Saqlashni kutib turish mumkin edi, lekin unda admin natijani
  /// ko'rmasdan "Saqlash" bosardi va rasm noto'g'ri kesilgani faqat
  /// maketda ma'lum bo'lardi. Yuklash zahoti ko'rinish qaytadi.
  Future<void> _pickCover() async {
    final res = await FilePicker.platform
        .pickFiles(type: FileType.image, withData: true);
    final bytes = res?.files.single.bytes;
    if (bytes == null) return;

    setState(() {
      _coverBytes = bytes;
      _coverName = res!.files.single.name;
      _uploadingCover = true;
      _error = null;
    });
    try {
      final url = await api.uploadBookCover(bytes, _coverName ?? 'cover.jpg');
      if (!mounted) return;
      setState(() {
        _coverUrl = url;
        _uploadingCover = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = 'Muqova yuklanmadi: $e';
        _coverBytes = null;
        _uploadingCover = false;
      });
    }
  }

  /// PDF tanlanadi, yuklanadi va serverdan ajratilgan matn olinadi.
  Future<void> _pickPdf() async {
    final res = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['pdf'],
      withData: true,
    );
    final file = res?.files.single;
    final bytes = file?.bytes;
    if (file == null || bytes == null) return;

    setState(() {
      _pdfName = file.name;
      _pdfSize = bytes.length;
      _uploadingPdf = true;
      _error = null;
      _notice = null;
    });
    try {
      final data = await api.uploadBookPdf(bytes, file.name);
      if (!mounted) return;
      setState(() {
        _pdfUrl = '${data['url'] ?? ''}';
        _text = '${data['text'] ?? ''}';
        _uploadingPdf = false;
        final pages = data['pages'];
        final warning = data['warning'];
        if (warning != null) {
          // Skanerlangan kitob — xato emas, lekin maketda sahifalar
          // bo'sh chiqadi. Admin buni SAQLASHDAN OLDIN bilishi kerak.
          _notice = '$warning';
        } else {
          _notice = '$pages sahifa o\'qildi, ${_text.length} belgi ajratildi';
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = 'PDF yuklanmadi: $e';
        _pdfName = null;
        _pdfSize = null;
        _uploadingPdf = false;
      });
    }
  }

  Future<void> _save() async {
    final title = _title.text.trim();
    if (title.isEmpty) {
      setState(() => _error = 'Nom kiritilmagan');
      return;
    }
    if (_pdfUrl.isEmpty && _text.trim().isEmpty) {
      setState(() => _error = 'PDF yuklanmagan');
      return;
    }

    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final existing = widget.book;
      if (existing == null) {
        await api.createBook(
          restaurantId: widget.restaurantId,
          title: title,
          author: _author.text.trim(),
          coverUrl: _coverUrl,
          pdfUrl: _pdfUrl,
          text: _text,
        );
      } else {
        await api.updateBook(
          '${existing['id']}',
          title: title,
          author: _author.text.trim(),
          coverUrl: _coverUrl,
          pdfUrl: _pdfUrl,
          text: _text,
        );
      }
      if (!mounted) return;
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _saving = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final isNew = widget.book == null;
    final busy = _saving || _uploadingCover || _uploadingPdf;
    return AlertDialog(
      title: Text(isNew ? 'Yangi kitob' : 'Kitobni tahrirlash'),
      content: SizedBox(
        width: 640,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _coverPreview(),
                  const SizedBox(width: 20),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        TextField(
                          controller: _title,
                          decoration: const InputDecoration(
                            labelText: 'Kitob nomi',
                            border: OutlineInputBorder(),
                          ),
                        ),
                        const SizedBox(height: 14),
                        TextField(
                          controller: _author,
                          decoration: const InputDecoration(
                            labelText: 'Muallif',
                            border: OutlineInputBorder(),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 20),
              _pdfPicker(),
              if (_notice != null) ...[
                const SizedBox(height: 12),
                Row(
                  children: [
                    const Icon(Icons.info_outline,
                        size: 18, color: Colors.blueGrey),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(_notice!,
                          style: const TextStyle(color: Colors.blueGrey)),
                    ),
                  ],
                ),
              ],
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: const TextStyle(color: Colors.red)),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: busy ? null : () => Navigator.pop(context, false),
          child: const Text('Bekor qilish'),
        ),
        FilledButton(
          onPressed: busy ? null : _save,
          child: Text(_saving ? 'Saqlanmoqda...' : 'Saqlash'),
        ),
      ],
    );
  }

  /// Muqova: bosiladigan tik to'rtburchak (2:3 — server ham shu
  /// nisbatga kesadi, ya'ni bu yerdagi ko'rinish natijaga to'g'ri keladi).
  Widget _coverPreview() {
    const w = 140.0;
    const h = w * 3 / 2;
    Widget inner;
    if (_uploadingCover) {
      inner = const Center(child: CircularProgressIndicator());
    } else if (_coverBytes != null) {
      inner = Image.memory(_coverBytes!, fit: BoxFit.cover, width: w, height: h);
    } else if (_coverUrl.isNotEmpty) {
      inner = Image.network(
        imageUrl(_coverUrl),
        fit: BoxFit.cover,
        width: w,
        height: h,
        errorBuilder: (_, __, ___) => const Center(
            child: Icon(Icons.broken_image_outlined, color: Colors.black26)),
      );
    } else {
      inner = const Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.add_photo_alternate_outlined,
              size: 32, color: Colors.black38),
          SizedBox(height: 6),
          Text('Muqova\nrasmi',
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.black45, fontSize: 12)),
        ],
      );
    }
    return InkWell(
      onTap: _uploadingCover ? null : _pickCover,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        width: w,
        height: h,
        decoration: BoxDecoration(
          color: Colors.black12,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: Colors.black26),
        ),
        clipBehavior: Clip.antiAlias,
        child: inner,
      ),
    );
  }

  Widget _pdfPicker() {
    final has = _pdfUrl.isNotEmpty || _pdfName != null;
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: has ? Colors.green.withValues(alpha: 0.06) : Colors.black12,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: has ? Colors.green : Colors.black26),
      ),
      child: Row(
        children: [
          Icon(has ? Icons.picture_as_pdf : Icons.upload_file,
              color: has ? Colors.green.shade700 : Colors.black45),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _pdfName ?? (has ? 'PDF yuklangan' : 'Kitob PDF fayli'),
                  style: const TextStyle(fontWeight: FontWeight.w600),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  _pdfSize != null
                      ? '${(_pdfSize! / 1024 / 1024).toStringAsFixed(1)} MB'
                      : 'Maks 25 MB. Matn serverda o\'zi ajratiladi.',
                  style: const TextStyle(color: Colors.black54, fontSize: 12),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          if (_uploadingPdf)
            const SizedBox(
                width: 22,
                height: 22,
                child: CircularProgressIndicator(strokeWidth: 2))
          else
            OutlinedButton(
              onPressed: _pickPdf,
              child: Text(has ? 'Almashtirish' : 'PDF tanlash'),
            ),
        ],
      ),
    );
  }
}
