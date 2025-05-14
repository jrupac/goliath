export const enum GReaderURI {
  Login = '/greader/reader/api/0/login',
  Token = '/greader/reader/api/0/token',
  EditTag = '/greader/reader/api/0/edit-tag',
  SubscriptionList = '/greader/reader/api/0/subscription/list',
  StreamItemIds = '/greader/reader/api/0/stream/items/ids',
  StreamItemContents = '/greader/reader/api/0/stream/items/contents',
  MarkAllAsRead = '/greader/reader/api/0/mark-all-as-read',
}

export const enum GReaderTag {
  MarkRead = 'user/-/state/com.google/read',
}

export const enum GReaderStream {
  ReadingList = 'user/-/state/com.google/reading-list',
}

/**
 * The following several interfaces conform to GReader API responses.
 */
export interface GReaderHandleLogin {
  SID: string;
  LSID: string;
  Auth: string;
}

interface GReaderCategory {
  id: string;
  label: string;
}

export interface GReaderSubscription {
  title: string;
  firstItemMsec: string;
  htmlUrl: string;
  iconUrl: string;
  sortId: string;
  id: string;
  categories: GReaderCategory[];
}

export interface GReaderSubscriptionList {
  subscriptions: GReaderSubscription[];
}

interface GReaderCanonical {
  href: string;
}

interface GReaderContent {
  direction: string;
  content: string;
}

interface GReaderOrigin {
  streamId: string;
  title: string;
  htmlUrl: string;
}

export interface GReaderItemContent {
  crawlTimeMsec: string;
  timestampUsec: string;
  id: string;
  categories: string[];
  title: string;
  published: number;
  canonical: GReaderCanonical[];
  alternate: GReaderCanonical[];
  summary: GReaderContent;
  origin: GReaderOrigin;
}

export interface GReaderItemRef {
  id: string;
  directStreamIds: string[];
  timestampUsec: string;
}

export interface GReaderStreamIds {
  itemRefs: GReaderItemRef[];
  continuation: string;
}

export interface GReaderStreamContents {
  id: string;
  updated: number;
  items: GReaderItemContent[];
}