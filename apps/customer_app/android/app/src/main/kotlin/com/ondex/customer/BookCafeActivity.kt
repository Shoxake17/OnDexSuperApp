package com.ondex.customer

import android.content.Intent
import android.os.Bundle
import android.util.Log
import org.godotengine.godot.GodotActivity
import java.io.File

/**
 * Book Cafe 3D sayohati — OnDex ILOVASI ICHIDA.
 *
 * ┌─ NEGA ALOHIDA JARAYON ──────────────────────────────────────────────┐
 * Manifestda bu faoliyat `android:process=":bookcafe"` bilan e'lon
 * qilingan, ya'ni u OnDex bilan bir xil ilovada, lekin BOSHQA
 * jarayonda ishlaydi.
 *
 * Sabab: 3D dvigatel ko'p xotira oladi va u qulasa, oddiy jarayonda
 * BUTUN OnDex yiqilardi — katalog ham, savat ham, buyurtmalar ham.
 * Alohida jarayonda esa faqat 3D oyna yopiladi, foydalanuvchi
 * ilovaga qaytadi.
 *
 * Xotira ham shu yo'l bilan qaytariladi: jarayon tugagach tizim uni
 * butunlay tozalaydi.
 * └─────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ SAHNA ILOVA ICHIDA EMAS ───────────────────────────────────────────┐
 * Kafe sahnasi ~104 MB. Uni APK ichiga qo'ysak, 3D ni hech qachon
 * ochmaydigan foydalanuvchi ham shuncha joyni bekorga band qilardi.
 *
 * Shuning uchun sahna R2 dan YUKLAB OLINADI va bu yerga faylning
 * yo'li beriladi. Dvigatel uni `--main-pack` argumenti orqali oladi.
 * └─────────────────────────────────────────────────────────────────────┘
 */
class BookCafeActivity : GodotActivity() {

    companion object {
        private const val TAG = "BookCafe"

        /** Yuklab olingan sahna faylining yo'li. */
        const val EXTRA_PCK_PATH = "pck_path"

        const val EXTRA_RESTAURANT = "ondex_restaurant"
        const val EXTRA_TABLE = "ondex_table"
        const val EXTRA_API = "ondex_api"

        // Yuklanish ekranida ko'rsatiladigan kafe nomi va logotipi.
        //
        // Ular API'dan qayta so'ralmaydi: OnDex menyu ekranida bu
        // ma'lumot allaqachon bor. Qo'shimcha so'rov yuklanishni
        // yarim soniyaga cho'zardi va internet uzilganda logotip
        // umuman chiqmasdi.
        const val EXTRA_NAME = "ondex_name"
        const val EXTRA_LOGO = "ondex_logo"

        // ┌─ SAVAT VA SEVIMLILAR ─────────────────────────────────────┐
        // O'yin bu ma'lumotni serverdan O'ZI olmaydi: buning uchun
        // foydalanuvchi tokeni kerak bo'lardi va u ataylab
        // berilmaydi. OnDex hozirgi holatni uzatadi, o'yin uni
        // ko'rsatadi va o'zgarishni faylga qaytaradi.
        // └───────────────────────────────────────────────────────────┘
        const val EXTRA_FAV = "ondex_fav"
        const val EXTRA_CART = "ondex_cart"

        /** O'yin natijani shu faylga yozadi. Yo'lni OnDex beradi. */
        const val EXTRA_CART_OUT = "ondex_cart_out"

        private const val MAX_VALUE_LEN = 512

        /** Savat va sevimlilar ro'yxati uzunroq bo'lishi mumkin. */
        private const val MAX_LIST_LEN = 4096
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Log.i(TAG, "3D sayohat ochildi (alohida jarayon)")
    }

    override fun getCommandLine(): MutableList<String> {
        val args = ArrayList(super.getCommandLine())
        val intent: Intent = intent ?: return args

        // ── Sahna fayli ──────────────────────────────────────────────
        val pck = intent.getStringExtra(EXTRA_PCK_PATH)
        if (pck.isNullOrEmpty()) {
            // Bu holat bo'lmasligi kerak: Flutter tomoni faylni
            // yuklab, tekshirib, keyin ochadi. Lekin jimgina bo'sh
            // ekran ko'rsatishdan ko'ra jurnalga yozib qo'ygan
            // yaxshiroq.
            Log.e(TAG, "sahna fayli ko'rsatilmagan")
            return args
        }
        val file = File(pck)
        if (!file.isFile) {
            Log.e(TAG, "sahna fayli topilmadi: $pck")
            return args
        }

        // ┌─ `--main-pack` EMAS ───────────────────────────────────────┐
        // Dastlab bu yerda `--main-pack` ishlatilgan edi va dvigatel
        // ishga tushishdan bosh tortdi:
        //
        //   ERROR: --main-pack is attempting to load from outside of
        //   the executable, but this Godot binary was compiled without
        //   support for path overrides.
        //
        // Godot'ning rasmiy Android shabloni ataylab shunday:
        // diskdagi ixtiyoriy fayldan LOYIHA yuklashni taqiqlaydi.
        //
        // Shuning uchun ilova ichida kichik yuklovchi loyiha keladi
        // (`assets/`, 4 KB) va u paketni ish paytida
        // `ProjectSettings.load_resource_pack()` bilan ochadi — bu
        // API taqiq ostida emas.
        // └────────────────────────────────────────────────────────────┘
        args.add("--ondex-pack=" + file.absolutePath)

        // ── OnDex parametrlari ───────────────────────────────────────
        addArg(args, "--ondex-restaurant=", intent.getStringExtra(EXTRA_RESTAURANT))
        addArg(args, "--ondex-table=", intent.getStringExtra(EXTRA_TABLE))
        addArg(args, "--ondex-api=", intent.getStringExtra(EXTRA_API))
        addArg(args, "--ondex-name=", intent.getStringExtra(EXTRA_NAME))
        addArg(args, "--ondex-logo=", intent.getStringExtra(EXTRA_LOGO))

        // ── Boshlang'ich holat va qaytarish yo'li ──────────────────
        addArg(args, "--ondex-fav=", intent.getStringExtra(EXTRA_FAV), MAX_LIST_LEN)
        addArg(args, "--ondex-cart=", intent.getStringExtra(EXTRA_CART), MAX_LIST_LEN)
        addArg(args, "--ondex-cart-out=", intent.getStringExtra(EXTRA_CART_OUT))

        return args
    }

    /**
     * Qiymatni argument sifatida qo'shadi.
     *
     * Bu yerda FAQAT shakl tekshiriladi: uzunlik va boshqaruv
     * belgilari. Mazmun qoidasi (qaysi domen ruxsat etilgan, ID
     * qanday ko'rinishda bo'lishi) `ondex_config.gd` da — bitta
     * joyda, ikki nusxa bo'lib bir-biridan farq qilib ketmasligi
     * uchun.
     */
    /**
     * Argumentni qo'shadi.
     *
     * `maxLen` — savat va sevimlilar ro'yxati oddiy qiymatdan uzunroq
     * bo'lishi mumkin, shuning uchun chegara chaqiruvchidan beriladi.
     * Chegara umuman bo'lmasa, juda uzun qiymat buyruq satrini
     * shishirib yuborardi.
     */
    private fun addArg(
        args: MutableList<String>,
        prefix: String,
        value: String?,
        maxLen: Int = MAX_VALUE_LEN,
    ) {
        if (value.isNullOrEmpty() || value.length > maxLen) return
        for (c in value) {
            if (c.code < 0x20 || c.code == 0x7F) {
                Log.w(TAG, "qo'shimchada boshqaruv belgisi bor, o'tkazilmadi")
                return
            }
        }
        args.add(prefix + value)
    }
}
