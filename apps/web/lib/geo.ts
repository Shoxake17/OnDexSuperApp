// Joylashuvni aniqlash — ikki manba, bir xil interfeys:
//
//  1. Flutter WebView ichida — `FlutterGeo` JS kanali (mini_app_webview.dart).
//     Bu SHART, chunki Chromium `navigator.geolocation`ni faqat "secure
//     context"da (HTTPS yoki localhost) ruxsat beradi; ilova esa dev'da
//     `http://<LAN-IP>:3000` orqali ochiladi va so'rov jimgina rad etilib,
//     "ruxsat berilmadi" degan CHALG'ITUVCHI xato chiqardi.
//  2. Oddiy brauzer (mustaqil sayt / Telegram) — standart
//     `navigator.geolocation`.
//
// Ikkalasi ham bir xil Promise qaytaradi, chaqiruvchi farqni bilmaydi.

export type GeoPoint = { lat: number; lng: number };

type GeoReply = { lat?: number; lng?: number; error?: string };

declare global {
  interface Window {
    FlutterGeo?: { postMessage: (msg: string) => void };
    __onFlutterGeo?: (reply: GeoReply) => void;
  }
}

const DEFAULT_TIMEOUT_MS = 25000;

function fromFlutter(timeoutMs: number): Promise<GeoPoint> {
  return new Promise((resolve, reject) => {
    let settled = false;
    const finish = () => {
      settled = true;
      clearTimeout(timer);
      delete window.__onFlutterGeo;
    };
    const timer = setTimeout(() => {
      if (settled) return;
      finish();
      reject(new Error("Joylashuvni aniqlash vaqti tugadi"));
    }, timeoutMs);

    window.__onFlutterGeo = (reply) => {
      if (settled) return;
      finish();
      if (reply.error || typeof reply.lat !== "number" || typeof reply.lng !== "number") {
        reject(new Error(reply.error ?? "Joylashuvni aniqlab bo'lmadi"));
        return;
      }
      resolve({ lat: reply.lat, lng: reply.lng });
    };

    window.FlutterGeo!.postMessage("get");
  });
}

function fromBrowser(timeoutMs: number): Promise<GeoPoint> {
  return new Promise((resolve, reject) => {
    if (!("geolocation" in navigator)) {
      reject(new Error("Brauzeringiz joylashuvni aniqlay olmaydi"));
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lat: pos.coords.latitude, lng: pos.coords.longitude }),
      (err) => {
        if (err.code === err.TIMEOUT) {
          reject(new Error("Joylashuvni aniqlash vaqti tugadi"));
        } else if (!window.isSecureContext) {
          // Aniq sabab: brauzer HTTPS'siz joylashuvni umuman bermaydi.
          reject(new Error("Joylashuv faqat HTTPS orqali ishlaydi"));
        } else {
          reject(new Error("Joylashuvga ruxsat berilmadi"));
        }
      },
      { timeout: timeoutMs, enableHighAccuracy: true },
    );
  });
}

export function getCurrentPosition(timeoutMs = DEFAULT_TIMEOUT_MS): Promise<GeoPoint> {
  if (typeof window !== "undefined" && window.FlutterGeo) {
    return fromFlutter(timeoutMs);
  }
  return fromBrowser(timeoutMs);
}
