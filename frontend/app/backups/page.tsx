"use client";

import { Suspense } from "react";
import { BackupsPageClient } from "./BackupsPageClient";

export default function BackupsPage() {
  return (
    <Suspense fallback={<div className="card">…</div>}>
      <BackupsPageClient />
    </Suspense>
  );
}
