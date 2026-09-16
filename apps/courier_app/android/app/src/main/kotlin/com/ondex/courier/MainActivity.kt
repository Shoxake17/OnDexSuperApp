package com.ondex.courier

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        // "Orqaga" tugmasi ilovani YOPMAYDI — fonga o'tkazadi (Yandex Pro
        // kabi). Avval Android Activity'ni yo'q qilardi: Flutter jarayoni,
        // WebSocket va joylashuv oqimi to'xtardi, kuryer "liniyada" bo'lsa
        // ham taklif kelmasdi, ko'rilayotgan taklif esa yo'qolardi.
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "com.ondex.courier/app")
            .setMethodCallHandler { call, result ->
                if (call.method == "moveToBack") {
                    result.success(moveTaskToBack(true))
                } else {
                    result.notImplemented()
                }
            }
    }
}
