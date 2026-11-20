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

        foreach (['/', '/deploy', '/rollback', '/scale', '/maintenance', '/up', '/doctor', '/logs', '/config', '/history'] as $path) {
            $this->get($path)->assertOk();
        }
    }

    public function test_a_change_page_runs_nothing_until_the_user_confirms(): void
    {
        Http::fake(['*' => Http::response(['app' => 'shop', 'server' => 'x', 'output' => ''])]);

        $this->get('/rollback')->assertOk()->assertDontSee('Yes, run it');
        Http::assertNotSent(fn ($request) => $request->method() === 'POST');
    }
}
