// apps/customer_app/lib/api.dart'dagi formatSum() bilan bir xil: tiyin'ni
// butun so'mga aylantiradi (kasr ko'rsatilmaydi) va minglik guruhlarni
// bo'sh joy bilan ajratadi.
export function formatSum(tiyin: number): string {
  const sum = Math.floor(tiyin / 100);
  const withSpaces = sum.toString().replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  return `${withSpaces} so'm`;
}
