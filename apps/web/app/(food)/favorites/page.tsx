import { requireAuth } from "@/lib/require-auth";
import FavoritesClient from "./favorites-client";

export default async function FavoritesPage() {
  await requireAuth("/favorites");
  return <FavoritesClient />;
}
