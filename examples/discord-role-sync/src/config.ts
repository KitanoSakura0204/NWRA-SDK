/** Configuration from the environment (see README.md). Parsed once at start; a bad value stops the bot. */
import { SNOWFLAKE } from './discord.js';

export interface LevelRole {
  level: number;
  roleId: string;
}

export interface Config {
  secret: string;
  keysPath: string;
  tolerance: number;
  host: string;
  port: number;
  path: string;
  seenCapacity: number;
  botToken: string;
  guildId: string;
  /** Given to every member whose Lounge account is linked; empty: none. */
  linkedRoleId: string;
  /** "Level N and above" roles, ascending by level. */
  levelRoles: LevelRole[];
  linksFile: string;
  dryRun: boolean;
}

/** "2:111…,3:222…" -> [{ level: 2, roleId }, …]; each role is held from its level upwards. */
export function parseLevelRoles(spec: string): LevelRole[] {
  const out: LevelRole[] = [];
  for (const part of spec.split(',').map((s) => s.trim()).filter(Boolean)) {
    const m = /^(\d+):(\d{17,20})$/.exec(part);
    if (!m) throw new Error(`LEVEL_ROLES: "${part}" is not <level>:<role id>`);
    if (out.some((r) => r.roleId === m[2])) throw new Error(`LEVEL_ROLES: role ${m[2]} is listed twice`);
    out.push({ level: Number(m[1]), roleId: m[2] });
  }
  return out.sort((a, b) => a.level - b.level);
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  const need = (name: string, hint: string): string => {
    const v = (env[name] ?? '').trim();
    if (!v) throw new Error(`set ${name} (${hint})`);
    return v;
  };
  const snowflake = (name: string, v: string): string => {
    if (v && !SNOWFLAKE.test(v)) throw new Error(`${name}: "${v}" is not a Discord id`);
    return v;
  };
  const secret = need('LOUNGE_WEBHOOK_SECRET', 'admin console: 开放平台 -> Webhook -> 签名密钥');
  const dryRun = env.DRY_RUN === '1';
  const botToken = dryRun ? '' : need('DISCORD_BOT_TOKEN', 'Discord Developer Portal -> your application -> Bot -> Reset Token');
  const guildId = snowflake('DISCORD_GUILD_ID', need('DISCORD_GUILD_ID', 'Discord Developer Mode -> right-click the server -> Copy Server ID'));
  const linkedRoleId = snowflake('LINKED_ROLE_ID', (env.LINKED_ROLE_ID ?? '').trim());
  const levelRoles = parseLevelRoles(env.LEVEL_ROLES ?? '');
  if (!linkedRoleId && levelRoles.length === 0) throw new Error('set LINKED_ROLE_ID and / or LEVEL_ROLES: there is no role to manage');
  if (levelRoles.some((r) => r.roleId === linkedRoleId)) throw new Error('LEVEL_ROLES: use a different role than LINKED_ROLE_ID');
  return {
    secret,
    keysPath: env.LOUNGE_DECRYPTION_KEYS ?? '',
    tolerance: Number(env.WEBHOOK_TOLERANCE ?? 300),
    host: env.HOST ?? '0.0.0.0',
    port: Number(env.PORT ?? 8093),
    path: env.WEBHOOK_PATH ?? '/lounge/webhook',
    seenCapacity: Number(env.SEEN_CAPACITY ?? 10000),
    botToken,
    guildId,
    linkedRoleId,
    levelRoles,
    linksFile: env.LINKS_FILE ?? 'links.json',
    dryRun,
  };
}
