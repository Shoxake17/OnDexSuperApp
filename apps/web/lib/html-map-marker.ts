import type { LatLng } from "./delivery-tracking";

// Google Maps ustidagi HTML belgi (`OverlayView`).
//
// ┌─ NEGA ODDIY `google.maps.Marker` EMAS ─────────────────────────────┐
// Kuzatuv xaritasiga mobil ilovadagi belgilar kerak: dumaloq restoran
// LOGOSI, uy belgisi va harakat yo'nalishiga BURILADIGAN kuryer mashinasi.
// Oddiy `Marker` rasmni burmaydi va dumaloq qirqmaydi; logoni `canvas` ga
// chizib qirqish esa boshqa domendagi rasmda (R2) CORS sabab ishlamaydi.
// `AdvancedMarkerElement` esa Cloud Console'da Map ID talab qiladi.
//
// Belgi mazmuni `document.createElement` bilan quriladi — `innerHTML` YO'Q:
// restoran nomi va logo manzili restoran kiritgan matn, satr sifatida
// HTML'ga qo'yilsa XSS bo'lardi.
// └────────────────────────────────────────────────────────────────────┘

export type HtmlMapMarker = {
  setPosition(p: LatLng): void;
  /** Mazmunni markazi atrofida buradi (gradus, soat mili bo'yicha). */
  setRotation(deg: number): void;
  remove(): void;
};

/** `loadGoogleMaps()` tugagandan keyingina chaqiriladi. */
export function createHtmlMapMarker(
  map: google.maps.Map,
  position: LatLng,
  content: HTMLElement,
  zIndex: number,
): HtmlMapMarker {
  const wrapper = document.createElement("div");
  wrapper.style.position = "absolute";
  wrapper.style.transform = "translate(-50%, -50%)";
  wrapper.style.zIndex = String(zIndex);
  wrapper.appendChild(content);

  let current = position;

  class Overlay extends google.maps.OverlayView {
    onAdd() {
      this.getPanes()?.overlayMouseTarget.appendChild(wrapper);
    }

    draw() {
      const point = this.getProjection()?.fromLatLngToDivPixel(
        new google.maps.LatLng(current.lat, current.lng),
      );
      if (!point) return;
      wrapper.style.left = `${point.x}px`;
      wrapper.style.top = `${point.y}px`;
    }

    onRemove() {
      wrapper.remove();
    }
  }

  const overlay = new Overlay();
  overlay.setMap(map);

  return {
    setPosition(p) {
      current = p;
      overlay.draw();
    },
    setRotation(deg) {
      content.style.transform = `rotate(${deg}deg)`;
    },
    remove() {
      overlay.setMap(null);
    },
  };
}
