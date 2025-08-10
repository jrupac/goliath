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
  private folderFeedView: Map<FolderView, FeedView[]>;

  private constructor() {
    this.tree = new Map<FolderId, FolderCls>();
    this.unread_count = 0;
    this.folderFeedView = new Map<FolderView, FeedView[]>();
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
        break;
      case SelectionType.Feed:
        [feedId, folderId] = key as FeedSelection;
        folder = this.getFolderOrThrow(folderId);

        this.unread_count -= folder.UnreadCount();
        folder.MarkFeed(feedId, markState);
        this.unread_count += folder.UnreadCount();
        break;
      case SelectionType.Folder:
        folderId = key as FolderSelection;
        folder = this.getFolderOrThrow(folderId);

        this.unread_count -= folder.UnreadCount();
        folder.MarkFolder(markState);
        this.unread_count += folder.UnreadCount();
        break;
      case SelectionType.All:
        this.tree.forEach((f: FolderCls): void => {
          unread += f.MarkFolder(markState);
        });
        this.unread_count = unread;
        break;
      case SelectionType.Saved:
        // TODO: Support saved articles.
        console.log('Saving articles not yet supported!');
        break;
    }

    return this.unread_count;
  }

  public GetArticleView(key: SelectionKey, type: SelectionType): ArticleView[] {
    let articleId: ArticleId, feedId: FeedId, folderId: FolderId;
    let articleViews: ArticleView[] = [];

    switch (type) {
      case SelectionType.Article:
        [articleId, feedId, folderId] = key as ArticleSelection;
        articleViews = this.getFolderOrThrow(folderId).GetArticleView(
          feedId,
          articleId
        );
        break;
      case SelectionType.Feed:
        [feedId, folderId] = key as FeedSelection;
        articleViews = this.getFolderOrThrow(folderId).GetArticleView(feedId);
        break;
      case SelectionType.Folder:
        folderId = key as FolderSelection;
        articleViews = this.getFolderOrThrow(folderId).GetArticleView();
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

    return ArticleCls.SortAndFilterViews(articleViews);
  }

  public GetFolderFeedView(): Map<FolderView, FeedView[]> {
    this.tree.forEach((f: FolderCls): void => {
      this.folderFeedView.set(...f.GetFolderFeedView());
    });
    return this.folderFeedView;
  }

  public GetFaviconMap(): Map<FeedId, FaviconCls> {
    const faviconMap: Map<FeedId, FaviconCls> = new Map();
    this.tree.forEach((f: FolderCls): void => {
      const folderFavicons = f.GetFavicons();
      folderFavicons.forEach((favicon: FaviconCls, feedId: FeedId): void => {
        faviconMap.set(feedId, favicon);
      });
    });
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
