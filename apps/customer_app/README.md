# ChustApp — mijoz ilovasi (Flutter)

Ekranlar: telefon + SMS kod bilan kirish → restoranlar ro'yxati → menyu va savat
→ buyurtma → jonli kuzatuv (WebSocket).

## Ishga tushirish

1. Flutter SDK o'rnating: https://docs.flutter.dev/get-started/install/windows
   (Android Studio + emulyator ham kerak bo'ladi, yoki Windows desktop rejimida sinang)

2. Platforma fayllarini yaratish (bir marta):

   ```powershell
   cd F:\ChustApp\apps\customer_app
   flutter create . --project-name chust_customer
   flutter pub get
   ```

   `flutter create .` android/ios/windows papkalarini yaratadi; `lib/` va
   `pubspec.yaml` ni ustidan yozmaydi (so'rasa — "n" deb javob bering).

3. Backend server ishlab turishi kerak (`go run ./cmd/api` — F:\ChustApp da).

4. Ilovani ishga tushirish:

   ```powershell
   flutter run              # ulangan qurilma/emulyatorda
   flutter run -d windows   # Windows oynasida (eng tez sinov)
   ```

## Muhim: server manzili

`lib/api.dart` dagi `baseUrl`:
- Android **emulyator**: `http://10.0.2.2:8080` (hozirgi qiymat)
- **Windows desktop** rejimi: `http://localhost:8080` ga o'zgartiring
- **Haqiqiy telefon** (bir Wi-Fi'da): kompyuter IP si, masalan `http://192.168.1.5:8080`

## Eslatmalar

- Dev rejimda SMS kod avtomatik to'ldiriladi (server `dev_code` qaytaradi) —
  production'da bu o'chadi va haqiqiy SMS keladi.
- Yetkazish manzili hozircha Chust markazi qilib qotirilgan
  (`menu_screen.dart` dagi `createOrder` chaqiruvi) — keyingi bosqichda
  xaritadan tanlash qo'shiladi.
