# web

The overlay of the muken web UI. It is not a whole Laravel app: the image build runs `composer create-project laravel/laravel`, adds Livewire, then copies this folder over the result (see `src/Dockerfile`, target `web`). Only the files that differ from a new Laravel project live here.

Spec: `openspec/changes/add-web-ui/specs/{web,history}/spec.md`. The API it calls: `specs/api/spec.md`.

| Path | What it does |
|---|---|
| `app/Services/MukenClient.php` | Calls muken serve on `127.0.0.1:7600` |
| `app/Services/HistoryStore.php` | The only database use: one SQLite table, self-healing, never throws |
| `app/Livewire/` | One component per page |
| `database/migrations/history/` | The table of the history. The default `migrate` does not touch it |
| `config/muken.php` | `MUKEN_API_URL`, `MUKEN_HISTORY_DB` |
