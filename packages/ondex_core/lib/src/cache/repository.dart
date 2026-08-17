import 'dart:async';
import 'dart:convert';

import 'cache_store.dart';

/// Ekranga beriladigan qiymat — ma'lumot va uning holati birga.
///
/// Ekran `loading` bayrog'ini o'zi hisoblamaydi: bu yerda allaqachon
/// aytilgan. Shu tufayli har ekranda "spinner qachon chiqadi" degan
/// qoida qaytadan yozilmaydi — u BIR joyda.
class Cached<T> {
  /// Ma'lumot. `null` — hali hech qachon olinmagan.
  final T? value;

  /// Ma'lumot keshdan keldi va hozir tarmoqdan yangilanmoqda.
  final bool refreshing;

  /// Oxirgi urinish xatosi. Ma'lumot BOR bo'lsa ham to'lishi mumkin —
  /// o'shanda ekran eski ma'lumotni ko'rsatib, ustiga kichik ogohlantirish
  /// chiqaradi ("yangilab bo'lmadi"), spinner emas.
  final Object? error;

  const Cached({this.value, this.refreshing = false, this.error});

  /// Spinner FAQAT shu holatda ko'rsatiladi: ma'lumot umuman yo'q va
  /// hozir olinmoqda. Boshqa hamma holatda ekranda mazmun turadi.
  ///
  /// "Loading ko'rinmasin" talabi amalda AYNAN shu bitta getter bilan
  /// bajariladi — ekranlar `if (c.showSpinner)` dan boshqa hech qanday
  /// yuklanish mantig'ini bilmaydi.
  bool get showSpinner => value == null && refreshing;

  /// Ma'lumot ham, xato ham yo'q, yangilanish ham ketmayapti —
  /// ya'ni haqiqatan bo'sh.
  bool get isEmpty => value == null && !refreshing && error == null;
}

/// Kesh kaliti va uning yashash muddati.
class CachePolicy {
  /// Ombordagi kalit. Foydalanuvchiga bog'liq ma'lumot uchun kalitga
  /// foydalanuvchi ID'sini QO'SHMANG — chiqishda kesh baribir
  /// butunlay tozalanadi va ID kalitda turishi ortiqcha ma'lumot
  /// sizib chiqishi demak.
  final String key;

  /// Bu muddatdan keyin ma'lumot "eski" hisoblanadi va fonda
  /// yangilanadi. Ekran baribir DARHOL eski nusxani ko'rsatadi.
  final Duration ttl;

  const CachePolicy(this.key, {this.ttl = const Duration(minutes: 5)});
}

/// Kesh-birinchi repozitoriy.
///
/// ┌─ OQIM ────────────────────────────────────────────────────────────┐
/// 1. Keshda ma'lumot bo'lsa — DARHOL beriladi (`refreshing: true`).
/// 2. Tarmoqdan yangi ma'lumot olinadi va ustidan beriladi.
/// 3. Tarmoq yiqilsa — eski ma'lumot QOLADI, ustiga `error` qo'shiladi.
///
/// Ya'ni foydalanuvchi hech qachon bo'sh ekran ko'rmaydi, agar u
/// ma'lumotni bir marta ko'rgan bo'lsa. Internet umuman bo'lmasa ham
/// ilova ochiladi va ishlaydi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA BITTA IMPLEMENTATSIYA ──────────────────────────────────────┐
/// Bu naqsh har ekranda qo'lda yozilsa (`_loading`, `_error`,
/// `_items`, `initState` da `_load()`), u har safar bir oz boshqacha
/// chiqadi: birida xatoda eski ma'lumot o'chadi, boshqasida qoladi;
/// birida qayta so'rov bor, boshqasida yo'q. Aynan shu tarqoqlik
/// "ba'zan loading chiqadi, ba'zan chiqmaydi" holatini yaratadi.
///
/// Shuning uchun qoida: EKRANLAR bu sinfdan boshqa yuklanish mantig'i
/// YOZMAYDI.
/// └───────────────────────────────────────────────────────────────────┘
class Repository<T> {
  final CacheStore store;
  final CachePolicy policy;

  /// Tarmoqdan olish.
  final Future<T> Function() fetch;

  /// Obyektni JSON'ga va orqaga o'girish.
  final Object? Function(T value) encode;
  final T Function(Object? json) decode;

  Repository({
    required this.store,
    required this.policy,
    required this.fetch,
    required this.encode,
    required this.decode,
  });

  /// Kesh → tarmoq oqimi. Ekran shu oqimga obuna bo'ladi.
  ///
  /// `force: true` — "pastga tortib yangilash" uchun: kesh yangi
  /// bo'lsa ham tarmoqqa boradi.
  Stream<Cached<T>> observe({bool force = false}) async* {
    final entry = await _readEntry();

    // ── 1-qadam: keshdagi nusxa DARHOL ────────────────────────────
    final fresh = entry != null && !_isStale(entry.savedAt);
    if (entry != null) {
      // Kesh yangi bo'lsa va majburlanmagan bo'lsa — tarmoqqa
      // umuman bormaymiz. Bu batareyani va trafikni tejaydi.
      if (fresh && !force) {
        yield Cached<T>(value: entry.value);
        return;
      }
      yield Cached<T>(value: entry.value, refreshing: true);
    } else {
      // Hech qachon ko'rilmagan — YAGONA spinner holati.
      yield Cached<T>(refreshing: true);
    }

    // ── 2-qadam: tarmoq ───────────────────────────────────────────
    try {
      final value = await fetch();
      await _writeEntry(value);
      yield Cached<T>(value: value);
    } catch (e) {
      // Eski ma'lumot SAQLANADI. Uni o'chirish eng yomon xatti-harakat
      // bo'lardi: tarmoq uzilgani uchun foydalanuvchi allaqachon
      // ko'rgan ro'yxatini yo'qotardi.
      yield Cached<T>(value: entry?.value, error: e);
    }
  }

  /// Keshni tarmoqsiz o'qish (masalan ilova ishga tushganda
  /// oldindan chizish uchun).
  Future<T?> peek() async => (await _readEntry())?.value;

  /// Ma'lumotni qo'lda yozish — optimistik yangilanish uchun.
  ///
  /// Masalan sevimlilarga qo'shilganda ro'yxatni darhol yangilaymiz,
  /// server javobini kutmasdan.
  Future<void> put(T value) => _writeEntry(value);

  Future<void> invalidate() => store.remove(policy.key);

  // ── Ichki ─────────────────────────────────────────────────────────

  bool _isStale(DateTime savedAt) =>
      DateTime.now().difference(savedAt) > policy.ttl;

  Future<_Entry<T>?> _readEntry() async {
    final raw = await store.read(policy.key);
    if (raw == null) return null;
    try {
      final map = jsonDecode(raw) as Map<String, dynamic>;
      final at = DateTime.fromMillisecondsSinceEpoch(map['at'] as int);
      return _Entry(decode(map['v']), at);
    } catch (_) {
      // Buzilgan yoki eski formatdagi yozuv — o'chiriladi va
      // ma'lumot tarmoqdan olinadi. Ilova YIQILMAYDI.
      //
      // Bu naqsh sxema o'zgarganda ham ishlaydi: yangi versiya eski
      // JSON'ni o'qiy olmasa, u shunchaki keshni tashlab yuboradi.
      await store.remove(policy.key);
      return null;
    }
  }

  Future<void> _writeEntry(T value) async {
    try {
      await store.write(
        policy.key,
        jsonEncode({
          'at': DateTime.now().millisecondsSinceEpoch,
          'v': encode(value),
        }),
      );
    } catch (_) {
      // Kesh yozilmasa ilova ishlashda davom etadi.
    }
  }
}

class _Entry<T> {
  final T value;
  final DateTime savedAt;
  const _Entry(this.value, this.savedAt);
}
