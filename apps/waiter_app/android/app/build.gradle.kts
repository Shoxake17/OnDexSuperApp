import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
    // Firebase Cloud Messaging (`google-services.json` shu plagin
    // orqali o'qiladi). Fayl `android/app/google-services.json` da
    // bo'lishi SHART — u `.gitignore` da, chunki loyihaga xos
    // konfiguratsiya (mijoz ilovasida ham xuddi shunday).
    id("com.google.gms.google-services")
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
    namespace = "com.ondex.waiter"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        // `flutter_local_notifications` (19.x) java.time API'laridan
        // foydalanadi — ular Android 8 dan pastda yo'q. Desugaring
        // ularni build vaqtida eski API'larga o'giradi. Yoqilmasa
        // `:app:checkReleaseAarMetadata` build'ni YIQITADI:
        //   "Dependency ':flutter_local_notifications' requires core
        //    library desugaring to be enabled for :app"
        //
        // Bu OnDex ilovalari orasida faqat shu yerda kerak — boshqa
        // hech qaysisida `flutter_local_notifications` yo'q.
        isCoreLibraryDesugaringEnabled = true
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "com.ondex.waiter"
        // MUHIM: bu qiymat Firebase Console'dagi Android ilova paket
        // nomi bilan AYNAN mos bo'lishi kerak, aks holda
        // `google-services.json` topilmaydi va build yiqiladi.
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

dependencies {
    // `isCoreLibraryDesugaringEnabled = true` shu kutubxonasiz ishlamaydi —
    // AGP aynan shu artefaktni talab qiladi. Versiya
    // `flutter_local_notifications` 19.x talabiga mos (>= 2.1.4).
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.4")
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
