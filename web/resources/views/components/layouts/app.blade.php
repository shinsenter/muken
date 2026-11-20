<!DOCTYPE html>
<html lang="en" class="h-full bg-slate-50">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ $title ?? 'muken' }} · muken</title>
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body class="min-h-full text-slate-800">
    <header class="border-b border-slate-200 bg-white">
        <nav class="mx-auto flex max-w-5xl flex-wrap items-center gap-x-5 gap-y-2 px-4 py-3 text-sm">
            <a href="/" wire:navigate class="mr-2 text-lg font-semibold text-lime-700">muken</a>
            @foreach ([
                '/' => 'Status', '/deploy' => 'Deploy', '/rollback' => 'Rollback', '/scale' => 'Scale',
                '/maintenance' => 'Maintenance', '/up' => 'Set up', '/doctor' => 'Doctor', '/logs' => 'Logs',
                '/config' => 'Config', '/history' => 'History',
            ] as $href => $label)
                <a href="{{ $href }}" wire:navigate
                   class="{{ request()->is(ltrim($href, '/') ?: '/') ? 'font-semibold text-lime-700' : 'text-slate-600 hover:text-slate-900' }}">{{ $label }}</a>
            @endforeach
        </nav>
    </header>
    <main class="mx-auto max-w-5xl px-4 py-6">
        {{ $slot }}
    </main>
</body>
</html>
