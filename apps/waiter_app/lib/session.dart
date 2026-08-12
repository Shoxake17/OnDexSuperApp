import 'package:ondex_core/ondex_core.dart';

/// Affitsiant ilovasi sessiya kaliti.
///
/// `TokenStore` ning O'ZI `ondex_core` da (Android Keystore / iOS
/// Keychain bilan shifrlangan ombor). Bu yerda faqat KALIT nomi —
/// bitta qurilmada mijoz, kuryer va affitsiant ilovalari bir-birining
/// sessiyasini almashtirib yubormasligi uchun.
const tokenStore = TokenStore('waiter_token');
