/**
 * Lightweight URL routing utilities for Bob chat sessions.
 * Supports /c/:id, /chat/:id, and hash-based routes #/c/:id.
 */

export function getSessionIdFromUrl(): string | null {
  if (typeof window === 'undefined') return null;

  // 1. Check pathname: /c/:id or /chat/:id
  const pathname = window.location.pathname;
  const match = pathname.match(/^\/(?:c|chat)\/([a-zA-Z0-9_-]+)/);
  if (match && match[1]) {
    return match[1];
  }

  // 2. Check hash: #/c/:id or #/chat/:id or #/:id
  const hash = window.location.hash;
  if (hash) {
    const hashMatch = hash.match(/^#\/?(?:c|chat)?\/([a-zA-Z0-9_-]+)/);
    if (hashMatch && hashMatch[1]) {
      return hashMatch[1];
    }
  }

  return null;
}

export function navigateToChat(sessionId: string, replace: boolean = false): void {
  if (typeof window === 'undefined') return;
  const targetUrl = `/c/${sessionId}`;
  if (window.location.pathname === targetUrl) return;

  if (replace) {
    window.history.replaceState({ sessionId }, '', targetUrl);
  } else {
    window.history.pushState({ sessionId }, '', targetUrl);
  }
}

export function navigateToNewChat(replace: boolean = false): void {
  if (typeof window === 'undefined') return;
  const targetUrl = '/';
  if (window.location.pathname === targetUrl && !window.location.hash) return;

  if (replace) {
    window.history.replaceState({ sessionId: null }, '', targetUrl);
  } else {
    window.history.pushState({ sessionId: null }, '', targetUrl);
  }
}
