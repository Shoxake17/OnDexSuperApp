import 'package:ondex_core/ondex_core.dart';

/// Kuryer ilovasi sessiya kaliti.
///
/// `TokenStore` ning O'ZI endi `ondex_core` da (Android Keystore bilan
/// shifrlangan ombor + eski `SharedPreferences` nusxasidan avtomatik
/// migratsiya). Bu yerda faqat KALIT nomi belgilanadi — bitta qurilmada
/// mijoz va kuryer ilovalari bir-birining sessiyasini almashtirib
/// yubormasligi uchun.
const tokenStore = TokenStore('courier_token');
