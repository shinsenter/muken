// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package catalog lists the images of this project and the settings that suit each one.
package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type WorkerTemplate struct {
	Name    string
	Title   string
	Command string
}

type Preset struct {
	Name       string
	Title      string
	Kind       string // "server" for the plain PHP images, "app" for the webapp images
	Image      string // for servers, a format with one %s for the PHP version
	HealthPath string
	Stateful   bool // keep /var/www/html in a volume
	Env        map[string]string
	Migrate    string
	Workers    []WorkerTemplate
	Composer   []string // composer packages that identify the app
	Files      []string // files or folders that identify the app
}

const (
	healthWeb = "/readyz"
	healthRR  = "/health?plugin=http"
	appDir    = `cd "${APP_PATH:-/var/www/html}" && `
)

var PHPVersions = []string{"8.6", "8.5", "8.4", "8.3", "8.2"}

// PHPReleaseCandidate is the version in PHPVersions that is not released yet.
// Its image tag has no suffix, so only the question in `muken init` marks it.
// Set it to "" when the version is released.
const PHPReleaseCandidate = "8.6"

const DefaultPHP = "8.4"

var laravelEnv = map[string]string{"APP_ENV": "production", "APP_DEBUG": "false"}

const laravelMigrate = appDir + "php artisan migrate --force"

var laravelWorkers = []WorkerTemplate{
	{"queue", "Queue worker", appDir + "exec php artisan queue:work --tries=3 --max-time=3600"},
	{"scheduler", "Scheduler", appDir + "exec php artisan schedule:work"},
}

var symfonyEnv = map[string]string{"APP_ENV": "prod"}

const symfonyMigrate = appDir + "php bin/console doctrine:migrations:migrate --no-interaction"

var symfonyWorkers = []WorkerTemplate{
	{"messenger", "Messenger consumer", appDir + "exec php bin/console messenger:consume async --time-limit=3600"},
}

// presets is read only. The Env maps and Workers slices of several presets are
// the same value, so never change them in place.
var presets = []Preset{
	{Name: "php-nginx", Title: "PHP-FPM with Nginx", Kind: "server", Image: "shinsenter/php:%s-fpm-nginx", HealthPath: healthWeb},
	{Name: "php-apache", Title: "PHP-FPM with Apache", Kind: "server", Image: "shinsenter/php:%s-fpm-apache", HealthPath: healthWeb},
	{Name: "php-roadrunner", Title: "RoadRunner app server", Kind: "server", Image: "shinsenter/php:%s-roadrunner", HealthPath: healthRR},
	{Name: "php-frankenphp", Title: "FrankenPHP app server (beta)", Kind: "server", Image: "shinsenter/php:%s-frankenphp", HealthPath: healthWeb},

	{Name: "laravel", Title: "Laravel", Kind: "app", Image: "shinsenter/laravel:latest", HealthPath: healthWeb,
		Env: laravelEnv, Migrate: laravelMigrate, Workers: laravelWorkers, Composer: []string{"laravel/framework"}, Files: []string{"artisan"}},
	{Name: "symfony", Title: "Symfony", Kind: "app", Image: "shinsenter/symfony:latest", HealthPath: healthWeb,
		Env: symfonyEnv, Migrate: symfonyMigrate, Workers: symfonyWorkers, Composer: []string{"symfony/framework-bundle"}, Files: []string{"bin/console"}},
	{Name: "statamic", Title: "Statamic", Kind: "app", Image: "shinsenter/statamic:latest", HealthPath: healthWeb, Stateful: true,
		Env: laravelEnv, Migrate: laravelMigrate, Workers: laravelWorkers, Composer: []string{"statamic/cms"}},
	{Name: "bagisto", Title: "Bagisto", Kind: "app", Image: "shinsenter/bagisto:latest", HealthPath: healthWeb, Stateful: true,
		Env: laravelEnv, Migrate: laravelMigrate, Workers: laravelWorkers, Composer: []string{"bagisto/bagisto"}},
	{Name: "coolify", Title: "Coolify", Kind: "app", Image: "shinsenter/coolify:latest", HealthPath: healthWeb, Stateful: true,
		Env: laravelEnv, Migrate: laravelMigrate, Workers: laravelWorkers},
	{Name: "invoiceshelf", Title: "InvoiceShelf", Kind: "app", Image: "shinsenter/invoiceshelf:latest", HealthPath: healthWeb, Stateful: true,
		Env: laravelEnv, Migrate: laravelMigrate, Workers: laravelWorkers},
	{Name: "sulu", Title: "Sulu", Kind: "app", Image: "shinsenter/sulu:latest", HealthPath: healthWeb, Stateful: true,
		Env: symfonyEnv, Migrate: symfonyMigrate, Workers: symfonyWorkers, Composer: []string{"sulu/sulu"}},
	{Name: "wordpress", Title: "WordPress", Kind: "app", Image: "shinsenter/wordpress:latest", HealthPath: healthWeb, Stateful: true,
		Files: []string{"wp-config.php", "wp-content"}},
	{Name: "bedrock", Title: "Bedrock (WordPress)", Kind: "app", Image: "shinsenter/bedrock:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"roots/bedrock", "roots/wordpress"}},
	{Name: "drupal", Title: "Drupal", Kind: "app", Image: "shinsenter/drupal:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"drupal/core"}},
	{Name: "joomla", Title: "Joomla", Kind: "app", Image: "shinsenter/joomla:latest", HealthPath: healthWeb, Stateful: true},
	{Name: "craftcms", Title: "Craft CMS", Kind: "app", Image: "shinsenter/craftcms:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"craftcms/cms"}, Files: []string{"craft"}},
	{Name: "grav", Title: "Grav", Kind: "app", Image: "shinsenter/grav:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"getgrav/grav"}},
	{Name: "kirby", Title: "Kirby", Kind: "app", Image: "shinsenter/kirby:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"getkirby/cms"}},
	{Name: "flarum", Title: "Flarum", Kind: "app", Image: "shinsenter/flarum:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"flarum/core"}},
	{Name: "nextcloud", Title: "Nextcloud", Kind: "app", Image: "shinsenter/nextcloud:latest", HealthPath: healthWeb, Stateful: true},
	{Name: "matomo", Title: "Matomo", Kind: "app", Image: "shinsenter/matomo:latest", HealthPath: healthWeb, Stateful: true},
	{Name: "mautic", Title: "Mautic", Kind: "app", Image: "shinsenter/mautic:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"mautic/core-lib"}},
	{Name: "espocrm", Title: "EspoCRM", Kind: "app", Image: "shinsenter/espocrm:latest", HealthPath: healthWeb, Stateful: true},
	{Name: "prestashop", Title: "PrestaShop", Kind: "app", Image: "shinsenter/prestashop:latest", HealthPath: healthWeb, Stateful: true},
	{Name: "magento", Title: "Magento", Kind: "app", Image: "shinsenter/magento:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"magento/product-community-edition"}},
	{Name: "shopware", Title: "Shopware", Kind: "app", Image: "shinsenter/shopware:latest", HealthPath: healthWeb, Stateful: true,
		Composer: []string{"shopware/core", "shopware/production"}},
	{Name: "phpmyadmin", Title: "phpMyAdmin", Kind: "app", Image: "shinsenter/phpmyadmin:latest", HealthPath: healthWeb},
	{Name: "cakephp5", Title: "CakePHP 5", Kind: "app", Image: "shinsenter/cakephp5:latest", HealthPath: healthWeb,
		Composer: []string{"cakephp/cakephp"}},
	{Name: "codeigniter4", Title: "CodeIgniter 4", Kind: "app", Image: "shinsenter/codeigniter4:latest", HealthPath: healthWeb,
		Composer: []string{"codeigniter4/framework"}},
	{Name: "yii", Title: "Yii", Kind: "app", Image: "shinsenter/yii:latest", HealthPath: healthWeb,
		Composer: []string{"yiisoft/yii2"}},
	{Name: "flightphp", Title: "Flight", Kind: "app", Image: "shinsenter/flightphp:latest", HealthPath: healthWeb,
		Composer: []string{"flightphp/core"}},
	{Name: "hyperf", Title: "Hyperf", Kind: "app", Image: "shinsenter/hyperf:latest", HealthPath: healthWeb,
		Composer: []string{"hyperf/hyperf"}},
	{Name: "tempest", Title: "Tempest", Kind: "app", Image: "shinsenter/tempest:latest", HealthPath: healthWeb,
		Composer: []string{"tempest/framework"}},
	{Name: "spiral", Title: "Spiral", Kind: "app", Image: "shinsenter/spiral:latest", HealthPath: healthRR,
		Composer: []string{"spiral/framework"}},
}

func All() []Preset { return append([]Preset(nil), presets...) }

func Names() []string {
	var names []string
	for _, preset := range presets {
		names = append(names, preset.Name)
	}
	sort.Strings(names)
	return names
}

func Get(name string) (Preset, bool) {
	for _, preset := range presets {
		if preset.Name == name {
			return preset, true
		}
	}
	return Preset{}, false
}

// ImageFor returns the full image reference. phpVersion only matters for servers.
func (preset Preset) ImageFor(phpVersion string) string {
	if preset.Kind == "server" {
		if phpVersion == "" {
			phpVersion = DefaultPHP
		}
		return strings.Replace(preset.Image, "%s", phpVersion, 1)
	}
	return preset.Image
}

// Detect guesses the preset from the files in dir. It returns "" when unsure.
// Packages in composer.json win over file names, because they are more exact.
func Detect(dir string) string {
	required := composerPackages(dir)
	for _, preset := range presets {
		for _, pkg := range preset.Composer {
			if required[pkg] {
				return preset.Name
			}
		}
	}
	for _, preset := range presets {
		for _, file := range preset.Files {
			if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
				return preset.Name
			}
		}
	}
	return ""
}

// composerPackages returns the names in require and require-dev. A missing or
// broken composer.json gives an empty set.
func composerPackages(dir string) map[string]bool {
	found := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(dir, "composer.json"))
	if err != nil {
		return found
	}
	var manifest struct {
		Require    map[string]json.RawMessage `json:"require"`
		RequireDev map[string]json.RawMessage `json:"require-dev"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return found
	}
	for name := range manifest.Require {
		found[name] = true
	}
	for name := range manifest.RequireDev {
		found[name] = true
	}
	return found
}
