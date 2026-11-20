<?php

namespace App\Livewire;

use App\Services\HistoryStore;
use Livewire\Component;

class History extends Component
{
    public function render(HistoryStore $history)
    {
        // recent() returns [] when the file is missing or broken, so this page never fails.
        return view('livewire.history', ['rows' => $history->recent()]);
    }
}
