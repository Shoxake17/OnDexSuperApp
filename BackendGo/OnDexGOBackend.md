# ChustApp Go Backend Arxitekturasi

## Umumiy Tavsif

ChustApp - Go language yozilgan keng ko'lamli yetkazib berish platformasi. Backend Go 1.26.6 versiyasida yozilgan va professional arxitekturaga ega.

### Asosiy Texnologiyalar

- **Go 1.26.6** - Backend dasturlash tili
- **PostgreSQL** - Asosiy ma'lumotlar bazasi (pgx/v5 driver)
- **MongoDB** - NoSQL ma'lumotlar bazasi
- **Redis** - Keshlash
- **AWS S3 / Cloudflare R2** - Fayl saqlash
- **Google AI** - Sun'iy intellekt integratsiyasi
- **JWT** - Autentifikatsiya
- **WebSocket** - Real-time aloqa

---

## Loyiha Tuzilishi

```
ChustApp/
├── cmd/                    # CLI ilovalar
│   ├── api/               # Asosiy API server
│   ├── agentkey/          # Agent kalitlari boshqaruvi
│   ├── bookpages/         # Kitob sahifalari
│   ├── r2upload/          # R2 yuklash vositasi
│   └── wswatch/           # WebSocket monitoring
├── internal/              # Ichki paketlar
│   ├── httpapi/           # HTTP API qatlami
│   ├── users/             # Foydalanuvchilar boshqaruvi
│   ├── orders/            # Buyurtmalar tizimi
│   ├── catalog/           # Katalog/Menyu
│   ├── couriers/          # Kuryerlar boshqaruvi
│   ├── delivery/          # Yetkazib berish logikasi
│   ├── payments/          # To'lov tizimi
│   ├── tables/            # Stol/QR kodlar
│   ├── ws/                # WebSocket
│   ├── cache/             # Keshlash
│   ├── notify/            # Bildirishnomalar
│   ├── assistant/         # AI yordamchi
│   ├── agentapi/          # Tashqi AI agentlar
│   ├── model3d/           # 3D model generatsiyasi
│   ├── scenes/            # 3D maketlar
│   ├── books/             # Kitoblar
│   ├── promotions/        # Aksiyalar
│   ├── firebaseauth/      # Firebase autentifikatsiya
│   ├── telegram/          # Telegram integratsiyasi
│   ├── geo/               # Geografiya
│   ├── images/            # Rasm ishlash
│   ├── storage/           # Fayl saqlash
│   ├── ratelimit/         # Tezlik chegaralari
│   ├── revoke/            # Token bekor qilish
│   └── safego/            # Xavfsiz goroutine
├── apps/                  # Mobil ilovalar
│   ├── customer_app/      # Mijoz ilovasi
│   ├── courier_app/       # Kuryer ilovasi
│   ├── restaurant_panel/  # Restoran paneli
│   ├── waiter_app/        # Ofitsiant ilovasi
│   ├── admin_panel/       # Admin paneli
│   └── web/               # Veb versiya
└── packages/              # Umumiy paketlar
```

---

## Arxitektura Naqshlari

### 1. Layered Architecture (Qatlamli Arxitektura)

Backend to'rtta asosiy qatlamdan iborat:

```
┌─────────────────────────────────────┐
│   HTTP API Layer (httpapi)          │  - Endpointlar, Middleware
├─────────────────────────────────────┤
│   Service Layer (internal/*)         │  - Biznes logika
├─────────────────────────────────────┤
│   Repository Layer (internal/*)     │  - Ma'lumotlar bazasi
├─────────────────────────────────────┤
│   Infrastructure Layer              │  - Tashqi servislar
└─────────────────────────────────────┘
```

### 2. Dependency Injection

Bog'liqliklar `Deps` struct orqali beriladi:

```go
type Deps struct {
    OrderRepo   orders.Repository
    CourierRepo couriers.Repository
    UserRepo    users.Repository
    CatalogRepo catalog.Repository
    // ... 50+ bog'liqlik
}
```

### 3. Interface-based Design

Barcha servislar interfeys orqali ishlashadi:

```go
type Repository interface {
    GetByID(ctx context.Context, id string) (*Order, error)
    Save(ctx context.Context, o *Order) error
    // ...
}
```

---

## HTTP API Qatlami

### Server Initialization

```go
// cmd/api/main.go
func main() {
    // Bog'liqliklarni yig'ish
    deps := httpapi.Deps{
        OrderRepo:   ordersRepo,
        CourierRepo: couriersRepo,
        UserRepo:    usersRepo,
        // ...
    }
    
    // Server yaratish
    api := httpapi.New(deps)
    
    // HTTP serverni ishga tushirish
    srv := &http.Server{
        Addr:              ":8080",
        Handler:           api.Routes(allowedOrigins),
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      30 * time.Second,
    }
    
    log.Fatal(srv.ListenAndServe())
}
```

### Router Structure

`httpapi/server.go` da barcha marshrutlar ro'yxatga olinadi:

```go
func (s *Server) Routes(allowedOrigins []string) http.Handler {
    mux := http.NewServeMux()
    
    s.registerWsRoutes(mux)
    s.registerAuthRoutes(mux)
    s.registerMeRoutes(mux)
    s.registerNotificationRoutes(mux)
    s.registerCatalogRoutes(mux)
    s.registerFavoriteRoutes(mux)
    s.registerOrderRoutes(mux)
    s.registerTableRoutes(mux)
    s.registerBookRoutes(mux)
    s.registerWaiterRoutes(mux)
    s.registerCourierRoutes(mux)
    s.registerPromotionRoutes(mux)
    s.registerPaymentRoutes(mux)
    s.registerAdminRoutes(mux)
    s.registerAdminUserRoutes(mux)
    s.registerGeoRoutes(mux)
    s.registerMapPickerRoutes(mux)
    s.registerUploadRoutes(mux)
    s.registerModel3DRoutes(mux)
    s.registerAgentRoutes(mux)
    s.registerMeAgentRoutes(mux)
    s.registerAssistantRoutes(mux)
    s.registerAssistantLiveRoutes(mux, allowedOrigins)
    s.registerMeAIToolRoutes(mux)
    
    return withBodyLimit(withCORS(mux, allowedOrigins, s.DevMode))
}
```

### Middleware Chain

```go
// httpapi/middleware.go
func withCORS(next http.Handler, origins []string, devMode bool) http.Handler
func withBodyLimit(next http.Handler) http.Handler
func withAuth(next http.Handler, role users.Role) http.Handler
func withRateLimit(next http.Handler, limiter *ratelimit.Limiter) http.Handler
```

---

## Asosiy Modullar

### 1. Users Module (`internal/users`)

**Responsibility**: Foydalanuvchi autentifikatsiyasi va profil boshqaruvi

**Key Components**:
- `Service` - Asosiy biznes logika
- `Repository` - Ma'lumotlar bazasi interfeysi
- `TokenIssuer` - JWT token generatsiyasi
- `CodeStore` - OTP kodlari saqlash

**Main Endpoints**:
```
POST   /auth/code              - SMS kod yuborish
POST   /auth/verify            - Kod tasdiqlash
POST   /auth/firebase          - Firebase Phone Auth
POST   /auth/telegram/start    - Telegram bot kirish
POST   /auth/telegram/miniapp  - Telegram Mini App
POST   /auth/google            - Google kirish
GET    /me                     - Profil ma'lumotlari
PUT    /me                     - Profil yangilash
POST   /me/password            - Parol o'zgartirish
```

**Authentication Flow**:

```
1. SMS/Email/Telegram/Google/Firebase
   ↓
2. IssueCode() - Kod yaratish va saqlash
   ↓
3. Verify() - Kodni tekshirish
   ↓
4. finishPhoneLogin() - Token berish
   ↓
5. JWT token qaytariladi
```

**Security Features**:
- Hashed code storage (HMAC-SHA256)
- 5 minute code expiration
- 60 second resend cooldown
- 5 attempt limit
- Phone number normalization
- Email identity protection
- Telegram linkage verification

### 2. Orders Module (`internal/orders`)

**Responsibility**: Buyurtma boshqaruvi va narxlash

**Key Components**:
- `Service` - Buyurtma logikasi
- `Repository` - Buyurtma saqlash
- `Notifier` - Holat o'zgarish bildirishnomalari
- `PaymentGateway` - To'lov integratsiyasi

**Order Status Flow**:

```
Created → Accepted → Preparing → Ready → Delivering → Completed
                ↓
              Cancelled/Rejected
```

**Main Endpoints**:
```
POST   /orders               - Buyurtma yaratish
POST   /orders/quote         - Narx hisoblash (checkout'dan oldin)
GET    /orders/:id           - Buyurtma ma'lumotlari
GET    /orders               - Buyurtmalar tarixi
PUT    /orders/:id/cancel    - Buyurtmani bekor qilish
POST   /orders/:id/accept    - Buyurtmani qabul qilish (restoran)
POST   /orders/:id/ready     - Buyurtma tayyor (restoran)
POST   /orders/:id/dispatch  - Kuryer biriktirish
POST   /orders/:id/complete  - Buyurtmani yakunlash
```

**Pricing Logic**:

```go
func (s *Service) priceCart(ctx context.Context, restaurantID string, 
    items []Item, customerID string) (subtotal, discount int64, 
    applied promotions.Result, lineDiscounts []int64, err error)
```

**Discount System**:
1. Product-level discounts (individual item)
2. Promotion-level discounts (campaigns)
3. Loyalty-based discounts
4. Line-level discount distribution

**Idempotency**:
```go
// Mijoz shu kalit bilan avval buyurtma bergan bo'lsa,
// yangi buyurtma yaratmaydi, eskisini qaytaradi
if o.IdempotencyKey != "" {
    existing, err := s.repo.FindByIdempotencyKey(ctx, o.CustomerID, o.IdempotencyKey)
    if err == nil {
        return existing, nil
    }
}
```

**Price Protection**:
```go
// Mijoz ko'rgan summa o'zgarmasligini tekshiradi
if expectedTotalTiyin > 0 && o.TotalTiyin != expectedTotalTiyin {
    return nil, &TotalChangedError{
        ExpectedTiyin: expectedTotalTiyin,
        ActualTiyin:   o.TotalTiyin,
    }
}
```

### 3. Catalog Module (`internal/catalog`)

**Responsibility**: Menyu, mahsulotlar va restoranlar

**Key Components**:
- `Service` - Katalog logikasi
- `Repository` - Katalog ma'lumotlari
- `BookRepository` - Kitoblar (MongoDB)

**Main Endpoints**:
```
GET    /restaurants          - Restoranlar ro'yxati
GET    /restaurants/:id      - Restoran ma'lumotlari
GET    /restaurants/:id/menu - Menyu
GET    /products/:id         - Mahsulot ma'lumotlari
POST   /products             - Mahsulot qo'shish (admin)
PUT    /products/:id         - Mahsulot yangilash
DELETE /products/:id         - Mahsulot o'chirish
```

**Menu Caching**:
```go
// Redis keshlash
menuCacheKey := fmt.Sprintf("menu:%s", restaurantID)
cached, err := s.cache.Get(ctx, menuCacheKey)
if err == nil {
    return cached, nil
}
// Bazadan o'qish va keshlash
menu, err := s.repo.GetMenu(ctx, restaurantID)
s.cache.Set(ctx, menuCacheKey, menu, 30*time.Second)
```

### 4. Couriers Module (`internal/couriers`)

**Responsibility**: Kuryerlar boshqaruvi va dispatch

**Key Components**:
- `Service` - Kuryer logikasi
- `Repository` - Kuryer ma'lumotlari
- `Dispatcher` - Buyurtma taqsimoti

**Main Endpoints**:
```
GET    /couriers/location      - Kuryer joylashuvi
POST   /couriers/location      - Joylashuv yangilash
POST   /couriers/available     - Mavjudligi bildirish
POST   /couriers/unavailable   - Mavjud emasligi bildirish
GET    /couriers/orders        - Kuryer buyurtmalari
POST   /couriers/:id/accept    - Buyurtmani qabul qilish
POST   /couriers/:id/reject    - Buyurtmani rad etish
POST   /couriers/:id/complete  - Buyurtmani yakunlash
```

**Dispatch Logic**:
```go
// Geografik yaqinlik asosida kuryer tanlash
func (d *Dispatcher) Dispatch(order *Order) (*Courier, error) {
    // 1. Xizmat hududini tekshirish
    // 2. Mavjud kuryerlarni topish
    // 3. Masofa hisoblash
    // 4. Eng yaqin kuryerni tanlash
    // 5. Kuryerga bildirishnoma yuborish
}
```

**Speed Gate (Anti-GPS Spoofing)**:
```go
// Kuryer koordinatasining "teleport" qilishini aniqlaydi
func (sg *SpeedGate) CheckSpeed(courierID string, oldLat, oldLng, newLat, newLng float64, timeDelta time.Duration) error
```

### 5. Delivery Module (`internal/delivery`)

**Responsibility**: Yetkazib berish logikasi va geografik hisoblar

**Key Components**:
- `Service` - Yetkazib berish logikasi
- `SpeedGate` - GPS spoofing himoyasi

**Main Functions**:
- Distance calculation (Haversine formula)
- Service area validation
- Delivery time estimation
- Speed limit enforcement

### 6. Payments Module (`internal/payments`)

**Responsibility**: To'lov tizimi integratsiyasi

**Key Components**:
- `Service` - To'lov logikasi
- `OctoClient` - Octo Payments integratsiyasi
- `OrderSink` - Buyurtma to'lov oqimi

**Payment Flow**:

```
1. POST /orders (cart → order)
   ↓
2. Create → Status: Created, AwaitingPayment: true
   ↓
3. OctoClient.Hold() → Pul bloklanadi
   ↓
4. OnPaymentHeld → Status: Accepted
   ↓
5. Restoran buyurtmani ko'radi
   ↓
6. CaptureForOrder() → Pul yechiladi
   ↓
7. Complete order
```

**Main Endpoints**:
```
POST   /payments/hold       - Pul bloklash
POST   /payments/capture    - Pul yechish
POST   /payments/release    - Pul bo'shatish/qaytarish
POST   /payments/callback   - Callback qabul qilish
```

**Security**:
- Webhook signature verification
- Idempotent payment processing
- Amount validation from order (not client)
- Timeout handling

### 7. Tables Module (`internal/tables`)

**Responsibility**: Stol QR kodlari va dine-in buyurtmalari

**Key Components**:
- `Service` - Stol logikasi
- `Repository` - Stol ma'lumotlari

**Main Endpoints**:
```
POST   /tables/orders        - Stoldan buyurtma berish
GET    /tables/:token        - Stol ma'lumotlari
POST   /tables/:token/call   - Ofitsiant chaqirish
```

**Dine-in vs Delivery**:
- Dine-in: No address, no courier dispatch
- Payment: Cash to waiter OR card (same as delivery)
- QR code token validation

### 8. WebSocket Module (`internal/ws`)

**Responsibility**: Real-time aloqa

**Key Components**:
- `Hub` - WebSocket ulanish markazi
- `TicketStore` - Ulash ticketlari
- `Client` - Har bir ulanish

**Connection Flow**:

```
1. GET /ws/ticket → ticket olish
2. GET /ws?ticket=xxx → WebSocket ulanish
3. Hub.Register(client)
4. Real-time message exchange
5. Hub.Unregister(client)
```

**Event Types**:
```go
type Event struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

// Order events
"order_created"
"order_status_changed"
"order_dispatched"

// Courier events
"courier_location_updated"
"courier_status_changed"

// Restaurant events
"model3d_updated"
"menu_updated"
```

### 9. Notifications Module (`internal/notify`)

**Responsibility**: Bildirishnomalar va push bildirishnomalar

**Key Components**:
- `Service` - Bildirishnoma yuborish
- `Store` - Bildirishnomalar saqlash
- `TokenStore` - Push tokenlari

**Main Endpoints**:
```
GET    /notifications       - Bildirishnomalar ro'yxati
POST   /me/push-token       - Push token ro'yxatga olish
DELETE /me/push-token       - Token o'chirish
```

**Notification Channels**:
1. WebSocket (real-time)
2. Push notifications (FCM/APNs)
3. SMS (Eskiz.uz)
4. Email (SMTP)

### 10. Cache Module (`internal/cache`)

**Responsibility**: Redis keshlash

**Key Components**:
- `Cache` - Redis klient
- Keshlash strategiyalari

**Cache Keys**:
```go
menuCacheKey := fmt.Sprintf("menu:%s", restaurantID)
restaurantCacheKey := fmt.Sprintf("restaurant:%s", restaurantID)
userCacheKey := fmt.Sprintf("user:%s", userID)
```

**TTL Policies**:
- Menu: 30 seconds
- Restaurant: 5 minutes
- User: 10 minutes

### 11. Assistant Module (`internal/assistant`)

**Responsibility**: Ilova ichidagi AI yordamchi

**Key Components**:
- `Service` - Chat logikasi
- `LiveConfig` - Ovozli rejim (Gemini Live)

**Main Endpoints**:
```
POST   /ai/chat              - Chat bilan muloqot
GET    /ai/live              - Ovozli rejim sozlamalari
POST   /ai/tools             - AI tool'lar
```

**AI Integration**:
- Google GenAI API
- Tool calling (cart suggestions)
- Voice mode (Gemini Live)

### 12. Agent Module (`internal/agentapi`)

**Responsibility**: Tashqi AI agentlar integratsiyasi

**Key Components**:
- `Service` - Agent API logikasi

**Main Endpoints**:
```
POST   /agent/v1/create-order   - Agent buyurtma yaratish
GET    /agent/v1/restaurant     - Restoran ma'lumotlari
POST   /me/agent/grant          - Agent ruxsat berish
GET    /me/agent                - Agentlar ro'yxati
```

**Authentication**:
- Partner API key (for external agents)
- User JWT (for user-controlled agents)

### 13. Model3D Module (`internal/model3d`)

**Responsibility**: Taom rasmidan 3D model generatsiyasi

**Key Components**:
- `Service` - 3D model generatsiya
- `Limiter` - Tezlik chegarasi

**Main Endpoints**:
```
POST   /model3d/generate      - 3D model yaratish
GET    /model3d/status/:id     - Holatni tekshirish
```

**Flow**:
```
1. POST /model3d/generate (image_url)
   ↓
2. Service.CreateTask()
   ↓
3. External API (Tripo) call
   ↓
4. Process in background
   ↓
5. Update status + cache clear
   ↓
6. WebSocket event to restaurant
```

### 14. Promotions Module (`internal/promotions`)

**Responsibility**: Aksiyalar va chegirmalar

**Key Components**:
- `Service` - Aksiya logikasi
- `Repository` - Aksiya ma'lumotlari

**Promotion Types**:
```go
TypePercentage    // Foiz chegirma
TypeFixedAmount   // Qat'iy summa chegirma
TypeLoyalty       // Sadoqat chegirmasi
TypeBuyXGetY      // N ol, O bepul
```

**Main Endpoints**:
```
GET    /promotions          - Aksiyalar ro'yxati
POST   /promotions          - Aksiya yaratish (admin)
PUT    /promotions/:id      - Aksiya yangilash
DELETE /promotions/:id      - Aksiya o'chirish
```

### 15. Books Module (`internal/books`)

**Responsibility**: Kafe kutubxonasi (MongoDB)

**Key Components**:
- `Service` - Kitob logikasi
- `BookRepository` - MongoDB interfeysi

**Main Endpoints**:
```
GET    /books               - Kitoblar ro'yxati
GET    /books/:id           - Kitob ma'lumotlari
POST   /books               - Kitob qo'shish (admin)
PUT    /books/:id           - Kitob yangilash
```

---

Batafsil ma'lumotlar bazasi schema'si uchun OnDexPostgreSQL.md fayliga murojaat qiling.

---

## JWT Token Structure

### Claims
```go
type Claims struct {
    UserID      string `json:"user_id"`
    Role        string `json:"role"`
    PhoneProven bool   `json:"phone_proven"` // 15 min expiration
    Exp         int64  `json:"exp"`
    Iat         int64  `json:"iat"`
}
```

### Token Issuance
```go
// Oddiy token (24 soat)
func (ti *TokenIssuer) Issue(u *User) (string, error)

// Telefon isbotlangan token (15 daqiqa)
func (ti *TokenIssuer) IssuePhoneProven(u *User) (string, error)
```

---

## Error Handling

### Standard Errors
```go
var ErrUserNotFound = errors.New("user not found")
var ErrInvalidPhone = errors.New("invalid phone number")
var ErrInvalidCode = errors.New("invalid code")
var ErrTooManyAttempts = errors.New("too many attempts")
var ErrNotFound = errors.New("order not found")
var ErrConflict = errors.New("concurrent modification")
var ErrTotalChanged = errors.New("price changed")
```

### HTTP Error Response
```go
type ErrorResponse struct {
    Error string `json:"error"`
    Code  string `json:"code,omitempty"`
}
```

---

## Rate Limiting

### Upload Limiter
```go
// Xodim bo'yicha: ~6/minute, max 12 consecutive
UploadLimiter: ratelimit.New(6.0/60.0, 12)
```

### Model3D Limiter
```go
// Restoran bo'yicha chegara
Model3DLimiter: ratelimit.NewLimiter(restaurantID)
```

---

## Security Measures

### 1. Input Validation
- Phone number normalization
- Email format validation
- SQL injection prevention (parameterized queries)
- XSS prevention (proper escaping)

### 2. Authentication
- JWT tokens with expiration
- Phone verification
- Email verification
- Telegram linkage verification
- Firebase ID token verification

### 3. Authorization
- Role-based access control (RBAC)
- Endpoint-level permissions
- Resource ownership checks

### 4. Rate Limiting
- Per-user rate limits
- Per-restaurant limits
- Upload limits
- API endpoint throttling

### 5. Data Protection
- Hashed codes (HMAC-SHA256)
- Password hashing (bcrypt)
- Encrypted database connections
- TLS for all HTTP traffic

### 6. Anti-Abuse
- Speed gate (GPS spoofing detection)
- Idempotency keys
- Request deduplication
- Cooldown periods

---

## Deployment

### Docker Compose
```yaml
version: '3.8'
services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://...
      - REDIS_URL=redis://...
      - JWT_SECRET=...
    depends_on:
      - postgres
      - redis
  
  postgres:
    image: postgres:15
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  redis:
    image: redis:7
    volumes:
      - redis_data:/data
```

### Environment Variables
```env
# Database
DATABASE_URL=postgres://user:pass@localhost:5432/chustapp
MONGO_URL=mongodb://localhost:27017/chustapp

# Redis
REDIS_URL=redis://localhost:6379

# JWT
JWT_SECRET=your-secret-key

# Storage
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_PUBLIC_URL=...

# SMS
ESKIZ_EMAIL=...
ESKIZ_PASSWORD=...

# Email
SMTP_HOST=...
SMTP_PORT=587
SMTP_USER=...
SMTP_PASSWORD=...

# Firebase
FIREBASE_PROJECT_ID=...

# AI
TRIPO_API_KEY=...
SHADDIY_API_KEY=...
GEMINI_API_KEY=...
```

---

## Monitoring

### Structured Logging
```go
slog.Info("order created", 
    "order_id", order.ID,
    "customer_id", order.CustomerID,
    "total_tiyin", order.TotalTiyin)
```

### Metrics
- Request duration
- Error rates
- Database query times
- Cache hit rates
- Active connections

---

## Testing

### Unit Tests
```go
func TestService_Verify(t *testing.T) {
    // Given
    mockRepo := &MockRepository{}
    mockCodes := &MockCodeStore{}
    service := NewService(mockRepo, mockCodes, nil, nil, testIDGen)
    
    // When
    token, user, err := service.Verify(ctx, "+998901234567", "123456")
    
    // Then
    assert.NoError(t, err)
    assert.NotEmpty(t, token)
    assert.Equal(t, "+998901234567", user.Phone)
}
```

### Integration Tests
- Database integration
- External API mocking
- End-to-end scenarios

---

## Performance Optimization

### 1. Caching Strategy
- Menu caching (30s TTL)
- Restaurant caching (5m TTL)
- User session caching (10m TTL)

### 2. Database Optimization
- Connection pooling
- Query optimization
- Indexing strategy
- Read replicas

### 3. HTTP Optimization
- Keep-alive connections
- Response compression
- Static file serving
- CDN integration

### 4. Background Processing
- 3D model generation
- Email sending
- Push notifications
- Cleanup tasks

---

## Scalability

### Horizontal Scaling
- Stateless API servers
- Shared database
- Redis for session/cache
- Load balancer

### Vertical Scaling
- Connection pool tuning
- Memory optimization
- CPU profiling
- Garbage collection tuning

---

## Disaster Recovery

### Database Backups
- Daily full backups
- Point-in-time recovery
- Replication to standby

### Redis Persistence
- RDB snapshots
- AOF logging
- Backup strategy

### Application Recovery
- Graceful shutdown
- State recovery (dispatch, 3D tasks)
- Health checks
- Auto-restart

---

## Future Enhancements

### Planned Features
1. GraphQL API
2. Event sourcing
3. Microservices architecture
4. Advanced analytics
5. Machine learning integration
6. Multi-tenancy support

### Technical Debt
1. Reduce code duplication
2. Improve test coverage
3. Standardize error handling
4. Documentation improvements
5. Performance profiling

---

## Development Guidelines

### Code Style
- Follow Go conventions
- Use meaningful variable names
- Keep functions small
- Write comprehensive comments
- Handle errors properly

### Git Workflow
- Feature branches
- Pull request reviews
- CI/CD pipeline
- Automated testing

### Code Review Checklist
- Security considerations
- Performance impact
- Error handling
- Testing coverage
- Documentation

---

## Contact & Support

### Development Team
- Backend Team
- DevOps Team
- QA Team

### Documentation
- API Documentation
- Architecture diagrams
- Deployment guides
- Troubleshooting guides

---

*Bu hujjat ChustApp Go backendining to'liq arxitekturasi va amaliyotlarini o'z ichiga oladi. Har qanday savol yoki taklif uchun development team bilan bog'laning.*