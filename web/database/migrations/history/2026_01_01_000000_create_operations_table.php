<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    protected $connection = 'history';

    public function up(): void
    {
        Schema::connection('history')->create('operations', function (Blueprint $table) {
            $table->id();
            $table->string('action');
            $table->text('args')->nullable();
            $table->string('state'); // running, ok, failed
            $table->string('job_id')->nullable();
            $table->dateTime('started_at');
            $table->dateTime('finished_at')->nullable();
            $table->text('output_tail')->nullable();
        });
    }

    public function down(): void
    {
        Schema::connection('history')->dropIfExists('operations');
    }
};
