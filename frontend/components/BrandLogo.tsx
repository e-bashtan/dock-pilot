import { AppVersion } from "@/components/AppVersion";
import { ServerIPBadge } from "@/components/ServerIPBadge";

type Props = {
  showVersion?: boolean;
  showServerIP?: boolean;
  /** Nav bar vs login screen */
  size?: "nav" | "auth";
};

/** Plain <img> — preserves PNG alpha; avoids Next Image optimizer cache of old assets. */
export function BrandLogo({
  showVersion = false,
  showServerIP = false,
  size = "nav",
}: Props) {
  const isNav = size === "nav";

  return (
    <span className={`brand-logo brand-logo-${size}`}>
      {isNav ? (
        <>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/logo-small.png"
            alt="Barn"
            width={512}
            height={512}
            className="brand-logo-img brand-logo-small"
            decoding="async"
          />
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/logo-full.png"
            alt="Barn"
            width={512}
            height={512}
            className="brand-logo-img brand-logo-full"
            decoding="async"
          />
        </>
      ) : (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src="/logo-full.png"
          alt="Barn"
          width={512}
          height={512}
          className="brand-logo-img brand-logo-full"
          decoding="async"
        />
      )}
      {showVersion && <AppVersion />}
      {showServerIP && <ServerIPBadge />}
    </span>
  );
}
