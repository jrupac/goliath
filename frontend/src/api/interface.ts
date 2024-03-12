import {ContentTree, SelectionKey, Status} from "../utils/types";

export interface FetchAPI {
  // initialize will return a promise that when resolved returns a fully
  // populated map of folder IDs to folders, each of which has nested feeds and
  // articles.
  // The given `cb` callback will be invoked to update status as the fetching is
  // progressing.
  initialize(cb: (s: Status) => void): Promise<[Status, ContentTree]>;

  // markArticle will mark the specified article with the specified mark status.
  markArticle(mark: string, entity: SelectionKey): Promise<Response>;

  // markFolder will mark the specified folder with the specified mark status.
  markFolder(mark: string, entity: SelectionKey): Promise<Response>;

  // markFeed will mark the specified feed with the specified mark status.
  markFeed(mark: string, entity: SelectionKey): Promise<Response>;

  // markAll will mark all items with the specified mark status.
  markAll(mark: string, entity: SelectionKey): Promise<Response>;
}