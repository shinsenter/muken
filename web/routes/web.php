<?php

use App\Livewire\ConfigPage;
use App\Livewire\Doctor;
use App\Livewire\History;
use App\Livewire\Logs;
use App\Livewire\Operate;
use App\Livewire\Setup;
use App\Livewire\Status;
use Illuminate\Support\Facades\Route;

// No login. Operate reads its action from the first segment of the path.
Route::get('/', Status::class);
foreach (Operate::ACTIONS as $action => $title) {
    Route::get('/'.$action, Operate::class);
}
Route::get('/setup', Setup::class);
Route::get('/doctor', Doctor::class);
Route::get('/logs', Logs::class);
Route::get('/config', ConfigPage::class);
Route::get('/history', History::class);
