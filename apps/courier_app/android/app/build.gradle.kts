import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// +- FIREBASE (TAKLIF PUSH'I) ----------------------------------------+
// `google-services.json` (Firebase Console -> com.ondex.courier va
// com.ondex.courier.dev) shu papkada bo'lsa plagin qo'llanadi. Fayl
// `.gitignore` da. Yo'q bo'lsa build YIQILMAYDI: ilova push'siz
// ishlaydi (taklif WebSocket va ilova ochilganda tiklash orqali keladi),
// `lib/push.dart` Firebase xatosini yutadi.
// +------------------------------------------------------------------+
if (file("google-services.json").exists()) {
    apply(plugin = "com.google.gms.google-services")
}

// +- RELIZ IMZOSI ---------------------------------------------------+
// Kalit `android/key.properties` dan o'qiladi. U `.gitignore` da -
// imzolash kaliti repoga HECH QACHON tushmasligi kerak.
//
// FAYL YO'Q BO'LSA build YIQILMAYDI, debug kalitiga tushadi. Bu
// ataylab: `flutter run --release` kalitsiz ham ishlashi kerak.
//
// DIQQAT: bu mantiq uzoq vaqt FAQAT customer_app da bor edi. Kuryer va
// affitsiant ilovalari `key.properties` ni umuman o'qimasdi va reliz
// build JIMGINA debug kaliti bilan imzolanardi - `gradlew signingReport`
// buni ochib berdi (2026-08-27). Debug kaliti ochiq, ya'ni bunday APK ni
// istalgan odam "xuddi shu ilova" deb yangilay olardi.
//
// KALIT YO'QOLSA ILOVANI BOSHQA YANGILAB BO'LMAYDI - zaxirasini saqlang.
// +------------------------------------------------------------------+
val keystoreProperties = Properties().apply {
    val f = rootProject.file("key.properties")
    if (f.exists()) f.inputStream().use { load(it) }
}
val hasReleaseKey = keystoreProperties.getProperty("storeFile") != null

android {
    namespace = "com.ondex.courier"
    // `flutter.compileSdkVersion` shu muhitda 33ga tushib qolgan —
    // `audioplayers_android`ning AndroidX bog'liqliklari (fragment 1.7.1,
    // core 1.13.1 va h.k.) kamida 34 talab qiladi, shu sabab qat'iy 36ga
    // qayd etildi (Gradlening o'zi tavsiya qilgan qiymat).
    compileSdk = 36
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
        // `flutter_local_notifications` 19.x talabi (affitsiant ilovasi
        // bilan bir xil).
        isCoreLibraryDesugaringEnabled = true
    }

    defaultConfig {
        // TODO: Specify your own unique Application ID (https://developer.android.com/studio/build/application-id.html).
        applicationId = "com.ondex.courier"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    // Kalit bo'lsa reliz konfiguratsiyasi yaratiladi; bo'lmasa
    // yaratilmaydi va quyida debug kalitiga tushiladi.
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
        // +- DEV VA PROD YONMA-YON --------------------------------------+
        // Android ilovalarni PAKET NOMI bo'yicha ajratadi. Suffikssiz
        // dev build prod'ni almashtirib yuborardi (yoki imzo boshqacha
        // bo'lgani uchun umuman o'rnatilmasdi).
        //
        // Dev  -> com.ondex.*.dev   nomi: "... Dev"
        // Prod -> com.ondex.*       nomi: o'zining nomi
        //
        // DIQQAT: `.dev` paketi Firebase va Google Maps kalitida
        // ALOHIDA ro'yxatdan o'tishi SHART. Aks holda dev build'da
        // google-services plagini yiqiladi va xarita ochilmaydi.
        // +--------------------------------------------------------------+
        debug {
            applicationIdSuffix = ".dev"
        }

        release {
            // Kalit bo'lsa RELIZ, bo'lmasa debug - va bu holat
            // build vaqtida ko'rinib turadi (`gradlew signingReport`).
            signingConfig = if (hasReleaseKey) {
                signingConfigs.getByName("release")
            } else {
                logger.warn("OGOHLANTIRISH: key.properties yo'q - APK DEBUG kaliti bilan imzolanadi, TARQATISH UCHUN YAROQSIZ")
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

dependencies {
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.4")
}
