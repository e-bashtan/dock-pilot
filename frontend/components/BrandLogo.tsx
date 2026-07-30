import Image from "next/image";
import { AppVersion } from "@/components/AppVersion";
import { ServerIPBadge } from "@/components/ServerIPBadge";

type Props = {
  showVersion?: boolean;
  showServerIP?: boolean;
  /** Nav bar vs login screen */
  size?: "nav" | "auth";
};

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
          <Image
            src="/logo-small.png"
            alt="Barn"
            width={1024}
            height={682}
            priority
            className="brand-logo-img brand-logo-small"
          />
          <Image
            src="/logo-full.png"
            alt="Barn"
            width={1024}
            height={682}
            priority
            className="brand-logo-img brand-logo-full"
          />
        </>
      ) : (
        <Image
          src="/logo-full.png"
          alt="Barn"
          width={1024}
          height={682}
          priority
          className="brand-logo-img brand-logo-full"
        />
      )}
      {showVersion && <AppVersion />}
      {showServerIP && <ServerIPBadge />}
    </span>
  );
}
