import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
    // Firebase Phone Auth (`google-services.json` shu plagin orqali
    // o'qiladi). Fayl `android/app/google-services.json` da bo'lishi
    // SHART — u `.gitignore` da, chunki loyihaga xos konfiguratsiya.
    id("com.google.gms.google-services")
}

// ┌─ RELIZ IMZOSI ─────────────────────────────────────────────────────┐
// Kalit `android/key.properties` dan o'qiladi. U `.gitignore` da —
// imzolash kaliti repoga HECH QACHON tushmasligi kerak.
//
// CI'da fayl sirlardan (secrets) yoziladi — `.github/workflows/
// shorebird.yml` ga qarang.
//
// FAYL YO'Q BO'LSA build YIQILMAYDI, debug kalitiga tushadi. Bu
// ATAYLAB: ishlab chiquvchi `flutter run --release` ni kalitsiz ham
// bajara olishi kerak. Lekin TARQATILADIGAN build har doim haqiqiy
// kalit bilan imzolanishi shart — pastdagi `isSigned` tekshiruvi
// buni build vaqtida ogohlantirib turadi.
//
// ⚠️ KALIT YO'QOLSA ILOVANI BOSHQA YANGILAB BO'LMAYDI. Play Store
// yangilanishni faqat AYNI kalit bilan imzolangan paketdan qabul
// qiladi. Zaxira nusxasini xavfsiz joyda saqlang.
// └────────────────────────────────────────────────────────────────────┘
val keystoreProperties = Properties().apply {
    val f = rootProject.file("key.properties")
    if (f.exists()) f.inputStream().use { load(it) }
}
val hasReleaseKey = keystoreProperties.getProperty("storeFile") != null

android {
    namespace = "com.ondex.customer"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        // TODO: Specify your own unique Application ID (https://developer.android.com/studio/build/application-id.html).
        applicationId = "com.ondex.customer"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    signingConfigs {
        if (hasReleaseKey) {
            create("release") {
                storeFile = rootProject.file(keystoreProperties.getProperty("storeFile"))
                storePassword = keystoreProperties.getProperty("storePassword")
                keyAlias = keystoreProperties.getProperty("keyAlias")
                keyPassword = keystoreProperties.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            signingConfig = if (hasReleaseKey) {
                signingConfigs.getByName("release")
            } else {
                // Lokal ishlab chiqish uchun zaxira. Bunday build
                // TARQATILMAYDI — quyidagi ogohlantirish shu haqda.
                logger.warn(
                    "\n⚠️  RELEASE BUILD DEBUG KALITI BILAN IMZOLANMOQDA.\n" +
                    "   `android/key.properties` yo'q. Bunday paketni Play Store\n" +
                    "   qabul qilmaydi va Shorebird yamoqlari ham ishlamaydi.\n"
                )
                signingConfigs.getByName("debug")
            }
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
