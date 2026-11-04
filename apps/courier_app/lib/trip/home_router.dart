import 'package:flutter/widgets.dart';

import '../api.dart';
import '../screens/courier_shell.dart';
import 'trip_shell.dart';

/// Kuryer akkauntining bosh ekrani: Qurt kuryeri bo'lsa Reys ekrani, aks holda oddiy kuryer ekrani.
///
/// Server `GET /courier/qurt/me` ga reys kuryeriga 200, oddiy (restoran/platforma) kuryerga 403
/// beradi. Boshqa xato (tarmoq, server) YUTILMAYDI: reys kuryerini jimgina oddiy ekranga tushirib
/// yubormaslik uchun chaqiruvchi xatoni ko'rsatadi va qayta urinadi.
Future<Widget> courierHome(String entityId) async {
  try {
    await api.send('GET', '/courier/qurt/me');
    return const TripShell();
  } on ApiException catch (e) {
    if (e.statusCode == 403) return CourierShell(courierId: entityId);
    rethrow;
  }
}
