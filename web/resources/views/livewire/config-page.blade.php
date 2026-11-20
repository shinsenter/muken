<div>
    <h1 class="mb-4 text-xl font-semibold">Config</h1>
    <p class="mb-4 text-sm text-slate-600">Read only. Values that look like secrets are hidden. Edit the files on the host.</p>

    @if ($yaml)
        <h2 class="mb-1 text-sm font-semibold">muken.yaml</h2>
        <pre class="mb-6 max-h-80 overflow-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ $yaml }}</pre>
    @endif

    <h2 class="mb-1 text-sm font-semibold">Working folder: /{{ $path }}</h2>
    @include('livewire._error')
    @if ($content !== null)
        <button type="button" wire:click="open(@js($this->parent()))" class="mb-2 text-sm text-lime-700 hover:underline">&larr; Back</button>
        <pre class="max-h-96 overflow-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ $content }}</pre>
    @else
        <ul class="divide-y divide-slate-100 rounded-lg border border-slate-200 bg-white text-sm">
            @if ($path !== '')
                <li><button type="button" wire:click="open(@js($this->parent()))" class="w-full px-3 py-2 text-left text-lime-700 hover:bg-slate-50">..</button></li>
            @endif
            @forelse ($entries as $e)
                <li>
                    <button type="button" wire:click="open(@js(trim($path.'/'.$e['name'], '/')))" class="flex w-full justify-between px-3 py-2 text-left hover:bg-slate-50">
                        <span>{{ $e['name'] }}{{ $e['dir'] ? '/' : '' }}</span>
                        <span class="text-slate-400">{{ $e['dir'] ? '' : number_format($e['size']).' B' }}</span>
                    </button>
                </li>
            @empty
                <li class="px-3 py-3 text-slate-500">Empty.</li>
            @endforelse
        </ul>
    @endif
</div>
