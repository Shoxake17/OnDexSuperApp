import { getSessionToken } from "@/lib/session";
import { requireAuth } from "@/lib/require-auth";
import DesktopNotifications from "./desktop-notifications";
import NotificationsClient from "./notifications-client";

export default async function NotificationsPage() {
  await requireAuth("/notifications");
  const signedIn = Boolean(await getSessionToken());

  return (
    <>
      <div className="hidden xl:block">
        <DesktopNotifications signedIn={signedIn} />
      </div>
      <div className="xl:hidden">
        <NotificationsClient />
      </div>
    </>
  );
}
