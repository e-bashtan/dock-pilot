import type { Metadata, Viewport } from "next";
import { cookies } from "next/headers";
import { AppShell } from "@/components/AppShell";
import { LocaleProvider } from "@/lib/i18n/context";
import { readLocaleCookie } from "@/lib/i18n/locale";
import { en } from "@/lib/i18n/messages/en";
import "./globals.css";

export const metadata: Metadata = {
  title: en.meta.title,
  description: en.meta.description,
  applicationName: "Barn",
  appleWebApp: {
    capable: true,
    title: "Barn",
    // Opaque status bar — avoids logo/IP colliding with the clock on iOS PWA.
    statusBarStyle: "black",
  },
  formatDetection: {
    telephone: false,
  },
  icons: {
    apple: [{ url: "/apple-icon.png", sizes: "180x180", type: "image/png" }],
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: "#0f1419",
  colorScheme: "dark",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const cookieStore = await cookies();
  const locale = readLocaleCookie((name) => cookieStore.get(name)?.value);

  return (
    <html lang={locale}>
      <body>
        <LocaleProvider initialLocale={locale}>
          <AppShell>{children}</AppShell>
        </LocaleProvider>
      </body>
    </html>
  );
}
