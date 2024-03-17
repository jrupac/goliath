import {Folder, FolderCls, FolderId, FolderView} from "./folder";
import {FeedId, FeedView} from "./feed";
import {
  ArticleListEntry,
  ArticleSelection,
  FeedSelection,
  FolderSelection,
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {ArticleId} from "./article";

/** ContentTree is a map of FolderID -> Folder with all associated content. */
export type ContentTree = Map<FolderId, Folder>;
/** Create a new, empty ContentTree object. */
export const initContentTree = () => new Map<FolderId, Folder>();

export class ContentTreeCls {
  private tree: Map<FolderId, FolderCls>;
  private unread_count: number;

  private constructor() {
    this.tree = new Map<FolderId, FolderCls>();
    this.unread_count = 0;
  }

  public AddFolder(folder: FolderCls): void {
    const existing = this.tree.get(folder.Id());
    if (existing !== undefined) {
      console.log(`WARNING: Replacing folder: ${existing.Title()} in ContentTree.`);
      this.unread_count -= existing.UnreadCount();
    }

    this.tree.set(folder.Id(), folder);
    this.sort();
    this.unread_count += folder.UnreadCount();
  }

  public UnreadCount(): number {
    return this.unread_count;
  }

  public Mark(markState: MarkState, key: SelectionKey, type: SelectionType): number {
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
    }

    return this.unread_count;
  }

  public GetEntries(key: SelectionKey, type: SelectionType): ArticleListEntry[] {
    let articleId: ArticleId, feedId: FeedId, folderId: FolderId;
    let entries: ArticleListEntry[] = [];

    switch (type) {
    case SelectionType.Article:
      [articleId, feedId, folderId] = key as ArticleSelection;
      return this.getFolderOrThrow(folderId).GetArticleEntry(articleId, feedId);
    case SelectionType.Feed:
      [feedId, folderId] = key as FeedSelection;
      return this.getFolderOrThrow(folderId).GetFeedEntry(feedId);
    case SelectionType.Folder:
      folderId = key as FolderSelection;
      return this.getFolderOrThrow(folderId).GetFolderEntry();
    case SelectionType.All:
      this.tree.forEach((f: FolderCls): void => {
        entries = entries.concat(f.GetFolderEntry());
      });
    }

    return entries;
  }

  public GetFolderFeedView(): Map<FolderView, FeedView[]> {
    const view: Map<FolderView, FeedView[]> = new Map();
    this.tree.forEach((f: FolderCls): void => {
      view.set(...f.GetViewEntry());
    });
    return view;
  }

  private getFolderOrThrow(folderId: FolderId): FolderCls {
    const folder = this.tree.get(folderId);
    if (folder === undefined) {
      throw new Error(`No folder by ID: ${folderId}`);
    }
    return folder;
  }

  private sort(): void {
    const comparator = (a: [FolderId, FolderCls], b: [FolderId, FolderCls]) => FolderCls.Comparator(a[1], b[1]);
    this.tree = new Map([...this.tree.entries()].sort(comparator));
  }

  static new(): ContentTreeCls {
    return new ContentTreeCls();
  }
}

