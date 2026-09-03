import { requireAuth } from "@/lib/require-auth";
import ProfileClient from "./profile-client";

export default async function ProfilePage() {
  await requireAuth("/profile");
  return <ProfileClient />;
}
