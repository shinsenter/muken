<?php

namespace App\Livewire;

use App\Services\MukenClient;
use Livewire\Attributes\Url;
use Livewire\Component;

/** muken.yaml and the working folder of muken, read only. */
class ConfigPage extends Component
{
    #[Url]
    public string $path = '';

    public string $yaml = '';

    public array $entries = [];

    public ?string $content = null;

    public ?string $error = null;

    public ?string $hint = null;

    public function mount(MukenClient $muken): void
    {
        $config = $muken->get('config');
        $this->yaml = $config['data']['yaml'] ?? '';
        $this->open($muken, $this->path);
    }

    public function open(MukenClient $muken, string $path = ''): void
    {
        $this->path = $path;
        $this->error = $this->hint = null;
        $this->entries = [];
        $this->content = null;

        $r = $muken->get('files', ['path' => $path]);
        if (! $r['ok']) {
            $this->error = $r['error'];
            $this->hint = $r['hint'];

            return;
        }
        $this->entries = $r['data']['entries'] ?? [];
        $this->content = $r['data']['content'] ?? null;
    }

    public function parent(): string
    {
        return ltrim(dirname('/'.$this->path), '/');
    }

    public function render()
    {
        return view('livewire.config-page');
    }
}
