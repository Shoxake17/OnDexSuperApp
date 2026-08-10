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

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
