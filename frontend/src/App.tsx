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
  KeySaved,
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
  Button,
  CssBaseline,
  Dialog,
  Divider,
  Drawer,
  IconButton,
  LinearProgress,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  ThemeProvider,
  Typography,
} from '@mui/material';
import { FetchAPI, FetchAPIFactory } from './api/interface';
import { GetVersion, VersionData } from './api/goliath';
import { Navigate } from 'react-router-dom';
import { ContentTreeCls } from './models/contentTree';
import { ArticleId } from './models/article';
import { FeedId } from './models/feed';
import { FolderId } from './models/folder';
import {
  getAdjacentFeed,
  getAdjacentFolder,
  populateThemeInfo,
} from './utils/helpers';
import AccountCircleTwoToneIcon from '@mui/icons-material/AccountCircleTwoTone';
import ChevronLeftTwoToneIcon from '@mui/icons-material/ChevronLeftTwoTone';
import KeyboardTwoToneIcon from '@mui/icons-material/KeyboardTwoTone';
import LogoutTwoToneIcon from '@mui/icons-material/LogoutTwoTone';
import MenuTwoToneIcon from '@mui/icons-material/MenuTwoTone';
import SettingsTwoToneIcon from '@mui/icons-material/SettingsTwoTone';
import KeybindingsModal from './components/KeybindingsModal';
import SettingsModal from './components/SettingsModal';
import QuickAddDialog from './components/QuickAddDialog';
import { Keybindings, getTinykeysSequence } from './utils/keybindings';
import { keybindRegistry } from './utils/keybindRegistry';

export function getLayoutMetrics(width: number, height: number) {
  const isMobile = width < 600 || height < 500;
  const isTabletPortrait =
    !isMobile &&
    width >= 600 &&
    (width < 900 || (width <= 1024 && height > width));
  const isTabletLandscape =
    !isMobile &&
    !isTabletPortrait &&
    width >= 900 &&
    width < 1200 &&
    height >= 500;
  return { isMobile, isTabletPortrait, isTabletLandscape };
}

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
  showSettingsModal: boolean;
  showQuickAdd: boolean;
  showLogoutConfirm: boolean;
  menuAnchor: HTMLElement | null;
  reloading: boolean;
  isMobile: boolean;
  isTabletPortrait: boolean;
  isTabletLandscape: boolean;
  mobilePane: 'list' | 'card';
  tabletShowFeedList: boolean;
  drawerOpen: boolean;
}

export default class App extends React.Component<AppProps, AppState> {
  private fetchApi: FetchAPI;
  private globalHandlers: Record<string, () => void>;
  private resizeListener?: () => void;

  constructor(props: AppProps) {
    super(props);
    const metrics =
      typeof window !== 'undefined'
        ? getLayoutMetrics(window.innerWidth, window.innerHeight)
        : {
            isMobile: false,
            isTabletPortrait: false,
            isTabletLandscape: false,
          };

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
      showSettingsModal: false,
      showQuickAdd: false,
      showLogoutConfirm: false,
      menuAnchor: null,
      reloading: false,
      isMobile: metrics.isMobile,
      isTabletPortrait: metrics.isTabletPortrait,
      isTabletLandscape: metrics.isTabletLandscape,
      mobilePane: 'list',
      tabletShowFeedList: true,
      drawerOpen: false,
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
      toggleSettingsModal: () => {
        this.setState((prevState: AppState): AppState => {
          return {
            ...prevState,
            showSettingsModal: !prevState.showSettingsModal,
          };
        });
      },
    };
  }

  // Whether anything is layered over the page. Key handlers are suspended
  // while something is: the dialogs hold text fields, and a letter typed into
  // one must not also scroll the article list or switch the theme. The menu
  // has no field, but it does take the arrow keys for its own navigation.
  private isModalOpen(state: Readonly<AppState>): boolean {
    return (
      state.showKeybindingsModal ||
      state.showSettingsModal ||
      state.showQuickAdd ||
      state.showLogoutConfirm ||
      state.menuAnchor !== null
    );
  }

  // Ends the session and returns to the login page.
  //
  // The redirect happens whether or not the server answered. A browser asking
  // to sign out ends up signed out, and leaving it on a page whose credential
  // may or may not still work is the worse of the two failures.
  private handleLogout = async (): Promise<void> => {
    try {
      await this.fetchApi.Logout();
    } catch (e) {
      console.log('Logout failed: ' + e);
    }
    // Back to the state startup leaves behind when verification says no,
    // which is what the redirect below reads. Becoming ready overwrites the
    // status rather than adding to it, so the verification bit has to be put
    // back for the two to agree.
    this.setState({
      showLogoutConfirm: false,
      loginVerified: false,
      status: Status.LoginVerification,
    });
  };

  componentWillUnmount() {
    keybindRegistry.unregister('global');
    if (this.resizeListener) {
      window.removeEventListener('resize', this.resizeListener);
    }
  }

  componentDidMount() {
    this.resizeListener = () => {
      const metrics = getLayoutMetrics(window.innerWidth, window.innerHeight);
      this.setState({
        isMobile: metrics.isMobile,
        isTabletPortrait: metrics.isTabletPortrait,
        isTabletLandscape: metrics.isTabletLandscape,
      });
    };
    window.addEventListener('resize', this.resizeListener);

    // This is defense-in-depth to redirect to the login page if the session
    // cookie is not present or no longer valid. The same check is done on the
    // server side, which returns an HTTP redirect.
    this.fetchApi.ResumeSession().then(async (ok: boolean): Promise<void> => {
      console.log('Resumed session.');
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
        // The shortcuts dialog is the one a shortcut may still close.
        if (this.isModalOpen(this.state)) {
          if (
            !this.state.showKeybindingsModal ||
            kb.handlerKey !== 'toggleKeybindingsModal'
          ) {
            return;
          }
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
      case SelectionType.Saved:
        functor = async (m: MarkState, e: SelectionKey) => {
          const articles = this.state.contentTreeCls.GetArticleView(
            e,
            SelectionType.Saved
          );
          const promises = articles.map((article) => {
            const isTargetState =
              m === MarkState.Unsaved ? article.isSaved : !article.isSaved;
            if (isTargetState) {
              return this.fetchApi.MarkArticle(m, [
                article.id,
                article.feedId,
                article.folderId,
              ]);
            }
            return Promise.resolve(null);
          });
          await Promise.all(promises);
          return new Response('OK');
        };
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

  handleUpdateArticleParsed = (
    articleId: ArticleId,
    feedId: FeedId,
    folderId: FolderId,
    parsed: string
  ) => {
    this.setState((prevState: AppState): AppState => {
      const contentTreeCls = prevState.contentTreeCls;
      contentTreeCls.UpdateArticleParsed(articleId, feedId, folderId, parsed);
      return {
        ...prevState,
        contentTreeCls: contentTreeCls,
      };
    });
  };

  // Re-reads everything from the server and swaps the tree in place. Used
  // after the subscription list changes, which nothing else here can reflect.
  // Selection is kept where it still exists; a feed or folder that is now
  // gone falls back to the unread stream, since the article view would
  // otherwise ask for a folder the tree no longer has.
  reloadContent = async (): Promise<void> => {
    const treeCls: ContentTreeCls = await this.fetchApi.InitializeContent(
      () => {}
    );
    this.setState((prevState: AppState): AppState => {
      const next: AppState = { ...prevState, contentTreeCls: treeCls };
      const folderFeedView = treeCls.GetFolderFeedView();
      let stillExists = true;
      if (prevState.selectionType === SelectionType.Folder) {
        const folderId = prevState.selectionKey as FolderSelection;
        stillExists = Array.from(folderFeedView.keys()).some(
          (f) => f.id === folderId
        );
      } else if (prevState.selectionType === SelectionType.Feed) {
        const [feedId, folderId] = prevState.selectionKey as FeedSelection;
        stillExists = Array.from(folderFeedView.entries()).some(
          ([folder, feeds]) =>
            folder.id === folderId && feeds.some((f) => f.id === feedId)
        );
      }
      if (!stillExists) {
        next.selectionKey = KeyUnread;
        next.selectionType = SelectionType.Unread;
      }
      return next;
    });
    console.log('Reloaded content.');
  };

  handleFeedsChanged = () => {
    this.setState({ reloading: true });
    this.reloadContent()
      .catch((err: Error) => {
        console.error(`Failed to reload content: ${err}`);
      })
      .finally(() => this.setState({ reloading: false }));
  };

  // Stable, so that a component re-reading the folder list when its view of
  // the tree changes does not also re-read it on every render of this one.
  listFolders = () => this.fetchApi.ListFolders();

  handleSelect = (type: SelectionType, key: SelectionKey) => {
    this.setState((prevState) => {
      const nextState: Partial<AppState> = {
        selectionKey: key,
        selectionType: type,
        drawerOpen: false,
      };
      if (prevState.isMobile) {
        nextState.mobilePane = 'list';
      }
      return nextState as AppState;
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

  getSelectionTitle(): string {
    const { selectionKey, selectionType, contentTreeCls } = this.state;
    if (selectionKey === KeyUnread) {
      return 'Unread items';
    } else if (selectionKey === KeyAllItems) {
      return 'All items';
    } else if (selectionKey === KeySaved) {
      return 'Saved items';
    }

    const folderFeedView = contentTreeCls.GetFolderFeedView();
    if (selectionType === SelectionType.Folder) {
      for (const folder of folderFeedView.keys()) {
        if (folder.id === selectionKey) {
          return folder.title;
        }
      }
    } else if (selectionType === SelectionType.Feed) {
      const [feedId, folderId] = selectionKey as FeedSelection;
      for (const [folder, feeds] of folderFeedView.entries()) {
        if (folder.id === folderId) {
          const feed = feeds.find((f) => f.id === feedId);
          if (feed) {
            return feed.title;
          }
        }
      }
    }
    return '';
  }

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
        <Box className={`GoliathAppShell ${this.state.themeInfo.themeClasses}`}>
          <Drawer
            variant={this.state.isMobile ? 'temporary' : 'permanent'}
            open={this.state.isMobile ? this.state.drawerOpen : undefined}
            onClose={
              this.state.isMobile
                ? () => this.setState({ drawerOpen: false })
                : undefined
            }
            anchor="left"
            className={
              this.state.isTabletPortrait && !this.state.tabletShowFeedList
                ? 'GoliathDrawer GoliathPaneHidden'
                : 'GoliathDrawer'
            }
          >
            <Box className="GoliathDrawerActionBar">
              {this.state.isMobile && (
                <IconButton
                  aria-label="Hide feed list"
                  className="GoliathButton"
                  size="small"
                  onClick={() => this.setState({ drawerOpen: false })}
                >
                  <ChevronLeftTwoToneIcon />
                </IconButton>
              )}
              <IconButton
                aria-label="Account"
                className="GoliathButton"
                size="small"
              >
                <AccountCircleTwoToneIcon />
              </IconButton>
              <div className="GoliathActionBarSpacer"></div>
              <IconButton
                aria-label="Menu"
                className="GoliathButton"
                size="small"
                aria-haspopup="true"
                aria-expanded={this.state.menuAnchor !== null}
                onClick={(e) => this.setState({ menuAnchor: e.currentTarget })}
              >
                <MenuTwoToneIcon />
              </IconButton>
            </Box>
            {this.state.reloading && (
              <LinearProgress
                className="GoliathReloadProgress"
                aria-label="Refreshing subscriptions"
              />
            )}
            <Box className="GoliathLogo">Goliath</Box>
            <FolderFeedList
              folderFeedView={this.state.contentTreeCls.GetFolderFeedView()}
              unreadCount={unreadCount}
              selectedKey={selectionKey}
              selectionType={selectionType}
              handleSelect={this.handleSelect}
              hideEmpty={this.state.hideEmpty}
              onAddFeed={() => this.setState({ showQuickAdd: true })}
            />
          </Drawer>
          <Box component="main" className="GoliathMainContainer">
            <ArticleList
              fetchApi={this.fetchApi}
              handleUpdateArticleParsed={this.handleUpdateArticleParsed}
              selectionTitle={this.getSelectionTitle()}
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
              selectSavedCallback={() =>
                this.handleSelect(SelectionType.Saved, KeySaved)
              }
              buildTimestamp={this.state.buildTimestamp}
              buildHash={this.state.buildHash}
              navigateToAdjacentEntry={
                selectionType === SelectionType.Feed ||
                selectionType === SelectionType.Folder
                  ? this.handleNavigateToAdjacentEntry
                  : undefined
              }
              modalOpen={this.isModalOpen(this.state)}
              isMobile={this.state.isMobile}
              isTabletPortrait={this.state.isTabletPortrait}
              isTabletLandscape={this.state.isTabletLandscape}
              mobilePane={this.state.mobilePane}
              tabletShowFeedList={this.state.tabletShowFeedList}
              onMobileNavigate={(pane: 'list' | 'card') =>
                this.setState({ mobilePane: pane })
              }
              onArticleSelect={() => {
                if (this.state.isTabletPortrait) {
                  this.setState({ tabletShowFeedList: false });
                }
              }}
              openDrawer={() => {
                if (this.state.isTabletPortrait) {
                  this.setState({ tabletShowFeedList: true });
                } else {
                  this.setState({ drawerOpen: true });
                }
              }}
            />
          </Box>
        </Box>
        <Menu
          open={this.state.menuAnchor !== null}
          anchorEl={this.state.menuAnchor}
          onClose={() => this.setState({ menuAnchor: null })}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          transformOrigin={{ vertical: 'top', horizontal: 'right' }}
          slotProps={{
            paper: { className: 'GoliathMenuPaper GoliathAppMenuPaper' },
            list: { 'aria-label': 'Menu', dense: true },
          }}
        >
          <MenuItem
            onClick={() =>
              this.setState({ menuAnchor: null, showSettingsModal: true })
            }
          >
            <ListItemIcon>
              <SettingsTwoToneIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>Settings</ListItemText>
          </MenuItem>
          <MenuItem
            onClick={() =>
              this.setState({ menuAnchor: null, showKeybindingsModal: true })
            }
          >
            <ListItemIcon>
              <KeyboardTwoToneIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>Keyboard shortcuts</ListItemText>
          </MenuItem>
          <Divider className="GoliathAppMenuDivider" />
          <MenuItem
            className="GoliathAppMenuDangerItem"
            onClick={() =>
              this.setState({ menuAnchor: null, showLogoutConfirm: true })
            }
          >
            <ListItemIcon>
              <LogoutTwoToneIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>Log out</ListItemText>
          </MenuItem>
        </Menu>
        <Dialog
          open={this.state.showLogoutConfirm}
          onClose={() => this.setState({ showLogoutConfirm: false })}
          maxWidth="xs"
          fullWidth
          slotProps={{
            backdrop: { className: 'GoliathModalOverlay' },
            paper: { className: 'GoliathModalPaper' },
          }}
        >
          <Box className="GoliathDialogHeader">
            <Typography component="h2" className="GoliathDialogHeading">
              Log out
            </Typography>
          </Box>
          <Box className="GoliathDialogContent">
            <Typography className="GoliathDialogText">
              Are you sure you want to log out?
            </Typography>
          </Box>
          <Box className="GoliathDialogFooter">
            <Button
              className="GoliathQuietButton"
              onClick={() => this.setState({ showLogoutConfirm: false })}
            >
              Cancel
            </Button>
            <Button
              variant="contained"
              className="GoliathDangerButton"
              onClick={this.handleLogout}
              startIcon={<LogoutTwoToneIcon />}
            >
              Log out
            </Button>
          </Box>
        </Dialog>
        <KeybindingsModal
          open={this.state.showKeybindingsModal}
          onClose={() => this.setState({ showKeybindingsModal: false })}
        />
        <SettingsModal
          open={this.state.showSettingsModal}
          onClose={() => this.setState({ showSettingsModal: false })}
          isMobile={this.state.isMobile}
          reloading={this.state.reloading}
          theme={this.state.theme}
          onToggleTheme={this.globalHandlers.toggleTheme}
          hideEmpty={this.state.hideEmpty}
          onToggleHideEmpty={this.globalHandlers.toggleHideEmpty}
          onShowKeybindings={() =>
            this.setState({
              showSettingsModal: false,
              showKeybindingsModal: true,
            })
          }
          buildTimestamp={this.state.buildTimestamp}
          buildHash={this.state.buildHash}
          folderFeedView={this.state.contentTreeCls.GetFolderFeedView()}
          onAddFeed={() => this.setState({ showQuickAdd: true })}
          renameFeed={(feedId, title) =>
            this.fetchApi.RenameFeed(feedId, title)
          }
          moveFeed={(feedId, to, from) =>
            this.fetchApi.MoveFeed(feedId, to, from)
          }
          unsubscribeFeed={(feedId) => this.fetchApi.UnsubscribeFeed(feedId)}
          listFolders={this.listFolders}
          moveFeedToNewFolder={(feedId, name, from) =>
            this.fetchApi.MoveFeedToNewFolder(feedId, name, from)
          }
          renameFolder={(folderId, name) =>
            this.fetchApi.RenameFolder(folderId, name)
          }
          deleteFolder={(folderId) => this.fetchApi.DeleteFolder(folderId)}
          onFeedsChanged={this.handleFeedsChanged}
        />
        <QuickAddDialog
          open={this.state.showQuickAdd}
          onClose={() => this.setState({ showQuickAdd: false })}
          folderFeedView={this.state.contentTreeCls.GetFolderFeedView()}
          addFeed={(url) => this.fetchApi.AddFeed(url)}
          moveFeed={(feedId, to) => this.fetchApi.MoveFeed(feedId, to)}
          listFolders={this.listFolders}
          moveFeedToNewFolder={(feedId, name) =>
            this.fetchApi.MoveFeedToNewFolder(feedId, name)
          }
          unsubscribeFeed={(feedId) => this.fetchApi.UnsubscribeFeed(feedId)}
          onChanged={this.handleFeedsChanged}
        />
      </ThemeProvider>
    );
  }
}
