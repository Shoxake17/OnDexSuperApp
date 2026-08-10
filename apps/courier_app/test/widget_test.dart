import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:chust_courier/main.dart';

void main() {
  testWidgets('CourierApp yuklanadi va qulamaydi', (WidgetTester tester) async {
    await tester.pumpWidget(const CourierApp());
    await tester.pump();
    expect(find.byType(MaterialApp), findsOneWidget);
  });
}
