<?php

namespace App\Livewire;

use App\Services\MukenClient;
use Livewire\Attributes\Layout;
use Livewire\Component;

#[Layout('components.layouts.app')]
class Status extends Component
{
    public array $data = [];

    public ?string $error = null;

    public ?string $hint = null;

    public bool $noConfig = false;

    public function mount(MukenClient $muken)
    {
        $this->refresh($muken);

        // No muken.yaml yet: the first visit goes to the setup form.
        if ($this->noConfig) {
            return $this->redirect('/setup');
        }
    }

    public function refresh(MukenClient $muken): void
    {
        $r = $muken->get('status');
        $this->data = $r['ok'] ? $r['data'] : [];
        $this->noConfig = ($r['data']['error']['code'] ?? null) === 'no_config';
        $this->error = $r['error'];
        $this->hint = $r['hint'];
    }

    public function render()
    {
        return view('livewire.status');
    }
}
