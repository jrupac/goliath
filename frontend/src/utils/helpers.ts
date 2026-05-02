import moment from 'moment';
import { Decimal } from 'decimal.js-light';
import { Readability } from '@mozilla/readability';
import * as LosslessJSON from 'lossless-json';

import { ArticleId, ArticleView } from '../models/article';
import { FeedId, FeedView } from '../models/feed';
import { FolderId, FolderView } from '../models/folder';
import {
  ArticleImagePreview,
  FeedSelection,
  FolderSelection,
  GoliathTheme,
  NavigationDirection,
  ThemeInfo,
} from './types';
import { createTheme, darkScrollbar, PaletteMode, Theme } from '@mui/material';

export function extractText(html: string): string | null {
  return new DOMParser().parseFromString(html, 'text/html').documentElement
    .textContent;
}

export function makeAbsolute(url: string): string {
  const a = document.createElement('a');
  a.href = url;
  return a.href;
}

export function formatFull(date: Date) {
  return moment(date).format('dddd, MMMM Do YYYY, h:mm:ss A');
}

export function formatFriendly(date: Date) {
  const now = moment();
  const before = moment(now).subtract(1, 'days');
  const then = moment(date);
  if (then.isBetween(before, now)) {
    return then.fromNow();
  } else {
    return then.format('ddd, MMM D, h:mm A');
  }
}

export function maxDecimal(
  a: Decimal | ArticleId,
  b: Decimal | ArticleId
): Decimal {
  a = new Decimal(a.toString());
  b = new Decimal(b.toString());
  return a.greaterThan(b) ? a : b;
}

export function fetchReadability(url: string): Promise<string> {
  return new Promise((resolve, reject) => {
    fetch(url)
      .then((response) => response.text())
      .then((result) => {
        const doc = new DOMParser().parseFromString(result, 'text/html');
        const parsed = new Readability(doc).parse();

        if (!parsed || !parsed.content) {
          reject(new Error('Could not parse document'));
        } else {
          resolve(parsed.content);
        }
      })
      .catch(reject);
  });
}

export function parseJson(text: string): any {
  // Parse as Lossless numbers since values from the server are 64-bit
  // Integer, but then convert back to String for use going forward.
  return LosslessJSON.parse(text, (_: string, v: any) => {
    if (v && v.isLosslessNumber) {
      return String(v);
    }
    return v;
  });
}

export function cookieExists(name: string): boolean {
  const cookies: string[] = document.cookie.split(';');
  return cookies.some((cookie: string) => cookie.trim().startsWith(`${name}=`));
}

export function populateThemeInfo(themeSetting: GoliathTheme): ThemeInfo {
  let themeClasses: string, paletteMode: PaletteMode;

  if (themeSetting === GoliathTheme.Default) {
    themeClasses = 'default-theme';
    paletteMode = 'light';
  } else {
    themeClasses = 'dark-theme';
    paletteMode = 'dark';
  }

  const theme: Theme = createTheme({
    palette: {
      mode: paletteMode,
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          body: paletteMode === 'dark' ? darkScrollbar() : null,
        },
      },
    },
  });
  return { themeClasses, theme };
}

export function getAdjacentFeed(
  folderFeedView: Map<FolderView, FeedView[]>,
  currentFeedId: FeedId,
  direction: NavigationDirection
): FeedSelection | null {
  const flatFeeds: Array<{
    feedId: FeedId;
    folderId: FolderId;
    unreadCount: number;
  }> = [];
  folderFeedView.forEach((feeds) => {
    feeds.forEach((feed) => {
      flatFeeds.push({
        feedId: feed.id,
        folderId: feed.folder_id,
        unreadCount: feed.unread_count,
      });
    });
  });

  const currentIdx = flatFeeds.findIndex((f) => f.feedId === currentFeedId);
  if (currentIdx === -1) return null;

  if (direction === NavigationDirection.Next) {
    for (let i = currentIdx + 1; i < flatFeeds.length; i++) {
      if (flatFeeds[i].unreadCount > 0) {
        return [flatFeeds[i].feedId, flatFeeds[i].folderId];
      }
    }
  } else {
    for (let i = currentIdx - 1; i >= 0; i--) {
      if (flatFeeds[i].unreadCount > 0) {
        return [flatFeeds[i].feedId, flatFeeds[i].folderId];
      }
    }
  }

  return null;
}

export function getAdjacentFolder(
  folderFeedView: Map<FolderView, FeedView[]>,
  currentFolderId: FolderId,
  direction: NavigationDirection
): FolderSelection | null {
  const flatFolders = Array.from(folderFeedView.keys());
  const currentIdx = flatFolders.findIndex((f) => f.id === currentFolderId);
  if (currentIdx === -1) return null;

  if (direction === NavigationDirection.Next) {
    for (let i = currentIdx + 1; i < flatFolders.length; i++) {
      if (flatFolders[i].unread_count > 0) {
        return flatFolders[i].id;
      }
    }
  } else {
    for (let i = currentIdx - 1; i >= 0; i--) {
      if (flatFolders[i].unread_count > 0) {
        return flatFolders[i].id;
      }
    }
  }

  return null;
}

const STOP_WORDS = new Set(['the', 'a', 'an', 'of', 'for']);

export function getFeedInitials(title: string): string {
  let s = title;
  const firstColon = s.indexOf(':');
  const firstDash = s.indexOf('-');
  const splitIdx = Math.min(
    firstColon >= 0 ? firstColon : s.length,
    firstDash >= 0 ? firstDash : s.length
  );
  s = s.slice(0, splitIdx).trim();

  const words = s.split(/\s+/).filter(Boolean);
  const candidates = words.filter((w) => !STOP_WORDS.has(w.toLowerCase()));

  function isCapitalized(w: string): boolean {
    return (
      w.length > 0 && w[0] === w[0].toUpperCase() && w[0] >= 'A' && w[0] <= 'Z'
    );
  }

  function hasMultipleCaps(w: string): boolean {
    let count = 0;
    for (const ch of w) {
      if (ch >= 'A' && ch <= 'Z' && count++ >= 1) return true;
    }
    return false;
  }

  const filtered = candidates.filter((w) => isCapitalized(w));
  const pool = filtered.length > 0 ? filtered : candidates;

  // CamelCase rule
  for (const w of pool) {
    if (hasMultipleCaps(w)) {
      const caps = w.match(/[A-Z]/g);
      return caps ? caps.slice(0, 2).join('') : '';
    }
  }

  // Multi-word rule
  if (pool.length >= 2) {
    return (pool[0][0] + pool[1][0]).toUpperCase();
  }

  // Single-word rule
  if (pool.length === 1) {
    const w = pool[0];
    return (w[0] + (w.length > 1 ? w[1] : w[0])).toUpperCase();
  }

  // Fallback
  if (!title) return '';
  return (title[0] + (title.length > 1 ? title[1] : title[0])).toUpperCase();
}

export function hashToSwatchIndex(id: string): number {
  let h = 5381;
  for (let i = 0; i < id.length; i++) {
    h = (h * 33) ^ id.charCodeAt(i);
  }
  return Math.abs(h) % 8;
}

export async function getPreviewImage(
  article: ArticleView
): Promise<ArticleImagePreview | undefined> {
  const minPixelSize = 100;

  let imageElements: HTMLCollectionOf<HTMLImageElement>;
  try {
    imageElements = new DOMParser().parseFromString(
      article.html,
      'text/html'
    ).images;
  } catch {
    return;
  }

  if (!imageElements || imageElements.length === 0) {
    return;
  }

  const candidates = Array.from(imageElements)
    .filter(
      (img) =>
        img.src && (img.src.startsWith('http') || img.src.startsWith('data:'))
    )
    .slice(0, 3)
    .map((img) => img.src);

  if (candidates.length === 0) {
    return;
  }

  const results = await Promise.all(
    candidates.map(
      (src) =>
        new Promise<{ src: string; valid: boolean }>((resolve) => {
          const img = new Image();
          img.onload = () =>
            resolve({
              src,
              valid:
                img.naturalWidth >= minPixelSize &&
                img.naturalHeight >= minPixelSize,
            });
          img.onerror = () => resolve({ src, valid: false });
          img.src = src;
        })
    )
  );

  const first = results.find((r) => r.valid);
  if (!first) return undefined;

  return { src: first.src, x: 0, y: 0, width: 0, height: 0, origWidth: 0 };
}
