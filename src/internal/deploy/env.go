// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

type accessorySpec struct {
	image   string
	version string // default tag, accessories[].version overrides it
	data    string // folder inside the container that goes to a volume
	cmd     []string
	env     func(pass string) map[string]string       // settings for the container
	vars    func(host, pass string) map[string]string // settings for the app
}

var accessories = map[string]accessorySpec{
	"postgres": {
		image: "postgres", version: "17", data: "/var/lib/postgresql/data",
		env: func(pass string) map[string]string {
			return map[string]string{"POSTGRES_USER": "app", "POSTGRES_PASSWORD": pass, "POSTGRES_DB": "app"}
		},
		vars: postgresVars,
	},
	"mysql": {
		image: "mysql", version: "8.4", data: "/var/lib/mysql",
		env: func(pass string) map[string]string {
			return map[string]string{"MYSQL_USER": "app", "MYSQL_PASSWORD": pass, "MYSQL_DATABASE": "app", "MYSQL_RANDOM_ROOT_PASSWORD": "yes"}
		},
		vars: mysqlVars,
	},
	"mariadb": {
		image: "mariadb", version: "11", data: "/var/lib/mysql",
		env: func(pass string) map[string]string {
			return map[string]string{"MARIADB_USER": "app", "MARIADB_PASSWORD": pass, "MARIADB_DATABASE": "app", "MARIADB_RANDOM_ROOT_PASSWORD": "1"}
		},
		vars: mysqlVars,
	},
	"redis": {
		image: "redis", version: "7-alpine", data: "/data",
		cmd: []string{"sh", "-c", `exec redis-server --appendonly yes --requirepass "$REDIS_PASSWORD"`},
		env: func(pass string) map[string]string {
			return map[string]string{"REDIS_PASSWORD": pass}
		},
		vars: func(host, pass string) map[string]string {
			return map[string]string{
				"REDIS_URL":      fmt.Sprintf("redis://:%s@%s:6379", pass, host),
				"REDIS_HOST":     host,
				"REDIS_PORT":     "6379",
				"REDIS_PASSWORD": pass,
			}
		},
	},
}

func postgresVars(host, pass string) map[string]string {
	return map[string]string{
		"DATABASE_URL":  fmt.Sprintf("postgresql://app:%s@%s:5432/app", pass, host),
		"DB_CONNECTION": "pgsql",
		"DB_HOST":       host,
		"DB_PORT":       "5432",
		"DB_DATABASE":   "app",
		"DB_USERNAME":   "app",
		"DB_PASSWORD":   pass,
	}
}

// mysqlVars serves both MySQL and MariaDB.
func mysqlVars(host, pass string) map[string]string {
	return map[string]string{
		"DATABASE_URL":  fmt.Sprintf("mysql://app:%s@%s:3306/app", pass, host),
		"DB_CONNECTION": "mysql",
		"DB_HOST":       host,
		"DB_PORT":       "3306",
		"DB_DATABASE":   "app",
		"DB_USERNAME":   "app",
		"DB_PASSWORD":   pass,
	}
}

func (e *Engine) accessoryName(name string) string { return "muken-" + e.Cfg.App + "-" + name }

// ensureAccessories creates missing databases and returns the variables for the app. It never replaces one.
func (e *Engine) ensureAccessories(ctx context.Context) (map[string]string, error) {
	vars := map[string]string{}
	for _, accessory := range e.Cfg.Accessories {
		spec := accessories[accessory.Type]
		containerName := e.accessoryName(accessory.Name)
		pass, err := e.accessoryPassword(ctx, accessory.Name)
		if err != nil {
			return nil, err
		}
		if err := e.startAccessory(ctx, accessory, spec, containerName, pass); err != nil {
			return nil, err
		}
		for k, v := range spec.vars(containerName, pass) {
			vars[k] = v
		}
	}
	return vars, nil
}

// accessoryPassword reads the password file on the server and creates it the first time.
func (e *Engine) accessoryPassword(ctx context.Context, name string) (string, error) {
	file := quote(e.appDir() + "/" + name + ".pass")
	out, err := e.R.Run(ctx, "mkdir -p "+quote(e.appDir())+" && { [ -s "+file+" ] || (umask 077; openssl rand -hex 16 > "+file+"); } && cat "+file)
	return strings.TrimSpace(out), err
}

func (e *Engine) startAccessory(ctx context.Context, accessory config.Accessory, spec accessorySpec, containerName, pass string) error {
	state, _ := e.R.Run(ctx, "docker inspect -f '{{.State.Status}}' "+quote(containerName)+" 2>/dev/null")
	switch strings.TrimSpace(state) {
	case "running":
		return nil
	case "":
		return e.createAccessory(ctx, accessory, spec, containerName, pass)
	}
	_, err := e.R.Run(ctx, "docker start "+quote(containerName))
	return err
}

func (e *Engine) createAccessory(ctx context.Context, accessory config.Accessory, spec accessorySpec, containerName, pass string) error {
	e.UI.Step("Creating %s %s", accessory.Type, accessory.Name)
	version := accessory.Version
	if version == "" {
		version = spec.version
	}
	envFile := e.appDir() + "/" + accessory.Name + ".env"
	if err := remote.Upload(ctx, e.R, envFile, []byte(renderEnv(spec.env(pass))), "600"); err != nil {
		return err
	}
	args := []string{"docker", "run", "-d", "--name", containerName, "--network", "muken", "--restart", "unless-stopped",
		"--env-file", envFile, "--label", "muken.app=" + e.Cfg.App + "-accessory", "-v", containerName + ":" + spec.data, spec.image + ":" + version}
	args = append(args, spec.cmd...)
	if _, err := e.R.Run(ctx, cmdline(args...)); err != nil {
		return ui.Wrap(err, "check the version in accessories", "cannot start %s", containerName)
	}
	return nil
}

// renderEnv sorts the lines, so the file changes only when a value does.
func renderEnv(envMap map[string]string) string {
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + envMap[k] + "\n")
	}
	return b.String()
}

// buildEnv merges accessory variables, defaults, config values and secrets.
// Later sources win.
func (e *Engine) buildEnv(accessoryVars map[string]string, release string) (map[string]string, error) {
	env := map[string]string{
		// Lets the web server trust Traefik for the client address and scheme.
		"TRUSTED_PROXIES": "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16",
	}
	for _, src := range []map[string]string{accessoryVars, e.Cfg.Web.Env, e.Secrets} {
		for k, v := range src {
			env[k] = v
		}
	}
	env["MUKEN_APP"] = e.Cfg.App
	env["MUKEN_RELEASE"] = release
	for k, v := range env {
		if strings.ContainsAny(v, "\n\r") {
			return nil, ui.Errf("keep each value on one line", "variable %s has a line break", k)
		}
	}
	return env, nil
}
