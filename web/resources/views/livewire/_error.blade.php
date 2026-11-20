@if ($error)
    <div class="mb-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800" role="alert">
        <p class="font-medium">{{ $error }}</p>
        @if ($hint)
            <p class="mt-1 text-red-700">{{ $hint }}</p>
        @endif
    </div>
@endif
