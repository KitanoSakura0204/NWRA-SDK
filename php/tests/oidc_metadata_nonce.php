<?php

declare(strict_types=1);

// Run: php tests/oidc_metadata_nonce.php. All HTTP fixtures stay on loopback.
if (PHP_SAPI === 'cli-server') {
    $path = (string) parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
    header('Content-Type: application/json');
    if ($_SERVER['REQUEST_METHOD'] !== 'GET' || isset($_SERVER['HTTP_AUTHORIZATION']) || isset($_SERVER['HTTP_X_SERVER_KEY']) || $_GET !== []
        || file_get_contents('php://input') !== '') {
        http_response_code(400);
        echo '{}';
    } elseif (str_starts_with($path, '/redirect/')) {
        header('Location: /jwks.json', true, 302);
        echo '{"keys":[]}';
    } elseif (str_starts_with($path, '/large/')) {
        for ($i = 0; $i < 512; $i++) {
            echo str_repeat(' ', 8192);
            flush();
        }
    } elseif (str_starts_with($path, '/boundary/')) {
        echo '{"keys":[]}' . str_repeat(' ', 1048576 - strlen('{"keys":[]}'));
    } elseif (str_starts_with($path, '/slow/')) {
        usleep(500000);
        echo '{"keys":[]}';
    } elseif (str_ends_with($path, '/.well-known/openid-configuration')) {
        $issuer = 'http://' . $_SERVER['HTTP_HOST'];
        echo json_encode(['issuer' => $issuer, 'authorization_endpoint' => $issuer . '/oauth2/authorize',
            'token_endpoint' => $issuer . '/oauth2/token', 'jwks_uri' => 'https://untrusted.example/jwks']);
    } else {
        echo '{"keys":[],"future":true}';
    }
    return;
}

use NoWork\Sdk\CurlTransport;
use NoWork\Sdk\Delivery;
use NoWork\Sdk\Exceptions\DecryptError;
use NoWork\Sdk\Exceptions\TransportError;
use NoWork\Sdk\NoworkOidc;

spl_autoload_register(static function (string $class): void {
    $prefix = 'NoWork\\Sdk\\';
    if (str_starts_with($class, $prefix)) {
        $file = __DIR__ . '/../src/' . str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
        if (is_file($file)) {
            require_once $file;
        }
    }
});

function om_assert(bool $condition, string $message = 'assertion failed'): void
{
    if (!$condition) {
        throw new RuntimeException($message);
    }
}

function om_throws(string $class, callable $fn, string $message = ''): Throwable
{
    try {
        $fn();
    } catch (Throwable $e) {
        om_assert($e instanceof $class, 'expected ' . $class . ', received ' . $e::class . ': ' . $e->getMessage());
        om_assert($message === '' || str_contains($e->getMessage(), $message), 'unexpected error: ' . $e->getMessage());
        return $e;
    }
    throw new RuntimeException('expected ' . $class);
}

function om_doc(): array
{
    return ['issuer' => 'https://account.example/prefix', 'authorization_endpoint' => 'https://account.example/oauth2/authorize',
        'token_endpoint' => 'https://account.example/oauth2/token', 'jwks_uri' => 'https://untrusted.example/advertised',
        'future' => ['enabled' => false, 'items' => [1, 2]]];
}

function om_oidc(array $response): NoworkOidc
{
    return new NoworkOidc('https://account.example/prefix/', 'client', 'secret', '', 2.5, static fn () => $response);
}

function om_b64(string $binary): string
{
    return rtrim(strtr(base64_encode($binary), '+/', '-_'), '=');
}

function om_sign(array $claims, OpenSSLAsymmetricKey $key): string
{
    $unsigned = om_b64('{"alg":"RS256","kid":"signing"}') . '.' . om_b64(json_encode($claims, JSON_THROW_ON_ERROR));
    om_assert(openssl_sign($unsigned, $signature, $key, OPENSSL_ALGO_SHA256), 'could not sign fixture');
    return $unsigned . '.' . om_b64($signature);
}

function om_encrypt(string $inner, array $recipient): string
{
    $ephemeral = random_bytes(32);
    $public = sodium_crypto_scalarmult_base($ephemeral);
    $z = sodium_crypto_scalarmult($ephemeral, base64_decode(strtr($recipient['x'], '-_', '+/')));
    $alg = 'ECDH-ES+A256KW';
    $kek = hash('sha256', pack('N', 1) . $z . pack('N', strlen($alg)) . $alg . pack('N', 0) . pack('N', 0) . pack('N', 256), true);
    $cek = random_bytes(32);
    // RFC 3394 wrap is the inverse of Delivery's unwrap, with independently generated keys.
    $a = str_repeat("\xa6", 8);
    $r = str_split($cek, 8);
    for ($j = 0; $j < 6; $j++) {
        for ($i = 1; $i <= 4; $i++) {
            $b = openssl_encrypt($a . $r[$i - 1], 'aes-256-ecb', $kek, OPENSSL_RAW_DATA | OPENSSL_ZERO_PADDING);
            om_assert($b !== false, 'could not wrap fixture key');
            $a = substr($b, 0, 8) ^ pack('J', 4 * $j + $i);
            $r[$i - 1] = substr($b, 8, 8);
        }
    }
    $header = om_b64(json_encode(['alg' => $alg, 'enc' => 'A256GCM', 'kid' => 'recipient', 'cty' => 'JWT',
        'epk' => ['kty' => 'OKP', 'crv' => 'X25519', 'x' => om_b64($public)]], JSON_THROW_ON_ERROR));
    $iv = random_bytes(12);
    $ciphertext = openssl_encrypt($inner, 'aes-256-gcm', $cek, OPENSSL_RAW_DATA, $iv, $tag, $header);
    om_assert($ciphertext !== false, 'could not encrypt fixture');
    return implode('.', [$header, om_b64($a . implode('', $r)), om_b64($iv), om_b64($ciphertext), om_b64($tag)]);
}

$tests = [];
$tests['discovery and JWKS retain fields and use fixed unauthenticated five-argument requests'] = static function (): void {
    $requests = [];
    $transport = static function (string $method, string $url, array $headers, ?string $body, float $timeout) use (&$requests): array {
        om_assert(func_num_args() === 5, 'custom callable contract changed');
        $requests[] = compact('method', 'url', 'headers', 'body', 'timeout');
        return [200, json_encode(count($requests) === 1 ? om_doc() : ['keys' => [['kty' => 'RSA', 'future' => false]], 'extra' => 7])];
    };
    $oidc = new NoworkOidc('https://account.example/prefix///', 'client', 'secret', '', 2.5, $transport);
    om_assert($oidc->discover() === om_doc(), 'discovery fields changed');
    om_assert($oidc->jwks() === ['keys' => [['kty' => 'RSA', 'future' => false]], 'extra' => 7], 'JWK fields changed');
    foreach ($requests as $i => $request) {
        om_assert($request === ['method' => 'GET', 'url' => 'https://account.example/prefix' . ($i === 0 ? '/.well-known/openid-configuration' : '/jwks.json'),
            'headers' => ['Accept' => 'application/json'], 'body' => null, 'timeout' => 2.5], 'unexpected metadata request');
    }
    om_assert(count($requests) === 2, 'advertised endpoint fetched');
};
$tests['metadata rejects non-2xx malformed JSON and every non-object root'] = static function (): void {
    foreach ([199, 300, 302, 304, 400, 503] as $status) {
        foreach (['discover', 'jwks'] as $method) {
            om_throws(TransportError::class, static fn () => om_oidc([$status, json_encode($method === 'discover' ? om_doc() : ['keys' => []])])->$method(), 'http ' . $status);
        }
    }
    foreach (['', '{', '[]', '[{}]', 'null', '42', '"text"', 'true', '{"keys":[]}', '{}'] as $raw) {
        om_throws(TransportError::class, static fn () => om_oidc([200, $raw])->discover());
    }
    foreach (['', '{', '[]', '[{}]', 'null', '42', '"text"', 'true', '{}'] as $raw) {
        om_throws(TransportError::class, static fn () => om_oidc([200, $raw])->jwks());
    }
};
$tests['discovery requires four nonempty string fields and exact normalized issuer'] = static function (): void {
    foreach (['issuer', 'authorization_endpoint', 'token_endpoint', 'jwks_uri'] as $field) {
        foreach ([null, '', false, 7, [], '   '] as $bad) {
            $doc = om_doc();
            $doc[$field] = $bad;
            om_throws(TransportError::class, static fn () => om_oidc([200, json_encode($doc)])->discover());
        }
        $doc = om_doc();
        unset($doc[$field]);
        om_throws(TransportError::class, static fn () => om_oidc([200, json_encode($doc)])->discover());
    }
    foreach (['https://account.example/prefix/', 'https://other.example', 'HTTPS://account.example/prefix'] as $issuer) {
        $doc = om_doc();
        $doc['issuer'] = $issuer;
        om_throws(TransportError::class, static fn () => om_oidc([200, json_encode($doc)])->discover(), 'issuer mismatch');
    }
};
$tests['JWKS requires an array of objects including correct empty object handling'] = static function (): void {
    foreach (['{"keys":{}}', '{"keys":{"0":{}}}', '{"keys":null}', '{"keys":1}', '{"keys":"text"}',
        '{"keys":[[]]}', '{"keys":[1]}', '{"keys":[null]}', '{"keys":["key"]}'] as $raw) {
        om_throws(TransportError::class, static fn () => om_oidc([200, $raw])->jwks());
    }
    om_assert(om_oidc([200, '{"keys":[]}'])->jwks() === ['keys' => []]);
    om_assert(om_oidc([200, '{"keys":[{}]}'])->jwks() === ['keys' => [[]]]);
};
$tests['metadata custom responses are size checked at exactly one MiB and exceptions remain wrapped'] = static function (): void {
    foreach (['discover' => json_encode(om_doc()), 'jwks' => '{"keys":[]}'] as $method => $raw) {
        $atLimit = $raw . str_repeat(' ', 1048576 - strlen($raw));
        om_assert(is_array(om_oidc([200, $atLimit])->$method()));
        om_throws(TransportError::class, static fn () => om_oidc([200, $atLimit . ' '])->$method(), 'exceeds');
    }
    $oidc = new NoworkOidc('https://account.example', 'client', transport: static fn () => throw new RuntimeException('fixture network failure'));
    $error = om_throws(TransportError::class, static fn () => $oidc->discover(), 'fixture network failure');
    om_assert($error->getPrevious() instanceof RuntimeException);
};
$tests['metadata rejects URL credentials and non-path issuer suffixes before HTTP'] = static function (): void {
    $calls = 0;
    $transport = static function () use (&$calls): array { $calls++; return [200, '{"keys":[]}']; };
    foreach (['https://user:secret@account.example', 'https://account.example?token=secret', 'https://account.example#part', "https://account.example/\r\n"] as $issuer) {
        $oidc = new NoworkOidc($issuer, 'client', transport: $transport);
        om_throws(TransportError::class, static fn () => $oidc->jwks());
    }
    om_assert($calls === 0);
};
$tests['metadata validates UTF-8 throughout unknown fields and preserves valid Unicode'] = static function (): void {
    foreach (["\xff", "\xc2", "\xc0\xaf", "\xed\xa0\x80"] as $invalid) {
        $discovery = substr(json_encode(om_doc()), 0, -1) . ',"unknown":"' . $invalid . '"}';
        $jwks = '{"keys":[{"unknown":"' . $invalid . '"}]}';
        om_throws(TransportError::class, static fn () => om_oidc([200, $discovery])->discover(), 'json');
        om_throws(TransportError::class, static fn () => om_oidc([200, $jwks])->jwks(), 'json');
    }
    $doc = om_doc();
    $doc['unknown'] = '未来🙂';
    om_assert(om_oidc([200, json_encode($doc, JSON_UNESCAPED_UNICODE)])->discover() === $doc);
    om_assert(om_oidc([200, '{"keys":[{"unknown":"未来🙂"}]}'])->jwks()['keys'][0]['unknown'] === '未来🙂');
};

$rsa = openssl_pkey_new(['private_key_bits' => 2048, 'private_key_type' => OPENSSL_KEYTYPE_RSA]);
om_assert($rsa !== false, 'ext-openssl is required for these tests');
$details = openssl_pkey_get_details($rsa);
$jwks = ['keys' => [['kty' => 'RSA', 'kid' => 'signing', 'n' => om_b64($details['rsa']['n']), 'e' => om_b64($details['rsa']['e'])]]];
$claims = ['iss' => 'https://account.example', 'aud' => 'client', 'sub' => 'subject', 'exp' => time() + 1200, 'nonce' => 'session-nonce'];

$tests['nonce binds the signed string while old verification calls and audience arrays work'] = static function () use ($rsa, $jwks, $claims): void {
    $token = om_sign($claims, $rsa);
    om_assert(Delivery::verifyIdToken($token, $jwks) === $claims);
    om_assert(Delivery::verifyIdToken($token, json_encode($jwks), $claims['iss'], 'client') === $claims);
    om_assert(Delivery::verifyIdToken($token, $jwks, $claims['iss'], 'client', expectedNonce: 'session-nonce') === $claims);
    om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken($token, $jwks, $claims['iss'], 'client', ''), 'nonce');
    om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken($token, $jwks, $claims['iss'], 'client', null), 'nonce');
    om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken($token, $jwks, $claims['iss'], 'client', 'different'), 'nonce');
    $multi = $claims;
    $multi['aud'] = ['other', 'client'];
    om_assert(Delivery::verifyIdToken(om_sign($multi, $rsa), $jwks, $claims['iss'], 'client', 'session-nonce') === $multi);
};
$tests['missing non-string and mismatched nonce claims fail without coercion'] = static function () use ($rsa, $jwks, $claims): void {
    $missing = $claims;
    unset($missing['nonce']);
    om_assert(Delivery::verifyIdToken(om_sign($missing, $rsa), $jwks) === $missing, 'legacy behavior changed');
    om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken(om_sign($missing, $rsa), $jwks, null, null, 'session-nonce'), 'nonce');
    foreach ([null, 123, true, [], ['session-nonce'], '', 'different'] as $nonce) {
        $bad = $claims;
        $bad['nonce'] = $nonce;
        om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken(om_sign($bad, $rsa), $jwks, null, null, 'session-nonce'), 'nonce');
    }
};
$tests['signature issuer audience and clock checks still precede nonce validation'] = static function () use ($rsa, $jwks, $claims): void {
    $parts = explode('.', om_sign($claims, $rsa));
    $badSignature = $parts[0] . '.' . $parts[1] . '.' . om_b64(str_repeat("\0", 256));
    om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken($badSignature, $jwks, $claims['iss'], 'client', ''), 'signature');
    foreach (['iss' => 'wrong-issuer', 'aud' => 'wrong-audience', 'exp' => time() - 1200, 'nbf' => time() + 1200] as $field => $value) {
        $bad = $claims;
        $bad[$field] = $value;
        $message = ['iss' => 'issuer', 'aud' => 'audience', 'exp' => 'expired', 'nbf' => 'not valid yet'][$field];
        om_throws(DecryptError::class, static fn () => Delivery::verifyIdToken(om_sign($bad, $rsa), $jwks, $claims['iss'], 'client', ''), $message);
    }
};
$tests['encrypted ID tokens retain inner RS256 verification and append the nonce argument'] = static function () use ($rsa, $jwks, $claims): void {
    $private = random_bytes(32);
    $recipient = ['kty' => 'OKP', 'crv' => 'X25519', 'kid' => 'recipient', 'd' => om_b64($private), 'x' => om_b64(sodium_crypto_scalarmult_base($private))];
    $encrypted = om_encrypt(om_sign($claims, $rsa), $recipient);
    om_assert(Delivery::decryptIdToken($encrypted, $recipient, $jwks, $claims['iss'], 'client') === $claims);
    om_assert(Delivery::decryptIdToken($encrypted, [$recipient], $jwks, $claims['iss'], 'client', 'session-nonce') === $claims);
    om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken($encrypted, $recipient, $jwks, $claims['iss'], 'client', 'wrong'), 'nonce');
    om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken($encrypted, $recipient, $jwks, $claims['iss'], 'client', null), 'nonce');
    om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken($encrypted, $recipient, $jwks, $claims['iss'], 'client', ''), 'nonce');
    $missingNonce = $claims;
    unset($missingNonce['nonce']);
    foreach ([$missingNonce, array_replace($claims, ['nonce' => 123])] as $bad) {
        om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken(om_encrypt(om_sign($bad, $rsa), $recipient), $recipient, $jwks, $claims['iss'], 'client', 'session-nonce'), 'nonce');
    }
    $expired = array_replace($claims, ['exp' => time() - 1200]);
    om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken(om_encrypt(om_sign($expired, $rsa), $recipient), $recipient, $jwks, $claims['iss'], 'client', ''), 'expired');
    $parts = explode('.', om_sign($claims, $rsa));
    $unsigned = $parts[0] . '.' . $parts[1] . '.' . om_b64(str_repeat("\0", 256));
    om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken(om_encrypt($unsigned, $recipient), $recipient, $jwks, $claims['iss'], 'client', 'session-nonce'), 'signature');
    om_throws(DecryptError::class, static fn () => Delivery::decryptIdToken(om_encrypt(json_encode($claims), $recipient), $recipient, $jwks, $claims['iss'], 'client', 'session-nonce'), 'signed JWT');
};
$tests['default and injected cURL bound downloads reject redirects and retain timeout options'] = static function (): void {
    $socket = stream_socket_server('tcp://127.0.0.1:0', $errno, $errstr);
    om_assert($socket !== false, 'cannot bind loopback');
    $address = stream_socket_get_name($socket, false);
    fclose($socket);
    $null = PHP_OS_FAMILY === 'Windows' ? 'NUL' : '/dev/null';
    $process = proc_open([PHP_BINARY, '-S', $address, __FILE__], [0 => ['pipe', 'r'], 1 => ['file', $null, 'a'], 2 => ['file', $null, 'a']], $pipes, __DIR__);
    om_assert(is_resource($process), 'could not start loopback server');
    fclose($pipes[0]);
    try {
        $ready = false;
        for ($i = 0; $i < 100; $i++) {
            $probe = @stream_socket_client('tcp://' . $address, $errno, $errstr, 0.02);
            if ($probe !== false) { fclose($probe); $ready = true; break; }
            usleep(20000);
        }
        om_assert($ready, 'loopback server did not become ready');
        $base = 'http://' . $address;
        $oidc = new NoworkOidc($base . '/', 'client', 'secret', timeout: 2.0);
        om_assert($oidc->discover()['issuer'] === $base);
        om_assert($oidc->jwks()['future'] === true);
        om_assert((new NoworkOidc($base . '/boundary', 'client'))->jwks() === ['keys' => []]);
        foreach ([null, new CurlTransport()] as $transport) {
            $error = om_throws(TransportError::class, static fn () => (new NoworkOidc($base . '/large', 'client', transport: $transport))->jwks(), 'exceeds');
            om_assert($error->getCode() === CURLE_WRITE_ERROR, 'download must stop in the write callback');
        }
        $transport = new CurlTransport([CURLOPT_USERPWD => 'client:secret', CURLOPT_HTTPAUTH => CURLAUTH_BASIC,
            CURLOPT_POSTFIELDS => 'client_secret=secret', CURLOPT_CUSTOMREQUEST => 'POST']);
        om_assert((new NoworkOidc($base, 'client', 'secret', transport: $transport))->jwks()['future'] === true,
            'cURL origin credentials or a caller request body reached metadata');
        $file = tmpfile();
        om_assert($file !== false);
        try {
            $transport = new CurlTransport([CURLOPT_FILE => $file]);
            $error = om_throws(TransportError::class, static fn () => (new NoworkOidc($base . '/large', 'client', transport: $transport))->jwks(), 'exceeds');
            om_assert($error->getCode() === CURLE_WRITE_ERROR, 'file option bypassed bounded callback');
        } finally {
            fclose($file);
        }
        $writes = 0;
        $transport = new CurlTransport([CURLOPT_WRITEFUNCTION => static function ($handle, string $chunk) use (&$writes): int {
            $writes++;
            return 0;
        }]);
        $error = om_throws(TransportError::class, static fn () => (new NoworkOidc($base, 'client', transport: $transport))->jwks(), 'curl error');
        om_assert($error->getCode() === CURLE_WRITE_ERROR && $writes === 1, 'caller write limit bypassed');
        $transport = new CurlTransport([CURLOPT_FOLLOWLOCATION => true, CURLOPT_USERAGENT => 'metadata-test']);
        om_throws(TransportError::class, static fn () => (new NoworkOidc($base . '/redirect', 'client', transport: $transport))->jwks(), 'http 302');
        $error = om_throws(TransportError::class, static fn () => (new NoworkOidc($base . '/slow', 'client', timeout: 0.05))->jwks(), 'curl error');
        om_assert($error->getCode() === CURLE_OPERATION_TIMEDOUT);
        $transport = new CurlTransport([CURLOPT_TIMEOUT_MS => 50]);
        $error = om_throws(TransportError::class, static fn () => (new NoworkOidc($base . '/slow', 'client', timeout: 2.0, transport: $transport))->jwks(), 'curl error');
        om_assert($error->getCode() === CURLE_OPERATION_TIMEDOUT, 'injected cURL deadline changed');
    } finally {
        proc_terminate($process);
        proc_close($process);
    }
};

$failed = 0;
foreach ($tests as $name => $test) {
    try {
        $test();
        echo "  ok $name\n";
    } catch (Throwable $e) {
        $failed++;
        echo '  FAIL ' . $name . ': ' . $e::class . ': ' . $e->getMessage() . "\n";
    }
}
echo count($tests) . ' groups, ' . $failed . " failures\n";
exit($failed === 0 ? 0 : 1);
