<div>
    <h1 class="mb-4 text-xl font-semibold">History</h1>
    <div class="overflow-x-auto rounded-lg border border-slate-200 bg-white">
        <table class="min-w-full text-left text-sm">
            <thead class="bg-slate-100 text-slate-600">
                <tr><th class="px-3 py-2">Started</th><th class="px-3 py-2">Action</th><th class="px-3 py-2">State</th><th class="px-3 py-2">Finished</th></tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
                @forelse ($rows as $row)
                    <tr>
                        <td class="px-3 py-2">{{ $row->started_at }}</td>
                        <td class="px-3 py-2">{{ $row->action }}</td>
                        <td class="px-3 py-2">{{ $row->state }}</td>
                        <td class="px-3 py-2">{{ $row->finished_at ?? '-' }}</td>
                    </tr>
                @empty
                    <tr><td colspan="4" class="px-3 py-3 text-slate-500">Nothing yet.</td></tr>
                @endforelse
            </tbody>
        </table>
    </div>
</div>
