# ChustApp PostgreSQL Ma'lumotlar Bazasi

## Umumiy Tavsif

ChustApp PostgreSQL asosiy ma'lumotlar bazasi sifatida ishlaydi. Barcha asosiy ma'lumotlar (foydalanuvchilar, buyurtmalar, restoranlar, mahsulotlar, kuryerlar va h.k.) shu yerda saqlanadi.

---

## Ma'lumotlar Bazasi Tuzilishi

### 1. users - Foydalanuvchilar jadvali

Foydalanuvchilar haqidagi barcha ma'lumotlar.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Foydalanuvchi ID (UUID) |
| phone | VARCHAR(20) | UNIQUE | Telefon raqami (+998901234567) |
| email | VARCHAR(255) | UNIQUE | Email manzil |
| phone_verified | BOOLEAN | DEFAULT FALSE | Telefon tasdiqlanganmi |
| email_verified | BOOLEAN | DEFAULT FALSE | Email tasdiqlanganmi |
| password_hash | VARCHAR(255) | - | Parol hash (bcrypt) |
| first_name | VARCHAR(100) | - | Ism |
| last_name | VARCHAR(100) | - | Familya |
| name | VARCHAR(200) | - | To'liq ism |
| role | VARCHAR(20) | DEFAULT 'customer' | Rol (customer, courier, restaurant_admin, superadmin) |
| telegram_id | BIGINT | UNIQUE | Telegram ID |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_users_phone` (phone)
- `idx_users_email` (email)
- `idx_users_telegram_id` (telegram_id)
- `idx_users_role` (role)

---

### 2. orders - Buyurtmalar jadvali

Barcha buyurtmalar ma'lumotlari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Buyurtma ID (UUID) |
| order_number | VARCHAR(20) | UNIQUE | Buyurtma raqami (masalan: ORD-12345) |
| customer_id | VARCHAR(36) | FOREIGN KEY → users.id | Mijoz ID |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| courier_id | VARCHAR(36) | FOREIGN KEY → couriers.id | Kuryer ID (mavjud bo'lsa) |
| status | VARCHAR(20) | - | Holat (created, accepted, preparing, ready, delivering, completed, cancelled, rejected) |
| items | JSONB | - | Mahsulotlar ro'yxati |
| subtotal_tiyin | BIGINT | - | Jami (chegirmasiz) tiyinda |
| discount_tiyin | BIGINT | - | Chegirma summasi tiyinda |
| total_tiyin | BIGINT | - | Yakuniy summa tiyinda |
| payment_method | VARCHAR(20) | - | To'lov usuli (cash, card) |
| awaiting_payment | BOOLEAN | DEFAULT FALSE | To'lovni kutmoqdamimi |
| promotion_id | VARCHAR(36) | - | Aksiya ID |
| promotion_name | VARCHAR(200) | - | Aksiya nomi |
| promotion_discount_tiyin | BIGINT | - | Aksiya chegirmasi tiyinda |
| preparation_minutes | INT | - | Tayyorlash vaqti (daqiqa) |
| delivery_address | JSONB | - | Yetkazib berish manzili |
| table_token | VARCHAR(100) | - | Stol tokeni (dine-in uchun) |
| party_size | INT | - | Odam soni (dine-in uchun) |
| idempotency_key | VARCHAR(100) | UNIQUE | Idempotency kaliti |
| version | INT | DEFAULT 1 | Versiya (optimistik qulflash uchun) |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_orders_customer_id` (customer_id)
- `idx_orders_restaurant_id` (restaurant_id)
- `idx_orders_courier_id` (courier_id)
- `idx_orders_status` (status)
- `idx_orders_created_at` (created_at DESC)
- `idx_orders_order_number` (order_number)
- `idx_orders_idempotency` (customer_id, idempotency_key) UNIQUE

**items JSONB tuzilishi:**
```json
[
  {
    "product_id": "uuid",
    "name": "Lag'mon",
    "price_tiyin": 25000,
    "discount_price_tiyin": 0,
    "qty": 2,
    "category": "main_dish"
  }
]
```

**delivery_address JSONB tuzilishi:**
```json
{
  "street": "Amir Temur ko'chasi",
  "house": "15",
  "apartment": "4",
  "floor": "2",
  "entrance": "1",
  "comment": "Eskiz qo'ng'irog'ini bosing",
  "lat": 41.311158,
  "lng": 69.279730
}
```

---

### 3. restaurants - Restoranlar jadvali

Restoranlar haqidagi ma'lumotlar.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Restoran ID (UUID) |
| name | VARCHAR(200) | - | Restoran nomi |
| description | TEXT | - | Tavsif |
| image_url | VARCHAR(500) | - | Logosining URL manzili |
| cover_url | VARCHAR(500) | - | Cover rasmi URL manzili |
| phone | VARCHAR(20) | - | Telefon raqami |
| address | JSONB | - | Manzil |
| location | JSONB | - | Joylashuv (lat, lng) |
| service_area | JSONB | - | Xizmat hududi (polygon) |
| opening_hours | JSONB | - | Ish vaqti |
| rating | DECIMAL(3,2) | - | Reyting (0.00 - 5.00) |
| is_active | BOOLEAN | DEFAULT TRUE | Faolmi |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_restaurants_name` (name)
- `idx_restaurants_is_active` (is_active)
- `idx_restaurants_location` (location)
- `idx_restaurants_rating` (rating)

**address JSONB tuzilishi:**
```json
{
  "street": "Amir Temur ko'chasi",
  "house": "15",
  "district": "Yunusobod tumani",
  "city": "Toshkent"
}
```

**location JSONB tuzilishi:**
```json
{
  "lat": 41.311158,
  "lng": 69.279730
}
```

**service_area JSONB tuzilishi:**
```json
{
  "type": "Polygon",
  "coordinates": [
    [[69.27, 41.31], [69.28, 41.31], [69.28, 41.32], [69.27, 41.32], [69.27, 41.31]]
  ]
}
```

**opening_hours JSONB tuzilishi:**
```json
{
  "monday": {"open": "09:00", "close": "23:00"},
  "tuesday": {"open": "09:00", "close": "23:00"},
  "wednesday": {"open": "09:00", "close": "23:00"},
  "thursday": {"open": "09:00", "close": "23:00"},
  "friday": {"open": "09:00", "close": "23:00"},
  "saturday": {"open": "10:00", "close": "00:00"},
  "sunday": {"open": "10:00", "close": "00:00"}
}
```

---

### 4. products - Mahsulotlar jadvali

Restoran menyusidagi mahsulotlar.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Mahsulot ID (UUID) |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| name | VARCHAR(200) | - | Mahsulot nomi |
| description | TEXT | - | Tavsif |
| image_url | VARCHAR(500) | - | Rasm URL manzili |
| price_tiyin | BIGINT | - | Asl narx tiyinda |
| discount_price_tiyin | BIGINT | - | Chegirma narxi tiyinda |
| category | VARCHAR(100) | - | Kategoriya |
| is_available | BOOLEAN | DEFAULT TRUE | Mavjudmi |
| preparation_minutes | INT | - | Tayyorlash vaqti (daqiqa) |
| model3d_status | VARCHAR(20) | - | 3D model holati (none, pending, processing, ready, failed) |
| model3d_url | VARCHAR(500) | - | 3D model URL manzili |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_products_restaurant_id` (restaurant_id)
- `idx_products_category` (category)
- `idx_products_is_available` (is_available)
- `idx_products_name` (name)

---

### 5. couriers - Kuryerlar jadvali

Kuryerlar haqidagi ma'lumotlar.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Kuryer ID (UUID) |
| user_id | VARCHAR(36) | FOREIGN KEY → users.id | Foydalanuvchi ID |
| phone | VARCHAR(20) | - | Telefon raqami |
| name | VARCHAR(200) | - | Ism |
| vehicle_type | VARCHAR(20) | - | Transport turi (bicycle, motorcycle, car) |
| is_available | BOOLEAN | DEFAULT TRUE | Mavjudmi |
| current_location | JSONB | - | Hozirgi joylashuv |
| active_order_id | VARCHAR(36) | FOREIGN KEY → orders.id | Faol buyurtma ID |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_couriers_user_id` (user_id)
- `idx_couriers_is_available` (is_available)
- `idx_couriers_active_order_id` (active_order_id)
- `idx_couriers_phone` (phone)

**current_location JSONB tuzilishi:**
```json
{
  "lat": 41.311158,
  "lng": 69.279730,
  "updated_at": "2026-09-09T12:00:00Z"
}
```

---

### 6. promotions - Aksiyalar jadvali

Restoran aksiyalari va chegirmalari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Aksiya ID (UUID) |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| name | VARCHAR(200) | - | Aksiya nomi |
| description | TEXT | - | Tavsif |
| type | VARCHAR(20) | - | Turi (percentage, fixed_amount, loyalty, buy_x_get_y) |
| value | BIGINT | - | Qiymat (foiz yoki summa) |
| min_order_tiyin | BIGINT | - | Minimal buyurtma summasi |
| max_discount_tiyin | BIGINT | - | Maksimal chegirma summasi |
| usage_count | INT | DEFAULT 0 | Foydalanish soni |
| sales_tiyin | BIGINT | DEFAULT 0 | Savdo summasi tiyinda |
| conditions | JSONB | - | Shartlar |
| is_active | BOOLEAN | DEFAULT TRUE | Faolmi |
| starts_at | TIMESTAMP | - | Boshlanish vaqti |
| ends_at | TIMESTAMP | - | Tugash vaqti |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_promotions_restaurant_id` (restaurant_id)
- `idx_promotions_is_active` (is_active)
- `idx_promotions_type` (type)
- `idx_promotions_dates` (starts_at, ends_at)

**conditions JSONB tuzilishi:**
```json
{
  "min_order_items": 2,
  "categories": ["main_dish", "drink"],
  "product_ids": ["uuid1", "uuid2"],
  "loyalty_min_orders": 5,
  "buy_x": 2,
  "get_y": 1
}
```

---

### 7. favorites - Sevimlilar jadvali

Mijozlarning sevimli mahsulotlari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | ID (UUID) |
| customer_id | VARCHAR(36) | FOREIGN KEY → users.id | Mijoz ID |
| product_id | VARCHAR(36) | FOREIGN KEY → products.id | Mahsulot ID |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |

**Indekslar:**
- `idx_favorites_customer_id` (customer_id)
- `idx_favorites_product_id` (product_id)
- `idx_favorites_restaurant_id` (restaurant_id)
- `idx_favorites_unique` (customer_id, product_id) UNIQUE

---

### 8. notifications - Bildirishnomalar jadvali

Foydalanuvchi bildirishnomalari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Bildirishnoma ID (UUID) |
| user_id | VARCHAR(36) | FOREIGN KEY → users.id | Foydalanuvchi ID |
| type | VARCHAR(50) | - | Turi (order_status, promotion, system) |
| title | VARCHAR(200) | - | Sarlavha |
| body | TEXT | - | Matn |
| data | JSONB | - | Qo'shimcha ma'lumotlar |
| read | BOOLEAN | DEFAULT FALSE | O'qilganmi |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |

**Indekslar:**
- `idx_notifications_user_id` (user_id)
- `idx_notifications_type` (type)
- `idx_notifications_read` (read)
- `idx_notifications_created_at` (created_at DESC)

**data JSONB tuzilishi:**
```json
{
  "order_id": "uuid",
  "status": "completed",
  "restaurant_name": "Bek's Restoran"
}
```

---

### 9. push_tokens - Push tokenlar jadvali

Foydalanuvchi qurilma tokenlari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | ID (UUID) |
| user_id | VARCHAR(36) | FOREIGN KEY → users.id | Foydalanuvchi ID |
| token | VARCHAR(500) | - | Push token |
| platform | VARCHAR(20) | - | Platforma (ios, android) |
| device_id | VARCHAR(100) | - | Qurilma ID |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_push_tokens_user_id` (user_id)
- `idx_push_tokens_token` (token)
- `idx_push_tokens_unique` (user_id, token) UNIQUE

---

### 10. tables - Stollar jadvali

Restoran stollari va QR kodlari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | Stol ID (UUID) |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| table_number | VARCHAR(20) | - | Stol raqami |
| token | VARCHAR(100) | UNIQUE | QR kod tokeni |
| capacity | INT | - | Sig'im (odam soni) |
| is_active | BOOLEAN | DEFAULT TRUE | Faolmi |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| updated_at | TIMESTAMP | - | Yangilangan vaqt |

**Indekslar:**
- `idx_tables_restaurant_id` (restaurant_id)
- `idx_tables_token` (token)
- `idx_tables_table_number` (restaurant_id, table_number) UNIQUE

---

### 11. verification_codes - Tasdiqlash kodlari jadvali

OTP kodlari (Redis'da saqlanadi, lekin tushuntirish uchun).

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| target | VARCHAR(255) | PRIMARY KEY | Telefon yoki email |
| code_hash | VARCHAR(255) | - | Kod hash (HMAC-SHA256) |
| attempts | INT | DEFAULT 0 | Urinishlar soni |
| expires_at | TIMESTAMP | - | Muddati tugash vaqti |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |

**Eslatma:** Bu jadval asosan Redis'da saqlanadi, PostgreSQL'da faqat backup uchun.

---

### 12. revoked_tokens - Bekor qilingan tokenlar jadvali

Bekor qilingan JWT tokenlar ro'yxati.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| jti | VARCHAR(100) | PRIMARY KEY | Token JTI (JWT ID) |
| user_id | VARCHAR(36) | FOREIGN KEY → users.id | Foydalanuvchi ID |
| expires_at | TIMESTAMP | - | Muddati tugash vaqti |
| revoked_at | TIMESTAMP | - | Bekor qilingan vaqt |

**Indekslar:**
- `idx_revoked_tokens_user_id` (user_id)
- `idx_revoked_tokens_expires_at` (expires_at)

---

### 13. devices - Qurilmalar jadvali

Foydalanuvchi qurilmalari tarixi.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | ID (UUID) |
| user_id | VARCHAR(36) | FOREIGN KEY → users.id | Foydalanuvchi ID |
| platform | VARCHAR(20) | - | Platforma (tma, mobile, web) |
| user_agent | VARCHAR(500) | - | User Agent |
| ip_address | VARCHAR(50) | - | IP manzil |
| last_seen | TIMESTAMP | - | Oxirgi ko'rilgan vaqt |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |

**Indekslar:**
- `idx_devices_user_id` (user_id)
- `idx_devices_platform` (platform)
- `idx_devices_last_seen` (last_seen)

---

### 14. restaurant_admins - Restoran administratorlari jadvali

Restoran administratorlari bog'lanish jadvali.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | ID (UUID) |
| user_id | VARCHAR(36) | FOREIGN KEY → users.id | Foydalanuvchi ID |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| role | VARCHAR(20) | - | Rol (admin, manager) |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |

**Indekslar:**
- `idx_restaurant_admins_user_id` (user_id)
- `idx_restaurant_admins_restaurant_id` (restaurant_id)
- `idx_restaurant_admins_unique` (user_id, restaurant_id) UNIQUE

---

### 15. waiter_calls - Ofitsiant chaqirishlari jadvali

Stoldan ofitsiant chaqirishlari.

| Ustun | Turi | Cheklovlar | Izoh |
|-------|------|------------|------|
| id | VARCHAR(36) | PRIMARY KEY | ID (UUID) |
| table_id | VARCHAR(36) | FOREIGN KEY → tables.id | Stol ID |
| restaurant_id | VARCHAR(36) | FOREIGN KEY → restaurants.id | Restoran ID |
| status | VARCHAR(20) | - | Holat (pending, answered, cancelled) |
| created_at | TIMESTAMP | - | Yaratilgan vaqt |
| answered_at | TIMESTAMP | - | Javob berilgan vaqt |

**Indekslar:**
- `idx_waiter_calls_table_id` (table_id)
- `idx_waiter_calls_restaurant_id` (restaurant_id)
- `idx_waiter_calls_status` (status)
- `idx_waiter_calls_created_at` (created_at)

---

## Ko'p jadvalli so'rovlar

### 1. Buyurtma bilan to'liq ma'lumot
```sql
SELECT 
    o.id,
    o.order_number,
    o.status,
    o.total_tiyin,
    o.created_at,
    u.phone AS customer_phone,
    u.name AS customer_name,
    r.name AS restaurant_name,
    r.phone AS restaurant_phone,
    c.name AS courier_name,
    c.phone AS courier_phone
FROM orders o
JOIN users u ON o.customer_id = u.id
JOIN restaurants r ON o.restaurant_id = r.id
LEFT JOIN couriers c ON o.courier_id = c.id
WHERE o.id = $1;
```

### 2. Restoran menyusi
```sql
SELECT 
    p.id,
    p.name,
    p.description,
    p.image_url,
    p.price_tiyin,
    p.discount_price_tiyin,
    p.category,
    p.is_available,
    p.preparation_minutes,
    p.model3d_status,
    p.model3d_url
FROM products p
WHERE p.restaurant_id = $1 
    AND p.is_available = TRUE
ORDER BY p.category, p.name;
```

### 3. Kuryer faol buyurtmasi
```sql
SELECT 
    o.id,
    o.order_number,
    o.status,
    o.total_tiyin,
    o.delivery_address,
    r.name AS restaurant_name,
    r.location AS restaurant_location,
    u.name AS customer_name,
    u.phone AS customer_phone
FROM orders o
JOIN restaurants r ON o.restaurant_id = r.id
JOIN users u ON o.customer_id = u.id
WHERE o.courier_id = $1 
    AND o.status IN ('delivering', 'accepted', 'ready')
ORDER BY o.created_at DESC
LIMIT 1;
```

### 4. Mijoz buyurtmalari tarixi
```sql
SELECT 
    o.id,
    o.order_number,
    o.status,
    o.total_tiyin,
    o.created_at,
    r.name AS restaurant_name,
    r.image_url AS restaurant_image
FROM orders o
JOIN restaurants r ON o.restaurant_id = r.id
WHERE o.customer_id = $1
ORDER BY o.created_at DESC
LIMIT 20;
```

### 5. Restoran so'nggi buyurtmalari
```sql
SELECT 
    o.id,
    o.order_number,
    o.status,
    o.total_tiyin,
    o.created_at,
    o.items,
    u.name AS customer_name,
    u.phone AS customer_phone
FROM orders o
JOIN users u ON o.customer_id = u.id
WHERE o.restaurant_id = $1
ORDER BY o.created_at DESC
LIMIT 50;
```

---

## Triggerlar

### 1. updated_at avtomatik yangilash
```sql
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_restaurants_updated_at BEFORE UPDATE ON restaurants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_couriers_updated_at BEFORE UPDATE ON couriers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### 2. Kuryer faol buyurtmasini yangilash
```sql
CREATE OR REPLACE FUNCTION update_courier_active_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status IN ('delivering', 'accepted', 'ready') AND OLD.status != NEW.status THEN
        UPDATE couriers 
        SET active_order_id = NEW.id 
        WHERE id = NEW.courier_id;
    ELSIF NEW.status = 'completed' AND OLD.status != NEW.status THEN
        UPDATE couriers 
        SET active_order_id = NULL 
        WHERE id = NEW.courier_id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_courier_active_order_trigger 
    AFTER UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_courier_active_order();
```

---

## View'lar

### 1. active_orders_view - Faol buyurtmalar
```sql
CREATE VIEW active_orders_view AS
SELECT 
    o.id,
    o.order_number,
    o.status,
    o.total_tiyin,
    o.created_at,
    r.name AS restaurant_name,
    r.location AS restaurant_location,
    u.name AS customer_name,
    u.phone AS customer_phone,
    c.name AS courier_name,
    c.phone AS courier_phone,
    c.current_location AS courier_location
FROM orders o
JOIN restaurants r ON o.restaurant_id = r.id
JOIN users u ON o.customer_id = u.id
LEFT JOIN couriers c ON o.courier_id = c.id
WHERE o.status NOT IN ('completed', 'cancelled', 'rejected');
```

### 2. restaurant_stats_view - Restoran statistikasi
```sql
CREATE VIEW restaurant_stats_view AS
SELECT 
    r.id,
    r.name,
    r.rating,
    COUNT(DISTINCT o.id) AS total_orders,
    COUNT(DISTINCT CASE WHEN o.created_at > NOW() - INTERVAL '7 days' THEN o.id END) AS orders_last_7_days,
    SUM(CASE WHEN o.status = 'completed' THEN o.total_tiyin ELSE 0 END) AS total_revenue,
    AVG(CASE WHEN o.status = 'completed' THEN o.total_tiyin ELSE NULL END) AS avg_order_value
FROM restaurants r
LEFT JOIN orders o ON r.id = o.restaurant_id
GROUP BY r.id, r.name, r.rating;
```

---

## Ma'lumotlar Bazasi Sozlamalari

### Connection Pool
```go
// pgx connection pool config
config, _ := pgxpool.ParseConfig(connectionString)
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute
```

### Performance Tavsiyalari

1. **Indekslar:**
   - Ko'p ishlatiladigan ustunlarga indeks qo'shing
   - Composite indekslardan foydalaning
   - Indekslarni muntazam tekshiring

2. **Query Optimization:**
   - EXPLAIN ANALYZE dan foydalaning
   - N+1 muammosidan qoching
   - Batch insertlaridan foydalaning

3. **Maintenance:**
   - VACUUM muntazam ishga tushiring
   - ANALYZE statistikasini yangilang
   - Log fayllarini kuzating

4. **Backup:**
   - Kunlik backup qiling
   - Point-in-time recovery sozlang
   - Backuplarni tekshiring

---

## Migration'lar

### Migration 001: Asosiy jadvallar
```sql
CREATE TABLE users (...);
CREATE TABLE restaurants (...);
CREATE TABLE products (...);
CREATE TABLE orders (...);
CREATE TABLE couriers (...);
```

### Migration 002: Indekslar
```sql
CREATE INDEX idx_users_phone ON users(phone);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
-- ...
```

### Migration 003: Triggerlar
```sql
CREATE FUNCTION update_updated_at_column() ...;
CREATE TRIGGER update_users_updated_at ...;
```

---

*Bu hujjat ChustApp PostgreSQL ma'lumotlar bazasining to'liq tuzilishini o'z ichiga oladi.*