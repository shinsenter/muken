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

    public function mount(MukenClient $muken): void
    {
        $this->refresh($muken);
    }

    public function refresh(MukenClient $muken): void
    {
        $r = $muken->get('status');
        $this->data = $r['ok'] ? $r['data'] : [];
        $this->error = $r['error'];
        $this->hint = $r['hint'];
    }

    public function render()
    {
        return view('livewire.status');
    }
}
