allprojects {
    repositories {
        google()
        mavenCentral()
    }
}

// ESLATMA: build chiqishini E: diskka yo'naltirishga urinildi (F: disk
// deyarli to'lib qolgani uchun), lekin `flutter build`/`flutter run`
// natija faylini QATTIQ KODLANGAN standart yo'ldan (`<loyiha>/build/...`)
// qidiradi — buildDir'ni boshqa joyga ko'chirish Flutter CLI bilan mos
// kelmaydi. Shuning uchun standart (F: diskdagi) joy qoldirilgan; disk
// joyi masalasi build orasida `flutter clean` bilan boshqariladi.
val newBuildDir: Directory =
    rootProject.layout.buildDirectory
        .dir("../../build")
        .get()
rootProject.layout.buildDirectory.value(newBuildDir)

subprojects {
    val newSubprojectBuildDir: Directory = newBuildDir.dir(project.name)
    project.layout.buildDirectory.value(newSubprojectBuildDir)
}
subprojects {
    project.evaluationDependsOn(":app")
}

// +- PLAGINNING JAVA 21 TALABI TUSHIRILADI ----------------------------+
// `maplibre_gl` (OnDexMap xaritasi) o'z modulida `JavaVersion.VERSION_21`
// deb yozgan, loyihaning JDK'si esa 17 va build yiqiladi:
//
//   Execution failed for task ':maplibre_gl:compileDebugJavaWithJavac'
//   > error: invalid source release: 21
//
// Mijoz ilovasida ham aynan shu yechim (`customer_app/android/build.gradle.kts`
// izohiga qarang): plagin kodi Java 21 ning o'ziga xos xususiyatlarini
// ishlatmaydi, FAQAT kompilyatsiya darajasi tushiriladi.
// +------------------------------------------------------------------+
subprojects {
    val capJavaVersion = {
        if (project.name == "maplibre_gl") {
            val android = project.extensions.findByName("android")
                as? com.android.build.gradle.BaseExtension
            android?.compileOptions {
                sourceCompatibility = JavaVersion.VERSION_17
                targetCompatibility = JavaVersion.VERSION_17
            }
            project.tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>()
                .configureEach {
                    compilerOptions.jvmTarget.set(
                        org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
                    )
                }
            logger.lifecycle("ondex: maplibre_gl Java 21 -> 17 (loyiha JDK'si 17)")
        }
    }
    if (state.executed) capJavaVersion() else afterEvaluate { capJavaVersion() }
}
tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
