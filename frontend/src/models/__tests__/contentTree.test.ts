import { describe, expect, it } from 'vitest';
import { ContentTreeCls } from '../contentTree';
import { FolderCls } from '../folder';
import { FeedCls } from '../feed';
import { ArticleCls, ReadStatus, SavedStatus } from '../article';
import { MarkState, SelectionType, KeyUnread, KeyAllItems } from '../../utils/types';

// Builds a tree with one folder, one feed, and two unread articles.
function buildTree(): ContentTreeCls {
  const article1 = new ArticleCls(
    'art1', 'Article 1', '', '', 'http://example.com/1',
    SavedStatus.Unsaved, 1000, ReadStatus.Unread
  );
  const article2 = new ArticleCls(
    'art2', 'Article 2', '', '', 'http://example.com/2',
    SavedStatus.Unsaved, 999, ReadStatus.Unread
  );

  const feed = new FeedCls('feed1', 'Feed 1', '', '', 0);
  feed.SetFolderId('folder1');
  feed.AddArticle(article1);
  feed.AddArticle(article2);

  const folder = new FolderCls('folder1', 'Folder 1');
  folder.AddFeed(feed);

  const tree = ContentTreeCls.new();
  tree.AddFolder(folder);
  return tree;
}

describe('ContentTreeCls article mark/display', () => {
  it('reflects isRead:true immediately after marking an article as read', () => {
    const tree = buildTree();

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);

    const views = tree.GetArticleView(['feed1', 'folder1'], SelectionType.Feed);
    expect(views.find((v) => v.id === 'art1')?.isRead).toBe(true);
    expect(views.find((v) => v.id === 'art2')?.isRead).toBe(false);
  });

  it('reflects isRead:false after toggling a read article back to unread', () => {
    const tree = buildTree();

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);
    tree.Mark(MarkState.Unread, ['art1', 'feed1', 'folder1'], SelectionType.Article);

    const views = tree.GetArticleView(['feed1', 'folder1'], SelectionType.Feed);
    expect(views.find((v) => v.id === 'art1')?.isRead).toBe(false);
  });

  it('keeps a marked-read article visible in the Unread stream on the same selection', () => {
    const tree = buildTree();
    // Seed the last-computed key so the pin-clear logic has a baseline.
    tree.GetArticleView(KeyUnread, SelectionType.Unread);

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);

    const views = tree.GetArticleView(KeyUnread, SelectionType.Unread);
    const art1 = views.find((v) => v.id === 'art1');
    expect(art1).toBeDefined();
    expect(art1?.isRead).toBe(true);
  });

  it('keeps a marked-read article visible in a Feed stream on the same selection', () => {
    const tree = buildTree();
    tree.GetArticleView(['feed1', 'folder1'], SelectionType.Feed);

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);

    const views = tree.GetArticleView(['feed1', 'folder1'], SelectionType.Feed);
    expect(views.find((v) => v.id === 'art1')).toBeDefined();
  });

  it('keeps a marked-read article visible in a Folder stream on the same selection', () => {
    const tree = buildTree();
    tree.GetArticleView('folder1', SelectionType.Folder);

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);

    const views = tree.GetArticleView('folder1', SelectionType.Folder);
    expect(views.find((v) => v.id === 'art1')).toBeDefined();
  });

  it('filters out a marked-read article after the selection changes', () => {
    const tree = buildTree();
    tree.GetArticleView(KeyUnread, SelectionType.Unread);

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);

    // Simulate navigating to a different stream and back.
    tree.GetArticleView(KeyAllItems, SelectionType.All);
    const views = tree.GetArticleView(KeyUnread, SelectionType.Unread);

    expect(views.find((v) => v.id === 'art1')).toBeUndefined();
    expect(views.find((v) => v.id === 'art2')).toBeDefined();
  });

  it('decrements the unread count when an article is marked as read', () => {
    const tree = buildTree();
    expect(tree.UnreadCount()).toBe(2);

    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);
    expect(tree.UnreadCount()).toBe(1);
  });

  it('restores the unread count when a read article is toggled back to unread', () => {
    const tree = buildTree();
    tree.Mark(MarkState.Read, ['art1', 'feed1', 'folder1'], SelectionType.Article);
    expect(tree.UnreadCount()).toBe(1);

    tree.Mark(MarkState.Unread, ['art1', 'feed1', 'folder1'], SelectionType.Article);
    expect(tree.UnreadCount()).toBe(2);
  });
});
