<div @if ($state === 'running') wire:poll.1s="poll" @endif>
    <h1 class="mb-1 text-xl font-semibold">{{ $title }}</h1>
    @if ($target)
        <p class="mb-4 text-sm text-slate-500">{{ $target }}</p>
    @endif
    @include('livewire._error')

    @if ($action === 'deploy')
        @if ($plan)
            <h2 class="mb-1 text-sm font-semibold">Plan</h2>
            <pre class="mb-4 overflow-x-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ $plan }}</pre>
        @endif
        <label class="mb-4 block text-sm">
            <span class="text-slate-600">Image (optional, empty uses muken.yaml)</span>
            <input type="text" wire:model="image" class="mt-1 block w-full rounded-md border-slate-300 text-sm" placeholder="example/app:1.2.3">
        </label>
    @elseif ($action === 'scale')
        <label class="mb-4 block text-sm">
            <span class="text-slate-600">Replicas, for example web=3 queue=2</span>
            <input type="text" wire:model="scale" class="mt-1 block w-full rounded-md border-slate-300 text-sm">
        </label>
    @elseif ($action === 'maintenance')
        <fieldset class="mb-4 flex gap-6 text-sm">
            <label><input type="radio" wire:model="maintenance" value="on"> Turn on</label>
            <label><input type="radio" wire:model="maintenance" value="off"> Turn off</label>
        </fieldset>
    @elseif ($action === 'rollback')
        <p class="mb-4 text-sm text-slate-600">Return to the previous release.</p>
    @elseif ($action === 'up')
        <p class="mb-4 text-sm text-slate-600">First time on a server: prepare it, set DNS and deploy. It is safe to repeat.</p>
    @endif

    @if ($state !== 'running')
        <button type="button" wire:click="ask" class="rounded-md bg-lime-700 px-4 py-2 text-sm font-medium text-white hover:bg-lime-800">{{ $title }}</button>
    @endif

    @if ($confirming)
        <div class="mt-4 rounded-md border border-amber-300 bg-amber-50 p-4 text-sm" role="alertdialog">
            <p class="font-medium">Run "{{ $title }}" on {{ $target ?: 'this server' }}?</p>
            <p class="mt-1 text-slate-600">This changes the live server. There is no login on this page.</p>
            <div class="mt-3 flex gap-3">
                <button type="button" wire:click="confirm" class="rounded-md bg-red-700 px-3 py-1.5 font-medium text-white hover:bg-red-800">Yes, run it</button>
                <button type="button" wire:click="cancel" class="rounded-md border border-slate-300 bg-white px-3 py-1.5">Cancel</button>
            </div>
        </div>
    @endif

    @if ($state !== 'idle')
        <h2 class="mb-1 mt-6 text-sm font-semibold">
            Output
            <span class="ml-2 rounded px-1.5 py-0.5 text-xs
                {{ $state === 'ok' ? 'bg-lime-100 text-lime-800' : ($state === 'failed' ? 'bg-red-100 text-red-800' : 'bg-slate-200 text-slate-700') }}">{{ $state }}</span>
        </h2>
        <pre class="max-h-96 overflow-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ implode("\n", $lines) }}</pre>
    @endif
</div>
