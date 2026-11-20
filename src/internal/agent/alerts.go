// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Alerter turns snapshots into FIRING and RESOLVED messages.
// A problem must last Threshold polls before it fires, so short blips stay quiet.
type Alerter struct {
	Threshold int
	counts    map[string]int    // polls in a row that a problem was seen
	fired     map[string]string // problems already announced, with their message
	restarts  map[string]int    // last restart count per container
}

// NewAlerter makes an Alerter. A threshold below 1 becomes 2.
func NewAlerter(threshold int) *Alerter {
	if threshold < 1 {
		threshold = 2
	}
	return &Alerter{
		Threshold: threshold,
		counts:    map[string]int{},
		fired:     map[string]string{},
		restarts:  map[string]int{},
	}
}

// problems lists what is wrong now, keyed by a stable id.
// Releases kept for rollback are stopped on purpose, so only current ones count.
func problems(snapshot Snapshot) map[string]string {
	found := map[string]string{}
	for _, c := range snapshot.Containers {
		if c.Current {
			containerProblems(c, found)
		}
	}
	if snapshot.Host.DiskUsedRatio > 0.9 {
		found["disk"] = fmt.Sprintf("disk is %.0f%% full", snapshot.Host.DiskUsedRatio*100)
	}
	return found
}

func containerProblems(container ContainerInfo, found map[string]string) {
	switch {
	case container.State != "running":
		found["down:"+container.Name] = fmt.Sprintf("%s is %s", container.Name, container.State)
	case container.Role == "web" && !container.Ready:
		found["notready:"+container.Name] = fmt.Sprintf("%s does not pass its health check", container.Name)
	}
	if container.MemLimit > 0 {
		ratio := float64(container.MemUsed) / float64(container.MemLimit)
		if ratio > 0.9 {
			found["mem:"+container.Name] = fmt.Sprintf("%s uses %.0f%% of its memory limit", container.Name, ratio*100)
		}
	}
}

func (a *Alerter) Evaluate(snapshot Snapshot) []string {
	if snapshot.Error != "" {
		return nil // cannot judge without data
	}
	currentProblems := problems(snapshot)
	out := a.fire(currentProblems)
	out = append(out, a.resolve(currentProblems)...)
	out = append(out, a.restarted(snapshot)...)
	sort.Strings(out)
	return out
}

func (a *Alerter) fire(currentProblems map[string]string) []string {
	var out []string
	for key, msg := range currentProblems {
		a.counts[key]++
		if _, done := a.fired[key]; !done && a.counts[key] >= a.Threshold {
			a.fired[key] = msg
			out = append(out, "FIRING: "+msg)
		}
	}
	return out
}

func (a *Alerter) resolve(currentProblems map[string]string) []string {
	var out []string
	for key := range a.counts {
		if _, still := currentProblems[key]; still {
			continue
		}
		delete(a.counts, key)
		if msg, ok := a.fired[key]; ok {
			delete(a.fired, key)
			out = append(out, "RESOLVED: "+msg)
		}
	}
	return out
}

// restarted compares restart counts with the previous poll.
// The first sight of a container sets the baseline and says nothing.
func (a *Alerter) restarted(snapshot Snapshot) []string {
	var out []string
	for _, c := range snapshot.Containers {
		if !c.Current {
			continue
		}
		if previousRestarts, seen := a.restarts[c.Name]; seen && c.Restarts > previousRestarts {
			out = append(out, fmt.Sprintf("RESTARTED: %s restarted (%d total)", c.Name, c.Restarts))
		}
		a.restarts[c.Name] = c.Restarts
	}
	return out
}

func (a *Alerter) Active() []string {
	var out []string
	for _, msg := range a.fired {
		out = append(out, msg)
	}
	sort.Strings(out)
	return out
}

// Webhook posts a message as JSON. The body works for Slack ("text") and Discord ("content").
type Webhook struct {
	URL  string
	HTTP *http.Client
	Host string
}

// Send posts msg, prefixed with the host name if set. A nil or empty Webhook does nothing.
func (w *Webhook) Send(ctx context.Context, msg string) error {
	if w == nil || w.URL == "" {
		return nil
	}
	if w.Host != "" {
		msg = "[" + w.Host + "] " + msg
	}
	body, _ := json.Marshal(map[string]string{"text": msg, "content": msg})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := w.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook answered HTTP %d", resp.StatusCode)
	}
	return nil
}

// Metrics renders the snapshot in the Prometheus text format.
func Metrics(snapshot Snapshot) string {
	var builder strings.Builder
	writeHostMetrics(&builder, snapshot.Host)
	writeContainerMetrics(&builder, snapshot.Containers)
	return builder.String()
}

func writeGauge(builder *strings.Builder, name, help string) {
	fmt.Fprintf(builder, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name)
}

func writeHostMetrics(builder *strings.Builder, host HostInfo) {
	writeGauge(builder, "muken_host_load1", "Host load average over one minute")
	fmt.Fprintf(builder, "muken_host_load1 %g\n", host.Load1)
	writeGauge(builder, "muken_host_memory_available_bytes", "Host memory available")
	fmt.Fprintf(builder, "muken_host_memory_available_bytes %d\n", host.MemAvailable)
	writeGauge(builder, "muken_host_disk_used_ratio", "Share of the disk in use")
	fmt.Fprintf(builder, "muken_host_disk_used_ratio %g\n", host.DiskUsedRatio)
}

func writeContainerMetrics(builder *strings.Builder, containers []ContainerInfo) {
	writeContainerGauge(builder, "muken_container_up", "1 when the container runs", containers,
		func(c ContainerInfo) float64 { return boolToFloat(c.State == "running") })
	writeContainerGauge(builder, "muken_container_ready", "1 when the web container passes its health check", containers,
		func(c ContainerInfo) float64 { return boolToFloat(c.Ready) })
	writeContainerGauge(builder, "muken_container_cpu_percent", "CPU use in percent of one core", containers,
		func(c ContainerInfo) float64 { return c.CPUPercent })
	writeContainerGauge(builder, "muken_container_memory_bytes", "Memory in use", containers,
		func(c ContainerInfo) float64 { return float64(c.MemUsed) })
	writeContainerGauge(builder, "muken_container_restarts", "Restart count", containers,
		func(c ContainerInfo) float64 { return float64(c.Restarts) })
}

func writeContainerGauge(builder *strings.Builder, name, help string, containers []ContainerInfo, value func(ContainerInfo) float64) {
	writeGauge(builder, name, help)
	for _, c := range containers {
		fmt.Fprintf(builder, "%s{app=%q,role=%q,name=%q,current=\"%t\"} %g\n", name, c.App, c.Role, c.Name, c.Current, value(c))
	}
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
