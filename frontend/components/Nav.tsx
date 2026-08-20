"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { useLogout } from "@/components/AuthGate";
import { useServersMode } from "@/lib/servers-mode";
import { useI18n } from "@/lib/i18n/context";

type NavItem = {
  href: string;
  icon: "projects" | "servers" | "database" | "backup" | "payments" | "notifications" | "settings" | "events";
  labelKey:
    | "nav.sites"
    | "nav.databases"
    | "nav.backups"
    | "nav.payments"
    | "nav.notifications"
    | "nav.servers"
    | "nav.serverEvents"
    | "nav.serversSettings";
  match: string;
};

const STANDALONE_LINKS: NavItem[] = [
  { href: "/sites", icon: "projects", labelKey: "nav.sites", match: "sites" },
  { href: "/databases", icon: "database", labelKey: "nav.databases", match: "databases" },
  { href: "/backups", icon: "backup", labelKey: "nav.backups", match: "backups" },
  { href: "/payments", icon: "payments", labelKey: "nav.payments", match: "payments" },
  { href: "/notifications", icon: "notifications", labelKey: "nav.notifications", match: "notifications" },
  { href: "/servers/settings", icon: "settings", labelKey: "nav.serversSettings", match: "servers-settings" },
];

const MASTER_PRIMARY: NavItem[] = [
  { href: "/sites", icon: "projects", labelKey: "nav.sites", match: "sites" },
  { href: "/servers", icon: "servers", labelKey: "nav.servers", match: "servers" },
  { href: "/payments", icon: "payments", labelKey: "nav.payments", match: "payments" },
];

const MASTER_MORE: NavItem[] = [
  { href: "/servers/events", icon: "events", labelKey: "nav.serverEvents", match: "servers-events" },
  { href: "/notifications", icon: "notifications", labelKey: "nav.notifications", match: "notifications" },
  { href: "/databases", icon: "database", labelKey: "nav.databases", match: "databases-only" },
  { href: "/backups", icon: "backup", labelKey: "nav.backups", match: "backups-only" },
  { href: "/servers/settings", icon: "settings", labelKey: "nav.serversSettings", match: "servers-settings" },
];

function NavIcon({ name }: { name: NavItem["icon"] }) {
  const paths: Record<NavItem["icon"], React.ReactNode> = {
    projects: <><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></>,
    servers: <><rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01"/></>,
    database: <><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v7c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 12v7c0 1.7 3.6 3 8 3s8-1.3 8-3v-7"/></>,
    backup: <><path d="M5 5h12l2 3v11H5z"/><path d="M9 5v5h6V5M9 19v-5h6v5"/></>,
    payments: <><rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 10h18M7 15h3"/></>,
    notifications: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></>,
    settings: <><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H3v-4h.1a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1a1.7 1.7 0 0 0 1.9.3A1.7 1.7 0 0 0 10 3V3h4v.1a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1z"/></>,
    events: <><path d="M4 19V5M4 19h16"/><path d="m7 15 4-4 3 2 5-6"/></>,
  };
  return <svg className="nav-icon" viewBox="0 0 24 24" aria-hidden="true">{paths[name]}</svg>;
}

function isPrimaryActive(pathname: string, match: string): boolean {
  if (match === "sites") {
    if (pathname === "/sites/new" || pathname.startsWith("/sites/new/")) {
      return false;
    }
    return pathname === "/sites" || pathname.startsWith("/sites/");
  }
  if (match === "databases-only") {
    return pathname === "/databases" || pathname.startsWith("/databases/");
  }
  if (match === "backups-only") {
    return pathname === "/backups" || pathname.startsWith("/backups/");
  }
  if (match === "servers") {
    if (
      pathname === "/servers/events" ||
      pathname.startsWith("/servers/events/") ||
      pathname === "/servers/settings" ||
      pathname.startsWith("/servers/settings/")
    ) {
      return false;
    }
    return pathname === "/servers" || pathname.startsWith("/servers/");
  }
  if (match === "servers-events") {
    return pathname === "/servers/events" || pathname.startsWith("/servers/events/");
  }
  if (match === "servers-settings") {
    return (
      pathname === "/servers/settings" || pathname.startsWith("/servers/settings/")
    );
  }
  return pathname === `/${match}` || pathname.startsWith(`/${match}/`);
}

export function Nav() {
  const logout = useLogout();
  const { t } = useI18n();
  const { isMaster } = useServersMode();
  const pathname = usePathname() || "";
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prevOverflow;
      window.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  const closeMenu = () => {
    setMenuOpen(false);
  };

  const primaryLinks = isMaster ? [...MASTER_PRIMARY, ...MASTER_MORE] : STANDALONE_LINKS;
  const mobileLinks = isMaster ? [...MASTER_PRIMARY, ...MASTER_MORE] : STANDALONE_LINKS;

  return (
    <>
      <nav className={`nav${menuOpen ? " nav-menu-open" : ""}`}>
        <div className="nav-bar">
          <Link href="/sites" className="nav-brand" onClick={closeMenu}>
            <BrandLogo showVersion showServerIP />
          </Link>
          <button
            type="button"
            className="nav-toggle btn btn-secondary"
            aria-expanded={menuOpen}
            aria-controls="nav-menu"
            onClick={() => setMenuOpen((open) => !open)}
          >
            {t("nav.menu")}
          </button>
        </div>

        {menuOpen && (
          <button
            type="button"
            className="nav-backdrop"
            aria-label={t("nav.closeMenu")}
            onClick={closeMenu}
          />
        )}

        <div
          id="nav-menu"
          className={`nav-links${menuOpen ? " nav-links-open" : ""}`}
        >
          <div className="nav-drawer-head">
            <span className="nav-drawer-title">{t("nav.menu")}</span>
            <button
              type="button"
              className="btn btn-secondary nav-drawer-close"
              onClick={closeMenu}
            >
              {t("nav.closeMenu")}
            </button>
          </div>

          {/* Desktop: primary + More dropdown */}
          <div className="nav-primary nav-primary-desktop">
            {primaryLinks.map((item) => {
              const active = isPrimaryActive(pathname, item.match);
              return (
                <Link
                  key={`${item.href}-${item.match}`}
                  href={item.href}
                  className={`nav-link${active ? " nav-link-active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={closeMenu}
                >
                  <NavIcon name={item.icon} />
                  {t(item.labelKey)}
                </Link>
              );
            })}
          </div>

          {/* Mobile drawer: flat list, no nested More */}
          <div className="nav-primary nav-primary-mobile">
            {mobileLinks.map((item) => {
              const active = isPrimaryActive(pathname, item.match);
              return (
                <Link
                  key={`m-${item.href}-${item.match}`}
                  href={item.href}
                  className={`nav-link${active ? " nav-link-active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={closeMenu}
                >
                  <NavIcon name={item.icon} />
                  {t(item.labelKey)}
                </Link>
              );
            })}
          </div>

          <div className="nav-actions">
            <button
              type="button"
              className="btn btn-secondary nav-logout"
              onClick={() => {
                closeMenu();
                logout();
              }}
            >
              {t("nav.logout")}
            </button>
          </div>
        </div>
      </nav>
    </>
  );
}
