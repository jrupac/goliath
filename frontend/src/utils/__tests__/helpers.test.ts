import { describe, expect, it } from 'vitest';
import {
  getAdjacentFeed,
  getAdjacentFolder,
  getFeedInitials,
  hashToSwatchIndex,
} from '../helpers';
import { NavigationDirection } from '../types';
import { FeedView } from '../../models/feed';
import { FolderView } from '../../models/folder';

const makeFolder = (id: string, title: string, unread: number): FolderView => ({
  id,
  title,
  unread_count: unread,
});

const makeFeed = (id: string, folderId: string, unread: number): FeedView => ({
  id,
  folder_id: folderId,
  favicon: null,
  title: `Feed ${id}`,
  unread_count: unread,
});

describe('getAdjacentFeed', () => {
  it('returns next unread feed in same folder', () => {
    const folder = makeFolder('f1', 'Folder 1', 3);
    const feed1 = makeFeed('feed1', 'f1', 2);
    const feed2 = makeFeed('feed2', 'f1', 1);
    const map = new Map<FolderView, FeedView[]>([[folder, [feed1, feed2]]]);

    const result = getAdjacentFeed(map, 'feed1', NavigationDirection.Next);
    expect(result).toEqual(['feed2', 'f1']);
  });

  it('returns next unread feed in a different folder', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 2);
    const folder2 = makeFolder('f2', 'Folder 2', 3);
    const feed1 = makeFeed('feed1', 'f1', 2);
    const feed2 = makeFeed('feed2', 'f2', 3);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, [feed1]],
      [folder2, [feed2]],
    ]);

    const result = getAdjacentFeed(map, 'feed1', NavigationDirection.Next);
    expect(result).toEqual(['feed2', 'f2']);
  });

  it('skips feeds with zero unread when going next', () => {
    const folder = makeFolder('f1', 'Folder 1', 1);
    const feed1 = makeFeed('feed1', 'f1', 1);
    const feed2 = makeFeed('feed2', 'f1', 0);
    const feed3 = makeFeed('feed3', 'f1', 5);
    const map = new Map<FolderView, FeedView[]>([
      [folder, [feed1, feed2, feed3]],
    ]);

    const result = getAdjacentFeed(map, 'feed1', NavigationDirection.Next);
    expect(result).toEqual(['feed3', 'f1']);
  });

  it('returns null when no next unread feed exists', () => {
    const folder = makeFolder('f1', 'Folder 1', 1);
    const feed1 = makeFeed('feed1', 'f1', 1);
    const feed2 = makeFeed('feed2', 'f1', 0);
    const map = new Map<FolderView, FeedView[]>([[folder, [feed1, feed2]]]);

    const result = getAdjacentFeed(map, 'feed1', NavigationDirection.Next);
    expect(result).toBeNull();
  });

  it('returns null when current feed is the last', () => {
    const folder = makeFolder('f1', 'Folder 1', 1);
    const feed1 = makeFeed('feed1', 'f1', 1);
    const map = new Map<FolderView, FeedView[]>([[folder, [feed1]]]);

    const result = getAdjacentFeed(map, 'feed1', NavigationDirection.Next);
    expect(result).toBeNull();
  });

  it('returns previous unread feed', () => {
    const folder = makeFolder('f1', 'Folder 1', 3);
    const feed1 = makeFeed('feed1', 'f1', 2);
    const feed2 = makeFeed('feed2', 'f1', 1);
    const map = new Map<FolderView, FeedView[]>([[folder, [feed1, feed2]]]);

    const result = getAdjacentFeed(map, 'feed2', NavigationDirection.Prev);
    expect(result).toEqual(['feed1', 'f1']);
  });

  it('returns previous unread feed across folder boundaries', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 2);
    const folder2 = makeFolder('f2', 'Folder 2', 3);
    const feed1 = makeFeed('feed1', 'f1', 2);
    const feed2 = makeFeed('feed2', 'f2', 3);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, [feed1]],
      [folder2, [feed2]],
    ]);

    const result = getAdjacentFeed(map, 'feed2', NavigationDirection.Prev);
    expect(result).toEqual(['feed1', 'f1']);
  });

  it('skips feeds with zero unread when going prev', () => {
    const folder = makeFolder('f1', 'Folder 1', 1);
    const feed1 = makeFeed('feed1', 'f1', 5);
    const feed2 = makeFeed('feed2', 'f1', 0);
    const feed3 = makeFeed('feed3', 'f1', 1);
    const map = new Map<FolderView, FeedView[]>([
      [folder, [feed1, feed2, feed3]],
    ]);

    const result = getAdjacentFeed(map, 'feed3', NavigationDirection.Prev);
    expect(result).toEqual(['feed1', 'f1']);
  });

  it('returns null when no previous unread feed exists', () => {
    const folder = makeFolder('f1', 'Folder 1', 1);
    const feed1 = makeFeed('feed1', 'f1', 0);
    const feed2 = makeFeed('feed2', 'f1', 1);
    const map = new Map<FolderView, FeedView[]>([[folder, [feed1, feed2]]]);

    const result = getAdjacentFeed(map, 'feed2', NavigationDirection.Prev);
    expect(result).toBeNull();
  });

  it('returns null when currentFeedId is not in the map', () => {
    const folder = makeFolder('f1', 'Folder 1', 1);
    const feed1 = makeFeed('feed1', 'f1', 1);
    const map = new Map<FolderView, FeedView[]>([[folder, [feed1]]]);

    const result = getAdjacentFeed(
      map,
      'nonexistent',
      NavigationDirection.Next
    );
    expect(result).toBeNull();
  });

  it('returns null on an empty map', () => {
    const map = new Map<FolderView, FeedView[]>();
    const result = getAdjacentFeed(map, 'feed1', NavigationDirection.Next);
    expect(result).toBeNull();
  });
});

describe('getAdjacentFolder', () => {
  it('returns the next unread folder', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 2);
    const folder2 = makeFolder('f2', 'Folder 2', 3);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, []],
      [folder2, []],
    ]);

    const result = getAdjacentFolder(map, 'f1', NavigationDirection.Next);
    expect(result).toBe('f2');
  });

  it('skips folders with zero unread when going next', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 1);
    const folder2 = makeFolder('f2', 'Folder 2', 0);
    const folder3 = makeFolder('f3', 'Folder 3', 4);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, []],
      [folder2, []],
      [folder3, []],
    ]);

    const result = getAdjacentFolder(map, 'f1', NavigationDirection.Next);
    expect(result).toBe('f3');
  });

  it('returns null when no next unread folder exists', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 1);
    const folder2 = makeFolder('f2', 'Folder 2', 0);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, []],
      [folder2, []],
    ]);

    const result = getAdjacentFolder(map, 'f1', NavigationDirection.Next);
    expect(result).toBeNull();
  });

  it('returns the previous unread folder', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 2);
    const folder2 = makeFolder('f2', 'Folder 2', 3);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, []],
      [folder2, []],
    ]);

    const result = getAdjacentFolder(map, 'f2', NavigationDirection.Prev);
    expect(result).toBe('f1');
  });

  it('skips folders with zero unread when going prev', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 5);
    const folder2 = makeFolder('f2', 'Folder 2', 0);
    const folder3 = makeFolder('f3', 'Folder 3', 1);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, []],
      [folder2, []],
      [folder3, []],
    ]);

    const result = getAdjacentFolder(map, 'f3', NavigationDirection.Prev);
    expect(result).toBe('f1');
  });

  it('returns null when no previous unread folder exists', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 0);
    const folder2 = makeFolder('f2', 'Folder 2', 1);
    const map = new Map<FolderView, FeedView[]>([
      [folder1, []],
      [folder2, []],
    ]);

    const result = getAdjacentFolder(map, 'f2', NavigationDirection.Prev);
    expect(result).toBeNull();
  });

  it('returns null when currentFolderId is not in the map', () => {
    const folder1 = makeFolder('f1', 'Folder 1', 1);
    const map = new Map<FolderView, FeedView[]>([[folder1, []]]);

    const result = getAdjacentFolder(
      map,
      'nonexistent',
      NavigationDirection.Next
    );
    expect(result).toBeNull();
  });

  it('returns null on an empty map', () => {
    const map = new Map<FolderView, FeedView[]>();
    const result = getAdjacentFolder(map, 'f1', NavigationDirection.Next);
    expect(result).toBeNull();
  });
});

describe('getFeedInitials', () => {
  it('strips text after colon', () => {
    expect(getFeedInitials('MacRumors: Mac News and Rumors')).toBe('MR');
  });

  it('strips text after dash', () => {
    expect(getFeedInitials('Ars Technica - News and Info')).toBe('AT');
  });

  it('splits on whichever separator comes first (colon or dash)', () => {
    // Dash comes first at index 11, colon at 34 → split at dash
    expect(getFeedInitials('The Verge - Tech Coverage: Reviews')).toBe('VE');
    // Colon comes first at index 9, dash at 21 → split at colon
    expect(getFeedInitials('Verge News: The Blog - Reviews')).toBe('VN');
  });

  it('filters stop words: the, a, an, of, for', () => {
    expect(getFeedInitials('The GitHub Blog')).toBe('GH');
    expect(getFeedInitials('A Swift Journey')).toBe('SJ');
    expect(getFeedInitials('News of the Week')).toBe('NW');
  });

  it('applies CamelCase rule: extracts first two uppercase letters', () => {
    expect(getFeedInitials('MacRumors')).toBe('MR');
    expect(getFeedInitials('GitHub')).toBe('GH');
    expect(getFeedInitials('iTunes')).toBe('IT');
    expect(getFeedInitials('PDFMonkey')).toBe('PD');
  });

  it('applies multi-word rule when no CamelCase words exist', () => {
    expect(getFeedInitials('Ars Technica')).toBe('AT');
    expect(getFeedInitials('Daily Cartoon')).toBe('DC');
    expect(getFeedInitials('Android Police')).toBe('AP');
  });

  it('applies single-word rule when exactly one candidate word remains', () => {
    expect(getFeedInitials('Engadget')).toBe('EN');
    expect(getFeedInitials('xkcd')).toBe('XK');
    expect(getFeedInitials('Reddit')).toBe('RE');
  });

  it('handles a single-character word', () => {
    expect(getFeedInitials('A')).toBe('AA');
  });

  it('falls back to first two characters for edge cases', () => {
    expect(getFeedInitials('1')).toBe('11');
  });

  it('returns empty string for empty input', () => {
    expect(getFeedInitials('')).toBe('');
  });

  it('handles stop-word-only input', () => {
    expect(getFeedInitials('the a an of for')).toBe('TH');
  });

  it('handles all stop words plus one word', () => {
    expect(getFeedInitials('the a an of for Wirecutter')).toBe('WI');
  });
});

describe('hashToSwatchIndex', () => {
  it('returns a value between 0 and 7 inclusive', () => {
    for (let i = 0; i < 1000; i++) {
      const idx = hashToSwatchIndex(`feed-${i}`);
      expect(idx).toBeGreaterThanOrEqual(0);
      expect(idx).toBeLessThan(8);
    }
  });

  it('returns deterministic results for the same input', () => {
    const id = 'feed-12345';
    expect(hashToSwatchIndex(id)).toBe(hashToSwatchIndex(id));
  });

  it('returns different indices for different inputs', () => {
    const indices = new Set<number>();
    for (let i = 0; i < 100; i++) {
      indices.add(hashToSwatchIndex(`feed-${i}`));
    }
    // With 8 buckets and 100 inputs, expect at least a few distinct values
    expect(indices.size).toBeGreaterThan(1);
  });

  it('returns 5 for empty string (djb2 base hash)', () => {
    expect(hashToSwatchIndex('')).toBe(5);
  });
});
