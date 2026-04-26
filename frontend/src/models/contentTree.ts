import { FolderCls, FolderId, FolderView } from './folder';
import { FaviconCls, FeedId, FeedView } from './feed';
import {
  ArticleSelection,
  FeedSelection,
  FolderSelection,
  MarkState,
  SelectionKey,
  SelectionType,
} from '../utils/types';
import { ArticleCls, ArticleId, ArticleView } from './article';

/** ContentTreeCls contains a tree of folders, feeds, and articles. */
export class ContentTreeCls {
  private tree: Map<FolderId, FolderCls>;
  private unread_count: number;
  // Derived view caches — null means dirty (needs recomputation).
  private cachedFolderFeedView: Map<FolderView, FeedView[]> | null;
  private cachedFaviconMap: Map<FeedId, FaviconCls> | null;
  private cachedArticleView: {
    keyStr: string;
    type: SelectionType;
    views: ArticleView[];
  } | null;
  // Articles pinned to remain visible in filtered streams after a
  // single-article mark. Cleared when the selection changes.
  private pinnedArticleIds: Set<string>;
  private lastComputedKeyStr: string | null;
  private lastComputedType: SelectionType | null;

  private constructor() {
    this.tree = new Map<FolderId, FolderCls>();
    this.unread_count = 0;
    this.cachedFolderFeedView = null;
    this.cachedFaviconMap = null;
    this.cachedArticleView = null;
    this.pinnedArticleIds = new Set();
    this.lastComputedKeyStr = null;
    this.lastComputedType = null;
  }

  private invalidateCaches(): void {
    this.cachedFolderFeedView = null;
    this.cachedFaviconMap = null;
    this.cachedArticleView = null;
  }

  public AddFolder(folder: FolderCls): void {
    const existing = this.tree.get(folder.Id());
    if (existing !== undefined) {
      console.log(
        `WARNING: Replacing folder: ${existing.Title()} in ContentTree.`
      );
      this.unread_count -= existing.UnreadCount();
    }

    this.tree.set(folder.Id(), folder);
    this.sort();
    this.unread_count += folder.UnreadCount();
    this.invalidateCaches();
  }

  public UnreadCount(): number {
    return this.unread_count;
  }

  public Mark(
    markState: MarkState,
    key: SelectionKey,
    type: SelectionType
  ): number {
    let articleId: ArticleId, feedId: FeedId, folderId: FolderId;
    let folder: FolderCls;
    let unread: number = 0;

    switch (type) {
      case SelectionType.Article:
        [articleId, feedId, folderId] = key as ArticleSelection;
        folder = this.getFolderOrThrow(folderId);

        this.unread_count -= folder.UnreadCount();
        folder.MarkArticle(articleId, feedId, markState);
        this.unread_count += folder.UnreadCount();

        // Pin article so it stays visible in filtered streams after
        // cache invalidation. Pins are cleared on next selection change.
        this.pinnedArticleIds.add(JSON.stringify(articleId));
        this.invalidateCaches();
        break;
      case SelectionType.Feed:
        [feedId, folderId] = key as FeedSelection;
        folder = this.getFolderOrThrow(folderId);

        this.unread_count -= folder.UnreadCount();
        folder.MarkFeed(feedId, markState);
        this.unread_count += folder.UnreadCount();

        this.pinnedArticleIds.clear();
        this.invalidateCaches();
        break;
      case SelectionType.Folder:
        folderId = key as FolderSelection;
        folder = this.getFolderOrThrow(folderId);

        this.unread_count -= folder.UnreadCount();
        folder.MarkFolder(markState);
        this.unread_count += folder.UnreadCount();

        this.pinnedArticleIds.clear();
        this.invalidateCaches();
        break;
      case SelectionType.Unread:
      case SelectionType.All:
        this.tree.forEach((f: FolderCls): void => {
          unread += f.MarkFolder(markState);
        });
        this.unread_count = unread;

        this.pinnedArticleIds.clear();
        this.invalidateCaches();
        break;
      case SelectionType.Saved:
        // TODO: Support saved articles.
        console.log('Saving articles not yet supported!');
        break;
    }

    return this.unread_count;
  }

  public GetArticleView(key: SelectionKey, type: SelectionType): ArticleView[] {
    const keyStr = JSON.stringify(key);

    // Selection changed — pins are stale, clear them before recomputing.
    if (
      this.lastComputedKeyStr !== null &&
      (keyStr !== this.lastComputedKeyStr || type !== this.lastComputedType)
    ) {
      this.pinnedArticleIds.clear();
    }

    if (
      this.cachedArticleView !== null &&
      this.cachedArticleView.keyStr === keyStr &&
      this.cachedArticleView.type === type
    ) {
      return this.cachedArticleView.views;
    }

    let articleId: ArticleId, feedId: FeedId, folderId: FolderId;
    let articleViews: ArticleView[] = [];
    const pinned = this.pinnedArticleIds;

    switch (type) {
      case SelectionType.Article:
        [articleId, feedId, folderId] = key as ArticleSelection;
        articleViews = this.getFolderOrThrow(folderId).GetArticleView(
          feedId,
          articleId
        );
        break;
      case SelectionType.Unread:
        this.tree.forEach((f: FolderCls): void => {
          articleViews.push(
            ...f
              .GetArticleView()
              .filter((v) => !v.isRead || pinned.has(JSON.stringify(v.id)))
          );
        });
        break;
      case SelectionType.Folder:
        folderId = key as FolderSelection;
        articleViews = this.getFolderOrThrow(folderId)
          .GetArticleView()
          .filter((v) => !v.isRead || pinned.has(JSON.stringify(v.id)));
        break;
      case SelectionType.Feed:
        [feedId, folderId] = key as FeedSelection;
        articleViews = this.getFolderOrThrow(folderId)
          .GetArticleView(feedId)
          .filter((v) => !v.isRead || pinned.has(JSON.stringify(v.id)));
        break;
      case SelectionType.All:
        this.tree.forEach((f: FolderCls): void => {
          articleViews.push(...f.GetArticleView());
        });
        break;
      case SelectionType.Saved:
        // TODO: Support saved articles.
        console.log('Showing saved articles not yet supported!');
        break;
    }

    const views = ArticleCls.SortViews(articleViews);
    this.cachedArticleView = { keyStr, type, views };
    this.lastComputedKeyStr = keyStr;
    this.lastComputedType = type;
    return views;
  }

  public GetFolderFeedView(): Map<FolderView, FeedView[]> {
    if (this.cachedFolderFeedView !== null) {
      return this.cachedFolderFeedView;
    }
    const view = new Map<FolderView, FeedView[]>();
    this.tree.forEach((f: FolderCls): void => {
      view.set(...f.GetFolderFeedView());
    });
    this.cachedFolderFeedView = view;
    return view;
  }

  public GetFaviconMap(): Map<FeedId, FaviconCls> {
    if (this.cachedFaviconMap !== null) {
      return this.cachedFaviconMap;
    }
    const faviconMap: Map<FeedId, FaviconCls> = new Map();
    this.tree.forEach((f: FolderCls): void => {
      const folderFavicons = f.GetFavicons();
      folderFavicons.forEach((favicon: FaviconCls, feedId: FeedId): void => {
        faviconMap.set(feedId, favicon);
      });
    });
    this.cachedFaviconMap = faviconMap;
    return faviconMap;
  }

  private getFolderOrThrow(folderId: FolderId): FolderCls {
    const folder: FolderCls | undefined = this.tree.get(folderId);
    if (folder === undefined) {
      throw new Error(`No folder by ID: ${folderId}`);
    }
    return folder;
  }

  private sort(): void {
    const comparator = (a: [FolderId, FolderCls], b: [FolderId, FolderCls]) =>
      FolderCls.Comparator(a[1], b[1]);
    this.tree = new Map([...this.tree.entries()].sort(comparator));
  }

  static new(): ContentTreeCls {
    return new ContentTreeCls();
  }
}
