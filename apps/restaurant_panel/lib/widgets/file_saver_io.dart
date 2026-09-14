import 'dart:io';
import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';

/// Saqlangan fayl yo'li; foydalanuvchi bekor qilsa `null`.
Future<String?> saveBytesAs({
  required String fileName,
  required Uint8List bytes,
  required String extension,
}) async {
  final picked = await FilePicker.platform.saveFile(
    dialogTitle: 'Saqlash',
    fileName: fileName,
    type: FileType.custom,
    allowedExtensions: [extension],
    lockParentWindow: true,
  );
  if (picked == null) return null;
  // Foydalanuvchi kengaytmani o'chirib yozgan bo'lsa ham fayl ochiladigan
  // bo'lsin.
  final path =
      picked.toLowerCase().endsWith('.$extension') ? picked : '$picked.$extension';
  await File(path).writeAsBytes(bytes, flush: true);
  return path;
}
