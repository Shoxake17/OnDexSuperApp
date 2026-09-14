/// Baytlarni foydalanuvchi tanlagan faylga saqlash: ish stolida
/// "Saqlash" oynasi, vebda brauzer yuklab olishi.
///
/// Ikki implementatsiya shartli import bilan: `dart:io` veb build'ni
/// buzadi, veb yo'li esa ish stolida faylni yozmaydi (`file_picker` ning
/// Windows `saveFile` i baytlarni e'tiborsiz qoldiradi va faqat yo'lni
/// qaytaradi).
library;

export 'file_saver_web.dart' if (dart.library.io) 'file_saver_io.dart';
