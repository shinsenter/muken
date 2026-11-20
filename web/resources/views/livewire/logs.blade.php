<div>
    <h1 class="mb-4 text-xl font-semibold">Logs</h1>
    @include('livewire._error')
    <form wire:submit="fetch" class="mb-4 flex flex-wrap items-end gap-3 text-sm">
        <label>Service<input type="text" wire:model="service" class="mt-1 block rounded-md border-slate-300 text-sm"></label>
        <label>Lines<input type="number" wire:model="tail" min="1" max="2000" class="mt-1 block w-28 rounded-md border-slate-300 text-sm"></label>
        <button type="submit" class="rounded-md bg-lime-700 px-4 py-2 font-medium text-white hover:bg-lime-800">Show</button>
    </form>
    @error('service') <p class="mb-2 text-sm text-red-700">{{ $message }}</p> @enderror
    @error('tail') <p class="mb-2 text-sm text-red-700">{{ $message }}</p> @enderror
    @if ($output !== '')
        <pre class="max-h-[32rem] overflow-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ $output }}</pre>
    @endif
</div>
