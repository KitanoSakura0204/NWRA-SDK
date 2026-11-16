import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { NoworkOIDC, TransportError } from '../dist/index.js';

const issuer = 'https://account.example/tenant';
const metadata = {
  issuer,
  authorization_endpoint: issuer + '/oauth2/authorize',
  token_endpoint: issuer + '/oauth2/token',
  jwks_uri: 'https://untrusted.example/advertised-keys',
  scopes_supported: ['openid', 'profile'],
  extension: { enabled: true },
};
const keys = { keys: [{ kty: 'RSA', kid: 'key-1', n: 'n', e: 'AQAB', custom: { tags: ['rotation'] } }], extension: true };
const json = value => new Response(JSON.stringify(value));

test('discovery and JWKS use fixed issuer paths without credentials and retain extensions', async () => {
  const calls = [];
  const oidc = new NoworkOIDC(issuer + '///', 'client', 'client-secret', 'https://app.example/callback', async (url, init) => {
    calls.push(url);
    assert.equal(init.method, 'GET');
    assert.equal(init.redirect, 'error');
    assert.equal(init.credentials, 'omit');
    assert.deepEqual(init.headers, { Accept: 'application/json' });
    assert.equal(init.body, undefined);
    assert.ok(init.signal instanceof AbortSignal);
    assert.equal(JSON.stringify(init).includes('client-secret'), false);
    return json(url.endsWith('/jwks.json') ? keys : metadata);
  });
  assert.deepEqual(await oidc.discover(), metadata);
  assert.deepEqual(await oidc.jwks(), keys);
  assert.deepEqual(calls, [issuer + '/.well-known/openid-configuration', issuer + '/jwks.json']);
});

for (const method of ['discover', 'jwks']) {
  const valid = method === 'discover' ? metadata : keys;
  test(`${method} rejects malformed metadata issuers before transport without disclosing configuration`, async () => {
    for (const configured of [
      issuer + '?client_secret=fixture', issuer + '?', issuer + '#fixture', issuer + '#',
      'https://user:fixture@account.example', 'https://@account.example',
      'ftp://account.example', 'file:///fixture', 'account.example', '/tenant',
      '//account.example', 'https:///tenant', 'https://', '',
      'https://account.example:invalid', 'https://account.example:65536', 'https://[broken',
      ' https://account.example', 'https://account.example/\tfixture', 'https://account.example\\fixture',
    ]) {
      let calls = 0;
      const oidc = new NoworkOIDC(configured, 'client', 'client-secret', '', async () => {
        calls++;
        return json({ ...valid, issuer: configured.replace(/\/+$/, '') });
      });
      await assert.rejects(oidc[method](), error => {
        assert.ok(error instanceof TransportError);
        assert.equal(error.message, 'nowork: transport error: invalid metadata issuer URL');
        assert.equal(error.cause, undefined);
        return true;
      }, configured);
      assert.equal(calls, 0, configured);
    }
  });

  test(`${method} rejects every non-2xx status, malformed JSON, and wrong root type`, async () => {
    for (const status of [199, 301, 302, 304, 400, 401, 500]) {
      // Injected adapters may surface statuses outside native Response's constructor range.
      let bodyRead = false;
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => ({
        status, text: async () => { bodyRead = true; return JSON.stringify(valid); },
      }));
      await assert.rejects(oidc[method](), TransportError);
      assert.equal(bodyRead, false);
    }
    for (const text of ['', '{broken', 'null', '[]', '"object"', 'true', '1']) {
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => new Response(text));
      await assert.rejects(oidc[method](), TransportError);
    }
  });

  test(`${method} bounds Content-Length and streamed UTF-8 wire bytes at 1 MiB`, async () => {
    const limit = 1024 * 1024;
    for (const declared of [String(limit + 1), '9007199254740993']) {
      let cancelled = false;
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => new Response(new ReadableStream({
        cancel() { cancelled = true; },
      }), { headers: { 'Content-Length': declared } }));
      await assert.rejects(oidc[method](), /1048576-byte limit/);
      assert.equal(cancelled, true);
    }
    const prefix = JSON.stringify({ ...valid, padding: '' }).slice(0, -2);
    const exact = prefix + ' '.repeat(limit - Buffer.byteLength(prefix) - 2) + '"}';
    const exactClient = new NoworkOIDC(issuer, 'client', '', '', async () => new Response(exact));
    assert.equal(Buffer.byteLength(exact), limit);
    assert.equal((await exactClient[method]()).padding.length, limit - Buffer.byteLength(prefix) - 2);
    for (const headers of [{}, { 'Content-Length': '1' }]) {
      let cancelled = false;
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => new Response(new ReadableStream({
        start(controller) {
          controller.enqueue(new TextEncoder().encode(prefix));
          // Fewer than 1 MiB characters, but more than 1 MiB UTF-8 bytes.
          controller.enqueue(new TextEncoder().encode('旅'.repeat(Math.ceil(limit / 3))));
        },
        cancel() { cancelled = true; },
      }), { headers }));
      await assert.rejects(oidc[method](), /1048576-byte limit/);
      assert.equal(cancelled, true);
    }
  });

  test(`${method} preserves text/json-only fetch adapters and rejects oversized decoded bodies`, async () => {
    for (const adapter of [
      { status: 200, text: async () => JSON.stringify(valid) },
      { status: 200, json: async () => valid },
    ]) {
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => adapter);
      assert.deepEqual(await oidc[method](), valid);
    }
    for (const adapter of [
      { status: 200, text: async () => JSON.stringify({ ...valid, padding: '旅'.repeat(400_000) }) },
      { status: 200, json: async () => ({ ...valid, padding: 'x'.repeat(1024 * 1024) }) },
    ]) {
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => adapter);
      await assert.rejects(oidc[method](), /1048576-byte limit/);
    }
  });

  test(`${method} timeout covers pending fetch, streamed bodies, and non-stream adapters`, async () => {
    let cancelled = false;
    const adapters = [
      async () => new Promise(() => {}),
      async () => new Response(new ReadableStream({ cancel() { cancelled = true; } })),
      async () => ({ status: 200, text: () => new Promise(() => {}) }),
    ];
    for (const adapter of adapters) {
      const oidc = new NoworkOIDC(issuer, 'client', '', '', adapter, 20);
      await assert.rejects(oidc[method](), error => error instanceof TransportError && error.cause?.name === 'AbortError');
    }
    assert.equal(cancelled, true);
  });

  test(`${method} respects caller abort before fetching and while reading the body`, async () => {
    const alreadyAborted = new AbortController();
    const reason = new Error('caller cancelled');
    alreadyAborted.abort(reason);
    let fetched = false;
    const idle = new NoworkOIDC(issuer, 'client', '', '', async () => { fetched = true; return json(valid); });
    await assert.rejects(idle[method](alreadyAborted.signal), error => error instanceof TransportError && error.cause === reason);
    assert.equal(fetched, false);
    const abort = new AbortController();
    let cancelled = false;
    const pending = new NoworkOIDC(issuer, 'client', '', '', async () => {
      setTimeout(() => abort.abort(reason), 10);
      return new Response(new ReadableStream({ cancel() { cancelled = true; } }));
    });
    await assert.rejects(pending[method](abort.signal), error => error instanceof TransportError && error.cause === reason);
    assert.equal(cancelled, true);
  });

  test(`${method} rejects invalid UTF-8 and wraps network failures`, async () => {
    const invalidUtf8 = new NoworkOIDC(issuer, 'client', '', '', async () => new Response(new Uint8Array([123, 34, 255, 34, 58, 49, 125])));
    await assert.rejects(invalidUtf8[method](), TransportError);
    const cause = new Error('offline');
    const offline = new NoworkOIDC(issuer, 'client', '', '', async () => { throw cause; });
    await assert.rejects(offline[method](), error => error instanceof TransportError && error.cause === cause);
  });
}

test('discovery requires nonempty string fields and the exactly normalized configured issuer', async () => {
  for (const field of ['issuer', 'authorization_endpoint', 'token_endpoint', 'jwks_uri']) {
    for (const value of [undefined, null, '', '   ', '\t', '\n', 7, [], {}]) {
      const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => json({ ...metadata, [field]: value }));
      await assert.rejects(oidc.discover(), /incomplete OIDC discovery/);
    }
  }
  for (const value of [issuer + '/', 'https://other.example', issuer.toUpperCase()]) {
    const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => json({ ...metadata, issuer: value }));
    await assert.rejects(oidc.discover(), /issuer mismatch/);
  }
});

test('metadata preserves valid issuer subpaths, case, trailing slashes and endpoint contents', async () => {
  for (const configured of ['https://account.example', issuer + '/v1///', 'HTTPS://Account.Example/tenant', 'http://localhost:8080/tenant']) {
    const expected = configured.replace(/\/+$/, '');
    const document = { ...metadata, issuer: expected, authorization_endpoint: ' https://account.example/authorize ' };
    const calls = [];
    const oidc = new NoworkOIDC(configured, 'client', '', '', async url => {
      calls.push(url);
      return json(url.endsWith('/jwks.json') ? keys : document);
    });
    assert.deepEqual(await oidc.discover(), document);
    assert.deepEqual(await oidc.jwks(), keys);
    assert.deepEqual(calls, [expected + '/.well-known/openid-configuration', expected + '/jwks.json']);
  }
});

test('JWKS requires an array of non-null objects, accepting an empty key set', async () => {
  for (const value of [undefined, null, {}, '', 1, [null], [[]], ['key'], [7], [true], [{ kty: 'RSA' }, null]]) {
    const oidc = new NoworkOIDC(issuer, 'client', '', '', async () => json({ keys: value }));
    await assert.rejects(oidc.jwks(), /keys must be an array of objects/);
  }
  const empty = new NoworkOIDC(issuer, 'client', '', '', async () => json({ keys: [], extension: true }));
  assert.deepEqual(await empty.jwks(), { keys: [], extension: true });
});

test('OIDC validates timeout configuration without breaking the five-argument constructor', () => {
  for (const timeout of [0, -1, 0.5, NaN, Infinity, 2_147_483_648]) {
    assert.throws(() => new NoworkOIDC(issuer, 'client', '', '', async () => json(metadata), timeout), RangeError);
  }
  assert.doesNotThrow(() => new NoworkOIDC(issuer, 'client', '', '', async () => json(metadata)));
});

for (const method of ['exchange', 'userInfo']) {
  test(`${method} deadline aborts its injected fetch and includes non-stream body reads`, { timeout: 500 }, async () => {
    for (const pendingBody of [false, true]) {
      let requestSignal;
      const oidc = new NoworkOIDC(issuer, 'client', 'secret', '', async (_url, { signal }) => {
        requestSignal = signal;
        if (!pendingBody) return new Promise(() => {});
        return { status: 200, text: () => new Promise(() => {}), json: () => new Promise(() => {}) };
      }, 20);
      await assert.rejects(oidc[method]('token-or-code'), TransportError);
      assert.ok(requestSignal instanceof AbortSignal);
      assert.equal(requestSignal.aborted, true);
    }
  });
}

test('native fetch reads configured paths and refuses redirects without fetching their targets', async t => {
  const requests = [];
  let redirectKeys = false;
  let base;
  const server = createServer((req, res) => {
    requests.push(req.url);
    assert.equal(req.method, 'GET');
    for (const header of ['authorization', 'cookie', 'x-server-key']) assert.equal(req.headers[header], undefined);
    if (req.url === '/jwks.json' && redirectKeys) {
      res.writeHead(302, { Location: '/redirect-target' }).end();
    } else {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(req.url === '/jwks.json' ? keys : { ...metadata, issuer: base }));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(() => new Promise(resolve => { server.closeAllConnections(); server.close(resolve); }));
  base = `http://127.0.0.1:${server.address().port}`;
  const oidc = new NoworkOIDC(base, 'client', 'client-secret');
  assert.equal((await oidc.discover()).issuer, base);
  assert.deepEqual(await oidc.jwks(), keys);
  redirectKeys = true;
  await assert.rejects(oidc.jwks(), TransportError);
  assert.deepEqual(requests, ['/.well-known/openid-configuration', '/jwks.json', '/jwks.json']);
});
