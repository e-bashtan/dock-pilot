"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

/** Single Postgres instance — always use /databases. */
export default function DatabaseInstanceRedirectPage() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/databases");
  }, [router]);
  return null;
}
