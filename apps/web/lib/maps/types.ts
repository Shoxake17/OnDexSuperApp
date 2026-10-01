// Xarita motoridan mustaqil interfeys: komponentlar (manzil tanlash,
// kuzatuv) `google.maps` yoki `maplibre-gl` ni to'g'ridan-to'g'ri ko'rmaydi.
//
// Qaysi motor ishlashini serverdagi `GOOGLE_MAP` bayrog'i hal qiladi
// (`/config/maps`, `lib/maps/index.ts`). Zoom HAR DOIM Google shkalasida:
// MapLibre 512 px plitka ishlatadi, shuning uchun u yerda ichkarida 1 ga
// ayriladi.

export type LatLng = { lat: number; lng: number };

export type MapProvider = "google" | "ondexmap";

/** Xaritadagi HTML belgi (restoran logosi, uy, burilaydigan mashina). */
export interface MapMarker {
  setPosition(p: LatLng): void;
  /** Mazmunni markazi atrofida buradi (gradus, soat mili bo'yicha). */
  setRotation(deg: number): void;
  remove(): void;
}

export type LineStyle = { color: string; width: number; zIndex: number };

export type CircleStyle = {
  color: string;
  strokeOpacity: number;
  strokeWidth: number;
  fillOpacity: number;
};

export interface MapView {
  readonly provider: MapProvider;
  getCenter(): LatLng;
  getZoom(): number;
  setCenter(p: LatLng): void;
  setZoom(z: number): void;
  panTo(p: LatLng): void;
  /** Nuqtalarni sig'dirish; bitta nuqta (yoki deyarli bir xil) — unga yaqinlashtirish. */
  fitPoints(points: LatLng[], padding: number): void;
  /** Kamera to'xtaganda (va xarita birinchi tayyor bo'lganda). Bekor qilish funksiyasini qaytaradi. */
  onIdle(cb: () => void): () => void;
  /** Foydalanuvchi xaritani O'ZI surganda (dastur emas). */
  onUserDrag(cb: () => void): () => void;
  addCircle(center: LatLng, radiusM: number, style: CircleStyle): void;
  addMarker(position: LatLng, content: HTMLElement, zIndex: number): MapMarker;
  /** `points === null` — chiziq olib tashlanadi. */
  setLine(key: string, points: LatLng[] | null, style: LineStyle): void;
  destroy(): void;
}

export type MapViewOptions = {
  center: LatLng;
  zoom: number;
};
