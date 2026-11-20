<?php

namespace App\Services;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;

/**
 * Calls the muken API. Every method returns an array and never throws:
 * ['ok' => bool, 'status' => int, 'data' => array, 'error' => ?string, 'hint' => ?string].
 * status 0 means that muken serve does not answer.
 */
class MukenClient
{
    public function get(string $path, array $query = []): array
    {
        return $this->send('get', $path, $query);
    }

    public function post(string $path, array $body = []): array
    {
        return $this->send('post', $path, $body);
    }

    /** The version of muken serve, or null. Cached for a minute, and a failure is never cached. */
    public function version(): ?string
    {
        $cached = Cache::get('muken.version');
        if ($cached !== null) {
            return $cached;
        }
        $r = $this->get('version');
        $version = $r['ok'] ? ($r['data']['version'] ?? null) : null;
        if ($version !== null) {
            Cache::put('muken.version', $version, 60);
        }

        return $version;
    }

    private function send(string $method, string $path, array $payload): array
    {
        try {
            $response = Http::baseUrl(config('muken.api'))
                ->acceptJson()
                ->timeout(60)
                ->{$method}('/v1/'.ltrim($path, '/'), $payload);
        } catch (ConnectionException $e) {
            return [
                'ok' => false, 'status' => 0, 'data' => [],
                'error' => 'muken serve does not answer at '.config('muken.api'),
                'hint' => 'Run `s6-svstat /run/service/muken-api` or `muken serve` inside the container.',
            ];
        }

        $data = $response->json() ?? [];

        return [
            'ok' => $response->successful(),
            'status' => $response->status(),
            'data' => $data,
            'error' => $data['error']['message'] ?? ($response->successful() ? null : 'HTTP '.$response->status()),
            'hint' => $data['error']['hint'] ?? null,
        ];
    }
}
