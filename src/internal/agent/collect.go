// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package agent is the monitoring sidecar. It reads the Docker API (GET only),
// shows a small status page, serves Prometheus metrics and sends alerts.
package agent

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Docker interface {
	Get(ctx context.Context, path string) ([]byte, error)
}

type unixDocker struct {
	client *http.Client
}

func NewUnixDocker(socketPath string) Docker {
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "unix", socketPath)
	}
	return &unixDocker{client: &http.Client{
		Timeout:   20 * time.Second,
		Transport: &http.Transport{DialContext: dial},
	}}
}

func (u *unixDocker) Get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker %s: HTTP %d", path, resp.StatusCode)
	}
	return body, nil
}

type ContainerInfo struct {
	Name       string  `json:"name"`
	App        string  `json:"app"`
	Release    string  `json:"release"`
	Role       string  `json:"role"`
	State      string  `json:"state"`
	Current    bool    `json:"current"`
	Ready      bool    `json:"ready"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsed    uint64  `json:"mem_used"`
	MemLimit   uint64  `json:"mem_limit"`
	Restarts   int     `json:"restarts"`
	StartedAt  string  `json:"started_at"`
}

type HostInfo struct {
	Load1         float64 `json:"load1"`
	MemTotal      uint64  `json:"mem_total"`
	MemAvailable  uint64  `json:"mem_available"`
	DiskTotal     uint64  `json:"disk_total"`
	DiskUsedRatio float64 `json:"disk_used_ratio"`
}

type DeployEvent struct {
	Time, App, Release, Image, Status string
}

type LogLine struct {
	Container string `json:"container"`
	Line      string `json:"line"`
}

type Snapshot struct {
	Time       string          `json:"time"`
	Host       HostInfo        `json:"host"`
	Containers []ContainerInfo `json:"containers"`
	Deploys    []DeployEvent   `json:"deploys"`
	Errors     []LogLine       `json:"errors"`
	Alerts     []string        `json:"alerts"`
	Error      string          `json:"error,omitempty"`
}

type Collector struct {
	Docker   Docker
	HTTP     *http.Client
	ProcDir  string
	DiskPath string
	BaseDir  string
	Now      func() time.Time
}

const (
	maxErrorLines      = 30 // in a snapshot
	maxErrorsPerLogs   = 10 // per container
	maxDeploys         = 15 // in a snapshot
	maxDeploysPerApp   = 10 // read from each history.log
	maxLogLineLength   = 300
	logTail            = 100
	containerListQuery = "/containers/json?all=1&filters="
)

var errorPattern = regexp.MustCompile(`(?i)\b(error|fatal|critical|exception|emergency|panic)\b`)

type listItem struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// Collect reads Docker and the host. A Docker failure goes into Snapshot.Error.
func (c *Collector) Collect(ctx context.Context) Snapshot {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	snap := Snapshot{
		Time:    now().UTC().Format(time.RFC3339),
		Host:    c.host(),
		Deploys: c.deploys(),
	}

	items, err := c.listContainers(ctx)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.Containers, snap.Errors = c.inspectAll(ctx, items, c.currentReleases())
	if len(snap.Errors) > maxErrorLines {
		snap.Errors = snap.Errors[len(snap.Errors)-maxErrorLines:]
	}
	return snap
}

func (c *Collector) listContainers(ctx context.Context) ([]listItem, error) {
	filter := url.QueryEscape(`{"label":["muken.app"]}`)
	raw, err := c.Docker.Get(ctx, containerListQuery+filter)
	if err != nil {
		return nil, errors.New("cannot read Docker: " + err.Error())
	}
	var items []listItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, errors.New("unreadable Docker answer")
	}
	return items, nil
}

// inspectAll looks at the containers in parallel, because each needs several API calls.
func (c *Collector) inspectAll(ctx context.Context, items []listItem, current map[string]string) ([]ContainerInfo, []LogLine) {
	infos := make([]ContainerInfo, len(items))
	lines := make([][]LogLine, len(items))

	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			infos[i], lines[i] = c.inspectOne(ctx, items[i], current)
		}()
	}
	wg.Wait()

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	var errs []LogLine
	for _, lineGroup := range lines {
		errs = append(errs, lineGroup...)
	}
	return infos, errs
}

func (c *Collector) inspectOne(ctx context.Context, item listItem, current map[string]string) (ContainerInfo, []LogLine) {
	info := newContainerInfo(item, current)
	c.readInspect(ctx, item.ID, &info)
	if item.State != "running" {
		return info, nil
	}
	if statsBody, err := c.Docker.Get(ctx, "/containers/"+item.ID+"/stats?stream=false"); err == nil {
		info.CPUPercent, info.MemUsed, info.MemLimit = parseStats(statsBody)
	}
	if info.Role == "web" {
		info.Ready = c.probe(ctx, info.Name, item.Labels)
	}
	return info, c.errorLines(ctx, item.ID, info.Name)
}

func newContainerInfo(item listItem, current map[string]string) ContainerInfo {
	info := ContainerInfo{
		App:     item.Labels["muken.app"],
		Release: item.Labels["muken.release"],
		Role:    item.Labels["muken.role"],
		State:   item.State,
	}
	if len(item.Names) > 0 {
		info.Name = strings.TrimPrefix(item.Names[0], "/")
	}
	if strings.HasSuffix(info.App, "-accessory") {
		info.Role, info.Current = "accessory", true
	} else if release, ok := current[info.App]; ok {
		info.Current = release == info.Release
	}
	return info
}

func (c *Collector) readInspect(ctx context.Context, containerID string, info *ContainerInfo) {
	body, err := c.Docker.Get(ctx, "/containers/"+containerID+"/json")
	if err != nil {
		return
	}
	var inspect struct {
		RestartCount int
		State        struct{ StartedAt string }
	}
	if json.Unmarshal(body, &inspect) == nil {
		info.Restarts = inspect.RestartCount
		info.StartedAt = inspect.State.StartedAt
	}
}

func (c *Collector) errorLines(ctx context.Context, containerID, name string) []LogLine {
	body, err := c.Docker.Get(ctx, "/containers/"+containerID+"/logs?stdout=1&stderr=1&tail="+strconv.Itoa(logTail))
	if err != nil {
		return nil
	}
	var lines []LogLine
	for _, line := range demux(body) {
		if errorPattern.MatchString(line) {
			lines = append(lines, LogLine{Container: name, Line: truncate(line, maxLogLineLength)})
		}
	}
	if len(lines) > maxErrorsPerLogs {
		lines = lines[len(lines)-maxErrorsPerLogs:]
	}
	return lines
}

func truncate(text string, maxLength int) string {
	if len(text) > maxLength {
		return text[:maxLength] + "..."
	}
	return text
}

// probe asks the container's health URL. Only a 200 counts as ready.
func (c *Collector) probe(ctx context.Context, name string, labels map[string]string) bool {
	port, path := labels["muken.health_port"], labels["muken.health"]
	if port == "" || path == "" {
		return false
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+name+":"+port+path, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type cpuStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs     int    `json:"online_cpus"`
}

type statsResponse struct {
	CPU    cpuStats `json:"cpu_stats"`
	PreCPU cpuStats `json:"precpu_stats"`
	Mem    struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

func parseStats(body []byte) (cpu float64, used, limit uint64) {
	var stats statsResponse
	if json.Unmarshal(body, &stats) != nil {
		return 0, 0, 0
	}
	return cpuPercent(stats), memoryInUse(stats), stats.Mem.Limit
}

// cpuPercent follows `docker stats`: 100 means one full core.
func cpuPercent(stats statsResponse) float64 {
	usage := float64(stats.CPU.CPUUsage.TotalUsage) - float64(stats.PreCPU.CPUUsage.TotalUsage)
	system := float64(stats.CPU.SystemCPUUsage) - float64(stats.PreCPU.SystemCPUUsage)
	if usage <= 0 || system <= 0 {
		return 0
	}
	cpus := stats.CPU.OnlineCPUs
	if cpus == 0 {
		cpus = 1
	}
	return usage / system * float64(cpus) * 100
}

// memoryInUse leaves out the page cache, like `docker stats` does.
func memoryInUse(stats statsResponse) uint64 {
	used := stats.Mem.Usage
	cache := stats.Mem.Stats["inactive_file"]
	if cache == 0 {
		cache = stats.Mem.Stats["cache"]
	}
	if cache < used {
		used -= cache
	}
	return used
}

// demux strips the 8-byte frames Docker puts around log lines.
func demux(data []byte) []string {
	framed := len(data) >= 8 && data[0] <= 2 && data[1] == 0 && data[2] == 0 && data[3] == 0
	if !framed {
		return plainLines(data)
	}
	var lines []string
	for len(data) >= 8 {
		n := int(binary.BigEndian.Uint32(data[4:8]))
		data = data[8:]
		if n > len(data) {
			n = len(data)
		}
		for _, line := range strings.Split(strings.TrimRight(string(data[:n]), "\n"), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
		data = data[n:]
	}
	return lines
}

func plainLines(data []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func (c *Collector) procDir() string {
	if c.ProcDir != "" {
		return c.ProcDir
	}
	return "/proc"
}

func (c *Collector) host() HostInfo {
	var info HostInfo
	if data, err := os.ReadFile(filepath.Join(c.procDir(), "loadavg")); err == nil {
		if fields := strings.Fields(string(data)); len(fields) > 0 {
			info.Load1, _ = strconv.ParseFloat(fields[0], 64)
		}
	}
	if data, err := os.ReadFile(filepath.Join(c.procDir(), "meminfo")); err == nil {
		info.MemTotal, info.MemAvailable = parseMeminfo(string(data))
	}
	if c.DiskPath != "" {
		info.DiskTotal, info.DiskUsedRatio = diskUsage(c.DiskPath)
	}
	return info
}

func diskUsage(path string) (total uint64, usedRatio float64) {
	var stat syscall.Statfs_t
	if syscall.Statfs(path, &stat) != nil || stat.Blocks == 0 {
		return 0, 0
	}
	return stat.Blocks * uint64(stat.Bsize), 1 - float64(stat.Bavail)/float64(stat.Blocks)
}

// parseMeminfo reads /proc/meminfo, which lists sizes in kB.
func parseMeminfo(text string) (total, avail uint64) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kb, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = kb * 1024
		case "MemAvailable:":
			avail = kb * 1024
		}
	}
	return total, avail
}

// currentReleases maps each app to the release in its state.json.
func (c *Collector) currentReleases() map[string]string {
	releases := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(c.BaseDir, "apps", "*", "state.json"))
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var state struct {
			Release string `json:"release"`
		}
		if json.Unmarshal(b, &state) == nil {
			releases[filepath.Base(filepath.Dir(file))] = state.Release
		}
	}
	return releases
}

// deploys returns the newest events of all apps. A history line is time|app|release|image|status.
func (c *Collector) deploys() []DeployEvent {
	files, _ := filepath.Glob(filepath.Join(c.BaseDir, "apps", "*", "history.log"))
	var events []DeployEvent
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > maxDeploysPerApp {
			lines = lines[len(lines)-maxDeploysPerApp:]
		}
		for _, line := range lines {
			if fields := strings.Split(line, "|"); len(fields) == 5 {
				events = append(events, DeployEvent{fields[0], fields[1], fields[2], fields[3], fields[4]})
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time > events[j].Time })
	if len(events) > maxDeploys {
		events = events[:maxDeploys]
	}
	return events
}
