import 'api.dart';

/// Reys ekrani uchun JONLI kanal (WebSocket): do'kon buyurtma biriktirganda/olganda va holat
/// o'zgarganda server shaxsiy kanalga `qurt_order` xabarini yuboradi.
///
/// Oddiy kuryer ekrani (`courier_shell.dart`) o'zining taklif kanalini ishlatadi; reys ekrani esa
/// taklif/dispatch oqimiga ulanmaydi, shuning uchun alohida yengil kanal.
final tripLive = LiveBus(
  ticketProvider: api.wsTicket,
  urlBuilder: (t) => wsUrl(t),
);
