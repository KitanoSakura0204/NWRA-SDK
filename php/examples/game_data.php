<?php

declare(strict_types=1);

/**
 * Read-only CLI example (PHP 8.1+, ext-curl):
 *   NOWORK_BASE=https://your-site.example NOWORK_TOKEN=<site JWT>
 *   NOWORK_PROVIDER=hypergryph NOWORK_GAME=arknights php examples/game_data.php
 *
 * Optional NOWORK_ROLE is a stored miHoYo role ID or a returned Hypergryph
 * bound-role string ID. Optional NOWORK_BINDING is the provider binding account
 * ID used to list saved archive metadata. Neither is an upstream game UID.
 * With no token, only the public Hypergryph games/snapshot routes are used.
 * Keep tokens in a secret environment source, not command-line arguments.
 */

use NoWork\Sdk\Exceptions\NoWorkException;
use NoWork\Sdk\GameDataClient;

$autoload = __DIR__ . '/../vendor/autoload.php';
if (is_file($autoload)) {
    require $autoload;
} else {
    spl_autoload_register(static function (string $class): void {
        $prefix = 'NoWork\\Sdk\\';
        if (str_starts_with($class, $prefix)) {
            $file = __DIR__ . '/../src/' . str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
            if (is_file($file)) {
                require_once $file;
            }
        }
    });
}

$base = getenv('NOWORK_BASE') ?: '';
if ($base === '') {
    fwrite(STDERR, "Set NOWORK_BASE to your NoWork site URL.\n");
    exit(2);
}
$token = getenv('NOWORK_TOKEN') ?: '';
$provider = getenv('NOWORK_PROVIDER') ?: 'hypergryph';
$game = getenv('NOWORK_GAME') ?: 'arknights';
$role = getenv('NOWORK_ROLE') ?: '';
$binding = getenv('NOWORK_BINDING') ?: '';

try {
    $client = $token === '' ? GameDataClient::forPublic($base) : GameDataClient::forAccount($base, $token);
    $games = $client->games($provider);
    echo 'Available game descriptors: ' . count($games) . "\n";
    $options = ['limit' => 50];
    if ($role !== '') {
        if ($provider === 'mihoyo') {
            $storedRole = filter_var($role, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
            if ($storedRole === false) {
                throw new InvalidArgumentException('NOWORK_ROLE must be a positive stored miHoYo role ID');
            }
            $options['roleId'] = $storedRole;
        } else {
            $options['roleId'] = $role;
        }
    }
    foreach ($client->snapshotPages($provider, $game, $options, maxPages: 10) as $page) {
        // Print availability/counts only. Treat all role fields as private data.
        foreach ($page['sections'] as $section) {
            printf("%s: %s (%d entities)\n", $section['id'], $section['status'], count($section['entities'] ?? []));
        }
    }
    if ($token !== '') {
        echo 'Asset providers: ' . count($client->assetProviders()) . "\n";
        if ($binding !== '') {
            $bindingId = filter_var($binding, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
            if ($bindingId === false) {
                throw new InvalidArgumentException('NOWORK_BINDING must be a positive provider binding ID');
            }
            echo 'Saved snapshots: ' . count($client->archiveSnapshots($provider, $bindingId)) . "\n";
        }
    }
} catch (NoWorkException | LogicException $e) {
    fwrite(STDERR, $e->getMessage() . "\n");
    exit(1);
}
