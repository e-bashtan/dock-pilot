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
import type { FleetMode, FleetSettings } from "@/lib/types";

const DEFAULT_SETTINGS: FleetSettings = {
  mode: "standalone",
  node_uid: "",
  node_name: "",
  public_url: "",
  master_url: "",
  notification_mode: "local",
  has_master_token: false,
};

type FleetModeContextValue = {
  settings: FleetSettings;
  mode: FleetMode;
  loading: boolean;
  refresh: () => Promise<void>;
  isMaster: boolean;
};

const FleetModeContext = createContext<FleetModeContextValue | null>(null);

export function FleetModeProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<FleetSettings>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const row = await api.getFleetSettings();
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
    <FleetModeContext.Provider value={value}>{children}</FleetModeContext.Provider>
  );
}

export function useFleetMode() {
  const ctx = useContext(FleetModeContext);
  if (!ctx) {
    throw new Error("useFleetMode must be used within FleetModeProvider");
  }
  return ctx;
}
