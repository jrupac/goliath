import './App.css';
import ArticleList from './components/ArticleList';
import {Decimal} from 'decimal.js-light';
import FolderFeedList from './components/FolderFeedList';
import Loading from './components/Loading';
import React from 'react';
import * as LosslessJSON from 'lossless-json';
import {
  Article,
  ArticleId,
  ArticleListEntry,
  ArticleSelection,
  FaviconId,
  Feed,
  FeedId,
  FeedSelection,
  Folder,
  FolderId,
  FolderSelection,
  KeyAll,
  MarkState,
  SelectionKey,
  SelectionType,
  Status,
  Theme
} from "./utils/types";

import './themes/default.css';
import './themes/dark.css';
import {
  Box,
  createTheme,
  CssBaseline,
  darkScrollbar,
  Divider,
  Drawer,
  PaletteMode,
  ThemeProvider
} from "@mui/material";

export interface AppProps {
}

export interface AppState {
  buildTimestamp: string;
  buildHash: string;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  status: Status;
  structure: Map<FolderId, Folder>;
  unreadCount: number;
  theme: Theme;
  // Fever response types
  feverFetchGroupsResponse: FeverFetchGroupsType;
  feverFetchFeedsResponse: FeverFetchFeedsType;
  feverFetchFaviconsResponse: FeverFetchFaviconsType;
  feverFetchItemsResponse: FeverFetchItemsType;
}

// Version matches the response from a /version API call.
interface Version {
  build_timestamp: string;
  build_hash: string;
}

// The following several interfaces conform to the Fever API.
interface FeverFeedGroupType {
  group_id: string;
  feed_ids: string;
}

interface FeverGroupType {
  id: string;
  title: string;
}

interface FeverFaviconType {
  id: FaviconId;
  data: string;
}

interface FeverFetchGroupsType {
  groups: FeverGroupType[];
  feeds_groups: FeverFeedGroupType[];
}

interface FeverFetchFeedsType {
  feeds: Feed[];
  feeds_groups: FeverFeedGroupType[];
}

interface FeverFetchFaviconsType {
  favicons: FeverFaviconType[];
}

interface FeverFetchItemsType {
  items: Article[];
  total_items: number;
}

export default class App extends React.Component<AppProps, AppState> {
  constructor(props: AppProps) {
    super(props);
    this.state = {
      buildTimestamp: "",
      buildHash: "",
      selectionKey: KeyAll,
      selectionType: SelectionType.All,
      status: Status.Start,
      structure: new Map<FolderId, Folder>(),
      unreadCount: 0,
      theme: Theme.Dark,
      // Fever response types
      feverFetchGroupsResponse: {groups: [], feeds_groups: []},
      feverFetchFeedsResponse: {feeds: [], feeds_groups: []},
      feverFetchFaviconsResponse: {favicons: []},
      feverFetchItemsResponse: {items: [], total_items: 0}
    };
  }

  componentWillMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  };

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  };

  componentDidMount() {
    Promise.all(
      [this.fetchFolders(), this.fetchFeeds(), this.fetchItems(),
        this.fetchFavicons(), this.fetchVersion()])
      .then(() => {
        console.log('Completed all requests to server.');
      });
  }

  buildStructure() {
    // Only build the structure once everything else is fetched.
    if ((this.state.status !== Status.Ready) &&
      (this.state.status !==
        (Status.Folder | Status.Feed | Status.Article | Status.Favicon))) {
      return;
    }

    this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
      const structure = new Map<FolderId, Folder>();

      // Map of (Folder ID) -> (Feed ID).
      const folderToFeeds = new Map<FolderId, FeedId[]>();
      prevState.feverFetchGroupsResponse.feeds_groups.forEach(
        (feedGroup: FeverFeedGroupType) => {
          folderToFeeds.set(feedGroup.group_id, feedGroup.feed_ids.split(','));
        });

      // Map of all (Feed ID) -> (Feed Data).
      const globalFeedMap = new Map<FeedId, Feed>();
      prevState.feverFetchFeedsResponse.feeds.forEach(
        (feed: Feed) => globalFeedMap.set(feed.id, feed));

      // Map of (Feed ID) -> (list of Article Data).
      const feedToArticles = new Map<FeedId, Article[]>();
      prevState.feverFetchItemsResponse.items.forEach(
        (article: Article) => {
          const entry = feedToArticles.get(article.feed_id) || [];
          feedToArticles.set(article.feed_id, [...entry, article]);
        });

      // Map of (Favicon ID) -> (Favicon Data).
      const globalFaviconMap = new Map<FaviconId, string>();
      prevState.feverFetchFaviconsResponse.favicons.forEach(
        (favicon: FeverFaviconType) => {
          globalFaviconMap.set(favicon.id, favicon.data)
        });

      prevState.feverFetchGroupsResponse.groups.forEach(
        (group: FeverGroupType) => {
          const folderId = group.id as FolderId;

          const feedIdList = folderToFeeds.get(folderId);
          // Not all folders have feeds, so nothing more to be done here.
          if (feedIdList === undefined) {
            return;
          }

          // Populate feeds in this folder.
          const feeds = new Map<FeedId, Feed>();
          feedIdList.forEach(
            (feedId: FeedId) => {
              const feed = globalFeedMap.get(feedId);
              if (feed === undefined) {
                throw new Error("Unknown feed ID: " + feedId);
              }

              // Populate articles in this feed.
              const articles = feedToArticles.get(feedId) || [];
              feed.articles = new Map<ArticleId, Article>();
              articles.forEach((article: Article) => {
                feed.articles.set(article.id, article);
              });

              // Compute other metadata about this feed.
              feed.unread_count = articles.reduce(
                (acc: number, a: Article) => acc + (1 - a.is_read), 0);
              feed.favicon = globalFaviconMap.get(feed.favicon_id) || "";

              feeds.set(feedId, feed);
            }
          );

          // Compute other metadata about this folder.
          const unreadCount = Array.from(feeds.values()).reduce(
            (acc: number, f: Feed) => acc + f.unread_count, 0);

          const folderData: Folder = {
            feeds: feeds,
            title: group.title,
            unread_count: unreadCount
          };

          structure.set(folderId, folderData);
        }
      );

      // Compute other global metadata.
      const unreadCount = Array.from(structure.values()).reduce(
        (acc: number, f: Folder) => acc + f.unread_count, 0);

      return {
        structure,
        unreadCount,
        status: Status.Ready,
      } as AppState;
    });
  }

  parseJson(text: string): any {
    // Parse as Lossless numbers since values from the server are 64-bit
    // Integer, but then convert back to String for use going forward.
    return LosslessJSON.parse(text, (k: string, v: any) => {
      if (v && v.isLosslessNumber) {
        return String(v);
      }
      return v;
    });
  }

  fetchFolders() {
    return fetch('/fever/?api&groups', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchGroupsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          return {
            feverFetchGroupsResponse: body,
            status: prevState.status | Status.Folder,
          } as AppState
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchFeeds() {
    return fetch('/fever/?api&feeds', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchFeedsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          return {
            feverFetchFeedsResponse: body,
            status: prevState.status | Status.Feed,
          } as AppState
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchFavicons() {
    return fetch('/fever/?api&favicons', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchFaviconsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          return {
            feverFetchFaviconsResponse: body,
            status: prevState.status | Status.Favicon,
          } as AppState
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchItems(sinceId?: Decimal) {
    let since: Decimal, itemUri: string;

    if (sinceId !== undefined) {
      since = sinceId;
      itemUri = '/fever/?api&items&since_id=' + since.toString();
    } else {
      since = new Decimal(0);
      itemUri = '/fever/?api&items';
    }

    return fetch(itemUri, {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchItemsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const itemCount = body.items.length;
          const feverFetchItemsResponse: FeverFetchItemsType = {
            total_items: itemCount,
            items: [
              ...prevState.feverFetchItemsResponse.items,
              ...body.items
            ]
          };

          // Keep fetching until we see less than the max items returned.
          // Don't update the status field until we're done.
          if (itemCount === 50) {
            body.items.forEach((item: Article) => {
              // Update latest seen article ID.
              since = maxArticleId(since, item.id);
            });
            // Kick off a recursive call asynchronously. Errors will be thrown
            // within the call, so ignore the result here.
            this.fetchItems(since).finally();
            return {
              feverFetchItemsResponse
            } as AppState;
          }

          // If we're here, then this is the last fetch call so update status.
          return {
            feverFetchItemsResponse,
            status: prevState.status | Status.Article,
          } as AppState;
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchVersion() {
    return fetch('/version', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: Version) => {
        this.setState({
          buildTimestamp: body.build_timestamp,
          buildHash: body.build_hash
        })
      }).catch((e) => console.log(e));
  }

  handleMark = (mark: MarkState, entity: SelectionKey, type: SelectionType) => {
    let feverId: string;

    switch (type) {
    case SelectionType.Article:
      feverId = (entity as ArticleSelection)[0] as string;
      fetch('/fever/?api&mark=item&as=' + mark + '&id=' + feverId, {
        credentials: 'include'
      }).then(() => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.structure);

          const article = getArticleOrThrow(
            structure, entity as ArticleSelection);
          article.is_read = 1;

          const unreadCount = updateUnreadCount(structure);

          return {
            structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    case SelectionType.Feed:
      feverId = (entity as FeedSelection)[0] as string;
      fetch('/fever/?api&mark=feed&as=' + mark + '&id=' + feverId, {
        credentials: 'include'
      }).then(() => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.structure);

          const feed = getFeedOrThrow(structure, entity as FeedSelection);
          feed.articles.forEach(
            (article: Article) => article.is_read = 1);

          const unreadCount = updateUnreadCount(structure);

          return {
            structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    case SelectionType.Folder:
      feverId = (entity as FolderSelection) as string;
      fetch('/fever/?api&mark=group&as=' + mark + '&id=' + feverId, {
        credentials: 'include'
      }).then(() => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.structure);

          const folder = getFolderOrThrow(
            structure, entity as FolderSelection);

          folder.feeds.forEach(
            (feed: Feed) => {
              feed.articles.forEach(
                (article: Article) => {
                  article.is_read = 1;
                });
            });

          const unreadCount = updateUnreadCount(structure);

          return {
            structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    case SelectionType.All:
      fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
        credentials: 'include'
      }).then(() => {
        // Update the read buffer and unread counts.
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.structure);

          structure.forEach(
            (folder: Folder) => {
              folder.feeds.forEach(
                (feed: Feed) => {
                  feed.articles.forEach(
                    (article: Article) => {
                      article.is_read = 1;
                    });
                });
            });

          const unreadCount = updateUnreadCount(structure);

          return {
            structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    default:
      console.log("Unexpected enclosing type: ", type)
    }
  };

  handleSelect = (type: SelectionType, key: SelectionKey) => {
    this.setState({
      selectionKey: key,
      selectionType: type,
    });
  };

  render() {
    let themeClasses: string, paletteMode: PaletteMode;
    if (this.state.theme === Theme.Default) {
      themeClasses = 'default-theme';
      paletteMode = 'light';
    } else {
      themeClasses = 'dark-theme';
      paletteMode = 'dark';
    }

    const theme = createTheme({
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

    if (this.state.status !== Status.Ready) {
      return (
        <ThemeProvider theme={theme}>
          <CssBaseline/>
          <Loading status={this.state.status}/>
        </ThemeProvider>);
    }

    if (this.state.unreadCount === 0) {
      document.title = 'Goliath RSS';
    } else {
      document.title = `(${this.state.unreadCount})  Goliath RSS`;
    }

    return (
      <ThemeProvider theme={theme}>
        {/* TODO: Is there a better way to inject overrides than this? */}
        <Box sx={{display: 'flex'}} className={`${themeClasses}`}>
          <CssBaseline/>
          <Drawer
            variant="permanent"
            anchor="left"
            className="GoliathDrawer"
          >
            <Box
              className="GoliathLogo">
              Goliath
            </Box>
            <Box>
              <FolderFeedList
                tree={this.state.structure}
                unreadCount={this.state.unreadCount}
                selectedKey={this.state.selectionKey}
                selectionType={this.state.selectionType}
                handleSelect={this.handleSelect}/>
            </Box>
            <Divider variant="middle"/>
            <Box className="GoliathFooter">
              Goliath RSS
              <br/>
              Built at: {this.state.buildTimestamp}
              <br/>
              {this.state.buildHash}
            </Box>
          </Drawer>

          <Box
            component="main"
            className="GoliathMainContainer"
          >
            <Box>
              <ArticleList
                articleEntries={this.populateArticleListEntries()}
                selectionKey={this.state.selectionKey}
                selectionType={this.state.selectionType}
                handleMark={this.handleMark}
                selectAllCallback={() => this.handleSelect(SelectionType.All, KeyAll)}/>
            </Box>
          </Box>
        </Box>
      </ThemeProvider>
    );
  }

  populateArticleListEntries(): ArticleListEntry[] {
    let key: SelectionKey;
    let feedId: FeedId, folderId: FolderId;
    let folderData: Folder, feed: Feed, favicon: string, title: string,
      article: Article;
    const entries = [] as ArticleListEntry[];

    switch (this.state.selectionType) {
    case SelectionType.Article:
      key = this.state.selectionKey as ArticleSelection;
      feedId = key[1];
      folderId = key[2];

      feed = getFeedOrThrow(this.state.structure, [feedId, folderId]);
      title = feed.title;
      favicon = feed.favicon;
      article = getArticleOrThrow(this.state.structure, key);

      entries.push([article, title, favicon, feedId, folderId]);
      break;
    case SelectionType.Feed:
      [feedId, folderId] = this.state.selectionKey as FeedSelection;

      feed = getFeedOrThrow(this.state.structure, [feedId, folderId]);
      title = feed.title;
      favicon = feed.favicon;

      feed.articles.forEach((article: Article) => {
        entries.push([article, title, favicon, feedId, folderId]);
      });
      break;
    case SelectionType.Folder:
      folderId = this.state.selectionKey as FolderSelection;

      folderData = getFolderOrThrow(this.state.structure, folderId);

      folderData.feeds.forEach(
        (feed: Feed) => {
          feed.articles.forEach(
            (article: Article) => {
              entries.push([article, feed.title, feed.favicon, feed.id, folderId])
            });
        });
      break;
    case SelectionType.All:
      this.state.structure.forEach(
        (folder: Folder, folderId: FolderId) => {
          folder.feeds.forEach(
            (feed: Feed) => {
              feed.articles.forEach(
                (article: Article) => {
                  entries.push([article, feed.title, feed.favicon, feed.id, folderId])
                });
            });
        });
      break
    }

    return sortArticles(entries.filter(articleIsUnread));
  }

  handleKeyDown = (event: KeyboardEvent) => {
    // Ignore keypress events when some modifiers are also enabled to avoid
    // triggering on (e.g.) browser shortcuts. Shift is the exception here since
    // we do care about Shift+I.
    if (event.altKey || event.metaKey || event.ctrlKey) {
      return;
    }

    switch (event.key) {
    case 't':
      console.log("THEME CHANGE");
      this.toggleColorMode();
    }
  }

  toggleColorMode = () => {
    this.setState((prevState) => {
      let theme: Theme = prevState.theme;

      if (theme === Theme.Default) {
        theme = Theme.Dark;
      } else {
        theme = Theme.Default;
      }

      return {
        theme: theme
      };
    });
  }
}

function maxArticleId(a: Decimal | ArticleId, b: Decimal | ArticleId): Decimal {
  a = new Decimal(a.toString());
  b = new Decimal(b.toString());
  return a > b ? a : b;
}

function getFolderOrThrow(structure: Map<FolderId, Folder>, folderId: FolderSelection): Folder {
  const folderData = structure.get(folderId);
  if (folderData === undefined) {
    throw new Error("Unknown group: " + folderId);
  }
  return folderData;
}

function getFeedOrThrow(structure: Map<FolderId, Folder>, feedSelection: FeedSelection): Feed {
  const [feedId, folderId] = feedSelection;
  const folder = getFolderOrThrow(structure, folderId);
  const feed = folder.feeds.get(feedId);
  if (feed === undefined) {
    throw new Error("Unknown feed: " + feedSelection);
  }
  return feed;
}

function getArticleOrThrow(structure: Map<FolderId, Folder>, articleSelection: ArticleSelection): Article {
  const [articleId, feedId, folderId] = articleSelection;
  const feed = getFeedOrThrow(structure, [feedId, folderId]);
  const article = feed.articles.get(articleId);
  if (article === undefined) {
    throw new Error("Unknown feed: " + articleId + " in feed " + feed.id);
  }
  return article;
}

function articleIsUnread(articleEntry: ArticleListEntry): boolean {
  const [article] = articleEntry;
  return !(article.is_read === 1) as boolean;
}

function sortArticles(articles: ArticleListEntry[]) {
  // Sort by descending time.
  return articles.sort(
    (a: ArticleListEntry, b: ArticleListEntry) =>
      b[0].created_on_time - a[0].created_on_time);
}

function updateUnreadCount(structure: Map<FolderId, Folder>): number {
  structure.forEach((folder: Folder) => {
    folder.feeds.forEach((feed: Feed) => {
      const articles = Array.from(feed.articles.values());
      feed.unread_count = articles.reduce(
        (acc: number, a: Article) => acc + (1 - a.is_read), 0);
    });

    const feeds = Array.from(folder.feeds.values());
    folder.unread_count = Array.from(feeds.values()).reduce(
      (acc: number, f: Feed) => acc + f.unread_count, 0);
  });

  return Array.from(structure.values()).reduce(
    (acc: number, f: Folder) => acc + f.unread_count, 0);
}