package metrics

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Snapshot is a normalized host metrics payload used by Barn and the agent.
type Snapshot struct {
	Hostname       string      `json:"hostname"`
	OSName         string      `json:"os_name"`
	OSVersion      string      `json:"os_version"`
	Kernel         string      `json:"kernel"`
	Architecture   string      `json:"architecture"`
	UptimeSeconds  int64       `json:"uptime_seconds"`
	CPUPercent     float64     `json:"cpu_percent"`
	Load1          float64     `json:"load_1"`
	Load5          float64     `json:"load_5"`
	Load15         float64     `json:"load_15"`
	MemoryUsed     uint64      `json:"memory_used_bytes"`
	MemoryTotal    uint64      `json:"memory_total_bytes"`
	SwapUsed       uint64      `json:"swap_used_bytes"`
	SwapTotal      uint64      `json:"swap_total_bytes"`
	DiskUsedPct    float64     `json:"disk_used_percent"`
	Filesystems    []Filesystem `json:"filesystems"`
	CollectedAt    time.Time   `json:"collected_at"`
}

type Filesystem struct {
	Mountpoint  string  `json:"mountpoint"`
	Device      string  `json:"device"`
	Filesystem  string  `json:"filesystem"`
	UsedBytes   uint64  `json:"used_bytes"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// Collector reads host metrics. root is "" for native, or a chroot/host mount like "/host".
type Collector struct {
	Root string
}

func New(root string) *Collector {
	return &Collector{Root: strings.TrimSpace(root)}
}

func (c *Collector) path(p string) string {
	if c.Root == "" {
		return p
	}
	return filepath.Join(c.Root, strings.TrimPrefix(p, "/"))
}

func (c *Collector) Collect() (Snapshot, error) {
	out := Snapshot{
		Architecture: runtime.GOARCH,
		CollectedAt:  time.Now().UTC(),
		Filesystems:  []Filesystem{},
	}
	if hn, err := os.Hostname(); err == nil {
		out.Hostname = hn
	}
	out.OSName, out.OSVersion = c.readOSRelease()
	if b, err := os.ReadFile(c.path("/proc/sys/kernel/hostname")); err == nil {
		if h := strings.TrimSpace(string(b)); h != "" {
			out.Hostname = h
		}
	}
	if b, err := os.ReadFile(c.path("/proc/sys/kernel/osrelease")); err == nil {
		out.Kernel = strings.TrimSpace(string(b))
	}
	out.UptimeSeconds = c.readUptime()
	out.Load1, out.Load5, out.Load15 = c.readLoad()
	memTotal, memAvail, swapTotal, swapFree := c.readMeminfo()
	out.MemoryTotal = memTotal
	if memTotal >= memAvail {
		out.MemoryUsed = memTotal - memAvail
	}
	out.SwapTotal = swapTotal
	if swapTotal >= swapFree {
		out.SwapUsed = swapTotal - swapFree
	}
	out.CPUPercent = c.sampleCPU()
	if fs, err := c.rootFS(); err == nil {
		out.Filesystems = append(out.Filesystems, fs)
		out.DiskUsedPct = fs.UsedPercent
	}
	return out, nil
}

func (c *Collector) readOSRelease() (name, version string) {
	f, err := os.Open(c.path("/etc/os-release"))
	if err != nil {
		return runtime.GOOS, ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), `"`)
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}
	if name == "" {
		name = runtime.GOOS
	}
	return name, version
}

func (c *Collector) readUptime() int64 {
	b, err := os.ReadFile(c.path("/proc/uptime"))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return int64(v)
}

func (c *Collector) readLoad() (float64, float64, float64) {
	b, err := os.ReadFile(c.path("/proc/loadavg"))
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	a, _ := strconv.ParseFloat(fields[0], 64)
	b5, _ := strconv.ParseFloat(fields[1], 64)
	c15, _ := strconv.ParseFloat(fields[2], 64)
	return a, b5, c15
}

func (c *Collector) readMeminfo() (total, avail, swapTotal, swapFree uint64) {
	f, err := os.Open(c.path("/proc/meminfo"))
	if err != nil {
		return 0, 0, 0, 0
	}
	defer f.Close()
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
		case "SwapTotal:":
			swapTotal = v
		case "SwapFree:":
			swapFree = v
		}
	}
	return total, avail, swapTotal, swapFree
}

func (c *Collector) sampleCPU() float64 {
	a, err := c.readCPUTimes()
	if err != nil {
		return 0
	}
	time.Sleep(200 * time.Millisecond)
	b, err := c.readCPUTimes()
	if err != nil {
		return 0
	}
	idle := b.idle - a.idle
	total := b.total - a.total
	if total == 0 {
		return 0
	}
	used := float64(total-idle) / float64(total) * 100
	if used < 0 {
		return 0
	}
	return used
}

type cpuTimes struct{ idle, total uint64 }

func (c *Collector) readCPUTimes() (cpuTimes, error) {
	b, err := os.ReadFile(c.path("/proc/stat"))
	if err != nil {
		return cpuTimes{}, err
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, fmt.Errorf("bad /proc/stat")
	}
	var vals []uint64
	var sum uint64
	for _, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		vals = append(vals, v)
		sum += v
	}
	idle := vals[3]
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}
	return cpuTimes{idle: idle, total: sum}, nil
}

func (c *Collector) rootFS() (Filesystem, error) {
	path := c.path("/")
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Filesystem{}, err
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	used := total - st.Bfree*bsize
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return Filesystem{
		Mountpoint:  "/",
		Device:      "root",
		Filesystem:  "local",
		UsedBytes:   used,
		TotalBytes:  total,
		UsedPercent: pct,
	}, nil
}
