package com.ondex.customer

import io.flutter.embedding.android.FlutterFragmentActivity

// FlutterActivity EMAS, FlutterFragmentActivity.
//
// `local_auth` (barmoq izi / yuz / grafik kalit / PIN) Android'ning
// BiometricPrompt'iga tayanadi, u esa FragmentActivity talab qiladi.
// Oddiy FlutterActivity bilan plagin ishga tushishda xato beradi va
// qulf oynasi UMUMAN ochilmaydi.
class MainActivity : FlutterFragmentActivity()
