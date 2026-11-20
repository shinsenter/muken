<?php

namespace App\Livewire;

use App\Services\HistoryStore;
use App\Services\MukenClient;
use Livewire\Attributes\Locked;
use Livewire\Attributes\Layout;
use Livewire\Component;

/** Deploy, rollback, scale, maintenance and up: a change that runs as a job. */
#[Layout('components.layouts.app')]
class Operate extends Component
{
    public const ACTIONS = [
        'deploy' => 'Deploy',
        'rollback' => 'Rollback',
        'scale' => 'Scale',
        'maintenance' => 'Maintenance',
        'up' => 'Set up the server',
    ];

    #[Locked]
    public string $action = 'deploy';

    public string $image = '';

    public string $scale = 'web=1';

    public string $maintenance = 'on'; // on or off

    public bool $confirming = false;

    public string $target = '';

    public string $plan = '';

    public ?string $jobId = null;

    public array $lines = [];

    public int $next = 0;

    public string $state = 'idle'; // idle, running, ok, failed

    public ?string $error = null;

    public ?string $hint = null;

    public ?int $historyId = null;

    public function mount(MukenClient $muken): void
    {
        $action = request()->segment(1);
        abort_unless(isset(self::ACTIONS[$action]), 404);
        $this->action = $action;

        $status = $muken->get('status');
        if ($status['ok']) {
            $this->target = ($status['data']['app'] ?? '?').' on '.($status['data']['server'] ?? '?');
        } else {
            $this->error = $status['error'];
            $this->hint = $status['hint'];
        }
        if ($action === 'deploy' && $status['ok']) {
            $plan = $muken->get('plan');
            $this->plan = $plan['ok'] ? ($plan['data']['output'] ?? '') : '';
        }
    }

    public function ask(): void
    {
        $this->error = null;
        $this->confirming = true;
    }

    public function cancel(): void
    {
        $this->confirming = false;
    }

    public function confirm(MukenClient $muken, HistoryStore $history): void
    {
        $this->confirming = false;
        $this->error = $this->hint = null;

        [$path, $body] = $this->request();
        if ($path === null) {
            return;
        }
        $r = $muken->post($path, $body);
        if ($r['status'] !== 202) {
            $this->error = $r['error'];
            $this->hint = $r['hint'];

            return;
        }
        $this->jobId = $r['data']['job_id'];
        $this->lines = [];
        $this->next = 0;
        $this->state = 'running';
        $this->historyId = $history->start($this->action, $body, $this->jobId);
    }

    /** Called every second by wire:poll while the job runs. */
    public function poll(MukenClient $muken, HistoryStore $history): void
    {
        if ($this->jobId === null || $this->state !== 'running') {
            return;
        }
        $r = $muken->get('jobs/'.$this->jobId, ['after' => $this->next]);
        if (! $r['ok']) {
            $this->error = $r['error'];
            $this->hint = $r['hint'];

            return;
        }
        $this->lines = array_merge($this->lines, $r['data']['lines'] ?? []);
        $this->next = $r['data']['next'] ?? $this->next;
        $this->state = $r['data']['state'] ?? 'running';
        if ($this->state !== 'running') {
            if ($this->state === 'failed') {
                $this->error = $r['data']['error'] ?: 'The job failed.';
            }
            $history->finish($this->historyId, $this->state, $this->lines);
        }
    }

    /** @return array{0: ?string, 1: array} */
    private function request(): array
    {
        switch ($this->action) {
            case 'deploy':
                return ['deploy', $this->image !== '' ? ['image' => trim($this->image)] : []];
            case 'rollback':
            case 'up':
                return [$this->action, []];
            case 'maintenance':
                return ['maintenance', ['on' => $this->maintenance === 'on']];
            case 'scale':
                $scale = [];
                foreach (preg_split('/\s+/', trim($this->scale)) as $pair) {
                    if (! preg_match('/^([a-z0-9][a-z0-9_-]*)=(\d{1,3})$/', $pair, $m)) {
                        $this->error = 'Write it like web=3 queue=2.';

                        return [null, []];
                    }
                    $scale[$m[1]] = (int) $m[2];
                }

                return ['scale', ['scale' => $scale]];
        }

        return [null, []];
    }

    public function render()
    {
        return view('livewire.operate', ['title' => self::ACTIONS[$this->action]]);
    }
}
