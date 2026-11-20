<?php

namespace Tests\Feature;

use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class PagesTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        config(['muken.history_db' => sys_get_temp_dir().'/muken-pages-'.uniqid().'/h.sqlite']);
    }

    public function test_the_status_page_needs_no_login(): void
    {
        Http::fake(['*/v1/version' => Http::response(['version' => 'v9.9.9']), '*/v1/status' => Http::response([
            'app' => 'shop', 'server' => '203.0.113.7', 'release' => 'r1', 'image' => 'example/shop:1',
            'proxy' => true, 'maintenance' => false, 'rollback_note' => '', 'containers' => [],
        ])]);

        $this->get('/')->assertOk()->assertSee('shop')->assertSee('r1')
            ->assertSee('v9.9.9')->assertSee('Copyright')->assertSee('Mai Nhut Tan');
    }

    public function test_a_stopped_api_shows_a_message_and_keeps_the_menu(): void
    {
        Http::fake(fn () => throw new \Illuminate\Http\Client\ConnectionException('refused'));

        $this->get('/')->assertOk()->assertSee('muken serve does not answer')->assertSee('History')
            ->assertSee('Copyright', false);
    }

    public function test_every_page_opens_without_the_database(): void
    {
        Http::fake(['*' => Http::response(['error' => ['code' => 'no_config', 'message' => 'no muken.yaml', 'hint' => 'run muken init']], 409)]);

        foreach (['/deploy', '/rollback', '/scale', '/maintenance', '/up', '/doctor', '/logs', '/config', '/history', '/setup'] as $path) {
            $this->get($path)->assertOk();
        }
        // The first visit, with no muken.yaml, goes to the setup form.
        $this->get('/')->assertRedirect('/setup');
    }

    public function test_the_setup_page_lists_the_presets_when_there_is_no_config(): void
    {
        Http::fake([
            '*/v1/config' => Http::response(['error' => ['code' => 'no_config', 'message' => 'x', 'hint' => '']], 409),
            '*/v1/presets' => Http::response(['presets' => [['name' => 'laravel', 'title' => 'Laravel', 'kind' => 'app']]]),
            '*/v1/server-key' => Http::response(['error' => ['code' => 'not_found', 'message' => 'no', 'hint' => '']], 404),
            '*' => Http::response([]),
        ]);

        $this->get('/setup')->assertOk()->assertSee('Laravel')->assertSee('Make muken.yaml')->assertSee('Make an SSH key');
    }

    public function test_the_setup_page_never_offers_a_form_when_the_config_exists(): void
    {
        Http::fake(['*/v1/config' => Http::response(['path' => '/work/muken.yaml', 'yaml' => 'app: x']), '*' => Http::response([])]);

        $this->get('/setup')->assertOk()->assertSee('already exists')->assertDontSee('Make muken.yaml');
    }

    public function test_the_setup_form_sends_init_and_goes_to_status(): void
    {
        Http::fake([
            '*/v1/config' => Http::response([], 409),
            '*/v1/presets' => Http::response(['presets' => []]),
            '*/v1/server-key' => Http::response(['name' => 'id_ed25519', 'path' => '/work/.muken/.ssh/id_ed25519', 'public_key' => 'ssh-ed25519 AAAA muken']),
            '*/v1/init' => Http::response(['output' => 'ok'], 201),
        ]);

        \Livewire\Livewire::test(\App\Livewire\Setup::class)
            ->assertSet('publicKey', 'ssh-ed25519 AAAA muken')
            ->set('preset', 'laravel')->set('app', 'shop')->set('host', '203.0.113.7')
            ->set('domain', 'a.example.com, b.example.com')->set('dbs', ['mariadb', 'redis'])
            ->call('save')
            ->assertRedirect('/');

        Http::assertSent(function ($request) {
            return str_ends_with($request->url(), '/v1/init')
                && $request['preset'] === 'laravel' && $request['app'] === 'shop'
                && $request['domain'] === 'a.example.com,b.example.com'
                && $request['db'] === 'mariadb,redis' && $request['ssh_key'] === 'id_ed25519'
                && $request['cloudflare'] === false;
        });
    }

    public function test_the_setup_form_checks_the_name_before_it_calls_the_api(): void
    {
        Http::fake(['*/v1/config' => Http::response([], 409), '*' => Http::response([])]);

        \Livewire\Livewire::test(\App\Livewire\Setup::class)
            ->set('preset', 'laravel')->set('app', 'Bad Name')
            ->call('save')
            ->assertHasErrors(['app']);

        Http::assertNotSent(fn ($request) => str_ends_with($request->url(), '/v1/init'));
    }

    public function test_a_change_page_runs_nothing_until_the_user_confirms(): void
    {
        Http::fake(['*' => Http::response(['app' => 'shop', 'server' => 'x', 'output' => ''])]);

        $this->get('/rollback')->assertOk()->assertDontSee('Yes, run it');
        Http::assertNotSent(fn ($request) => $request->method() === 'POST');
    }
}
