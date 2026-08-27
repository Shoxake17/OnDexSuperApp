# ┌─ GODOT SINFLARI CHALKASHTIRILMASIN ────────────────────────────────┐
# Book Cafe 3D sayohati OnDex ichida Godot dvigateli bilan ishlaydi.
# Dvigatelning C++ qismi Java sinflarini NOMI BO'YICHA, JNI orqali
# chaqiradi.
#
# R8 release qurilishida nomlarni qisqartiradi va natijada dvigatel
# o'z sinfini topa olmay ilova qulaydi:
#
#   java.lang.NoSuchMethodError: no non-static method
#     "Lorg/godotengine/godot/nativeapi/GodotNativeBridge;
#      .getRenderView()Lorg/godotengine/godot/GodotRenderView;"
#
# Xato faqat RELEASE da chiqadi — debug qurilishida R8 ishlamaydi va
# hammasi joyida ko'rinadi. Shuning uchun uni telefonda release bilan
# sinamaguncha payqash mumkin emas edi.
# └────────────────────────────────────────────────────────────────────┘

-keep class org.godotengine.** { *; }
-keepclassmembers class org.godotengine.** { *; }
-keep interface org.godotengine.** { *; }

# Godot plaginlari ham nomi bo'yicha topiladi.
-keep class * extends org.godotengine.godot.plugin.GodotPlugin { *; }

# ┌─ NATIVE METODLAR ──────────────────────────────────────────────────┐
# `native` deb belgilangan metodning nomi C++ tomondagi funksiya nomi
# bilan bog'langan. Qisqartirilsa bog'lanish uziladi.
#
# Bu qoida butun ilovaga tegishli: Flutter plaginlari ham JNI
# ishlatadi.
# └────────────────────────────────────────────────────────────────────┘
-keepclasseswithmembernames class * {
    native <methods>;
}

# 3D oynani ochadigan faoliyat manifestda e'lon qilingan, lekin uning
# `getCommandLine()` metodi dvigatel tomonidan chaqiriladi.
-keep class com.ondex.customer.BookCafeActivity { *; }
