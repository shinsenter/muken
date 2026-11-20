<div>
    <h1 class="mb-4 text-xl font-semibold">Doctor</h1>
    <p class="mb-4 text-sm text-slate-600">Checks the config, SSH, Docker, DNS and disk. It changes nothing.</p>
    @include('livewire._error')
    <button type="button" wire:click="run" wire:loading.attr="disabled" class="rounded-md bg-lime-700 px-4 py-2 text-sm font-medium text-white hover:bg-lime-800">
        <span wire:loading.remove wire:target="run">Run the checks</span><span wire:loading wire:target="run">Checking...</span>
    </button>
    @if ($output)
        <pre class="mt-4 max-h-96 overflow-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ $output }}</pre>
    @endif
</div>
