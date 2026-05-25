import './App.css';
import ArticleList from './components/ArticleList';
import FolderFeedList from './components/FolderFeedList';
import Loading from './components/Loading';
import React from 'react';
import {
  FeedSelection,
  FolderSelection,
  GoliathPath,
  GoliathTheme,
  KeyUnread,
  KeyAllItems,
  MarkState,
  NavigationDirection,
  SelectionKey,
  SelectionType,
  Status,
  ThemeInfo,
} from './utils/types';

import './themes/default.css';
import './themes/dark.css';
import {
  Box,
  CssBaseline,
  Drawer,
  IconButton,
  ThemeProvider,
} from '@mui/material';
import { FetchAPI, FetchAPIFactory } from './api/interface';
import { GetVersion, VersionData } from './api/goliath';
import { Navigate } from 'react-router-dom';
import { ContentTreeCls } from './models/contentTree';
import { ArticleId } from './models/article';
import {
  getAdjacentFeed,
  getAdjacentFolder,
  populateThemeInfo,
} from './utils/helpers';
import AccountCircleTwoToneIcon from '@mui/icons-material/AccountCircleTwoTone';
import LogoutTwoToneIcon from '@mui/icons-material/LogoutTwoTone';
import KeybindingsModal from './components/KeybindingsModal';
import { Keybindings, getTinykeysSequence } from './utils/keybindings';
import { keybindRegistry } from './utils/keybindRegistry';

export interface AppProps {}

export interface AppState {
  buildTimestamp: string;
  buildHash: string;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  status: Status;
  contentTreeCls: ContentTreeCls;
  theme: GoliathTheme;
  themeInfo: ThemeInfo;
  loginVerified: boolean;
  hideEmpty: boolean;
  showKeybindingsModal: boolean;
}

export default class App extends React.Component<AppProps, AppState> {
  private fetchApi: FetchAPI;
  private globalHandlers: Record<string, () => void>;

  constructor(props: AppProps) {
    super(props);
    this.state = {
      buildTimestamp: '',
      buildHash: '',
      selectionKey: KeyUnread,
      selectionType: SelectionType.Unread,
      status: Status.Start,
      contentTreeCls: ContentTreeCls.new(),
      theme: GoliathTheme.Dark,
      themeInfo: populateThemeInfo(GoliathTheme.Dark),
      loginVerified: false,
      hideEmpty: true,
      showKeybindingsModal: false,
    };
    this.fetchApi = FetchAPIFactory.Create();
    this.globalHandlers = {
      toggleTheme: () => {
        this.setState((prevState: AppState): AppState => {
          const newTheme =
            prevState.theme === GoliathTheme.Default
              ? GoliathTheme.Dark
              : GoliathTheme.Default;
          return {
            ...prevState,
            theme: newTheme,
            themeInfo: populateThemeInfo(newTheme),
          };
        });
      },
      toggleHideEmpty: () => {
        this.setState((prevState: AppState): AppState => {
          return {
            ...prevState,
            hideEmpty: !prevState.hideEmpty,
          };
        });
      },
      toggleKeybindingsModal: () => {
        this.setState((prevState: AppState): AppState => {
          return {
            ...prevState,
            showKeybindingsModal: !prevState.showKeybindingsModal,
          };
        });
      },
    };
  }

  componentWillUnmount() {
    keybindRegistry.unregister('global');
  }

  componentDidMount() {
    // This is defense-in-depth to redirect to the login page if the
    // appropriate cookie is not present. This check is also done on the
    // server side and returns an HTTP redirect.
    this.fetchApi.VerifyAuth().then(async (ok: boolean): Promise<void> => {
      console.log('Fetched login verification.');
      this.setState({ loginVerified: ok });
      this.updateState(Status.LoginVerification);
      // Only try to initialize data if login verification succeeded. Otherwise,
      // the user will be redirected to the login page anyway.
      if (ok) {
        await this.init();
      }
    });
  }

  async init(): Promise<void> {
    const keymap: Record<string, (event: KeyboardEvent) => void> = {};
    Keybindings.global.forEach((kb) => {
      const sequence = getTinykeysSequence(kb);
      keymap[sequence] = (event: KeyboardEvent) => {
        if (
          this.state.showKeybindingsModal &&
          kb.handlerKey !== 'toggleKeybindingsModal'
        ) {
          return;
        }
        const handler = this.globalHandlers[kb.handlerKey];
        if (handler) {
          event.preventDefault();
          handler();
        }
      };
    });
    keybindRegistry.register('global', keymap);

    const versionData: VersionData = await GetVersion();
    this.setState({
      buildTimestamp: versionData.build_timestamp,
      buildHash: versionData.build_hash,
    });
    console.log('Fetched version info.');

    const treeCls: ContentTreeCls = await this.fetchApi.InitializeContent(
      this.updateState
    );
    this.setState({
      contentTreeCls: treeCls,
      status: Status.Ready,
    });
    console.log('Completed all Fetch API requests.');
  }

  updateState = (status: Status): void => {
    this.setState((prevState: Readonly<AppState>): { status: Status } => {
      return { status: prevState.status | status };
    });
  };

  handleMark = (mark: MarkState, entity: SelectionKey, type: SelectionType) => {
    let functor;

    switch (type) {
      case SelectionType.Article:
        functor = (m: MarkState, e: SelectionKey) =>
          this.fetchApi.MarkArticle(m, e);
        break;
      case SelectionType.Feed:
        functor = (m: MarkState, e: SelectionKey) =>
          this.fetchApi.MarkFeed(m, e);
        break;
      case SelectionType.Folder:
        functor = (m: MarkState, e: SelectionKey) =>
          this.fetchApi.MarkFolder(m, e);
        break;
      case SelectionType.Unread:
        functor = (m: MarkState, e: SelectionKey) =>
          this.fetchApi.MarkAll(m, e);
        break;
      case SelectionType.All:
        functor = (m: MarkState, e: SelectionKey) =>
          this.fetchApi.MarkAll(m, e);
        break;
      default:
        throw new Error(`Unexpected enclosing type: ${type}`);
    }

    // Optimistically update the state immediately.
    this.setState((prevState: AppState): AppState => {
      const contentTreeCls: ContentTreeCls = prevState.contentTreeCls;
      contentTreeCls.Mark(mark, entity, type);
      return {
        ...prevState,
        contentTreeCls: contentTreeCls,
      };
    });

    // Perform the API call in the background and log any errors.
    functor(mark, entity).catch((err: Error) => {
      console.error(
        `Failed to persist mark state change to server for key ${entity}: ${err}`
      );
    });
  };

  handleClearRead = (selectedArticleId: ArticleId | null) => {
    this.setState((prevState: AppState): AppState => {
      const contentTreeCls = prevState.contentTreeCls;
      contentTreeCls.PruneReadPins(selectedArticleId);
      return {
        ...prevState,
        contentTreeCls: contentTreeCls,
      };
    });
  };

  handleToggleHideEmpty = () => {
    this.setState((prevState: AppState): AppState => {
      return {
        ...prevState,
        hideEmpty: !prevState.hideEmpty,
      };
    });
  };

  handleSelect = (type: SelectionType, key: SelectionKey) => {
    this.setState({
      selectionKey: key,
      selectionType: type,
    });
  };

  handleNavigateToAdjacentEntry = (direction: NavigationDirection) => {
    const { selectionKey, selectionType } = this.state;
    const folderFeedView = this.state.contentTreeCls.GetFolderFeedView();

    if (selectionType === SelectionType.Feed) {
      const [currentFeedId] = selectionKey as FeedSelection;
      const result = getAdjacentFeed(folderFeedView, currentFeedId, direction);
      if (result !== null) {
        this.handleSelect(SelectionType.Feed, result);
      }
    } else if (selectionType === SelectionType.Folder) {
      const currentFolderId = selectionKey as FolderSelection;
      const result = getAdjacentFolder(
        folderFeedView,
        currentFolderId,
        direction
      );
      if (result !== null) {
        this.handleSelect(SelectionType.Folder, result);
      }
    }
  };

  render() {
    // If verification has completed and failed, redirect to the login page.
    if (
      this.state.status & Status.LoginVerification &&
      !this.state.loginVerified
    ) {
      return <Navigate to={GoliathPath.Login} replace={true} />;
    }

    if (this.state.status !== Status.Ready) {
      return (
        <ThemeProvider theme={this.state.themeInfo.theme}>
          <CssBaseline />
          <Loading status={this.state.status} />
        </ThemeProvider>
      );
    }

    const unreadCount: number = this.state.contentTreeCls.UnreadCount();
    if (unreadCount === 0) {
      document.title = 'Goliath RSS';
    } else {
      document.title = `(${unreadCount}) Goliath RSS`;
    }

    const selectionKey: SelectionKey = this.state.selectionKey;
    const selectionType: SelectionType = this.state.selectionType;

    // Sync theme class to documentElement so CSS variables are accessible
    // to portaled content (e.g. MUI Dialog) outside the React tree.
    document.documentElement.className = this.state.themeInfo.themeClasses;

    return (
      <ThemeProvider theme={this.state.themeInfo.theme}>
        {/* TODO: Is there a better way to inject overrides than this? */}
        <CssBaseline />
        <Box
          sx={{ display: 'flex', overflow: 'hidden', height: '100vh' }}
          className={`${this.state.themeInfo.themeClasses}`}
        >
          <Drawer variant="permanent" anchor="left" className="GoliathDrawer">
            <Box className="GoliathDrawerActionBar">
              <IconButton
                aria-label="Account"
                className="GoliathButton"
                size="small"
              >
                <AccountCircleTwoToneIcon />
              </IconButton>
              <div className="GoliathActionBarSpacer"></div>
              <IconButton
                aria-label="Account"
                className="GoliathButton"
                size="small"
              >
                <LogoutTwoToneIcon />
              </IconButton>
            </Box>
            <Box className="GoliathLogo">Goliath</Box>
            <FolderFeedList
              folderFeedView={this.state.contentTreeCls.GetFolderFeedView()}
              unreadCount={unreadCount}
              selectedKey={selectionKey}
              selectionType={selectionType}
              handleSelect={this.handleSelect}
              hideEmpty={this.state.hideEmpty}
              toggleHideEmpty={() => this.handleToggleHideEmpty()}
            />
          </Drawer>
          <Box
            component="main"
            className="GoliathMainContainer"
            sx={{ display: 'flex', flexGrow: 1 }}
          >
            <ArticleList
              articleEntriesCls={this.state.contentTreeCls.GetArticleView(
                selectionKey,
                selectionType
              )}
              faviconMap={this.state.contentTreeCls.GetFaviconMap()}
              selectionKey={selectionKey}
              selectionType={selectionType}
              handleMark={this.handleMark}
              clearReadCallback={this.handleClearRead}
              selectAllCallback={() =>
                this.handleSelect(SelectionType.All, KeyAllItems)
              }
              selectUnreadCallback={() =>
                this.handleSelect(SelectionType.Unread, KeyUnread)
              }
              buildTimestamp={this.state.buildTimestamp}
              buildHash={this.state.buildHash}
              navigateToAdjacentEntry={
                selectionType === SelectionType.Feed ||
                selectionType === SelectionType.Folder
                  ? this.handleNavigateToAdjacentEntry
                  : undefined
              }
              showKeybindingsModal={this.state.showKeybindingsModal}
            />
          </Box>
        </Box>
        <KeybindingsModal
          open={this.state.showKeybindingsModal}
          onClose={() => this.setState({ showKeybindingsModal: false })}
        />
      </ThemeProvider>
    );
  }
}
