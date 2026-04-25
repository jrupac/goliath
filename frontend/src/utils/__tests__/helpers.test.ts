import { describe, expect, it } from 'vitest';
import { getAdjacentFeed, getAdjacentFolder } from '../helpers';
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
