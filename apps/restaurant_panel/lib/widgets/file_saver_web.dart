import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';

/// Vebda brauzer faylni "Yuklab olishlar"ga saqlaydi; yo'l noma'lum,
/// shuning uchun fayl nomi qaytadi.
Future<String?> saveBytesAs({
  required String fileName,
  required Uint8List bytes,
  required String extension,
}) async {
  await FilePicker.platform.saveFile(fileName: fileName, bytes: bytes);
  return fileName;
}
