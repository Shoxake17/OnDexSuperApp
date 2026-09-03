import { requireAuth } from "@/lib/require-auth";
import NotificationsClient from "./notifications-client";

export default async function NotificationsPage() {
  await requireAuth("/notifications");
  return <NotificationsClient />;
}
