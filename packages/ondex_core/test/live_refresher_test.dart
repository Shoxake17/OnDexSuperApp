import 'dart:async';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';

/// Qo'lda boshqariladigan jonli kanal: hodisa va ulanish holatini sinov o'zi beradi.
class _FakeBus extends LiveBus {
  _FakeBus()
    : super(ticketProvider: () async => 't', urlBuilder: (t) => 'ws://x/');

  final _ev = StreamController<Map<String, dynamic>>.broadcast();
  final _conn = StreamController<bool>.broadcast();
  bool up = false;

  @override
  Stream<Map<String, dynamic>> get events => _ev.stream;
  @override
  Stream<bool> get connection => _conn.stream;
  @override
  bool get connected => up;
  @override
  void start() {}

  void emit(String type) => _ev.add({'type': type});
  void setConnected(bool v) {
    up = v;
    _conn.add(v);
  }
}

void main() {
  test(
    'ulangan paytda so\'rov YO\'Q (connectedInterval: null): faqat hodisada yangilanadi',
    () {
      fakeAsync((async) {
        final bus = _FakeBus()..up = true;
        var n = 0;
        final r = LiveRefresher(
          bus: bus,
          types: const {'qurt_order'},
          onRefresh: () async => n++,
          connectedInterval: null,
        )..start();
        async.elapse(const Duration(minutes: 10));
        expect(
          n,
          0,
          reason:
              'soket ulangan paytda vaqt bo\'yicha so\'rov bo\'lmasligi kerak',
        );
        bus.emit('qurt_order');
        async.flushMicrotasks();
        expect(n, 1);
        bus.emit('boshqa_tur');
        async.flushMicrotasks();
        expect(n, 1, reason: 'begona hodisa yangilamaydi');
        r.dispose();
      });
    },
  );

  test(
    'soket uzilsa vaqtincha so\'rov ishlaydi, qayta ulanganda to\'xtaydi va bir marta darrov yangilanadi',
    () {
      fakeAsync((async) {
        final bus = _FakeBus()..up = true;
        var n = 0;
        final r = LiveRefresher(
          bus: bus,
          types: const {'qurt_order'},
          onRefresh: () async => n++,
          connectedInterval: null,
          offlineInterval: const Duration(seconds: 10),
        )..start();

        bus.setConnected(false);
        async.flushMicrotasks();
        async.elapse(const Duration(seconds: 35));
        expect(n, 3, reason: 'uzilgan paytda har 10 s so\'raladi');

        bus.setConnected(true);
        async.flushMicrotasks();
        expect(
          n,
          4,
          reason:
              'qayta ulanganda uzilish paytidagi o\'zgarishlar uchun darrov yangilanadi',
        );
        async.elapse(const Duration(minutes: 5));
        expect(n, 4, reason: 'ulangach so\'rov to\'xtaydi');
        r.dispose();
      });
    },
  );
}
