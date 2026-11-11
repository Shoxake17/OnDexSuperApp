import { proxyWellKnown } from "@/lib/well-known";

// PK-1: passkey ilova<->domen bog'lanishi (lib/well-known.ts).
export const dynamic = "force-dynamic";

export function GET() {
  return proxyWellKnown("/.well-known/assetlinks.json");
}
