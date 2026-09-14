import 'dart:io' show File;

import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';

import 'models.dart';

/// Chatga yuborish uchun rasm tanlash oynasi (Windows/web).
///
/// Hajm fayl O'QILISHIDAN OLDIN tekshiriladi: 2 GB video tanlab
/// qo'yilsa ham panel uni xotiraga yuklab qotib qolmaydi.
/// Shakl (JPG/PNG/WEBP imzosi) keyin `validateSupportImage` da tekshiriladi.
Future<SupportImageDraft?> pickSupportImage() async {
  final res = await FilePicker.platform.pickFiles(
    type: FileType.custom,
    allowedExtensions: kSupportImageExtensions,
    allowMultiple: false,
    withData: kIsWeb,
    dialogTitle: 'Rasm tanlang',
  );
  if (res == null || res.files.isEmpty) return null;
  final f = res.files.first;
  if (f.size > kSupportMaxImageBytes) {
    throw const SupportImageException('Rasm 10 MB dan oshmasligi kerak');
  }
  Uint8List? bytes = f.bytes;
  if (bytes == null && !kIsWeb && f.path != null) {
    final file = File(f.path!);
    if (await file.length() > kSupportMaxImageBytes) {
      throw const SupportImageException('Rasm 10 MB dan oshmasligi kerak');
    }
    bytes = await file.readAsBytes();
  }
  if (bytes == null || bytes.isEmpty) {
    throw const SupportImageException('Rasmni o\'qib bo\'lmadi');
  }
  return SupportImageDraft(bytes: bytes, filename: f.name);
}
