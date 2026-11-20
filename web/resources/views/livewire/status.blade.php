<div wire:poll.10s="refresh">
    <h1 class="mb-4 text-xl font-semibold">Status</h1>
    @include('livewire._error')

    @if ($data)
        <dl class="grid grid-cols-1 gap-3 rounded-lg border border-slate-200 bg-white p-4 text-sm sm:grid-cols-2">
            <div><dt class="text-slate-500">App</dt><dd class="font-medium">{{ $data['app'] }}</dd></div>
            <div><dt class="text-slate-500">Server</dt><dd class="font-medium">{{ $data['server'] }}</dd></div>
            <div><dt class="text-slate-500">Release</dt><dd class="font-medium">{{ $data['release'] ?: 'none yet' }}</dd></div>
            <div><dt class="text-slate-500">Image</dt><dd class="break-all font-medium">{{ $data['image'] ?: '-' }}</dd></div>
            <div><dt class="text-slate-500">Proxy</dt>
                <dd class="font-medium {{ $data['proxy'] ? 'text-lime-700' : 'text-red-700' }}">{{ $data['proxy'] ? 'running' : 'not running' }}</dd></div>
            <div><dt class="text-slate-500">Maintenance</dt>
                <dd class="font-medium {{ $data['maintenance'] ? 'text-amber-700' : '' }}">{{ $data['maintenance'] ? 'on, visitors see the maintenance page' : 'off' }}</dd></div>
        </dl>
        @if ($data['rollback_note'])
            <p class="mt-2 text-sm text-slate-500">{{ $data['rollback_note'] }}</p>
        @endif

        <h2 class="mb-2 mt-6 font-semibold">Containers</h2>
        <div class="overflow-x-auto rounded-lg border border-slate-200 bg-white">
            <table class="min-w-full text-left text-sm">
                <thead class="bg-slate-100 text-slate-600">
                    <tr><th class="px-3 py-2">Container</th><th class="px-3 py-2">Role</th><th class="px-3 py-2">State</th><th class="px-3 py-2">Release</th></tr>
                </thead>
                <tbody class="divide-y divide-slate-100">
                    @forelse ($data['containers'] as $c)
                        <tr>
                            <td class="px-3 py-2">{{ $c['name'] }}</td>
                            <td class="px-3 py-2">{{ $c['role'] }}</td>
                            <td class="px-3 py-2">{{ $c['state'] }}</td>
                            <td class="px-3 py-2">{{ $c['release'] }}{{ $c['live'] ? ' (live)' : '' }}</td>
                        </tr>
                    @empty
                        <tr><td colspan="4" class="px-3 py-3 text-slate-500">No containers.</td></tr>
                    @endforelse
                </tbody>
            </table>
        </div>
    @endif
</div>
