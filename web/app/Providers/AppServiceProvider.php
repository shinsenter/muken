<?php

namespace App\Providers;

use App\Services\HistoryStore;
use App\Services\MukenClient;
use Illuminate\Support\Facades\View;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(HistoryStore::class);
    }

    public function boot(): void
    {
        // The footer shows the version of muken. A stopped API gives null, and the footer still shows.
        View::composer('components.layouts.app', function ($view) {
            $view->with('mukenVersion', $this->app->make(MukenClient::class)->version());
        });

        // Prepare the history file at boot. It returns false on a problem, and the app still starts.
        if (! $this->app->runningUnitTests()) {
            $this->app->make(HistoryStore::class)->ensure();
        }
    }
}
