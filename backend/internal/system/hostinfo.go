package system

import (
	"context"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type HostInfo struct {
	IP        string `json:"ip"`
	Hostname  string `json:"hostname,omitempty"`
	CheckedAt string `json:"checked_at"`
}

var (
	srcIPRe = regexp.MustCompile(`\bsrc\s+(\d{1,3}(?:\.\d{1,3}){3})\b`)
)

type hostIPCache struct {
	mu      sync.Mutex
	ip      string
	until   time.Time
	hostname string
}

func (s *Service) GetHostInfo(ctx context.Context) HostInfo {
	info := HostInfo{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if s.hostIPCache == nil {
		s.hostIPCache = &hostIPCache{}
	}
	s.hostIPCache.mu.Lock()
	defer s.hostIPCache.mu.Unlock()

	if time.Now().Before(s.hostIPCache.until) && s.hostIPCache.ip != "" {
		info.IP = s.hostIPCache.ip
		info.Hostname = s.hostIPCache.hostname
		return info
	}

	ip := s.detectHostIP(ctx)
	hn, _ := os.Hostname()
	if s.host.UsesChroot() {
		if out, err := s.host.NsenterSh(ctx, "hostname -s"); err == nil {
			if h := strings.TrimSpace(out); h != "" {
				hn = h
			}
		}
	}
	s.hostIPCache.ip = ip
	s.hostIPCache.hostname = hn
	s.hostIPCache.until = time.Now().Add(5 * time.Minute)
	info.IP = ip
	info.Hostname = hn
	return info
}

func (s *Service) detectHostIP(ctx context.Context) string {
	// Prefer the source address used for public egress (host namespace when containerized).
	try := func(out string) string {
		if m := srcIPRe.FindStringSubmatch(out); len(m) == 2 && isUsableIP(m[1]) {
			return m[1]
		}
		return ""
	}

	if s.host.UsesChroot() {
		if out, err := s.host.NsenterSh(ctx, "ip -4 route get 1.1.1.1"); err == nil {
			if ip := try(out); ip != "" {
				return ip
			}
		}
	} else if out, err := s.host.RunHostCombined(ctx, "ip", "-4", "route", "get", "1.1.1.1"); err == nil {
		if ip := try(out); ip != "" {
			return ip
		}
	}

	// Fallback: first non-loopback IPv4 on host interfaces (API process / chroot view).
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil && isUsableIP(ip4.String()) {
				return ip4.String()
			}
		}
	}
	return ""
}

func isUsableIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
		return false
	}
	return parsed.To4() != nil
}
