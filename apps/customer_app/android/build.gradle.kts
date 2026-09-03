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

// ┌─ PLAGINLARNING ESKI compileSdk'SI KO'TARILADI ─────────────────────┐
// `flutter_pcm_sound` (ovozli rejim uchun PCM chalgich) o'z modulida
// `compileSdk 33` deb yozgan. Loyihaning AndroidX bog'liqliklari esa
// 34+ talab qiladi va build shu yerda YIQILADI:
//
//   Execution failed for task ':flutter_pcm_sound:checkDebugAarMetadata'
//   > Dependency 'androidx.fragment:fragment:1.7.1' requires ...
//     compile against version 34 or later
//   :flutter_pcm_sound is currently compiled against android-33.
//
// Bu plaginning o'zida tuzatilishi kerak, lekin biz uni kuta
// olmaymiz. Quyida FAQAT past qolganlari ilovaning `compileSdk`
// darajasiga ko'tariladi — yuqoriroq e'lon qilgan plaginlarga
// TEGILMAYDI (ularni pasaytirish yangi xato tug'dirardi).
//
// `compileSdk` — kod QAYSI API'larga qarshi kompilyatsiya qilinadi
// degani; `minSdk`/`targetSdk` ga ta'sir qilmaydi, ya'ni qurilma
// qamrovi ham, ish vaqti xulqi ham o'zgarmaydi.
// └────────────────────────────────────────────────────────────────────┘
// ┌─ NEGA `state.executed` TEKSHIRILADI ───────────────────────────────┐
// Yuqoridagi `evaluationDependsOn(":app")` sabab ba'zi submodullar bu
// blok ishga tushganda ALLAQACHON baholangan bo'ladi va ularda
// `afterEvaluate` chaqirilsa Gradle darhol yiqiladi:
//
//   Cannot run Project.afterEvaluate(Action) when the project is
//   already evaluated.
//
// Shuning uchun baholangani uchun DARHOL, qolganlari uchun
// `afterEvaluate` orqali qo'llanadi.
// └────────────────────────────────────────────────────────────────────┘
subprojects {
    val raiseCompileSdk = {
        val android = project.extensions.findByName("android")
            as? com.android.build.gradle.BaseExtension
        val appSdk = project(":app").extensions.findByName("android")
            as? com.android.build.gradle.BaseExtension
        val target = appSdk?.compileSdkVersion
            ?.removePrefix("android-")?.toIntOrNull()
        val current = android?.compileSdkVersion
            ?.removePrefix("android-")?.toIntOrNull()
        if (android != null && target != null && current != null && current < target) {
            android.compileSdkVersion(target)
            logger.lifecycle(
                "ondex: ${project.name} compileSdk $current -> $target (AAR metadata talabi)"
            )
        }
    }
    if (state.executed) raiseCompileSdk() else afterEvaluate { raiseCompileSdk() }
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
