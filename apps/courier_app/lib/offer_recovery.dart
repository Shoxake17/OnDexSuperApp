/// Serverdagi ochiq taklif bilan ekrandagisini solishtirib, nima qilishni
/// hal qiladi (`GET /couriers/{id}/offer` javobi kelganda).
enum OfferRecoveryAction {
  /// Hech narsa o'zgarmaydi.
  none,

  /// Server taklifini ko'rsatish (ekranda yo'q yoki boshqa taklif turibdi).
  show,

  /// Shu taklif ekranda — faqat qolgan soniyalarni serverdan olish.
  updateSeconds,

  /// Ekrandagi taklif eskirgan (boshqa kuryer oldi yoki muddati tugadi).
  dismiss,
}

/// [shownAt] — ekrandagi taklif qachon ko'rsatilgani; [requestedAt] —
/// so'rov qachon yuborilgani.
///
/// Server "taklif yo'q" desa ham ekrandagi taklif so'rovdan KEYIN kelgan
/// bo'lsa (WebSocket javobdan tezroq yetdi) unga tegilmaydi — aks holda
/// yangi taklif o'sha zahoti yopilib qolardi.
OfferRecoveryAction offerRecoveryAction({
  required Map<String, dynamic>? shown,
  required DateTime? shownAt,
  required Map<String, dynamic>? server,
  required DateTime requestedAt,
}) {
  final serverId = server?['order_id'];
  if (serverId is! String || serverId.isEmpty) {
    if (shown != null && shownAt != null && shownAt.isBefore(requestedAt)) {
      return OfferRecoveryAction.dismiss;
    }
    return OfferRecoveryAction.none;
  }
  if (shown?['order_id'] == serverId) return OfferRecoveryAction.updateSeconds;
  return OfferRecoveryAction.show;
}
