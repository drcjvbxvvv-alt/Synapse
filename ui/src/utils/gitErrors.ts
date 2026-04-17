import type { TFunction } from 'i18next';
import { parseApiError } from './api';

/**
 * Map backend git connection / repo validation errors to i18n messages.
 * Falls back to the raw error string if no pattern matches.
 */
export function translateGitError(error: unknown, t: TFunction): string {
  const raw = parseApiError(error);
  if (!raw) return '';

  // connection failed: cannot reach {url}: ...
  const unreachable = raw.match(/cannot reach ([^:]+)/);
  if (unreachable) {
    return t('cicd:connectionErrors.unreachable', { url: unreachable[1] });
  }

  // authentication failed
  if (raw.includes('authentication failed') || raw.includes('invalid access token')) {
    return t('cicd:connectionErrors.authFailed');
  }

  // access denied / token lacks permissions
  if (raw.includes('access denied') || raw.includes('lacks required permissions')) {
    return t('cicd:connectionErrors.accessDenied');
  }

  // repo not found
  const notFound = raw.match(/repository "?([^"]+)"? not found/);
  if (notFound) {
    return t('cicd:connectionErrors.repoNotFound', { repo: notFound[1] });
  }

  // GitLab page path
  if (raw.includes('GitLab page path')) {
    return t('cicd:connectionErrors.repoUrlGitlabPath');
  }

  // file/tree/commit path
  if (raw.includes('file/tree/commit path')) {
    return t('cicd:connectionErrors.repoUrlFilePath');
  }

  // duplicate name
  if (raw.includes('name already exists')) {
    return t('cicd:connectionErrors.duplicateName');
  }

  // duplicate repo
  if (raw.includes('already exists')) {
    return t('cicd:connectionErrors.duplicateRepo');
  }

  // unexpected HTTP status
  const httpStatus = raw.match(/HTTP (\d+)/);
  if (httpStatus) {
    return t('cicd:connectionErrors.unexpectedStatus', { status: httpStatus[1] });
  }

  // fallback: return raw error
  return raw;
}
