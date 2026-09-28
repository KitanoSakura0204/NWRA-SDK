/**
 * Business logic: the events this receiver understands. Field names are exactly what the
 * user center emits (docs/PROTOCOL.md section 5); anything else stays in ev.data.
 */
import type { WebhookEvent } from '../../../typescript/src/index.js';

/** Review results carried by game_role.reviewed -> data.status. */
export const RoleStatus = { Pending: 0, Approved: 1, Rejected: 2, Banned: 3 } as const;

/**
 * data.user. An endpoint bound to a delivery profile with subject "pairwise" gets no
 * userId / uid / username / ip / externalId / externalName / email / phone; data.user.sub
 * identifies the account instead (DELIVERY.md section 2).
 */
export interface PairwiseUser {
  sub?: string;
}

interface GameRoleReviewed {
  roleId: number;
  userId: number;
  serverId: number;
  roleName: string;
  roleUuid: string;
  status: number;
  reviewerId: number;
}

interface UserDeleted {
  userId: number;
  uid: number;
  username: string;
  reason: string;
}

interface ServerOffline {
  serverId: number;
  code: string;
  name: string;
  lastHeartbeat: string | null; // "2006-01-02 15:04:05" or null
}

/** user.level_changed: the trust level moved from -> to. */
interface UserLevelChanged {
  userId?: number;
  uid?: number;
  username?: string;
  from: number;
  to: number;
  user?: PairwiseUser;
}

/** connection.bound / connection.unbound: a third-party account was linked or unlinked. */
interface ConnectionChanged {
  userId?: number;
  uid?: number;
  provider: string; // "discord", "github", ...
  externalId?: string;
  externalName?: string; // bound only
  user?: PairwiseUser;
}

/** Names the account in logs, whichever identifier the endpoint receives. */
export function who(uid: number | undefined, user: PairwiseUser | undefined): string {
  return user?.sub ? `sub=${user.sub}` : `uid=${uid ?? 0}`;
}

/** Keep this short (the sender waits at most 10 s); queue anything heavier. */
export function dispatch(ev: WebhookEvent): void {
  const time = new Date(ev.timestamp * 1000);
  const at = Number.isNaN(time.getTime()) ? String(ev.timestamp) : time.toISOString();
  switch (ev.event) {
    case 'game_role.reviewed': {
      const d = ev.data as unknown as GameRoleReviewed;
      const role = `role="${d.roleName}" uuid=${d.roleUuid} user=${d.userId} server=${d.serverId} reviewer=${d.reviewerId}`;
      switch (d.status) {
        case RoleStatus.Approved:
          console.info(`[${at}] role APPROVED: ${role}`);
          // e.g. add the player to the server whitelist / grant a rank
          break;
        case RoleStatus.Rejected:
          console.info(`[${at}] role REJECTED: ${role}`);
          break;
        case RoleStatus.Banned:
          console.info(`[${at}] role BANNED: ${role}`);
          // e.g. kick the player if online
          break;
        case RoleStatus.Pending:
          console.info(`[${at}] role back to PENDING: ${role}`);
          break;
        default:
          console.info(`[${at}] role status ${d.status} (unknown): ${role}`);
      }
      break;
    }
    case 'user.deleted': {
      const d = ev.data as unknown as UserDeleted;
      console.info(`[${at}] user DELETED: userId=${d.userId} uid=${d.uid} username="${d.username}" reason="${d.reason}"`);
      // e.g. purge local data tied to this account
      break;
    }
    case 'server.offline': {
      const d = ev.data as unknown as ServerOffline;
      console.info(`[${at}] server OFFLINE: code=${d.code} name="${d.name}" id=${d.serverId} lastHeartbeat=${d.lastHeartbeat ?? 'null'}`);
      // e.g. page the on-call channel
      break;
    }
    case 'user.level_changed': {
      const d = ev.data as unknown as UserLevelChanged;
      console.info(`[${at}] trust level ${d.from} -> ${d.to}: ${who(d.uid, d.user)}`);
      // e.g. re-check a rank that requires a minimum trust level
      break;
    }
    case 'connection.bound':
    case 'connection.unbound': {
      const d = ev.data as unknown as ConnectionChanged;
      console.info(`[${at}] ${ev.event} provider=${d.provider} external="${d.externalId ?? ''}": ${who(d.uid, d.user)}`);
      // e.g. grant or revoke a Discord role (see ../discord-role-sync)
      break;
    }
    case 'ping':
      // sent by the "测试" (Test) button in the admin console
      console.info(`[${at}] ping from console: ${JSON.stringify(ev.data)}`);
      break;
    default:
      console.info(`[${at}] event ${ev.event} ignored (id=${ev.id})`);
  }
}
