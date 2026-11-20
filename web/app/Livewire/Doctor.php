<?php

namespace App\Livewire;

use App\Services\MukenClient;
use Livewire\Attributes\Layout;
use Livewire\Component;

#[Layout('components.layouts.app')]
class Doctor extends Component
{
    public string $output = '';

    public ?string $error = null;

    public ?string $hint = null;

    public function run(MukenClient $muken): void
    {
        $r = $muken->get('doctor');
        $this->output = $r['data']['output'] ?? '';
        $this->error = $r['error'];
        $this->hint = $r['hint'];
    }

    public function render()
    {
        return view('livewire.doctor');
    }
}
