<?php

return [
    // muken serve. It has no login and listens on loopback only.
    'api' => env('MUKEN_API_URL', 'http://127.0.0.1:7600'),

    // The only database of the app. It lives in the working folder, next to the keys, so
    // `-v "$PWD":/work` keeps it. A missing file is created again, with an empty history.
    'history_db' => env('MUKEN_HISTORY_DB', '/work/.muken/history.sqlite'),
];
