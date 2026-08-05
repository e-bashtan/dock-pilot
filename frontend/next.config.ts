import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: "standalone",
  async redirects() {
    return [
      { source: "/fleet", destination: "/servers", permanent: true },
      { source: "/fleet/settings", destination: "/servers/settings", permanent: true },
      { source: "/fleet/events", destination: "/servers/events", permanent: true },
      { source: "/fleet/servers/new", destination: "/servers/new", permanent: true },
      { source: "/fleet/servers/:id", destination: "/servers/:id", permanent: true },
    ];
  },
};

export default nextConfig;
