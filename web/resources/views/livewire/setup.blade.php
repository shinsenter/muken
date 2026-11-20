<div>
    <h1 class="mb-1 text-xl font-semibold">Setup</h1>

    @if ($exists)
        <p class="mt-2 rounded-md border border-slate-200 bg-white p-4 text-sm">
            muken.yaml already exists. This page never overwrites it: edit the file on the host.
            <a href="/" class="text-lime-700 hover:underline">Back to Status</a>
        </p>
    @else
        <p class="mb-4 text-sm text-slate-600">There is no muken.yaml in the working folder yet. Fill in this form to make it. It does the same as <code>muken init</code>.</p>
        @include('livewire._error')

        <form wire:submit="save" class="space-y-4 text-sm">
            <label class="block">
                <span class="text-slate-600">What do you deploy?</span>
                <select wire:model="preset" class="mt-1 block w-full rounded-md border-slate-300 text-sm">
                    <option value="">Choose one</option>
                    @foreach (collect($presets)->groupBy('kind') as $kind => $items)
                        <optgroup label="{{ $kind === 'server' ? 'PHP images (any PHP app)' : 'Apps and frameworks' }}">
                            @foreach ($items as $item)
                                <option value="{{ $item['name'] }}">{{ $item['title'] }}</option>
                            @endforeach
                        </optgroup>
                    @endforeach
                </select>
                @error('preset') <span class="text-red-700">Choose one.</span> @enderror
            </label>

            <label class="block">
                <span class="text-slate-600">App name</span>
                <input type="text" wire:model="app" class="mt-1 block w-full rounded-md border-slate-300 text-sm" placeholder="shop">
                @error('app') <span class="text-red-700">{{ $message }}</span> @enderror
            </label>

            <div class="grid gap-4 sm:grid-cols-2">
                <label class="block"><span class="text-slate-600">Server IP or host (empty: later)</span>
                    <input type="text" wire:model="host" class="mt-1 block w-full rounded-md border-slate-300 text-sm" placeholder="203.0.113.7"></label>
                <label class="block"><span class="text-slate-600">Domains, comma separated (empty: HTTP on the IP)</span>
                    <input type="text" wire:model="domain" class="mt-1 block w-full rounded-md border-slate-300 text-sm" placeholder="example.com"></label>
                <label class="block"><span class="text-slate-600">Email for certificate notices</span>
                    <input type="text" wire:model="email" class="mt-1 block w-full rounded-md border-slate-300 text-sm"></label>
                <label class="block"><span class="text-slate-600">Your own image (optional)</span>
                    <input type="text" wire:model="image" class="mt-1 block w-full rounded-md border-slate-300 text-sm" placeholder="ghcr.io/me/shop:1.0.0"></label>
                <label class="block"><span class="text-slate-600">PHP version (plain PHP images)</span>
                    <input type="text" wire:model="php" class="mt-1 block w-full rounded-md border-slate-300 text-sm" placeholder="8.4"></label>
                <label class="block"><span class="text-slate-600">Workers from the preset, for example queue,scheduler</span>
                    <input type="text" wire:model="workers" class="mt-1 block w-full rounded-md border-slate-300 text-sm"></label>
            </div>

            <fieldset class="flex flex-wrap gap-5">
                <legend class="mb-1 text-slate-600">Databases</legend>
                @foreach (['postgres', 'mysql', 'mariadb', 'redis'] as $db)
                    <label><input type="checkbox" wire:model="dbs" value="{{ $db }}"> {{ $db }}</label>
                @endforeach
            </fieldset>
            <div class="flex flex-wrap gap-6">
                <label><input type="checkbox" wire:model="cloudflare"> The domain is on Cloudflare</label>
                <label><input type="checkbox" wire:model="agent"> Install the monitoring page (agent)</label>
            </div>

            <div class="rounded-md border border-slate-200 bg-white p-4">
                <h2 class="mb-1 font-semibold">SSH key for the server</h2>
                @if ($publicKey)
                    <p class="mb-2 text-slate-600">Add this public key to the server (<code>~/.ssh/authorized_keys</code>, or the SSH keys of your provider). muken.yaml will use the key <code>{{ $keyName }}</code>.</p>
                    <pre class="overflow-x-auto rounded-md bg-slate-900 p-3 text-xs text-slate-100">{{ $publicKey }}</pre>
                @else
                    <p class="mb-2 text-slate-600">Make a key that muken uses to log in to the server. The private key stays in the working folder, in <code>.muken/.ssh</code>, and is never shown here.</p>
                    <button type="button" wire:click="makeKey" class="rounded-md border border-slate-300 bg-white px-3 py-1.5 hover:bg-slate-50">Make an SSH key</button>
                @endif
            </div>

            <button type="submit" class="rounded-md bg-lime-700 px-4 py-2 font-medium text-white hover:bg-lime-800">Make muken.yaml</button>
        </form>
    @endif
</div>
