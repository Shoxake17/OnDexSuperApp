import { getSessionToken } from "@/lib/session";
import { requireAuth } from "@/lib/require-auth";
import DesktopFavorites from "./desktop-favorites";
import FavoritesClient from "./favorites-client";

export default async function FavoritesPage() {
  await requireAuth("/favorites");
  const signedIn = Boolean(await getSessionToken());

  return (
    <>
      <div className="hidden xl:block">
        <DesktopFavorites signedIn={signedIn} />
      </div>
      <div className="xl:hidden">
        <FavoritesClient />
      </div>
    </>
  );
}
