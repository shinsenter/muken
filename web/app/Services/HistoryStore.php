<?php

namespace App\Services;

use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Facades\Schema;
use Throwable;

/**
 * The history of operations, in one SQLite file. This is the only database use of the app.
 *
 * A missing file is created and migrated again, so the history starts empty. A file that is not
 * SQLite is renamed and replaced. Every public method catches Throwable and returns an empty
 * value: a database problem must never break a page or an operation.
 */
class HistoryStore
{
    private const CONNECTION = 'history';

    private const MAGIC = "SQLite format 3\0";

    public function path(): string
    {
        return (string) config('muken.history_db');
    }

    /** Create the folder and the file and run the migration. Returns false when it cannot. */
    public function ensure(): bool
    {
        try {
            $path = $this->path();
            $this->configure($path);

            if (is_file($path) && filesize($path) > 0 && ! $this->isSqlite($path)) {
                rename($path, $path.'.bad-'.time());
                DB::purge(self::CONNECTION);
            }
            if (! is_dir(dirname($path))) {
                mkdir(dirname($path), 0775, true);
            }
            if (! is_file($path)) {
                DB::purge(self::CONNECTION);
                touch($path);
            }
            if (! Schema::connection(self::CONNECTION)->hasTable('operations')) {
                Artisan::call('migrate', [
                    '--database' => self::CONNECTION,
                    '--path' => 'database/migrations/history',
                    '--force' => true,
                ]);
            }

            return true;
        } catch (Throwable $e) {
            Log::warning('history: cannot prepare the database: '.$e->getMessage());

            return false;
        }
    }

    /** Write a row when an operation starts. Returns its id, or null when the write fails. */
    public function start(string $action, array $args, ?string $jobId): ?int
    {
        return $this->guard(function () use ($action, $args, $jobId) {
            return $this->table()->insertGetId([
                'action' => $action,
                'args' => json_encode($args),
                'state' => 'running',
                'job_id' => $jobId,
                'started_at' => now()->toDateTimeString(),
            ]);
        }, null);
    }

    /** Update the row when the operation ends. Keeps the last 200 lines. */
    public function finish(?int $id, string $state, array $lines): void
    {
        if ($id === null) {
            return;
        }
        $this->guard(function () use ($id, $state, $lines) {
            $this->table()->where('id', $id)->update([
                'state' => $state,
                'finished_at' => now()->toDateTimeString(),
                'output_tail' => implode("\n", array_slice($lines, -200)),
            ]);
        }, null);
    }

    /** @return array<int, object> */
    public function recent(int $limit = 50): array
    {
        return $this->guard(
            fn () => $this->table()->orderByDesc('id')->limit($limit)->get()->all(),
            []
        );
    }

    private function guard(callable $work, mixed $fallback): mixed
    {
        try {
            if (! $this->ensure()) {
                return $fallback;
            }

            return $work();
        } catch (Throwable $e) {
            Log::warning('history: '.$e->getMessage());

            return $fallback;
        }
    }

    private function table()
    {
        return DB::connection(self::CONNECTION)->table('operations');
    }

    private function configure(string $path): void
    {
        config(['database.connections.'.self::CONNECTION => [
            'driver' => 'sqlite',
            'database' => $path,
            'prefix' => '',
            'foreign_key_constraints' => false,
        ]]);
    }

    private function isSqlite(string $path): bool
    {
        $handle = fopen($path, 'rb');
        if ($handle === false) {
            return false;
        }
        $head = fread($handle, 16);
        fclose($handle);

        return $head === self::MAGIC;
    }
}
