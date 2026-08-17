import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';

/// Xotiradagi soxta ombor — haqiqiy plaginlarsiz sinash uchun.
class _MemStore implements CacheStore {
  final Map<String, String> data = {};
  int writes = 0;

  @override
  Future<String?> read(String key) async => data[key];

  @override
  Future<void> write(String key, String json) async {
    writes++;
    data[key] = json;
  }

  @override
  Future<void> remove(String key) async => data.remove(key);

  @override
  Future<void> clearAll() async => data.clear();
}

Repository<List<String>> _repo(
  _MemStore store, {
  required Future<List<String>> Function() fetch,
  Duration ttl = const Duration(minutes: 5),
}) {
  return Repository<List<String>>(
    store: store,
    policy: CachePolicy('test', ttl: ttl),
    fetch: fetch,
    encode: (v) => v,
    decode: (j) => (j as List).cast<String>(),
  );
}

void main() {
  // ★ ENG MUHIM KAFOLAT: tarmoq yiqilsa eski ma'lumot QOLADI.
  //
  // Busiz foydalanuvchi metroga kirganda yoki internet uzilganda
  // allaqachon ko'rgan ro'yxatini yo'qotardi va bo'sh ekran ko'rardi —
  // butun kesh qatlamining maqsadi aynan shuni oldini olish.
  test('tarmoq xatosida eski ma\'lumot saqlanadi', () async {
    final store = _MemStore();
    await _repo(store, fetch: () async => ['a', 'b']).observe().drain<void>();

    final states = await _repo(
      store,
      ttl: Duration.zero, // keshni darhol eski qilamiz
      fetch: () async => throw Exception('tarmoq yo\'q'),
    ).observe().toList();

    expect(states.first.value, ['a', 'b'], reason: 'kesh darhol berilishi kerak');
    expect(states.first.refreshing, isTrue);

    expect(states.last.value, ['a', 'b'], reason: 'xatoda ham ma\'lumot QOLADI');
    expect(states.last.error, isNotNull);
    expect(states.last.showSpinner, isFalse, reason: 'spinner chiqmasligi kerak');
  });

  test('bo\'sh keshda spinner faqat bir marta, keyin ma\'lumot', () async {
    final states = await _repo(
      _MemStore(),
      fetch: () async => ['x'],
    ).observe().toList();

    expect(states.first.showSpinner, isTrue, reason: 'birinchi ko\'rish — spinner');
    expect(states.last.value, ['x']);
    expect(states.last.showSpinner, isFalse);
  });

  // Yangi kesh bo'lsa tarmoqqa UMUMAN borilmaydi — trafik va
  // batareya tejaladi, ekran esa bir zumda ochiladi.
  test('yangi kesh bo\'lsa tarmoq chaqirilmaydi', () async {
    final store = _MemStore();
    await _repo(store, fetch: () async => ['a']).observe().drain<void>();

    var called = false;
    final states = await _repo(store, fetch: () async {
      called = true;
      return ['yangi'];
    }).observe().toList();

    expect(called, isFalse);
    expect(states.single.value, ['a']);
    expect(states.single.refreshing, isFalse);
  });

  test('force: yangi kesh bo\'lsa ham tarmoqdan yangilanadi', () async {
    final store = _MemStore();
    await _repo(store, fetch: () async => ['eski']).observe().drain<void>();

    final states = await _repo(store, fetch: () async => ['yangi'])
        .observe(force: true)
        .toList();

    expect(states.first.value, ['eski'], reason: 'avval eski nusxa ko\'rinadi');
    expect(states.last.value, ['yangi']);
  });

  // Sxema o'zgarganda eski JSON o'qilmay qolishi MUMKIN — o'shanda
  // ilova yiqilmasligi, shunchaki tarmoqdan olishi kerak.
  test('buzilgan kesh ilovani yiqitmaydi', () async {
    final store = _MemStore();
    store.data['test'] = '{buzilgan json';

    final states = await _repo(store, fetch: () async => ['tiklandi'])
        .observe()
        .toList();

    expect(states.first.showSpinner, isTrue);
    expect(states.last.value, ['tiklandi']);
  });

  test('chiqishda kesh tozalanadi', () async {
    final store = _MemStore();
    final repo = _repo(store, fetch: () async => ['shaxsiy']);
    await repo.observe().drain<void>();
    expect(await repo.peek(), isNotNull);

    await store.clearAll();
    expect(await repo.peek(), isNull);
  });
}
