// Sessiya tugagan/yo'q bo'lganda (`/config/maps` 401) chaqiruvchi buni
// "internet uzildi" emas, "qayta kirish kerak" deb bilishi uchun alohida
// belgi bilan. `requireAuth` (server) buni ODATDA oldindan tutadi —
// bu FAQAT sessiya SAHIFA OCHIQ TURGANDA muddati tugagan holat uchun
// zaxira (masalan token uzoq umr surmagan yoki serverda bekor qilingan).
export class UnauthorizedError extends Error {
  constructor() {
    super(`kirish talab qilinadi`);
    this.name = `UnauthorizedError`;
  }
}
