package com.ondex.customer

import android.content.Intent
import android.util.Log
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import java.io.File
import java.io.FileInputStream
import java.security.MessageDigest
import java.util.Locale

// FlutterActivity EMAS, FlutterFragmentActivity.
//
// `local_auth` (barmoq izi / yuz / grafik kalit / PIN) Android'ning
// BiometricPrompt'iga tayanadi, u esa FragmentActivity talab qiladi.
// Oddiy FlutterActivity bilan plagin ishga tushishda xato beradi va
// qulf oynasi UMUMAN ochilmaydi.
class MainActivity : FlutterFragmentActivity() {

    /**
     * O'yin yopilishini kutayotgan Flutter so'rovi.
     *
     * `MethodChannel.Result` FAQAT BIR MARTA javob berishi shart,
     * shuning uchun u shu yerda saqlanadi va ishlatilgach nolga
     * qaytariladi.
     */
    private var pendingGameResult: MethodChannel.Result? = null

    companion object {
        private const val TAG = "OnDexGame"
        private const val CHANNEL = "uz.ondex.customer/game"

        /** O'yin activity'si uchun so'rov kodi. */
        private const val GAME_REQUEST = 4711

        /** O'yin qaytargan holat shu faylga yoziladi. */
        private const val STATE_FILE = "bookcafe_state.json"

        /**
         * Holat faylining eng katta hajmi.
         *
         * Fayl o'yin tomonidan yoziladi va u xotiraga to'liq o'qiladi.
         * Chegarasiz qoldirilsa, buzilgan yoki ataylab shishirilgan
         * fayl ilovani yiqitishi mumkin edi.
         */
        private const val MAX_STATE_BYTES = 256L * 1024L

        /** Savat/sevimlilarda uzatiladigan eng ko'p element. */
        private const val MAX_LIST_ITEMS = 200

        /** Restoran ID: faqat harf, raqam, tire va pastki chiziq. */
        private val ID_RE = Regex("^[A-Za-z0-9_-]{1,64}$")

        /** Stol belgisi: qisqa va zararsiz belgilar. */
        private val TABLE_RE = Regex("^[A-Za-z0-9 _.\\-]{1,32}$")

        /** SHA-256: aynan 64 ta o'n oltilik belgi. */
        private val SHA_RE = Regex("^[a-f0-9]{64}$")
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "sceneStatus" -> sceneStatus(call, result)
                    "scenePaths" -> scenePaths(call, result)
                    "installScene" -> installScene(call, result)
                    "deleteScene" -> deleteScene(call, result)
                    "open" -> openScene(call, result)
                    else -> result.notImplemented()
                }
            }
    }

    // ┌─ HAR RESTORANNING O'Z MAKETI ──────────────────────────────────┐
    // Maket restoran ID siga bog'langan: fayl nomi ham shu ID.
    //
    // Avval bitta umumiy fayl ishlatilgan edi va u BARCHA restoranlar
    // uchun ko'rsatilardi — Book Cafe maketi Chust Burger menyusida
    // ham chiqib turardi. Endi har biri alohida faylda va faqat o'z
    // restoranida ochiladi.
    //
    // ID `ID_RE` bilan tekshiriladi, ya'ni fayl nomiga yo'l belgisi
    // (`/`, `..`) tushmaydi.
    // └────────────────────────────────────────────────────────────────┘
    private fun sceneDir(): File = File(filesDir, "scenes")

    private fun sceneFile(restaurantId: String): File =
        File(sceneDir(), "$restaurantId.pck")

    private fun partFile(restaurantId: String): File =
        File(sceneDir(), "$restaurantId.pck.part")

    /**
     * Maket qurilmada bormi va KUTILGAN nusxami.
     *
     * ┌─ XESH ENDI API'DAN KELADI ─────────────────────────────────────┐
     * Avval u ilova ichida yozilgan edi va har maket yangilanganda
     * ilovaning yangi versiyasi kerak bo'lardi.
     *
     * Endi qiymat restoran yozuvida turadi. Ishonch zanjiri:
     * ilova → HTTPS → bizning API → xesh → fayl. Ya'ni faylni
     * almashtirish uchun API ni ham egallash kerak.
     *
     * Shakl BU YERDA tekshiriladi: 64 ta o'n oltilik belgidan boshqa
     * hech narsa qabul qilinmaydi.
     * └────────────────────────────────────────────────────────────────┘
     */
    private fun sceneStatus(
        call: io.flutter.plugin.common.MethodCall,
        result: MethodChannel.Result
    ) {
        val id = call.argument<String>("restaurantId").orEmpty()
        val want = call.argument<String>("sha256").orEmpty().lowercase(Locale.US)
        if (!ID_RE.matches(id)) {
            result.error("bad_restaurant", "restoran ID noto'g'ri", null)
            return
        }
        val file = sceneFile(id)
        if (!file.isFile) {
            result.success(mapOf("ready" to false, "reason" to "not_downloaded"))
            return
        }
        if (!SHA_RE.matches(want)) {
            result.success(mapOf("ready" to false, "reason" to "bad_expected_hash"))
            return
        }
        val ok = sha256(file) == want
        result.success(
            mapOf("ready" to ok, "reason" to if (ok) "ok" else "bad_hash")
        )
    }

    /** Flutter faylni qayerga yozishini aytadi. */
    private fun scenePaths(
        call: io.flutter.plugin.common.MethodCall,
        result: MethodChannel.Result
    ) {
        val id = call.argument<String>("restaurantId").orEmpty()
        if (!ID_RE.matches(id)) {
            result.error("bad_restaurant", "restoran ID noto'g'ri", null)
            return
        }
        sceneDir().mkdirs()
        result.success(
            mapOf(
                "part" to partFile(id).absolutePath,
                "final" to sceneFile(id).absolutePath
            )
        )
    }

    /**
     * Yuklab olingan faylni tekshirib, joyiga qo'yadi.
     *
     * Fayl avval `.part` nomida yotadi va faqat tekshiruvdan
     * o'tgandan keyin asosiy nomga o'tadi: yuklash yarmida uzilsa,
     * yarim fayl hech qachon "tayyor" deb hisoblanmaydi.
     */
    private fun installScene(
        call: io.flutter.plugin.common.MethodCall,
        result: MethodChannel.Result
    ) {
        val id = call.argument<String>("restaurantId").orEmpty()
        val want = call.argument<String>("sha256").orEmpty().lowercase(Locale.US)
        if (!ID_RE.matches(id)) {
            result.error("bad_restaurant", "restoran ID noto'g'ri", null)
            return
        }
        if (!SHA_RE.matches(want)) {
            result.error("bad_expected_hash", "kutilgan xesh noto'g'ri", null)
            return
        }
        val part = partFile(id)
        if (!part.isFile) {
            result.error("no_file", "yuklab olingan fayl yo'q", null)
            return
        }
        if (sha256(part) != want) {
            part.delete()
            result.error("bad_hash", "fayl butunligi tasdiqlanmadi", null)
            return
        }
        val target = sceneFile(id)
        target.delete()
        if (!part.renameTo(target)) {
            part.delete()
            result.error("rename_failed", "fayl joyiga qo'yilmadi", null)
            return
        }
        result.success(true)
    }

    /** Maketni o'chiradi — foydalanuvchi joy bo'shatmoqchi bo'lsa. */
    private fun deleteScene(
        call: io.flutter.plugin.common.MethodCall,
        result: MethodChannel.Result
    ) {
        val id = call.argument<String>("restaurantId").orEmpty()
        if (!ID_RE.matches(id)) {
            result.error("bad_restaurant", "restoran ID noto'g'ri", null)
            return
        }
        partFile(id).delete()
        result.success(sceneFile(id).delete())
    }

    private fun openScene(
        call: io.flutter.plugin.common.MethodCall,
        result: MethodChannel.Result
    ) {
        val restaurantId = call.argument<String>("restaurantId").orEmpty()
        val tableLabel = call.argument<String>("tableLabel").orEmpty()
        val apiBase = call.argument<String>("apiBase").orEmpty()
        val want = call.argument<String>("sha256").orEmpty().lowercase(Locale.US)

        // Qiymatlar 3D tomonga uzatiladi va u yerda API yo'liga
        // qo'shiladi. Tekshiruvsiz yo'l almashtirish mumkin bo'lardi.
        if (!ID_RE.matches(restaurantId)) {
            result.error("bad_restaurant", "restoran ID noto'g'ri", null)
            return
        }
        if (tableLabel.isNotEmpty() && !TABLE_RE.matches(tableLabel)) {
            result.error("bad_table", "stol belgisi noto'g'ri", null)
            return
        }
        if (!SHA_RE.matches(want)) {
            result.error("bad_expected_hash", "kutilgan xesh noto'g'ri", null)
            return
        }

        val file = sceneFile(restaurantId)
        if (!file.isFile) {
            result.error("not_downloaded", "maket yuklanmagan", null)
            return
        }

        // ┌─ XESH OCHISHDAN OLDIN QAYTA TEKSHIRILADI ──────────────────┐
        // Yuklab olish paytida ham tekshirilgan, lekin fayl diskda
        // turadi va oradan vaqt o'tishi mumkin. Bu - oxirgi va eng
        // ishonchli nuqta.
        // └────────────────────────────────────────────────────────────┘
        if (sha256(file) != want) {
            file.delete()
            result.error("bad_hash", "maket fayli buzilgan", null)
            return
        }

        // Yuklanish ekranida ko'rsatiladigan nom va logotip.
        //
        // Bular faqat KO'RSATISH uchun: ular bilan hech narsa
        // ochilmaydi va API yo'liga qo'shilmaydi. Shuning uchun bu
        // yerda faqat uzunlik cheklanadi, mazmun qoidasi esa
        // `ondex_config.gd` da (logotip manzili qaysi domenda
        // bo'lishi mumkinligi).
        val name = call.argument<String>("name").orEmpty().take(64)
        val logo = call.argument<String>("logoUrl").orEmpty().take(512)

        // â”Œâ”€ SAVAT VA SEVIMLILAR: HOLAT IKKI TOMONGA â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
        // O'yin bu ma'lumotni serverdan O'ZI olmaydi va yozmaydi:
        // buning uchun foydalanuvchi tokeni kerak bo'lardi va u
        // ataylab berilmaydi (`ondex_config.gd` izohiga qarang).
        //
        // Shuning uchun hozirgi holat kirishda uzatiladi, o'zgargani
        // esa faylga qaytariladi. Serverga yozishni OnDex o'z
        // huquqlari bilan bajaradi.
        //
        // Qiymatlar SHU YERDA ham tekshiriladi: ular Flutter'dan
        // keladi, lekin kanal orqali kelgan hamma narsa tashqi
        // ma'lumot deb qaraladi.
        // â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
        val favorites = sanitizeIds(call.argument<String>("favorites").orEmpty())
        val cart = sanitizeCart(call.argument<String>("cart").orEmpty())

        // Natija fayli ilovaning SHAXSIY papkasida. Yo'lni o'yin
        // tanlamaydi - aks holda u istalgan faylni qayta yozardi.
        val outFile = File(filesDir, STATE_FILE)
        outFile.delete()

        val intent = Intent(this, BookCafeActivity::class.java).apply {
            putExtra(BookCafeActivity.EXTRA_PCK_PATH, file.absolutePath)
            putExtra(BookCafeActivity.EXTRA_RESTAURANT, restaurantId)
            putExtra(BookCafeActivity.EXTRA_TABLE, tableLabel)
            if (apiBase.isNotEmpty()) putExtra(BookCafeActivity.EXTRA_API, apiBase)
            if (name.isNotEmpty()) putExtra(BookCafeActivity.EXTRA_NAME, name)
            if (logo.isNotEmpty()) putExtra(BookCafeActivity.EXTRA_LOGO, logo)
            if (favorites.isNotEmpty()) {
                putExtra(BookCafeActivity.EXTRA_FAV, favorites)
            }
            if (cart.isNotEmpty()) putExtra(BookCafeActivity.EXTRA_CART, cart)
            putExtra(BookCafeActivity.EXTRA_CART_OUT, outFile.absolutePath)
        }

        try {
            // â”Œâ”€ NATIJA KUTILADI â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
            // Avval `startActivity` ishlatilardi va Flutter darrov
            // javob olardi. Endi o'yin yopilgunicha kutiladi: holat
            // fayli faqat o'shanda tayyor bo'ladi.
            // â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
            pendingGameResult?.error("cancelled", "yangi o'yin ochildi", null)
            pendingGameResult = result
            startActivityForResult(intent, GAME_REQUEST)
        } catch (e: Exception) {
            pendingGameResult = null
            result.error("launch_failed", e.message, null)
        }
    }

    /** O'yin yopilgach holat faylini o'qib Flutter'ga qaytaradi. */
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != GAME_REQUEST) return
        val pending = pendingGameResult ?: return
        pendingGameResult = null

        val outFile = File(filesDir, STATE_FILE)
        var state: String? = null
        if (outFile.isFile && outFile.length() in 1..MAX_STATE_BYTES) {
            state = try {
                outFile.readText(Charsets.UTF_8)
            } catch (e: Exception) {
                Log.w(TAG, "holat fayli o'qilmadi: ${e.message}")
                null
            }
        }
        outFile.delete()
        pending.success(mapOf("ok" to true, "state" to state))
    }

    /**
     * ID ro'yxatini tozalaydi: faqat qoidaga mos ID'lar qoladi.
     *
     * Ro'yxat uzunligi ham cheklanadi - u buyruq satriga tushadi va
     * cheksiz uzun qiymat uni shishirib yuborardi.
     */
    private fun sanitizeIds(raw: String): String =
        raw.split(",")
            .map { it.trim() }
            .filter { it.isNotEmpty() && ID_RE.matches(it) }
            .take(MAX_LIST_ITEMS)
            .joinToString(",")

    /** "id:miqdor" juftliklarini tozalaydi. */
    private fun sanitizeCart(raw: String): String =
        raw.split(",")
            .mapNotNull { pair ->
                val parts = pair.trim().split(":")
                if (parts.size != 2) return@mapNotNull null
                val id = parts[0].trim()
                val qty = parts[1].trim().toIntOrNull() ?: return@mapNotNull null
                if (!ID_RE.matches(id) || qty !in 1..99) return@mapNotNull null
                "$id:$qty"
            }
            .take(MAX_LIST_ITEMS)
            .joinToString(",")

    /** Faylning SHA-256 yig'indisi (kichik harfda). */
    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        FileInputStream(file).use { input ->
            // Fayl ~104 MB. Uni butunlay xotiraga o'qish telefonda
            // xavfli, shuning uchun bo'lak-bo'lak o'qiladi.
            val buffer = ByteArray(64 * 1024)
            while (true) {
                val n = input.read(buffer)
                if (n <= 0) break
                digest.update(buffer, 0, n)
            }
        }
        val bytes = digest.digest()
        val sb = StringBuilder(bytes.size * 2)
        for (b in bytes) sb.append(String.format(Locale.US, "%02x", b))
        return sb.toString()
    }
}
