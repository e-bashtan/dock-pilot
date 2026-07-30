package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ebash/barn/backend/internal/docker"
	"github.com/ebash/barn/backend/internal/hostexec"
)

type DiskInfo struct {
	Path           string  `json:"path"`
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type MemoryInfo struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type ProcessInfo struct {
	PID        int     `json:"pid"`
	User       string  `json:"user"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
	Command    string  `json:"command"`
}

type Status struct {
	Disk      []DiskInfo               `json:"disk"`
	Memory    MemoryInfo               `json:"memory"`
	TopCPU    []ProcessInfo            `json:"top_cpu"`
	TopMem    []ProcessInfo            `json:"top_mem"`
	Docker    docker.DiskUsageSnapshot `json:"docker"`
	DockerDirs []DirUsage              `json:"docker_dirs"`
	CheckedAt time.Time                `json:"checked_at"`
}

// DirUsage is a host directory size under Docker's data root.
type DirUsage struct {
	Path      string `json:"path"`
	SizeBytes uint64 `json:"size_bytes"`
}

type Service struct {
	host        *hostexec.Runner
	docker      docker.Client
	hostIPCache *hostIPCache
}

func NewService(hostRoot string, dockerClient docker.Client) *Service {
	return &Service{
		host:   hostexec.New(hostRoot),
		docker: dockerClient,
	}
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	out := Status{
		CheckedAt:  time.Now().UTC(),
		Disk:       []DiskInfo{},
		DockerDirs: []DirUsage{},
		TopCPU:     []ProcessInfo{},
		TopMem:     []ProcessInfo{},
	}

	rootPath := "/"
	if s.host.UsesChroot() {
		rootPath = s.host.ChrootPath("/")
	}
	if d, err := diskUsage(rootPath, "/"); err == nil {
		out.Disk = append(out.Disk, d)
	}

	dockerPath := "/var/lib/docker"
	if s.host.UsesChroot() {
		dockerPath = s.host.ChrootPath("/var/lib/docker")
	}
	if d, err := diskUsage(dockerPath, "/var/lib/docker"); err == nil {
		if len(out.Disk) == 0 || out.Disk[0].TotalBytes != d.TotalBytes {
			out.Disk = append(out.Disk, d)
		}
	}

	if mem, err := readMemory("/proc/meminfo"); err == nil {
		out.Memory = mem
	}

	if du, err := s.docker.DiskUsage(ctx); err == nil {
		out.Docker = du
	}

	// Process lists and docker dir sizes are loaded on demand via Processes / DockerDirs.
	return out, nil
}

type ProcessesStatus struct {
	TopCPU    []ProcessInfo `json:"top_cpu"`
	TopMem    []ProcessInfo `json:"top_mem"`
	CheckedAt time.Time     `json:"checked_at"`
}

func (s *Service) Processes(ctx context.Context) (ProcessesStatus, error) {
	_ = ctx
	out := ProcessesStatus{
		CheckedAt: time.Now().UTC(),
		TopCPU:    []ProcessInfo{},
		TopMem:    []ProcessInfo{},
	}
	mem, err := readMemory("/proc/meminfo")
	if err != nil {
		return out, nil
	}
	procs, err := sampleTopProcesses(mem.TotalBytes)
	if err != nil {
		return out, nil
	}
	out.TopCPU = topByCPU(procs, 5)
	out.TopMem = topByMem(procs, 5)
	return out, nil
}

func (s *Service) DockerDirs(ctx context.Context) ([]DirUsage, error) {
	return s.dockerDataDirs(ctx), nil
}

func (s *Service) PruneDocker(ctx context.Context) (docker.PruneResult, error) {
	return s.docker.Prune(ctx)
}

// dockerDataDirs reports on-disk sizes of /var/lib/docker subdirectories (host view).
func (s *Service) dockerDataDirs(ctx context.Context) []DirUsage {
	names := []string{"overlay2", "image", "volumes", "containers", "buildkit", "tmp"}
	args := make([]string, 0, len(names)+1)
	args = append(args, "-sb")
	for _, name := range names {
		p := "/var/lib/docker/" + name
		if s.host.UsesChroot() {
			// du runs via chroot → host paths.
			p = "/var/lib/docker/" + name
		}
		args = append(args, p)
	}

	duCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var outRaw string
	var err error
	if s.host.UsesChroot() {
		outRaw, err = s.host.RunShellCombined(duCtx, "du -sb /var/lib/docker/overlay2 /var/lib/docker/image /var/lib/docker/volumes /var/lib/docker/containers /var/lib/docker/buildkit /var/lib/docker/tmp 2>/dev/null || true")
	} else {
		outRaw, err = s.host.RunHostCombined(duCtx, "du", args...)
	}
	if err != nil && outRaw == "" {
		return nil
	}

	var rows []DirUsage
	sc := bufio.NewScanner(strings.NewReader(outRaw))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		sz, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || sz == 0 {
			continue
		}
		rows = append(rows, DirUsage{Path: fields[1], SizeBytes: sz})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SizeBytes > rows[j].SizeBytes })
	return rows
}

func diskUsage(statPath, label string) (DiskInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(statPath, &st); err != nil {
		return DiskInfo{}, err
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	avail := st.Bavail * bsize
	used := total - st.Bfree*bsize
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return DiskInfo{
		Path:           label,
		TotalBytes:     total,
		UsedBytes:      used,
		AvailableBytes: avail,
		UsedPercent:    pct,
	}, nil
}

func readMemory(path string) (MemoryInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return MemoryInfo{}, err
	}
	defer f.Close()

	var total, avail uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		v *= 1024
		switch fields[0] {
		case "MemTotal:":
			total = v
		case "MemAvailable:":
			avail = v
		}
	}
	if total == 0 {
		return MemoryInfo{}, fmt.Errorf("meminfo: MemTotal missing")
	}
	used := total - avail
	return MemoryInfo{
		TotalBytes:     total,
		AvailableBytes: avail,
		UsedBytes:      used,
		UsedPercent:    float64(used) / float64(total) * 100,
	}, nil
}

type procSample struct {
	pid     int
	comm    string
	cmdline string
	utime   uint64
	stime   uint64
	rssPages uint64
}

func sampleTopProcesses(memTotalBytes uint64) ([]ProcessInfo, error) {
	first, err := readProcMap()
	if err != nil {
		return nil, err
	}
	time.Sleep(250 * time.Millisecond)
	second, err := readProcMap()
	if err != nil {
		return nil, err
	}

	pageSize := uint64(os.Getpagesize())
	hz := float64(sysconfClockTicks())
	out := make([]ProcessInfo, 0, len(second))
	for pid, b := range second {
		a, ok := first[pid]
		ticks := float64(b.utime + b.stime)
		cpu := 0.0
		if ok && hz > 0 {
			delta := float64((b.utime + b.stime) - (a.utime + a.stime))
			if delta < 0 {
				delta = 0
			}
			cpu = 100 * delta / (hz * 0.25)
		} else if hz > 0 {
			uptime := readUptimeSeconds()
			if uptime > 0 {
				cpu = 100 * ticks / (hz * uptime)
			}
		}
		rss := b.rssPages * pageSize
		memPct := 0.0
		if memTotalBytes > 0 {
			memPct = float64(rss) / float64(memTotalBytes) * 100
		}
		cmd := b.cmdline
		if cmd == "" {
			cmd = b.comm
		}
		if len(cmd) > 120 {
			cmd = cmd[:117] + "..."
		}
		out = append(out, ProcessInfo{
			PID:        pid,
			CPUPercent: cpu,
			MemPercent: memPct,
			RSSBytes:   rss,
			Command:    cmd,
		})
	}
	return out, nil
}

func readProcMap() (map[int]procSample, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make(map[int]procSample, 64)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		s, err := readProcSample(pid)
		if err != nil {
			continue
		}
		out[pid] = s
	}
	return out, nil
}

func readProcSample(pid int) (procSample, error) {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	raw, err := os.ReadFile(statPath)
	if err != nil {
		return procSample{}, err
	}
	s := string(raw)
	// Format: pid (comm) state ppid ... utime stime ... rss
	lparen := strings.IndexByte(s, '(')
	rparen := strings.LastIndexByte(s, ')')
	if lparen < 0 || rparen < 0 || rparen <= lparen {
		return procSample{}, fmt.Errorf("bad stat")
	}
	comm := s[lparen+1 : rparen]
	rest := strings.Fields(s[rparen+2:])
	// after state: fields[0]=state in full list but we sliced after ") "
	// rest[0]=state, rest[11]=utime, rest[12]=stime, rest[21]=rss (0-based from state)
	if len(rest) < 22 {
		return procSample{}, fmt.Errorf("short stat")
	}
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	rss, _ := strconv.ParseUint(rest[21], 10, 64)

	cmdline := ""
	if b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline")); err == nil {
		cmdline = strings.ReplaceAll(string(b), "\x00", " ")
		cmdline = strings.TrimSpace(cmdline)
	}

	return procSample{
		pid:      pid,
		comm:     comm,
		cmdline:  cmdline,
		utime:    utime,
		stime:    stime,
		rssPages: rss,
	}, nil
}

func readUptimeSeconds() float64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func sysconfClockTicks() int64 {
	// Linux default; reading CLK_TCK via syscall is awkward cross-platform.
	return 100
}

func topByCPU(procs []ProcessInfo, n int) []ProcessInfo {
	cp := append([]ProcessInfo(nil), procs...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].CPUPercent > cp[j].CPUPercent })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

func topByMem(procs []ProcessInfo, n int) []ProcessInfo {
	cp := append([]ProcessInfo(nil), procs...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].RSSBytes != cp[j].RSSBytes {
			return cp[i].RSSBytes > cp[j].RSSBytes
		}
		return cp[i].MemPercent > cp[j].MemPercent
	})
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

func FormatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
