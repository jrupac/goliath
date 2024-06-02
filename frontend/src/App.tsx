import './App.css';
import ArticleList from './components/ArticleList';
import FolderFeedList from './components/FolderFeedList';
import Loading from './components/Loading';
import React from 'react';
import {
  GoliathPath,
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
import {FetchAPI, FetchAPIFactory} from "./api/interface";
import {GetVersion, VersionData} from "./api/goliath";
import {Navigate} from "react-router-dom";
import {ContentTreeCls} from "./models/contentTree";

export interface AppProps {
}

export interface AppState {
  buildTimestamp: string;
  buildHash: string;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  status: Status;
  contentTreeCls: ContentTreeCls;
  theme: Theme;
  loginVerified: boolean;
}

export default class App extends React.Component<AppProps, AppState> {
  private fetchApi: FetchAPI;

  constructor(props: AppProps) {
    super(props);
    this.state = {
      buildTimestamp: "",
      buildHash: "",
      selectionKey: KeyAll,
      selectionType: SelectionType.All,
      status: Status.Start,
      contentTreeCls: ContentTreeCls.new(),
      theme: Theme.Dark,
      loginVerified: false,
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
    this.fetchApi.VerifyAuth().then(async (ok: boolean): Promise<void> => {
      this.setState({loginVerified: ok});
      this.updateState(Status.LoginVerification);
      // Don't try to initialize data if login verification failed since
      // at render, this will result in a redirect to the login page anyway.
      if (ok) {
        await this.init();
      }
    })
  }

  async init(): Promise<void> {
    window.addEventListener('keydown', this.handleKeyDown);

    const versionData: VersionData = await GetVersion();
    this.setState({
      buildTimestamp: versionData.build_timestamp,
      buildHash: versionData.build_hash
    })
    console.log("Fetched version info.")

    const treeCls: ContentTreeCls = await this.fetchApi.InitializeContent(
      this.updateState);
    this.setState({
      contentTreeCls: treeCls,
      status: Status.Ready
    });
    console.log("Completed all Fever requests.")
  }

  updateState = (status: Status): void => {
    this.setState((prevState: Readonly<AppState>): { status: Status } => {
      return {status: prevState.status | status};
    });
  }

  handleMark = (mark: MarkState, entity: SelectionKey, type: SelectionType) => {
    let functor;

    switch (type) {
    case SelectionType.Article:
      functor = this.fetchApi.MarkArticle;
      break;
    case SelectionType.Feed:
      functor = this.fetchApi.MarkFeed;
      break;
    case SelectionType.Folder:
      functor = this.fetchApi.MarkFolder;
      break;
    case SelectionType.All:
      functor = this.fetchApi.MarkAll;
      break;
    default:
      throw new Error(`Unexpected enclosing type: ${type}`);
    }

    functor(mark, entity).then((): void => {
      this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
        const contentTreeCls: ContentTreeCls = prevState.contentTreeCls;
        contentTreeCls.Mark(mark, entity, type);
        return {
          contentTreeCls: contentTreeCls,
        } as AppState
      });
    });
  };

  handleSelect = (type: SelectionType, key: SelectionKey) => {
    this.setState({
      selectionKey: key,
      selectionType: type,
    });
  };

  handleKeyDown = (event: KeyboardEvent) => {
    // Ignore keypress events when some modifiers are also enabled to avoid
    // triggering on (e.g.) browser shortcuts. Shift is the exception here since
    // we do care about Shift+I.
    if (event.altKey || event.metaKey || event.ctrlKey) {
      return;
    }

    switch (event.key) {
    case 't':
      this.setState((prevState: Readonly<AppState>) => {
        return {
          theme: prevState.theme === Theme.Default ? Theme.Dark : Theme.Default
        }
      });
      break;
    }
  }

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

    // If login verification has completed and failed, redirect to login page.
    if ((this.state.status & Status.LoginVerification) &&
      !this.state.loginVerified) {
      return <Navigate to={GoliathPath.Login} replace={true}/>;
    }

    if (this.state.status !== Status.Ready) {
      return (
        <ThemeProvider theme={theme}>
          <CssBaseline/>
          <Loading status={this.state.status}/>
        </ThemeProvider>);
    }

    const unreadCount: number = this.state.contentTreeCls.UnreadCount();
    if (unreadCount === 0) {
      document.title = 'Goliath RSS';
    } else {
      document.title = `(${unreadCount})  Goliath RSS`;
    }

    const selectionKey: SelectionKey = this.state.selectionKey;
    const selectionType: SelectionType = this.state.selectionType;

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
                folderFeedView={this.state.contentTreeCls.GetFolderFeedView()}
                unreadCount={unreadCount}
                selectedKey={selectionKey}
                selectionType={selectionType}
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
                articleEntriesCls={this.state.contentTreeCls.GetArticleView(
                  selectionKey, selectionType)}
                selectionKey={selectionKey}
                selectionType={selectionType}
                handleMark={this.handleMark}
                selectAllCallback={() => this.handleSelect(
                  SelectionType.All, KeyAll)}/>
            </Box>
          </Box>
        </Box>
      </ThemeProvider>
    );
  }
}