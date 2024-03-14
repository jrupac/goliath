import {Folder, FolderCls, FolderId} from "./folder";
import {FeedCls} from "./feed";

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

  public AddFeed(folderId: FolderId, feed: FeedCls): void {
    const existing = this.tree.get(folderId);
    if (existing === undefined) {
      console.log(`WARNING: Unknown folder: ${folderId} in ContentTree.`);
      return;
    }

    this.unread_count -= existing.UnreadCount();
    existing.AddFeed(feed);
    this.unread_count += existing.UnreadCount();
  }

  public UnreadCount(): number {
    return this.unread_count;
  }

  private sort(): void {
    const comparator = (a: [FolderId, FolderCls], b: [FolderId, FolderCls]) => FolderCls.Comparator(a[1], b[1]);
    this.tree = new Map([...this.tree.entries()].sort(comparator));
  }

  static new(): ContentTreeCls {
    return new ContentTreeCls();
  }
}

