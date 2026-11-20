<?php

namespace App\Providers;

use App\Services\HistoryStore;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(HistoryStore::class);
    }

    public function boot(): void
    {
        // Prepare the history file at boot. It returns false on a problem, and the app still starts.
        if (! $this->app->runningUnitTests()) {
            $this->app->make(HistoryStore::class)->ensure();
        }
    }
}
