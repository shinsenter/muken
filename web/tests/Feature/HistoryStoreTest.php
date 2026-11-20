<?php

namespace Tests\Feature;

use App\Services\HistoryStore;
use Tests\TestCase;

class HistoryStoreTest extends TestCase
{
    private string $dir;

    protected function setUp(): void
    {
        parent::setUp();
        $this->dir = sys_get_temp_dir().'/muken-history-'.uniqid();
        config(['muken.history_db' => $this->dir.'/.muken/history.sqlite']);
    }

    protected function tearDown(): void
    {
        foreach (glob($this->dir.'/.muken/*') ?: [] as $file) {
            @unlink($file);
        }
        @rmdir($this->dir.'/.muken');
        @rmdir($this->dir);
        parent::tearDown();
    }

    public function test_a_missing_file_is_created_with_an_empty_history(): void
    {
        $store = new HistoryStore();

        $this->assertSame([], $store->recent());
        $this->assertFileExists(config('muken.history_db'));
    }

    public function test_an_operation_is_recorded_and_finished(): void
    {
        $store = new HistoryStore();

        $id = $store->start('deploy', ['image' => 'x'], 'j1');
        $store->finish($id, 'ok', ['one', 'two']);

        $rows = $store->recent();
        $this->assertCount(1, $rows);
        $this->assertSame('ok', $rows[0]->state);
        $this->assertSame("one\ntwo", $rows[0]->output_tail);
    }

    public function test_deleting_the_file_while_running_gives_an_empty_history(): void
    {
        $store = new HistoryStore();
        $store->finish($store->start('deploy', [], 'j1'), 'ok', []);

        unlink(config('muken.history_db'));

        $this->assertSame([], $store->recent());
        $this->assertNotNull($store->start('rollback', [], 'j2'));
    }

    public function test_a_file_that_is_not_sqlite_is_renamed_and_replaced(): void
    {
        mkdir($this->dir.'/.muken', 0775, true);
        file_put_contents(config('muken.history_db'), 'this is not a database');

        $store = new HistoryStore();

        $this->assertSame([], $store->recent());
        $this->assertNotEmpty(glob($this->dir.'/.muken/history.sqlite.bad-*'));
    }

    public function test_an_unwritable_location_never_throws(): void
    {
        config(['muken.history_db' => '/proc/muken-no/history.sqlite']);
        $store = new HistoryStore();

        $this->assertSame([], $store->recent());
        $this->assertNull($store->start('deploy', [], 'j1'));
        $store->finish(null, 'ok', []);
    }
}
