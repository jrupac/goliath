import './App.css';
import ArticleList from './components/ArticleList';
import FolderFeedList from './components/FolderFeedList';
import Loading from './components/Loading';
import React from 'react';
import {
  Article,
  ArticleListEntry,
  ArticleSelection,
  ContentTree,
  Feed,
  FeedId,
  FeedSelection,
  Folder,
  FolderId,
  FolderSelection,
  initContentTree,
  KeyAll,
  MarkState,
  SelectionKey,
  SelectionType,
  Status,
  Theme,
  VersionData
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
import {FetchAPI, FetchAPIFactory} from "./api/interface";
import {GetVersion} from "./api/Goliath";
import {RouteComponentProps} from "react-router-dom";
import {LoginPath} from "./utils/helpers";

// AppProps needs to extend RouteComponentProps to get "history".
export interface AppProps extends RouteComponentProps {
}


export interface AppState {
  buildTimestamp: string;
  buildHash: string;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  status: Status;
  contentTree: ContentTree;
  unreadCount: number;
  theme: Theme;
}

export default class App extends React.Component<AppProps, AppState> {
  fetchApi: FetchAPI;

  constructor(props: AppProps) {
    super(props);
    this.state = {
      buildTimestamp: "",
      buildHash: "",
      selectionKey: KeyAll,
      selectionType: SelectionType.All,
      status: Status.Start,
      contentTree: initContentTree(),
      unreadCount: 0,
      theme: Theme.Dark,
    };
    this.fetchApi = FetchAPIFactory.Create();
  }

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  }

  componentDidMount() {
    // This is defense-in-depth to redirect to the login page if the
    // appropriate cookie is not present. This check is also done on the
    // server side and returns an HTTP redirect.
    if (!this.fetchApi.VerifyAuth()) {
      this.props.history.push({
        pathname: LoginPath
      });
      return;
    }

    window.addEventListener('keydown', this.handleKeyDown);

    this.fetchVersion().then(() => console.log("Fetched version info."));
    this.fetchApi.InitializeContent((status) =>
      this.setState((prevState) => ({status: prevState.status | status})))
      .then(([unreadCount, tree]) => {
        console.log("Completed all Fever requests.")
        this.setState({
          unreadCount: unreadCount,
          contentTree: tree,
          status: Status.Ready
        })
      });
  }

  async fetchVersion(): Promise<void> {
    return await GetVersion().then((versionData: VersionData) => {
      this.setState({
        buildTimestamp: versionData.build_timestamp,
        buildHash: versionData.build_hash
      })
    });
  }

  handleMark = (mark: MarkState, entity: SelectionKey, type: SelectionType) => {
    switch (type) {
    case SelectionType.Article:
      this.fetchApi.MarkArticle(mark, entity).then(() => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.contentTree);

          const article = this.getArticleOrThrow(
            structure, entity as ArticleSelection);
          article.is_read = 1;

          const unreadCount = this.updateUnreadCount(structure);

          return {
            contentTree: structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    case SelectionType.Feed:
      this.fetchApi.MarkFeed(mark, entity).then(() => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.contentTree);

          const feed = this.getFeedOrThrow(structure, entity as FeedSelection);
          feed.articles.forEach(
            (article: Article) => article.is_read = 1);

          const unreadCount = this.updateUnreadCount(structure);

          return {
            contentTree: structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    case SelectionType.Folder:
      this.fetchApi.MarkFolder(mark, entity).then(() => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.contentTree);

          const folder = this.getFolderOrThrow(
            structure, entity as FolderSelection);

          folder.feeds.forEach(
            (feed: Feed) => {
              feed.articles.forEach(
                (article: Article) => {
                  article.is_read = 1;
                });
            });

          const unreadCount = this.updateUnreadCount(structure);

          return {
            contentTree: structure,
            unreadCount,
          } as AppState
        });
      }).catch((e) => console.log(e));
      break;
    case SelectionType.All:
      this.fetchApi.MarkAll(mark, entity).then(() => {
        // Update the read buffer and unread counts.
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const structure = new Map(prevState.contentTree);

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

          const unreadCount = this.updateUnreadCount(structure);

          return {
            contentTree: structure,
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
        <Box
          sx={{display: 'flex', overflow: 'hidden', height: '100vh'}}
          className={`${themeClasses}`}>
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
                tree={this.state.contentTree}
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

      feed = this.getFeedOrThrow(this.state.contentTree, [feedId, folderId]);
      title = feed.title;
      favicon = feed.favicon;
      article = this.getArticleOrThrow(this.state.contentTree, key);

      entries.push([article, title, favicon, feedId, folderId]);
      break;
    case SelectionType.Feed:
      [feedId, folderId] = this.state.selectionKey as FeedSelection;

      feed = this.getFeedOrThrow(this.state.contentTree, [feedId, folderId]);
      title = feed.title;
      favicon = feed.favicon;

      feed.articles.forEach((article: Article) => {
        entries.push([article, title, favicon, feedId, folderId]);
      });
      break;
    case SelectionType.Folder:
      folderId = this.state.selectionKey as FolderSelection;

      folderData = this.getFolderOrThrow(this.state.contentTree, folderId);

      folderData.feeds.forEach(
        (feed: Feed) => {
          feed.articles.forEach(
            (article: Article) => {
              entries.push([article, feed.title, feed.favicon, feed.id, folderId])
            });
        });
      break;
    case SelectionType.All:
      this.state.contentTree.forEach(
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

    return this.sortArticles(entries.filter(this.articleIsUnread));
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
      this.toggleColorMode();
      break;
    }
  }

  toggleColorMode = () => {
    this.setState((prevState) => {
      if (prevState.theme === Theme.Default) {
        return {
          theme: Theme.Dark
        }
      } else {
        return {
          theme: Theme.Default
        }
      }
    });
  }

  getFolderOrThrow(structure: Map<FolderId, Folder>, folderId: FolderSelection): Folder {
    const folderData = structure.get(folderId);
    if (folderData === undefined) {
      throw new Error("Unknown group: " + folderId);
    }
    return folderData;
  }

  getFeedOrThrow(structure: Map<FolderId, Folder>, feedSelection: FeedSelection): Feed {
    const [feedId, folderId] = feedSelection;
    const folder = this.getFolderOrThrow(structure, folderId);
    const feed = folder.feeds.get(feedId);
    if (feed === undefined) {
      throw new Error("Unknown feed: " + feedSelection);
    }
    return feed;
  }

  getArticleOrThrow(structure: Map<FolderId, Folder>, articleSelection: ArticleSelection): Article {
    const [articleId, feedId, folderId] = articleSelection;
    const feed = this.getFeedOrThrow(structure, [feedId, folderId]);
    const article = feed.articles.get(articleId);
    if (article === undefined) {
      throw new Error("Unknown feed: " + articleId + " in feed " + feed.id);
    }
    return article;
  }

  articleIsUnread(articleEntry: ArticleListEntry): boolean {
    const [article] = articleEntry;
    return !(article.is_read === 1) as boolean;
  }

  sortArticles(articles: ArticleListEntry[]) {
    // Sort by descending time.
    return articles.sort(
      (a: ArticleListEntry, b: ArticleListEntry) =>
        b[0].created_on_time - a[0].created_on_time);
  }

  updateUnreadCount(structure: Map<FolderId, Folder>): number {
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
}