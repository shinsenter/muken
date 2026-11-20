<?php

namespace App\Livewire;

use App\Services\MukenClient;
use Livewire\Attributes\Layout;
use Livewire\Component;

/**
 * The first setup: makes muken.yaml (muken init) and an SSH key. It only works while muken.yaml
 * does not exist, and the API never overwrites the file.
 */
#[Layout('components.layouts.app')]
class Setup extends Component
{
    public bool $exists = false;

    public array $presets = [];

    public string $preset = '';

    public string $app = '';

    public string $host = '';

    public string $domain = '';

    public string $email = '';

    public string $image = '';

    public string $php = '';

    public string $workers = '';

    public array $dbs = [];

    public bool $cloudflare = false;

    public bool $agent = false;

    public ?string $keyName = null;

    public ?string $publicKey = null;

    public ?string $error = null;

    public ?string $hint = null;

    public function mount(MukenClient $muken): void
    {
        $this->exists = $muken->get('config')['ok'];
        if ($this->exists) {
            return;
        }
        $this->presets = $muken->get('presets')['data']['presets'] ?? [];
        $key = $muken->get('server-key');
        if ($key['ok'] && isset($key['data']['public_key'])) {
            $this->keyName = $key['data']['name'] ?? 'id_ed25519';
            $this->publicKey = $key['data']['public_key'];
        }
    }

    public function makeKey(MukenClient $muken): void
    {
        $this->error = $this->hint = null;
        $r = $muken->post('server-key');
        if (! $r['ok']) {
            $this->error = $r['error'];
            $this->hint = $r['hint'];

            return;
        }
        $this->keyName = $r['data']['name'] ?? 'id_ed25519';
        $this->publicKey = $r['data']['public_key'] ?? null;
    }

    public function save(MukenClient $muken)
    {
        $this->error = $this->hint = null;
        $this->validate([
            'preset' => ['required'],
            'app' => ['required', 'regex:/^[a-z0-9][a-z0-9-]*$/'],
        ], [
            'app.regex' => 'Use lowercase letters, digits and "-".',
        ]);

        $body = array_filter([
            'preset' => $this->preset,
            'app' => $this->app,
            'host' => trim($this->host),
            'domain' => str_replace(' ', '', $this->domain),
            'email' => trim($this->email),
            'image' => trim($this->image),
            'php' => trim($this->php),
            'workers' => str_replace(' ', '', $this->workers),
            'db' => implode(',', $this->dbs),
            'ssh_key' => $this->keyName,
        ], fn ($v) => $v !== null && $v !== '');
        $body['cloudflare'] = $this->cloudflare;
        $body['agent'] = $this->agent;

        $r = $muken->post('init', $body);
        if ($r['status'] === 201) {
            return $this->redirect('/');
        }
        $this->error = $r['error'];
        $this->hint = $r['hint'];
    }

    public function render()
    {
        return view('livewire.setup', ['title' => 'Setup']);
    }
}
