<?php

namespace App\Livewire;

use App\Services\MukenClient;
use Livewire\Attributes\Layout;
use Livewire\Component;

#[Layout('components.layouts.app')]
class Logs extends Component
{
    public string $service = 'web';

    public int $tail = 100;

    public string $output = '';

    public ?string $error = null;

    public ?string $hint = null;

    public function fetch(MukenClient $muken): void
    {
        $this->validate([
            'service' => ['required', 'regex:/^[a-z0-9][a-z0-9_-]{0,63}$/'],
            'tail' => ['required', 'integer', 'between:1,2000'],
        ]);
        $r = $muken->get('logs', ['service' => $this->service, 'tail' => $this->tail]);
        $this->output = $r['data']['output'] ?? '';
        $this->error = $r['error'];
        $this->hint = $r['hint'];
    }

    public function render()
    {
        return view('livewire.logs');
    }
}
