// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

const cfgText = `
app: blog
image: shinsenter/wordpress:latest
domains: [blog.example.com, www.blog.example.com]
email: me@example.com
server: {host: 203.0.113.10}
web:
  replicas: 2
  migrate: php artisan migrate --force
  smoke_path: /up
  volumes: ["blog-data:/var/www/html"]
workers:
  - {name: queue, command: "php artisan queue:work", replicas: 1}
accessories:
  - {type: postgres}
`

func engine(t *testing.T, yaml string, f *remote.Fake) *Engine {
	t.Helper()
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	p := &ui.Printer{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	e := New(cfg, f, p, map[string]string{"APP_KEY": "secret-value"})
	e.Now = func() time.Time { return time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC) }
	e.Sleep = func(time.Duration) {}
	e.Getenv = func(string) string { return "" }
	return e
}

// healthy answers the checks of a working server.
func healthy() *remote.Fake {
	return (&remote.Fake{}).
		On("{{.State.Running}}", "true", nil).
		On("{{.State.Status}}", "running", nil)
}

var b64 = regexp.MustCompile(`printf %s '([A-Za-z0-9+/=]*)' \| base64 -d > '([^']+)\.tmp'`)

// uploaded returns the content of the last file written to a path that ends with suffix.
func uploaded(f *remote.Fake, suffix string) string {
	out := ""
	for _, c := range f.Calls {
		if m := b64.FindStringSubmatch(c); m != nil && strings.HasSuffix(m[2], suffix) {
			raw, _ := base64.StdEncoding.DecodeString(m[1])
			out = string(raw)
		}
	}
	return out
}

func TestFirstDeployOrder(t *testing.T) {
	f := healthy()
	e := engine(t, cfgText, f)
	result, err := e.Deploy(context.Background(), DeployOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Release != "20261116T100000" {
		t.Fatalf("release %s", result.Release)
	}
	order := []string{"docker pull", "'--rm'", "'--name' 'muken-blog-web-20261116T100000-1'", "'--name' 'muken-blog-queue-20261116T100000-1'", "traefik/dynamic/blog.yml.tmp", "blog/state.json.tmp"}
	last := -1
	for _, s := range order {
		i := f.Index(s)
		if i < 0 {
			t.Fatalf("no call contains %q", s)
		}
		if i <= last {
			t.Fatalf("%q ran too early (call %d, previous step at %d)", s, i, last)
		}
		last = i
	}
	if f.Count("'--name' 'muken-blog-web-") != 2 {
		t.Fatal("want 2 web replicas")
	}
	if f.Count("docker stop") != 0 {
		t.Fatal("first deploy has nothing to stop")
	}
}

func TestRouteListsAllReplicasAndDomains(t *testing.T) {
	f := healthy()
	e := engine(t, cfgText, f)
	if _, err := e.Deploy(context.Background(), DeployOptions{}); err != nil {
		t.Fatal(err)
	}
	route := uploaded(f, "dynamic/blog.yml")
	for _, want := range []string{"Host(`blog.example.com`) || Host(`www.blog.example.com`)", "http://muken-blog-web-20261116T100000-1:80", "http://muken-blog-web-20261116T100000-2:80", "certResolver: le"} {
		if !strings.Contains(route, want) {
			t.Errorf("route lacks %q:\n%s", want, route)
		}
	}
}

func TestEnvFileMergesSecretsAndAccessories(t *testing.T) {
	f := healthy().On("openssl rand", "pw123\n", nil)
	e := engine(t, cfgText, f)
	if _, err := e.Deploy(context.Background(), DeployOptions{}); err != nil {
		t.Fatal(err)
	}
	env := uploaded(f, "apps/blog/env")
	for _, want := range []string{"APP_KEY=secret-value", "DATABASE_URL=postgresql://app:pw123@muken-blog-postgres:5432/app", "MUKEN_RELEASE=20261116T100000", "TRUSTED_PROXIES="} {
		if !strings.Contains(env, want) {
			t.Errorf("env lacks %q:\n%s", want, env)
		}
	}
}

func TestHealthFailureLeavesOldReleaseAlone(t *testing.T) {
	f := healthy().On("curl -fsS -m 3", "", errors.New("connection refused"))
	e := engine(t, cfgText, f)
	_, err := e.Deploy(context.Background(), DeployOptions{})
	if err == nil || !strings.Contains(err.Error(), "did not become healthy") {
		t.Fatalf("got %v", err)
	}
	if f.Index("traefik/dynamic/blog.yml.tmp") >= 0 {
		t.Fatal("traffic must not switch to an unhealthy release")
	}
	if f.Count("docker rm -f 'muken-blog-web-20261116T100000-1'") == 0 {
		t.Fatal("the new containers must be removed")
	}
	if f.Index("|failed") < 0 {
		t.Fatal("the failure must be in the history")
	}
	var he *ui.HintError
	if !errors.As(err, &he) || he.Hint == "" {
		t.Fatal("the error needs a hint")
	}
}

func TestCrashedContainerFailsFast(t *testing.T) {
	f := (&remote.Fake{}).On("{{.State.Running}}", "true", nil).On("{{.State.Status}}", "exited", nil).On("docker logs", "PHP Fatal error: boom", nil)
	e := engine(t, cfgText, f)
	_, err := e.Deploy(context.Background(), DeployOptions{})
	// The log tail is now printed before the error, so the hint stays last.
	if err == nil || !strings.Contains(e.UI.Out.(*bytes.Buffer).String(), "PHP Fatal error: boom") {
		t.Fatalf("the log tail must reach the user, got %v", err)
	}
	if f.Count("curl -fsS -m 3") != 0 {
		t.Fatal("no point probing a stopped container")
	}
}

func TestMigrationFailureStopsBeforeStartingAnything(t *testing.T) {
	f := healthy().On("'--rm'", "SQLSTATE[42P01]", errors.New("exit 1"))
	e := engine(t, cfgText, f)
	_, err := e.Deploy(context.Background(), DeployOptions{})
	if err == nil || !strings.Contains(err.Error(), "migration failed") {
		t.Fatalf("got %v", err)
	}
	if f.Count("'--name' 'muken-blog-web-") != 0 {
		t.Fatal("no new container may start after a failed migration")
	}
}

func TestSkipMigrate(t *testing.T) {
	f := healthy()
	e := engine(t, cfgText, f)
	if _, err := e.Deploy(context.Background(), DeployOptions{SkipMigrate: true, SkipPull: true}); err != nil {
		t.Fatal(err)
	}
	if f.Index("'--rm'") >= 0 || f.Index("docker pull") >= 0 {
		t.Fatal("skip flags ignored")
	}
}

func TestSmokeFailureRestoresRoute(t *testing.T) {
	f := healthy().On("-H 'Host: blog.example.com'", "", errors.New("503"))
	e := engine(t, cfgText, f)
	_, err := e.Deploy(context.Background(), DeployOptions{})
	if err == nil || !strings.Contains(err.Error(), "smoke check") {
		t.Fatalf("got %v", err)
	}
	if f.Count("route.prev.yml") < 2 {
		t.Fatal("the previous route must be copied back")
	}
	if f.Index("blog/state.json.tmp") >= 0 {
		t.Fatal("state must not move to a release that failed")
	}
}

const stateJSON = `{"release":"20261115T100000","image":"shinsenter/wordpress:old","previous":"20261114T100000","previous_image":"x"}`

func TestSecondDeployStopsOldAndRecordsPrevious(t *testing.T) {
	f := healthy().
		On("cat '/opt/muken/apps/blog/state.json'", stateJSON, nil).
		On("docker ps -a", "muken-blog-web-20261115T100000-1|20261115T100000|web|running\nmuken-blog-web-20261115T100000-2|20261115T100000|web|running\nmuken-blog-queue-20261115T100000-1|20261115T100000|queue|running\nmuken-blog-web-20261114T100000-1|20261114T100000|web|exited\n", nil)
	e := engine(t, cfgText, f)
	if _, err := e.Deploy(context.Background(), DeployOptions{}); err != nil {
		t.Fatal(err)
	}
	stop := ""
	for _, c := range f.Calls {
		if strings.HasPrefix(c, "docker stop") {
			stop = c
		}
	}
	for _, n := range []string{"muken-blog-web-20261115T100000-1", "muken-blog-web-20261115T100000-2", "muken-blog-queue-20261115T100000-1"} {
		if !strings.Contains(stop, n) {
			t.Errorf("old container %s was not stopped: %s", n, stop)
		}
	}
	if strings.Contains(stop, "20261116T100000") {
		t.Fatal("the new release must not be stopped")
	}
	if f.Count("docker rm -f 'muken-blog-web-20261114T100000-1'") == 0 {
		t.Fatal("releases older than keep=2 must be removed")
	}
	stateJSON := uploaded(f, "blog/state.json")
	if !strings.Contains(stateJSON, `"previous": "20261115T100000"`) || !strings.Contains(stateJSON, `"previous_image": "shinsenter/wordpress:old"`) {
		t.Fatalf("state did not keep the previous release:\n%s", stateJSON)
	}
	if f.Index("sleep") >= 0 { // drain is a Sleep call, never a shell sleep
		t.Fatal("drain must not block the shell")
	}
}

func TestRollback(t *testing.T) {
	f := healthy().
		On("cat '/opt/muken/apps/blog/state.json'", `{"release":"20261116T100000","image":"new","previous":"20261115T100000","previous_image":"old"}`, nil).
		On("docker ps -a", "muken-blog-web-20261116T100000-1|20261116T100000|web|running\nmuken-blog-web-20261115T100000-1|20261115T100000|web|exited\nmuken-blog-queue-20261115T100000-1|20261115T100000|queue|exited\n", nil)
	e := engine(t, cfgText, f)
	result, err := e.Rollback(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Release != "20261115T100000" {
		t.Fatalf("got %s", result.Release)
	}
	if f.Count("docker start 'muken-blog-queue-20261115T100000-1' 'muken-blog-web-20261115T100000-1'") != 1 {
		t.Fatalf("the previous release must start: %v", f.Calls)
	}
	route := uploaded(f, "dynamic/blog.yml")
	if !strings.Contains(route, "muken-blog-web-20261115T100000-1") || strings.Contains(route, "20261116") {
		t.Fatalf("route must point at the old release:\n%s", route)
	}
	stateJSON := uploaded(f, "blog/state.json")
	if !strings.Contains(stateJSON, `"release": "20261115T100000"`) || !strings.Contains(stateJSON, `"previous": "20261116T100000"`) {
		t.Fatalf("state not swapped:\n%s", stateJSON)
	}
}

func rollbackFake() *remote.Fake {
	return healthy().
		On("cat '/opt/muken/apps/blog/state.json'", `{"release":"20261116T100000","image":"new","previous":"20261115T100000","previous_image":"old"}`, nil).
		On("docker ps -a", "muken-blog-web-20261116T100000-1|20261116T100000|web|running\nmuken-blog-web-20261115T100000-1|20261115T100000|web|exited\nmuken-blog-queue-20261115T100000-1|20261115T100000|queue|exited\n", nil)
}

func TestRollbackTurnsMaintenanceOff(t *testing.T) {
	f := maintenanceOn(rollbackFake())
	result, err := engine(t, cfgText, f).Rollback(context.Background(), false)
	if err != nil || result.MaintenanceOn {
		t.Fatalf("err %v, maintenance on %v", err, result != nil && result.MaintenanceOn)
	}
	if !strings.Contains(uploaded(f, "dynamic/blog.yml"), "muken-blog-web-20261115T100000-1") {
		t.Fatal("traffic must move to the old release")
	}
	if f.Index("docker rm -f muken-maintenance") < f.Index("traefik/dynamic/blog.yml.tmp") || f.Index("docker rm -f muken-maintenance") < 0 {
		t.Fatal("the maintenance container goes after the switch")
	}
}

func TestRollbackKeepsMaintenanceWhenAsked(t *testing.T) {
	f := maintenanceOn(rollbackFake())
	result, err := engine(t, cfgText, f).Rollback(context.Background(), true)
	if err != nil || !result.MaintenanceOn {
		t.Fatalf("err %v, result %+v", err, result)
	}
	if uploaded(f, "dynamic/blog.yml") != "" || f.Count("docker rm -f muken-maintenance") != 0 {
		t.Fatal("the route and the maintenance container must stay as they are")
	}
	if !strings.Contains(uploaded(f, "blog/state.json"), `"release": "20261115T100000"`) {
		t.Fatal("the old release must be saved as the current one")
	}
}

func TestStatusShowsMaintenance(t *testing.T) {
	for _, on := range []bool{true, false} {
		f := rollbackFake()
		if on {
			maintenanceOn(f)
		}
		status, err := engine(t, cfgText, f).Status(context.Background())
		if err != nil || status.Maintenance != on {
			t.Fatalf("maintenance %v: got %v, err %v", on, status.Maintenance, err)
		}
	}
}

func TestRollbackWithoutHistory(t *testing.T) {
	f := healthy().On("cat '/opt/muken/apps/blog/state.json'", `{"release":"20261116T100000","image":"new"}`, nil)
	if _, err := engine(t, cfgText, f).Rollback(context.Background(), false); err == nil {
		t.Fatal("want an error")
	}
}

func TestRollbackAfterPrune(t *testing.T) {
	f := healthy().
		On("cat '/opt/muken/apps/blog/state.json'", `{"release":"B","image":"new","previous":"A","previous_image":"shinsenter/wordpress:old"}`, nil).
		On("docker ps -a", "muken-blog-web-B-1|B|web|running\n", nil)
	_, err := engine(t, cfgText, f).Rollback(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "pruned") {
		t.Fatalf("got %v", err)
	}
	var he *ui.HintError
	if !errors.As(err, &he) || !strings.Contains(he.Hint, "--image shinsenter/wordpress:old") {
		t.Fatal("the hint must give the command to redeploy the old image")
	}
}

func TestLockBlocksSecondDeploy(t *testing.T) {
	f := healthy().On("date -u +%FT%TZ", "", errors.New("File exists")).On("lock/info", "2026-11-16T09:59:00Z", nil)
	_, err := engine(t, cfgText, f).Deploy(context.Background(), DeployOptions{})
	if err == nil || !strings.Contains(err.Error(), "another deploy") {
		t.Fatalf("got %v", err)
	}
}

func TestLockIsReleasedAfterFailure(t *testing.T) {
	f := healthy().On("docker pull", "", errors.New("denied"))
	e := engine(t, cfgText, f)
	if _, err := e.Deploy(context.Background(), DeployOptions{}); err == nil {
		t.Fatal("want an error")
	}
	if f.Count("rm -rf '/opt/muken/apps/blog/lock'") != 1 {
		t.Fatal("the lock must be removed")
	}
}

func TestPreflightMessages(t *testing.T) {
	f := (&remote.Fake{}).On("docker info", "", errors.New("not found"))
	_, err := engine(t, cfgText, f).Deploy(context.Background(), DeployOptions{})
	if err == nil || !strings.Contains(err.Error(), "Docker is not available") {
		t.Fatalf("got %v", err)
	}
}

func TestPlanDescribesChange(t *testing.T) {
	f := healthy().On("cat '/opt/muken/apps/blog/state.json'", stateJSON, nil)
	lines, err := engine(t, cfgText, f).Plan(context.Background(), DeployOptions{Image: "shinsenter/wordpress:new"})
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(lines, "\n")
	for _, want := range []string{"shinsenter/wordpress:old -> shinsenter/wordpress:new", "Run migration", "Start 2 web", `"queue"`, "kept for rollback"} {
		if !strings.Contains(all, want) {
			t.Errorf("plan lacks %q:\n%s", want, all)
		}
	}
	if f.Index("docker run") >= 0 || f.Index("docker pull") >= 0 {
		t.Fatal("plan must not change anything")
	}
}

func TestRenderRouteWithoutTLS(t *testing.T) {
	cfg, _ := config.Parse([]byte("app: a\nimage: x\ntls: none\nserver: {host: 1.2.3.4}\n"))
	r := RenderRoute(cfg, []string{"c1"})
	if strings.Contains(r, "tls:") || !strings.Contains(r, "entryPoints: [web]") || !strings.Contains(r, "PathPrefix(`/`)") {
		t.Fatalf("unexpected route:\n%s", r)
	}
}

func TestProvisionTraefikFlags(t *testing.T) {
	f := &remote.Fake{}
	e := engine(t, cfgText, f)
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	var traefik string
	for _, c := range f.Calls {
		if strings.Contains(c, "muken-traefik --restart") {
			traefik = c
		}
	}
	for _, want := range []string{"httpchallenge.entrypoint=web", "acme.email=me@example.com", "-p 443:443/udp"} {
		if !strings.Contains(traefik, want) {
			t.Errorf("traefik command lacks %q:\n%s", want, traefik)
		}
	}
	if strings.Contains(traefik, "/var/run/docker.sock") {
		t.Error("Traefik must not get the Docker socket")
	}
	if f.Index("ufw allow 22/tcp") < 0 {
		t.Fatal("the SSH port must be allowed before the firewall turns on")
	}
	if f.Index("ufw allow 22/tcp") > f.Index("ufw --force enable") {
		t.Fatal("ufw rule order")
	}
}

func TestProvisionCloudflareNeedsToken(t *testing.T) {
	yaml := "app: a\nimage: x\ndomains: [a.example.com]\nemail: e@example.com\ntls: cloudflare\ncloudflare: {enabled: true}\nserver: {host: 1.2.3.4}\n"
	e := engine(t, yaml, &remote.Fake{})
	if err := e.Provision(context.Background()); err == nil || !strings.Contains(err.Error(), "CLOUDFLARE_API_TOKEN") {
		t.Fatalf("got %v", err)
	}
	f := &remote.Fake{}
	e = engine(t, yaml, f)
	e.Getenv = func(string) string { return "tok" }
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := uploaded(f, "traefik/cf.env"); got != "CF_DNS_API_TOKEN=tok\n" {
		t.Fatalf("cf.env: %q", got)
	}
	if f.Index("dnschallenge.provider=cloudflare") < 0 || f.Index("forwardedHeaders.trustedIPs") < 0 {
		t.Fatal("DNS challenge and trusted Cloudflare addresses are missing")
	}
}

func TestMaintenanceOnAndOff(t *testing.T) {
	f := healthy().
		On("cat '/opt/muken/apps/blog/state.json'", `{"release":"R","image":"i"}`, nil).
		On("docker ps -a", "muken-blog-web-R-1|R|web|running\n", nil)
	e := engine(t, cfgText, f)
	if err := e.Maintenance(context.Background(), true, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(uploaded(f, "dynamic/blog.yml"), "muken-maintenance") {
		t.Fatal("route must point at the maintenance container")
	}
	if err := e.Maintenance(context.Background(), false, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(uploaded(f, "dynamic/blog.yml"), "muken-blog-web-R-1") {
		t.Fatal("route must return to the release")
	}
}

func TestMaintenancePageIsUploadedAndMounted(t *testing.T) {
	f := healthy()
	e := engine(t, cfgText, f)
	if err := e.Maintenance(context.Background(), true, []byte("<h1>Custom</h1>")); err != nil {
		t.Fatal(err)
	}
	if got := uploaded(f, "/opt/muken/maintenance/maintenance.html"); got != "<h1>Custom</h1>" {
		t.Fatalf("page on the server = %q", got)
	}
	if f.Index("-v '/opt/muken/maintenance':/usr/share/nginx/html:ro") < 0 {
		t.Fatal("the container must mount the page folder")
	}
	if f.Index("return 503") < 0 {
		t.Fatal("the answer must stay HTTP 503")
	}
}

func TestMaintenanceDefaultPage(t *testing.T) {
	f := healthy()
	e := engine(t, cfgText, f)
	if err := e.Maintenance(context.Background(), true, nil); err != nil {
		t.Fatal(err)
	}
	if got := uploaded(f, "/opt/muken/maintenance/maintenance.html"); !strings.Contains(got, "Back soon") {
		t.Fatalf("default page = %q", got)
	}
}

// maintenanceOn makes `docker ps` report the maintenance container as running.
func maintenanceOn(f *remote.Fake) *remote.Fake {
	return f.On("docker ps -q --filter 'name=^muken-maintenance$'", "abc123\n", nil)
}

func TestDeployTurnsMaintenanceOff(t *testing.T) {
	f := maintenanceOn(healthy())
	e := engine(t, cfgText, f)
	result, err := e.Deploy(context.Background(), DeployOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.MaintenanceOn {
		t.Fatal("maintenance must be off after a normal deploy")
	}
	if !strings.Contains(uploaded(f, "dynamic/blog.yml"), "muken-blog-web-20261116T100000-1") {
		t.Fatal("traffic must move to the new release")
	}
	route, remove := f.Index("traefik/dynamic/blog.yml.tmp"), f.Index("docker rm -f muken-maintenance")
	if remove < 0 || remove < route {
		t.Fatalf("the maintenance container goes after the switch (route call %d, remove call %d)", route, remove)
	}
}

func TestDeployKeepsMaintenanceWhenAsked(t *testing.T) {
	f := maintenanceOn(healthy())
	e := engine(t, cfgText, f)
	result, err := e.Deploy(context.Background(), DeployOptions{KeepMaintenance: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.MaintenanceOn {
		t.Fatal("the result must say that maintenance is still on")
	}
	if got := uploaded(f, "dynamic/blog.yml"); got != "" {
		t.Fatalf("the route must not change, but a route was written:\n%s", got)
	}
	if f.Count("docker rm -f muken-maintenance") != 0 {
		t.Fatal("the maintenance container must stay")
	}
	if f.Index("'--name' 'muken-blog-web-20261116T100000-1'") < 0 || f.Index("blog/state.json.tmp") < 0 {
		t.Fatal("the new release must start and be saved as the current one")
	}
}

func TestKeepMaintenanceDoesNothingWhenMaintenanceIsOff(t *testing.T) {
	f := healthy()
	e := engine(t, cfgText, f)
	result, err := e.Deploy(context.Background(), DeployOptions{KeepMaintenance: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.MaintenanceOn || uploaded(f, "dynamic/blog.yml") == "" {
		t.Fatal("with maintenance off the deploy must switch traffic as usual")
	}
	if f.Count("docker rm -f muken-maintenance") != 0 {
		t.Fatal("there is no maintenance container to remove")
	}
}

func TestPlanMentionsMaintenance(t *testing.T) {
	cases := []struct {
		name string
		f    *remote.Fake
		opts DeployOptions
		want string
	}{
		{"on, default", maintenanceOn(healthy()), DeployOptions{}, "turn it off after the switch"},
		{"on, keep", maintenanceOn(healthy()), DeployOptions{KeepMaintenance: true}, "keep the maintenance page"},
	}
	for _, c := range cases {
		lines, err := engine(t, cfgText, c.f).Plan(context.Background(), c.opts)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(lines, "\n"), c.want) {
			t.Errorf("%s: want a line with %q in %v", c.name, c.want, lines)
		}
	}
	lines, _ := engine(t, cfgText, healthy()).Plan(context.Background(), DeployOptions{})
	if strings.Contains(strings.Join(lines, "\n"), "aintenance") {
		t.Errorf("maintenance off: the plan must not mention it, got %v", lines)
	}
}

func stderrOf(e *Engine) string { return e.UI.Err.(*bytes.Buffer).String() }

func TestSetupWarnsBeforeRestartingDocker(t *testing.T) {
	f := (&remote.Fake{}).On("[ ! -f /etc/docker/daemon.json ] && docker ps -q", "  3\n", nil)
	e := engine(t, cfgText, f)
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := stderrOf(e); !strings.Contains(got, "Docker restarts") || !strings.Contains(got, "3 container(s)") {
		t.Fatalf("want a Docker restart warning with the count, got %q", got)
	}
}

func TestSetupDoesNotWarnWithoutRunningContainers(t *testing.T) {
	e := engine(t, cfgText, &remote.Fake{})
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := stderrOf(e); got != "" {
		t.Fatalf("a new server needs no warning, got %q", got)
	}
}

func TestSetupWarnsWhenTheProxyWillRestart(t *testing.T) {
	f := (&remote.Fake{}).On("{{index .Config.Labels \"muken.hash\"}}' muken-traefik 2>/dev/null", "true oldhash\n", nil)
	e := engine(t, cfgText, f)
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := stderrOf(e); !strings.Contains(got, "proxy restarts") {
		t.Fatalf("want a proxy restart warning, got %q", got)
	}
}

func TestSetupDoesNotWarnWhenTheProxySettingsAreTheSame(t *testing.T) {
	probe := engine(t, cfgText, &remote.Fake{})
	_, hash, err := probe.traefikStep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	f := (&remote.Fake{}).On("{{index .Config.Labels \"muken.hash\"}}' muken-traefik 2>/dev/null", "true "+hash+"\n", nil)
	e := engine(t, cfgText, f)
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := stderrOf(e); got != "" {
		t.Fatalf("unchanged settings need no warning, got %q", got)
	}
}

func TestPrepareSkipsBaseStepsOnPreparedServer(t *testing.T) {
	f := &remote.Fake{}
	e := engine(t, cfgText, f)
	skipped, err := e.Prepare(context.Background())
	if err != nil || !skipped {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	for _, c := range f.Calls {
		if strings.Contains(c, "apt-get") || strings.Contains(c, "restart docker") || strings.Contains(c, "ufw") {
			t.Fatalf("a prepared server must not run the base steps: %s", c)
		}
	}
	if f.Index("muken-traefik --restart") < 0 {
		t.Fatal("the proxy step must still run, it applies proxy changes")
	}
}

func TestPrepareRunsEverythingOnNewServer(t *testing.T) {
	f := (&remote.Fake{}).On("docker info >/dev/null 2>&1 && docker network inspect", "", errors.New("missing"))
	e := engine(t, cfgText, f)
	skipped, err := e.Prepare(context.Background())
	if err != nil || skipped {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	if f.Index("apt-get install") < 0 || f.Index("ufw --force enable") < 0 {
		t.Fatal("a new server needs every base step")
	}
}

func TestProvisionRestartsDockerOnlyAfterNewSettings(t *testing.T) {
	f := &remote.Fake{}
	e := engine(t, cfgText, f)
	if err := e.Provision(context.Background()); err != nil {
		t.Fatal(err)
	}
	i := f.Index("get.docker.com")
	if i < 0 {
		t.Fatal("no Docker step")
	}
	c := f.Calls[i]
	if !strings.Contains(c, `[ -z "$new" ] || systemctl restart docker`) {
		t.Fatalf("restart must depend on a new daemon.json:\n%s", c)
	}
	pkg := f.Calls[f.Index("dpkg -s")]
	if strings.Index(pkg, "dpkg -s") > strings.Index(pkg, "apt-get") {
		t.Fatal("packages must be checked before apt runs")
	}
}

func TestHealthFailureKeepsLogsOutOfTheError(t *testing.T) {
	f := healthy().
		On("curl -fsS -m 3", "", errors.New("connection refused")).
		On("docker logs", "boot failed: no artisan", nil)
	e := engine(t, cfgText, f)
	_, err := e.Deploy(context.Background(), DeployOptions{})
	if err == nil || strings.Contains(err.Error(), "boot failed") {
		t.Fatalf("the error must stay short: %v", err)
	}
	if shown := e.UI.Out.(*bytes.Buffer).String(); !strings.Contains(shown, "| boot failed: no artisan") {
		t.Fatalf("the log lines must be printed before the error:\n%s", shown)
	}
}

func TestPlanWithoutDomainsDoesNotPrintAnEmptyName(t *testing.T) {
	yaml := "app: a\nimage: x\ntls: none\nserver: {host: 1.2.3.4}\n"
	lines, err := engine(t, yaml, healthy()).Plan(context.Background(), DeployOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "Switch traffic") && (strings.HasSuffix(l, "for ") || !strings.Contains(l, "no domain set")) {
			t.Fatalf("unclear plan line: %q", l)
		}
	}
}
