// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/api"
	"github.com/shinsenter/muken/internal/ui"
)

// serve is added in init because its backend looks up `commands`, and the table would otherwise
// refer to itself.
func init() {
	commands["serve"] = command{cmdServe, "Run the JSON API that the web app calls (loopback only)", "Operate"}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func cmdServe(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("serve", &global)
	addr := flagSet.String("addr", envOr("MUKEN_API_ADDR", "127.0.0.1:7600"), "listen address, loopback only")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	if err := api.CheckLoopback(*addr); err != nil {
		return err
	}
	// -c wins. Then MUKEN_CONFIG. Then the default file of the working folder.
	configPath := global.config
	if env := os.Getenv("MUKEN_CONFIG"); env != "" && !flagPassed(flagSet, "c", "config") {
		configPath = env
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(api.Options{Backend: &serveBackend{config: configPath}, ConfigPath: configPath, Version: version, Context: ctx}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	p.Info("muken API on http://%s, config %s. It has no login: keep it on loopback", *addr, configPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return ui.Wrap(err, "check that the port is free", "cannot listen on %s", *addr)
	}
	return nil
}

func flagPassed(flagSet *flag.FlagSet, names ...string) bool {
	seen := false
	flagSet.Visit(func(f *flag.Flag) {
		for _, n := range names {
			if f.Name == n {
				seen = true
			}
		}
	})
	return seen
}

// serveBackend runs the same command functions as the CLI, with the text going to the API.
type serveBackend struct{ config string }

// quiet is a printer for the API: plain text, no questions, "yes" to every confirmation.
func quiet(out io.Writer) *ui.Printer {
	return &ui.Printer{Out: out, Err: out, In: bufio.NewReader(strings.NewReader("")), Yes: true}
}

func (b *serveBackend) Run(ctx context.Context, out *api.Lines, args []string) error {
	cmd, ok := commands[args[0]]
	if !ok {
		return ui.Errf("run `muken help`", "unknown command %q", args[0])
	}
	p := quiet(out)
	err := cmd.run(ctx, p, append([]string{"-c", b.config, "--yes"}, args[1:]...))
	if err != nil {
		p.Fail(err)
	}
	return err
}

func (b *serveBackend) Status(ctx context.Context) (any, error) {
	sess, err := connectSession(ctx, quiet(io.Discard), commonFlags{config: b.config, yes: true})
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	status, err := sess.engine.Status(ctx)
	if err != nil {
		return nil, err
	}
	containers := make([]map[string]any, len(status.Containers))
	for i, c := range status.Containers {
		containers[i] = map[string]any{"name": c.Name, "role": c.Role, "state": c.State, "release": c.Release, "live": c.Release == status.State.Release}
	}
	return map[string]any{
		"app":            sess.cfg.App,
		"server":         sess.cfg.Server.Host,
		"release":        status.State.Release,
		"image":          status.State.Image,
		"previous":       status.State.Previous,
		"previous_image": status.State.PreviousImage,
		"updated_at":     status.State.UpdatedAt,
		"rollback_note":  rollbackLine(status),
		"proxy":          status.Proxy,
		"maintenance":    status.Maintenance,
		"containers":     containers,
	}, nil
}
