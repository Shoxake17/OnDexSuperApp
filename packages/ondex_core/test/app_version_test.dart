import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';

void main() {
  test('versiya foydalanuvchiga tushunarli ko\'rinishda', () {
    expect(formatAppVersion('0.2.3+15'), 'v0.2.3 (15)');
    expect(formatAppVersion('0.2.3-dev+15'), 'v0.2.3 (15) · dev');
    expect(formatAppVersion('0.2.3'), 'v0.2.3');
    expect(formatAppVersion('0.2.2-dev'), 'v0.2.2 · dev');
    expect(formatAppVersion(''), 'dev');
    expect(formatAppVersion('dev'), 'dev');
  });
}
