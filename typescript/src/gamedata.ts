import { ApiError, TransportError } from './index.js';

/** Normalized wire types from Backend/internal/pkg/gamedata/types.go. */
export type GameProvider = 'mihoyo' | 'hypergryph';
export type GameLocale = 'zh_CN' | 'zh_TW' | 'en' | 'ja';
export type GameLocalizedText = Partial<Record<GameLocale, string>> & Record<string, string | undefined>;
export type GameDataStatus = 'ok' | 'partial' | 'unavailable' | 'unsupported' | 'auth_required'
  | 'private' | 'rate_limited' | 'challenge_required' | 'stale';
export type GameAuthScope = 'public' | 'account' | 'role';
export type GameVisibility = 'owner' | 'shared' | 'public' | 'secret';
export interface GameFieldPolicy { visibility: GameVisibility; exportable: boolean }
export interface GameProvenance {
  source_id: string; kind: 'official' | 'community' | 'user'; name: string; url?: string;
  auth_scope: GameAuthScope; fetched_at: string; updated_at?: string; license?: string; attribution?: string;
}
export interface GameIcon { url: string; alt?: GameLocalizedText; source_id?: string; width?: number; height?: number }
export interface GameCapability {
  id: string; name: GameLocalizedText; supported: boolean; auth_scope: GameAuthScope;
  status: GameDataStatus; reason?: string; source?: GameProvenance;
}
export interface GameDescriptor {
  id: string; provider: string; name: GameLocalizedText; icon?: GameIcon; regions?: string[];
  capabilities: GameCapability[];
}
/** No internal site/account scope or upstream credential is part of this wire type. */
export interface GameBoundRole {
  id: string; game_id: string; uid: string; region: string; server?: string; nickname?: string;
  level?: number; icon?: GameIcon; policy: GameFieldPolicy;
}
export type GameValueKind = 'text' | 'number' | 'boolean' | 'timestamp' | 'text_list';
export interface GameField {
  key: string; name: GameLocalizedText; kind: GameValueKind; value?: string | number | boolean | string[] | null;
  unit?: string; status: GameDataStatus; policy: GameFieldPolicy; source?: GameProvenance; updated_at?: string;
}
export type GameEntityKind = 'character' | 'item' | 'weapon' | 'equipment' | 'quest' | 'achievement' | 'other';
/** A catalog entity does not imply ownership; ownership/quantity require explicit fields. */
export interface GameEntity {
  id: string; kind: GameEntityKind; name: GameLocalizedText; icon?: GameIcon; fields: GameField[];
  policy: GameFieldPolicy; source?: GameProvenance;
}
export interface GameSection {
  id: string; name: GameLocalizedText; status: GameDataStatus; reason?: string; fields: GameField[];
  entities: GameEntity[]; policy: GameFieldPolicy; source?: GameProvenance; updated_at?: string; next_cursor?: string;
}
export interface GameSnapshot {
  game_id: string; role?: GameBoundRole; sections: GameSection[]; fetched_at: string;
  expires_at?: string; next_cursor?: string;
}
export interface GameList { games: GameDescriptor[] }
export interface GameRoleDiscovery { status: GameDataStatus; updatedAt?: string | null; reason?: string }
export interface GameAccountData extends GameList {
  roles: GameBoundRole[]; discovery: GameRoleDiscovery; selectedRoleId: string;
}
/** miHoYo's existing status route uses numeric binding IDs and camelCase keys. */
export interface MihoyoBoundRole {
  id: number; gameBiz: string; gameId: string; gameName: string; region: string; regionName: string;
  gameUid: string; nickname: string; level: number; isChosen: boolean; syncTime: string | null;
}
export interface GameRequestOptions { signal?: AbortSignal }
export interface GameSnapshotQuery {
  /** miHoYo accepts its numeric binding ID; Hypergryph uses its discovered string ID. */
  roleId?: string | number; sections?: string[]; cursor?: string; limit?: number; locale?: GameLocale;
}
export interface GameClientOptions {
  token: string; timeoutMs?: number; fetch?: typeof fetch;
  /** These routes accept a site JWT only. Use NoworkClient for server-key routes. */
  serverKey?: never;
}
export interface GamePublicClientOptions { timeoutMs?: number; fetch?: typeof fetch; token?: never; serverKey?: never }

export interface GameAssetEndpoint { locale: string; version: string; url: string }
export interface GameAssetManifest {
  id: string; game_id: string; name: GameLocalizedText; source: GameProvenance; endpoints: GameAssetEndpoint[];
  media_hosts: string[]; ttl_seconds: number; max_bytes: number; max_entries: number;
}
export interface GameAssetCache {
  provider: string; locale: string; version: string; disabled: boolean; source_url: string;
  content_hash?: string; entry_count: number; fetched_at?: string; checked_at?: string;
  expires_at?: string; attempted_at?: string; last_error_code?: string;
}
export interface GameAssetProvider { manifest: GameAssetManifest; caches: GameAssetCache[] }
export interface GameAssetQuery {
  provider: string; locale?: string; version?: string; q?: string; kind?: GameEntityKind; page?: number; limit?: number;
}
export interface GameAssetPage {
  provider: GameAssetManifest; cache: GameAssetCache; status: 'empty' | 'fresh' | 'stale' | 'disabled'; list: GameEntity[];
  count: number; page: number; limit: number;
}
export interface GameAssetVariant { locale?: string; version?: string }
export interface GameStoredFieldPolicy extends GameFieldPolicy {
  id: number; game_id: string; path: string; revision: number; updated_at: string;
}
export interface GameFieldPolicyInput extends GameFieldPolicy {
  path: string;
  /** Zero creates a policy; updates require the revision returned by the last read. */
  revision: number;
}
export interface GameArchivedSnapshot {
  provider: string; role_id: string; version: number; revision: number; status: GameDataStatus;
  stale: boolean; expires_at: string; snapshot: GameSnapshot;
}
export interface GameSnapshotMetadata {
  game_id: string; role_id: string; version: number; revision: number; status: GameDataStatus;
  stale: boolean; bytes: number; fetched_at: string; expires_at: string;
}
export interface GameArchiveHistory {
  id: number; game_id: string; role_id: string; revision: number; status: GameDataStatus; bytes: number; created_at: string;
}
export interface GamePolicyAudit {
  id: number; game_id: string; path: string; revision: number;
  /** JSON-encoded policies; previous is "null" for creation. */
  previous: string; current: string; created_at: string;
}
export interface GameArchivePageQuery { before?: number; limit?: number }

/** API errors retain the SDK's ApiError identity and add optional safe server metadata. */
export interface GameDataApiError extends ApiError {
  readonly status: number;
  readonly data: unknown;
  readonly retryAfter: string | null;
}

export function isGameDataApiError(error: unknown): error is GameDataApiError {
  return error instanceof ApiError && 'status' in error && 'data' in error && 'retryAfter' in error;
}

class GameJSONClient {
  private readonly base: string;
  private readonly timeout: number;
  private readonly fetchImpl: typeof fetch;
  private readonly token?: string;

  constructor(base: string, options: GameClientOptions | GamePublicClientOptions, authenticated: boolean) {
    const url = new URL(base);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
      throw new TypeError('base must be an HTTP(S) URL without credentials, query, or fragment');
    }
    this.base = base.replace(/\/+$/, '');
    if (options.serverKey !== undefined) throw new TypeError('game data routes do not accept a server key');
    if (authenticated) {
      if (typeof options.token !== 'string' || !options.token.trim() || /[\r\n]/.test(options.token)) {
        throw new TypeError('a site JWT is required');
      }
      this.token = options.token;
    } else if (options.token !== undefined) {
      throw new TypeError('public game data requests do not accept a token');
    }
    this.timeout = options.timeoutMs ?? 10_000;
    if (!Number.isSafeInteger(this.timeout) || this.timeout <= 0 || this.timeout > 2_147_483_647) {
      throw new RangeError('timeoutMs must be a positive 32-bit integer');
    }
    this.fetchImpl = options.fetch ?? fetch;
  }

  protected async call<T>(method: string, path: string, query: Record<string, string> = {},
    body?: unknown, options: GameRequestOptions = {}): Promise<T> {
    const controller = new AbortController();
    const cancel = () => controller.abort(options.signal?.reason);
    if (options.signal?.aborted) cancel();
    else options.signal?.addEventListener('abort', cancel, { once: true });
    const timer = setTimeout(() => controller.abort(new DOMException('Request timed out', 'TimeoutError')), this.timeout);
    try {
      if (controller.signal.aborted) throw controller.signal.reason;
      const url = new URL(this.base + path);
      for (const [key, value] of Object.entries(query)) url.searchParams.set(key, value);
      const response = await this.fetchImpl(url, {
        method, redirect: 'error', credentials: 'omit', signal: controller.signal,
        headers: { Accept: 'application/json', ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      const envelope: unknown = await response.json();
      if (controller.signal.aborted) throw controller.signal.reason;
      if (!envelope || typeof envelope !== 'object' || Array.isArray(envelope)
        || !Number.isSafeInteger((envelope as { code?: unknown }).code)) {
        throw new TransportError(`invalid API envelope (${response.status})`);
      }
      const env = envelope as { code: number; message?: unknown; data?: T };
      if (env.code !== 0) {
        throw Object.assign(new ApiError(env.code, typeof env.message === 'string' ? env.message : ''), {
          status: response.status, data: env.data, retryAfter: response.headers.get('Retry-After'),
        });
      }
      if (!response.ok || env.data === undefined || env.data === null) {
        throw new TransportError(`invalid API response (${response.status})`);
      }
      return env.data;
    } catch (error) {
      if (error instanceof ApiError || error instanceof TransportError) throw error;
      throw new TransportError(controller.signal.aborted ? 'game data request aborted' : 'game data request failed',
        controller.signal.aborted ? controller.signal.reason : error);
    } finally {
      clearTimeout(timer);
      options.signal?.removeEventListener('abort', cancel);
    }
  }
}

function providerPath(provider: GameProvider): string {
  if (provider !== 'mihoyo' && provider !== 'hypergryph') throw new RangeError('unknown game provider');
  return provider;
}

function textParameter(value: string, name: string, maxBytes: number, allowEmpty = false): string {
  if (typeof value !== 'string' || (!allowEmpty && !value) || /[\u0000-\u001f\u007f]/.test(value)
    || new TextEncoder().encode(value).length > maxBytes) throw new RangeError(`invalid ${name}`);
  return value;
}

function segment(value: string, name: string, maxBytes = 128): string {
  textParameter(value, name, maxBytes);
  if (value === '.' || value === '..') throw new RangeError(`invalid ${name}`);
  return encodeURIComponent(value);
}

function integer(value: number, name: string, minimum: number, maximum = Number.MAX_SAFE_INTEGER): string {
  if (!Number.isSafeInteger(value) || value < minimum || value > maximum) throw new RangeError(`invalid ${name}`);
  return String(value);
}

function snapshotQuery(query: GameSnapshotQuery, maxLimit: number, allowRole = true): Record<string, string> {
  const result: Record<string, string> = {};
  if (query.roleId !== undefined) {
    if (!allowRole) throw new TypeError('public snapshots do not accept a roleId');
    result.roleId = typeof query.roleId === 'number' ? integer(query.roleId, 'roleId', 1)
      : textParameter(query.roleId, 'roleId', 128);
  }
  if (query.sections !== undefined) {
    if (!Array.isArray(query.sections) || query.sections.length > 16) throw new RangeError('at most 16 sections are allowed');
    result.sections = query.sections.map(section => {
      textParameter(section, 'section', 64);
      if (section.includes(',')) throw new RangeError('section must not contain a comma');
      return section;
    }).join(',');
  }
  if (query.cursor !== undefined) result.cursor = textParameter(query.cursor, 'cursor', 2048, true);
  if (query.limit !== undefined) result.limit = integer(query.limit, 'limit', 1, maxLimit);
  if (query.locale !== undefined) {
    if (!['zh_CN', 'zh_TW', 'en', 'ja'].includes(query.locale)) throw new RangeError('invalid locale');
    result.locale = query.locale;
  }
  return result;
}

function assetQuery(query: GameAssetQuery): Record<string, string> {
  const result: Record<string, string> = { provider: textParameter(query.provider, 'provider', 96) };
  if (query.locale !== undefined) result.locale = textParameter(query.locale, 'locale', 16, true);
  if (query.version !== undefined) result.version = textParameter(query.version, 'version', 64, true);
  if (query.q !== undefined) result.q = textParameter(query.q, 'q', 128, true);
  if (query.kind !== undefined) {
    if (!['character', 'item', 'weapon', 'equipment', 'quest', 'achievement', 'other'].includes(query.kind)) {
      throw new RangeError('invalid asset kind');
    }
    result.kind = query.kind;
  }
  if (query.page !== undefined) result.page = integer(query.page, 'page', 1, 20000);
  if (query.limit !== undefined) result.limit = integer(query.limit, 'limit', 1, 100);
  return result;
}

/** Public reference data uses no JWT. Capabilities explain which data a game actually exposes. */
export class GameDataPublicClient extends GameJSONClient {
  constructor(base: string, options: GamePublicClientOptions = {}) { super(base, options, false); }

  games(provider: 'hypergryph', options?: GameRequestOptions): Promise<GameList> {
    return this.call('GET', `/api/${hypergryphOnly(provider)}/games`, {}, undefined, options);
  }

  snapshot(provider: 'hypergryph', gameId: string, query: Omit<GameSnapshotQuery, 'roleId'> = {},
    options?: GameRequestOptions): Promise<GameSnapshot> {
    return this.call('GET', `/api/${hypergryphOnly(provider)}/games/${segment(gameId, 'gameId', 64)}/snapshot`,
      snapshotQuery(query, 100, false), undefined, options);
  }
}

class GameArchiveClient extends GameJSONClient {
  constructor(base: string, options: GameClientOptions, private readonly scope: 'account' | 'admin') {
    super(base, options, true);
  }

  protected archivePath(provider: GameProvider, accountId: number): string {
    return `/api/${this.scope}/${providerPath(provider)}/accounts/${integer(accountId, 'accountId', 1)}/archive`;
  }

  archiveSnapshots(provider: GameProvider, accountId: number, options?: GameRequestOptions): Promise<GameSnapshotMetadata[]> {
    return this.call('GET', `${this.archivePath(provider, accountId)}/snapshots`, {}, undefined, options);
  }

  archivedSnapshot(provider: GameProvider, accountId: number, gameId: string, roleId = '',
    options?: GameRequestOptions): Promise<GameArchivedSnapshot> {
    return this.call('GET', `${this.archivePath(provider, accountId)}/snapshots`,
      { game_id: textParameter(gameId, 'gameId', 64), role_id: textParameter(roleId, 'roleId', 128, true) }, undefined, options);
  }

  fieldPolicies(provider: GameProvider, accountId: number, gameId: string,
    options?: GameRequestOptions): Promise<GameStoredFieldPolicy[]> {
    return this.call('GET', `${this.archivePath(provider, accountId)}/policies`,
      { game_id: textParameter(gameId, 'gameId', 64) }, undefined, options);
  }

  /** Newest first. Continue with before set to the last returned ID; each request is bounded. */
  syncHistory(provider: GameProvider, accountId: number, query: GameArchivePageQuery = {},
    options?: GameRequestOptions): Promise<GameArchiveHistory[]> {
    return this.call('GET', `${this.archivePath(provider, accountId)}/history`, archivePageQuery(query), undefined, options);
  }

  policyAudit(provider: GameProvider, accountId: number, query: GameArchivePageQuery = {},
    options?: GameRequestOptions): Promise<GamePolicyAudit[]> {
    return this.call('GET', `${this.archivePath(provider, accountId)}/policy-audit`, archivePageQuery(query), undefined, options);
  }
}

/** Owner routes use a site JWT. A login/OAuth token for another API and a server key are not substitutes. */
export class GameDataClient extends GameArchiveClient {
  constructor(base: string, options: GameClientOptions) { super(base, options, 'account'); }

  games(provider: 'mihoyo', options?: GameRequestOptions): Promise<GameDescriptor[]>;
  games(provider: 'hypergryph', options?: GameRequestOptions): Promise<GameAccountData>;
  games(provider: GameProvider, options?: GameRequestOptions): Promise<GameDescriptor[] | GameAccountData>;
  games(provider: GameProvider, options?: GameRequestOptions): Promise<GameDescriptor[] | GameAccountData> {
    return this.call('GET', `/api/account/${providerPath(provider)}/games`, {}, undefined, options);
  }

  roles(provider: 'mihoyo', options?: GameRequestOptions): Promise<MihoyoBoundRole[]>;
  roles(provider: 'hypergryph', options?: GameRequestOptions): Promise<GameBoundRole[]>;
  roles(provider: GameProvider, options?: GameRequestOptions): Promise<MihoyoBoundRole[] | GameBoundRole[]>;
  async roles(provider: GameProvider, options?: GameRequestOptions): Promise<MihoyoBoundRole[] | GameBoundRole[]> {
    const path = `/api/account/${providerPath(provider)}/${provider === 'mihoyo' ? 'status' : 'games'}`;
    const data = await this.call<{ roles: MihoyoBoundRole[] | GameBoundRole[] }>('GET', path, {}, undefined, options);
    if (!Array.isArray(data.roles)) throw new TransportError('invalid game roles response');
    return data.roles;
  }

  selectRole(provider: 'hypergryph', roleId: string, options?: GameRequestOptions): Promise<GameAccountData> {
    return this.call('PUT', `/api/account/${hypergryphOnly(provider)}/role`, {},
      { roleId: textParameter(roleId, 'roleId', 128) }, options);
  }

  /** Fetches one page only. Pass next_cursor back unchanged; no automatic retries or inventory inference. */
  snapshot(provider: GameProvider, gameId: string, query: GameSnapshotQuery = {},
    options?: GameRequestOptions): Promise<GameSnapshot> {
    return this.call('GET', `/api/account/${providerPath(provider)}/games/${segment(gameId, 'gameId', 64)}`
      + `/${provider === 'mihoyo' ? 'data' : 'snapshot'}`,
      snapshotQuery(query, 100), undefined, options);
  }

  assetProviders(options?: GameRequestOptions): Promise<GameAssetProvider[]> {
    return this.call('GET', '/api/account/game-assets/providers', {}, undefined, options);
  }

  /** Page numbers and totals are returned without merging public catalog entries into owned entities. */
  assets(query: GameAssetQuery, options?: GameRequestOptions): Promise<GameAssetPage> {
    return this.call('GET', '/api/account/game-assets/catalog', assetQuery(query), undefined, options);
  }
}

/** Site-JWT administration; the server still checks provider permissions and account/site ownership. */
export class GameDataAdminClient extends GameArchiveClient {
  constructor(base: string, options: GameClientOptions) { super(base, options, 'admin'); }

  games(provider: 'mihoyo', options?: GameRequestOptions): Promise<GameDescriptor[]>;
  games(provider: 'hypergryph', options?: GameRequestOptions): Promise<GameList>;
  games(provider: GameProvider, options?: GameRequestOptions): Promise<GameDescriptor[] | GameList>;
  games(provider: GameProvider, options?: GameRequestOptions): Promise<GameDescriptor[] | GameList> {
    return this.call('GET', `/api/admin/${providerPath(provider)}/games`, {}, undefined, options);
  }

  accountGames(provider: 'hypergryph', accountId: number, options?: GameRequestOptions): Promise<GameAccountData> {
    return this.call('GET', `/api/admin/${hypergryphOnly(provider)}/accounts/${integer(accountId, 'accountId', 1)}/games`,
      {}, undefined, options);
  }

  snapshot(provider: 'hypergryph', accountId: number, gameId: string, query: GameSnapshotQuery = {},
    options?: GameRequestOptions): Promise<GameSnapshot> {
    return this.call('GET', `/api/admin/${hypergryphOnly(provider)}/accounts/${integer(accountId, 'accountId', 1)}`
      + `/games/${segment(gameId, 'gameId', 64)}/snapshot`, snapshotQuery(query, 100), undefined, options);
  }

  assetProviders(options?: GameRequestOptions): Promise<GameAssetProvider[]> {
    return this.call('GET', '/api/admin/game-assets/providers', {}, undefined, options);
  }

  assets(query: GameAssetQuery, options?: GameRequestOptions): Promise<GameAssetPage> {
    return this.call('GET', '/api/admin/game-assets/catalog', assetQuery(query), undefined, options);
  }

  refreshAssets(provider: string, variant: GameAssetVariant = {}, options?: GameRequestOptions): Promise<GameAssetCache> {
    return this.call('POST', `/api/admin/game-assets/providers/${segment(provider, 'provider', 96)}/refresh`,
      {}, assetVariant(variant), options);
  }

  setAssetState(provider: string, disabled: boolean, variant: GameAssetVariant = {},
    options?: GameRequestOptions): Promise<GameAssetCache> {
    if (typeof disabled !== 'boolean') throw new TypeError('disabled must be a boolean');
    return this.call('PUT', `/api/admin/game-assets/providers/${segment(provider, 'provider', 96)}/state`,
      {}, { ...assetVariant(variant), disabled }, options);
  }

  /** Restricts source grants; an owner-only field cannot become public through this override. */
  setFieldPolicy(provider: GameProvider, accountId: number, gameId: string, input: GameFieldPolicyInput,
    options?: GameRequestOptions): Promise<GameStoredFieldPolicy> {
    const path = textParameter(input.path, 'field path', 320);
    if (!/^\/sections\/(?:[^/~]|~[01])+\/(?:entities\/(?:[^/~]|~[01])+\/)?fields\/(?:[^/~]|~[01])+$/.test(path)) {
      throw new RangeError('invalid field path');
    }
    if (!['owner', 'shared', 'public', 'secret'].includes(input.visibility) || typeof input.exportable !== 'boolean') {
      throw new TypeError('invalid field policy');
    }
    integer(input.revision, 'revision', 0);
    return this.call('PUT', `${this.archivePath(provider, accountId)}/policies`, {}, {
      game_id: textParameter(gameId, 'gameId', 64), path, visibility: input.visibility,
      exportable: input.exportable, revision: input.revision,
    }, options);
  }
}

function assetVariant(variant: GameAssetVariant): Record<string, string> {
  const result: Record<string, string> = {};
  if (variant.locale !== undefined) result.locale = textParameter(variant.locale, 'locale', 16, true);
  if (variant.version !== undefined) result.version = textParameter(variant.version, 'version', 64, true);
  return result;
}

function hypergryphOnly(provider: 'hypergryph'): string {
  if (provider !== 'hypergryph') throw new RangeError('this route is only available for hypergryph');
  return provider;
}

function archivePageQuery(query: GameArchivePageQuery): Record<string, string> {
  const result: Record<string, string> = {};
  if (query.before !== undefined) result.before = integer(query.before, 'before', 0);
  if (query.limit !== undefined) result.limit = integer(query.limit, 'limit', 1, 100);
  return result;
}

/** JSON-pointer field selector matching gamearchive.FieldPath, including escaped upstream IDs. */
export function gameFieldPath(sectionId: string, fieldKey: string, entityId?: string): string {
  const escape = (value: string) => textParameter(value, 'field path component', 320).replace(/~/g, '~0').replace(/\//g, '~1');
  const result = `/sections/${escape(sectionId)}${entityId === undefined ? '' : `/entities/${escape(entityId)}`}/fields/${escape(fieldKey)}`;
  return textParameter(result, 'field path', 320);
}
