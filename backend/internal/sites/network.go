package sites

import "github.com/ebash/barn/backend/internal/db"

// UsesHostNetwork reports whether the site container runs with network_mode: host.
func UsesHostNetwork(site db.Site) bool {
	return site.DockerNetworkHost
}

// UpstreamPort returns the TCP port nginx should proxy to on the host.
func UpstreamPort(site db.Site) int {
	if UsesHostNetwork(site) {
		return int(site.ContainerPort)
	}
	if site.HostPort.Valid {
		return int(site.HostPort.Int32)
	}
	return int(site.ContainerPort)
}
