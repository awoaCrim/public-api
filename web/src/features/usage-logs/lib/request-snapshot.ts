/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import axios from 'axios'

import { api } from '@/lib/api'

/**
 * The backend permission resource/action that gates request snapshot reads.
 * Mirrors service/authz/resources_requestsnapshot.go. There is intentionally
 * no default admin grant; only the root superuser (implicit) and explicitly
 * granted operators can view request bodies.
 */
export const REQUEST_SNAPSHOT_RESOURCE = 'request_snapshot'
export const REQUEST_SNAPSHOT_READ_ACTION = 'read'

/** Secondary verification scope required to decrypt a snapshot. */
export const REQUEST_SNAPSHOT_PROOF_SCOPE = 'request_snapshot.read'

export interface RequestSnapshotPayload {
  request_id: string
  content_type: string
  size: number
  content_base64: string
}

/**
 * Structural subset of the authenticated dashboard user used for the gating
 * check. Kept local (instead of importing the auth store) so this module stays
 * dependency-free and cannot participate in import cycles.
 */
export interface SnapshotViewerUser {
  id?: number
  username?: string
  role?: number
  permissions?: {
    admin_permissions?: Record<string, Record<string, boolean>>
  }
}

/** Mirrors ROLE.SUPER_ADMIN from @/lib/roles. */
const SUPER_ADMIN_ROLE = 100

function userHasSnapshotRead(
  user: SnapshotViewerUser | null | undefined
): boolean {
  if (!user) return false
  if (user.role === SUPER_ADMIN_ROLE) return true
  return (
    user.permissions?.admin_permissions?.[REQUEST_SNAPSHOT_RESOURCE]?.[
      REQUEST_SNAPSHOT_READ_ACTION
    ] === true
  )
}

/**
 * Gating contract for the "View Request Body" control: it is shown only to
 * admins (role check), only when the log row carries a request id, and only
 * when the current user holds the request_snapshot.read permission. Root users
 * are implicitly allowed (superuser role).
 */
export function canViewRequestSnapshot(
  user: SnapshotViewerUser | null | undefined,
  isAdmin: boolean,
  requestId: string | undefined | null
): boolean {
  if (!isAdmin) return false
  if (!requestId) return false
  return userHasSnapshotRead(user)
}

/**
 * Decodes the base64 snapshot payload into its exact original bytes. The
 * backend preserves the captured bytes exactly, so the download path uses
 * these bytes (never the base64 text) as the file content.
 */
export function decodeSnapshotContent(
  payload: RequestSnapshotPayload
): Uint8Array<ArrayBuffer> {
  const binary = atob(payload.content_base64)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes
}

/**
 * Renders the captured bytes as text for display. Arbitrary binary content is
 * shown lossily (UTF-8 replacement characters), while JSON/text request bodies
 * display exactly.
 */
export function snapshotBytesToText(bytes: Uint8Array): string {
  return new TextDecoder('utf-8').decode(bytes)
}

/**
 * Builds a downloadable file name from the request id. The raw request id may
 * contain characters unsafe for file names, so it is sanitized defensively.
 */
export function snapshotFileName(requestId: string): string {
  const sanitized = requestId.replaceAll(/[^A-Za-z0-9._-]/g, '_')
  return `${sanitized || 'request'}-body.txt`
}

/**
 * Maps stable backend snapshot error codes to i18n keys so the UI can present
 * localized messages instead of raw server text.
 */
export function snapshotErrorKey(code: string | undefined): string | null {
  switch (code) {
    case 'SNAPSHOT_NOT_FOUND':
      return 'Snapshot not found'
    case 'SNAPSHOT_DELETED':
      return 'Snapshot deleted'
    case 'SNAPSHOT_MISSING':
      return 'Snapshot file missing'
    case 'SNAPSHOT_UNAVAILABLE':
      return 'Snapshot unavailable'
    case 'SNAPSHOT_CORRUPT':
      return 'Snapshot corrupt'
    case 'SNAPSHOT_WRONG_NODE':
      return 'Snapshot stored on another node'
    case 'SNAPSHOT_AUDIT_FAILED':
      return 'Snapshot access could not be audited'
    case 'SNAPSHOT_READ_FAILED':
      return 'Snapshot could not be read'
    default:
      return null
  }
}

export interface RequestSnapshotResponse {
  success: boolean
  code?: string
  message?: string
  data?: RequestSnapshotPayload
}

/**
 * Fetches the captured body of a request. The endpoint requires an admin
 * session, the request_snapshot.read permission, and a secondary security
 * proof for scope request_snapshot.read.
 */
export async function getRequestSnapshot(
  requestId: string,
  proofToken: string
): Promise<RequestSnapshotResponse> {
  try {
    const res = await api.get(
      `/api/log/${encodeURIComponent(requestId)}/snapshot`,
      {
        headers: { 'X-Security-Proof': proofToken },
        disableDuplicate: true,
        skipBusinessError: true,
        skipErrorHandler: true,
      }
    )
    return res.data
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.data) {
      return error.response.data as RequestSnapshotResponse
    }
    throw error
  }
}
