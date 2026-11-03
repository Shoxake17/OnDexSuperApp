import 'package:flutter/widgets.dart';

import '../api.dart';
import '../screens/courier_shell.dart';
import 'qurt_shell.dart';

/// Kuryer akkauntining bosh ekrani: Qurt kuryeri bo'lsa Qurt reysi, aks holda oddiy kuryer ekrani.
///
/// Server `GET /courier/qurt/me` ga Qurt kuryeriga 200, oddiy (restoran/platforma) kuryerga 403
/// beradi. Boshqa xato (tarmoq, server) YUTILMAYDI: Qurt kuryerini jimgina oddiy ekranga tushirib
/// yubormaslik uchun chaqiruvchi xatoni ko'rsatadi va qayta urinadi.
Future<Widget> courierHome(String entityId) async {
  try {
    await api.send('GET', '/courier/qurt/me');
    return const QurtShell();
  } on ApiException catch (e) {
    if (e.statusCode == 403) return CourierShell(courierId: entityId);
    rethrow;
  }
}
