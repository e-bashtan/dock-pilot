"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { AUTH_LOGOUT_EVENT } from "@/lib/auth-token";
import { api, ApiError } from "@/lib/api";
import type { ServersMode, ServersSettings } from "@/lib/types";

const DEFAULT_SETTINGS: ServersSettings = {
  mode: "standalone",
  node_uid: "",
  node_name: "",
  public_url: "",
  master_url: "",
  notification_mode: "local",
  has_master_token: false,
};

type ServersModeContextValue = {
  settings: ServersSettings;
  mode: ServersMode;
  loading: boolean;
  refresh: () => Promise<void>;
  isMaster: boolean;
};

const ServersModeContext = createContext<ServersModeContextValue | null>(null);

export function ServersModeProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<ServersSettings>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const row = await api.getServersSettings();
      setSettings(row);
    } catch (e) {
      if (e instanceof ApiError && (e.status === 404 || e.status === 503)) {
        setSettings(DEFAULT_SETTINGS);
      } else {
        setSettings(DEFAULT_SETTINGS);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    const onLogout = () => {
      setSettings(DEFAULT_SETTINGS);
      setLoading(true);
    };
    window.addEventListener(AUTH_LOGOUT_EVENT, onLogout);
    return () => window.removeEventListener(AUTH_LOGOUT_EVENT, onLogout);
  }, []);

  const value = useMemo(
    () => ({
      settings,
      mode: settings.mode,
      loading,
      refresh,
      isMaster: settings.mode === "master",
    }),
    [settings, loading, refresh],
  );

  return (
    <ServersModeContext.Provider value={value}>{children}</ServersModeContext.Provider>
  );
}

export function useServersMode() {
  const ctx = useContext(ServersModeContext);
  if (!ctx) {
    throw new Error("useServersMode must be used within ServersModeProvider");
  }
  return ctx;
}
