-- Tez-tez bajariladigan so'rovlar uchun indekslar (optimizatsiya, 2026-09-15).

-- 1) Kuryerning FAOL buyurtmasi: har joylashuv yuborilganda (≈10 s),
--    `GET /couriers/{id}/active-order` va xodim kirishini yopishda.
--    Avval `idx_orders_courier` (courier_id) bo'yicha kuryerning BUTUN
--    tarixi o'qilib, holat bo'yicha filtrlanardi. Qisman indeks faqat
--    yakunlanmagan buyurtmalarni saqlaydi — kichik va doim tez.
--
--    SHART `storage.activeOrderStatusFilter` bilan HARFMA-HARF bir xil
--    bo'lishi SHART, aks holda rejalashtiruvchi indeksni ishlatmaydi
--    (test: TestActiveCourierIndexMatchesQuery).
CREATE INDEX IF NOT EXISTS idx_orders_courier_active
    ON orders (courier_id, created_at DESC)
    WHERE status NOT IN ('delivered', 'served', 'rejected', 'cancelled');

-- 2) Obyekt akkaunti (restoran telefoni — kuryer buyurtmani har ochganda;
--    admin ro'yxatlari `role` bo'yicha). Avval `users` jadvali — barcha
--    MIJOZLAR bilan birga — to'liq ko'rib chiqilardi.
CREATE INDEX IF NOT EXISTS idx_users_role_entity ON users (role, entity_id);
