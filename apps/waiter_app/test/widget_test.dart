import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ondex_waiter/screens/login_screen.dart';

void main() {
  // DIQQAT: bu yerda butun `WaiterApp` emas, aynan `LoginScreen`
  // sinaladi.
  //
  // Sabab: `WaiterApp` ishga tushishida saqlangan tokenni o'qiydi va
  // o'sha paytda cheksiz aylanuvchi `CircularProgressIndicator`
  // ko'rsatiladi. `pumpAndSettle` esa "boshqa kadr rejalashtirilmagan"
  // holatni kutadi — aylanuvchi indikator har kadrda yangisini
  // rejalashtiradi, shuning uchun u HECH QACHON tinchimaydi va test
  // timeout bilan yiqiladi (birinchi urinishda aynan shunday bo'ldi).
  //
  // Login ekranining o'zi esa animatsiyasiz va platforma
  // pluginlariga bog'liq emas.

  testWidgets('login ekrani ko\'rsatiladi va kod so\'rash bosqichida turadi',
      (WidgetTester tester) async {
    await tester.pumpWidget(const MaterialApp(home: LoginScreen()));

    expect(find.text('OnDexPro'), findsOneWidget);
    // ┌─ ESKIRGAN KUTILMA TUZATILDI (2026-09-06) ────────────────────┐
    // Bu yerda `'Kod olish'` turardi. Ekran SMS'dan Telegram'ga
    // ko'chirilganda tugma matni `'Telegram orqali kod olish'` ga
    // o'zgargan, test esa yangilanmagan — ya'ni butun `waiter_app`
    // sinov to'plami YIQILGAN holda turgan edi.
    //
    // (Bu topilma bug.md ro'yxatida yo'q edi: sinov auditi Go
    // testlariga qaratilgan bo'lib, Dart testlari ishga
    // tushirilmagan.)
    // └──────────────────────────────────────────────────────────────┘
    expect(find.text('Telegram orqali kod olish'), findsOneWidget);
    // Kod maydoni HALI ko'rinmasligi kerak — u faqat raqam
    // yuborilgandan keyin chiqadi.
    expect(find.text('SMS kod'), findsNothing);
    expect(find.text('Kirish'), findsNothing);
  });

  testWidgets('telefon maydoni +998 bilan boshlanadi',
      (WidgetTester tester) async {
    await tester.pumpWidget(const MaterialApp(home: LoginScreen()));

    final field = tester.widget<TextField>(
      find.widgetWithText(TextField, '+998'),
    );
    expect(field.controller?.text, '+998');
  });
}
