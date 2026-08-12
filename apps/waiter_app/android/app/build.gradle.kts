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

    buildTypes {
        release {
            // TODO: Add your own signing config for the release build.
            // Signing with the debug keys for now, so `flutter run --release` works.
            signingConfig = signingConfigs.getByName("debug")
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
