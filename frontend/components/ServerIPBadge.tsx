"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";

export function ServerIPBadge() {
  const [ip, setIp] = useState<string>("");

  useEffect(() => {
    let cancelled = false;
    api
      .getSystemHost()
      .then((h) => {
        if (!cancelled && h.ip) setIp(h.ip);
      })
      .catch(() => {
        /* optional */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!ip) return null;
  return (
    <span className="app-server-ip" title={ip}>
      {ip}
    </span>
  );
}
