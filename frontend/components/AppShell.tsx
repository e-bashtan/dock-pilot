"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { AuthGate } from "@/components/AuthGate";
import { Nav } from "@/components/Nav";
import { ServersModeProvider } from "@/lib/servers-mode";

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isPublicAuth = pathname?.startsWith("/auth/mobile");

  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);

  if (isPublicAuth) {
    return <>{children}</>;
  }

  return (
    <AuthGate>
      <ServersModeProvider>
        <Nav />
        <main>{children}</main>
      </ServersModeProvider>
    </AuthGate>
  );
}
