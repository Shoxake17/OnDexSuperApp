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

// ┌─ EMULYATOR UCHUN x86_64 ───────────────────────────────────────────┐
// Ilova odatda FAQAT arm64 uchun quriladi (sabab pastdagi `packaging`
// izohida — Godot dvigateli har bir arxitektura uchun ~70 MB).
//
// Lekin Windows/Intel'dagi emulyator x86_64 — arm64 APK unda umuman
// ishga tushmaydi:
//
//   FATAL: Could not find 'libflutter.so'.
//   Looked for: [x86_64, arm64-v8a], but only found: []
//
// Shuning uchun x86_64 ni IXTIYORIY qildik. Standart holat
// o'zgarmaydi — tarqatiladigan build hamon faqat arm64 va kichik.
// Emulyator uchun:
//
//   flutter build apk --debug --target-platform android-x64 \
//       -P ondexX86=true
//
// `-P` (`--android-project-arg`) qiymatni Gradle'ga uzatadi.
// `ORG_GRADLE_PROJECT_...` muhit o'zgaruvchisi ISHLAMAYDI — Flutter
// Gradle'ni o'z jarayonida chaqiradi va daemon eski muhitni ushlab
// qoladi (sinab ko'rilgan: APK yana arm64 bo'lib chiqqan).
//
// APK ~70 MB kattaroq chiqadi — bu faqat lokal sinov uchun, hech
// qachon tarqatilmaydi.
// └────────────────────────────────────────────────────────────────────┘
val includeX86 = (project.findProperty("ondexX86") as String?)?.toBoolean() ?: false

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
        minSdk = maxOf(flutter.minSdkVersion, 24)
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName

        // Godot dvigateli faqat arm64 uchun olinadi - sabab
        // yuqoridagi izohda.
        ndk {
            abiFilters += if (includeX86) listOf("arm64-v8a", "x86_64")
                          else listOf("arm64-v8a")
        }
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

    // ┌─ FAQAT arm64 ──────────────────────────────────────────────────┐
    // `godot-lib.aar` ichida to'rtta arxitektura uchun dvigatel bor va
    // har biri ~70 MB. `defaultConfig.ndk.abiFilters` YETARLI EMAS -
    // Flutter gradle plagini uni o'zgartirib yuboradi va APK 384 MB
    // bo'lib chiqdi.
    //
    // Paketlash bosqichidagi istisno esa oxirgi so'zni aytadi.
    // └────────────────────────────────────────────────────────────────┘
    // ┌─ `.godot` PAPKASI PAKETGA TUSHSIN ─────────────────────────────┐
    // Godot loyihasini `.godot/` papkasida saqlaydi (uid_cache.bin,
    // global_script_class_cache.cfg, exported/...).
    //
    // AAPT esa NUQTA bilan boshlanadigan fayllarni assets'dan
    // ATAYLAB chiqarib tashlaydi — standart qoidada `.*` bor.
    // Natijada indeks (`assets.sparsepck`) paketda qoladi-yu, u
    // ishora qilgan fayllar yo'qoladi va dvigatel ishga tushmaydi:
    //
    //   ERROR: Can't open pack-referenced file
    //   "res://.godot/uid_cache.bin" from sparse pack
    //   "res://assets.sparsepck" due to error "Can't open".
    //
    // Quyidagi ro'yxat AAPT'ning standarti bilan bir xil, faqat
    // `.*` olib tashlangan: qolgan istisnolar (`.git`, `.svn`,
    // `thumbs.db`) o'z kuchida qoladi.
    // └────────────────────────────────────────────────────────────────┘
    androidResources {
        ignoreAssetsPattern =
            "!.svn:!.git:!.gitignore:!.ds_store:!*.scc:<dir>_*:!CVS:" +
            "!thumbs.db:!picasa.ini:!*~"
    }

    packaging {
        jniLibs {
            excludes += listOf(
                "**/armeabi-v7a/**",
                "**/x86/**"
            )
            // Emulyator uchun build'da x86_64 SAQLANADI (yuqoridagi
            // `includeX86` izohiga qarang), qolgan hamma holatda
            // chiqarib tashlanadi.
            if (!includeX86) excludes += "**/x86_64/**"
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
            // ┌─ GODOT UCHUN QOIDALAR ─────────────────────────────────┐
            // R8 release qurilishida sinf nomlarini qisqartiradi.
            // Godot dvigateli esa Java sinflarini NOMI BO'YICHA, JNI
            // orqali chaqiradi va nom o'zgargach ilova qulaydi.
            //
            // Sabab va aniq xato matni `proguard-rules.pro` da.
            // └────────────────────────────────────────────────────────┘
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )

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

// ┌─ BOOK CAFE 3D — GODOT DVIGATELI ────────────────────────────────────┐
// 3D kafe sayohati alohida ilova EMAS, OnDex ichida ishlaydi.
//
// Dvigatel (`godot-lib.aar`, arm64 uchun ~69 MB) ilova bilan birga
// keladi, KAFE SAHNASI esa kelmaydi: u ~104 MB va faqat foydalanuvchi
// "Kafeni aylanib ko'ring" tugmasini bosganda R2 dan yuklab olinadi.
//
// Shu sababli ilovani o'rnatgan HAR KIM 104 MB ni ko'tarib yurmaydi -
// 3D ni ochmaganlar uchun u umuman yuklanmaydi.
//
// Faqat arm64 qoldirilgan: `.aar` ichida to'rtta arxitektura bor va
// hammasi qo'shilsa APK bekorga uch barobar semirardi. Chust
// foydalanuvchilarining telefonlari arm64.
// └─────────────────────────────────────────────────────────────────────┘

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

dependencies {
    implementation(files("libs/godot-lib.aar"))
}

flutter {
    source = "../.."
}
