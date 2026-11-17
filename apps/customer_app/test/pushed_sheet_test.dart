import 'package:chust_customer/widgets/pushed_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// Push qilingan sahifa o'z Material qatlamiga ega bo'lishi SHART: aks holda
// matn "Material yo'q" ko'rinishida (qizil, sariq tagchiziq) chiziladi —
// bron ekranida shunday bo'lgan (2026-11-17).
void main() {
  testWidgets('PushedSheet matnni Material ichida chizadi', (t) async {
    await t.pumpWidget(MaterialApp(
      home: Builder(
        builder: (ctx) => TextButton(
          onPressed: () => Navigator.of(ctx).push(MaterialPageRoute(
              builder: (_) => const PushedSheet(title: 'Stol bron qilish', child: Text('salom')))),
          child: const Text('och'),
        ),
      ),
    ));
    await t.tap(find.text('och'));
    await t.pumpAndSettle();
    expect(find.text('Stol bron qilish'), findsOneWidget);
    expect(find.ancestor(of: find.text('salom'), matching: find.byType(Scaffold)), findsOneWidget);
    expect(find.ancestor(of: find.text('salom'), matching: find.byType(Material)), findsWidgets);
    expect(t.takeException(), isNull);
  });
}
