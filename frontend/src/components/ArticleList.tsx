import Article from './ArticleCard';
import React from "react";
import ReactList from 'react-list';
import {animateScroll as scroll} from 'react-scroll';
import {
  ArticleListEntry,
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {Box, Container, Grid, Typography} from "@mui/material";
import InboxIcon from '@mui/icons-material/Inbox';
import SplitViewArticleCard from "./SplitViewArticleCard";

const goToAllSequence = ['g', 'a'];
const markAllRead = ['Shift', 'I'];
const keyBufLength = 2;

export interface ArticleListProps {
  articleEntries: ArticleListEntry[];
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  selectAllCallback: () => void;
  handleMark: (mark: MarkState, entity: SelectionKey, type: SelectionType) => void;
}

export interface ArticleListState {
  articleEntries: ArticleListEntry[];

  scrollIndex: number;
  keypressBuffer: Array<string>;
}

export default class ArticleList extends React.Component<ArticleListProps, ArticleListState> {
  list: ReactList | null = null;

  constructor(props: ArticleListProps) {
    super(props);
    this.state = {
      articleEntries: props.articleEntries,
      scrollIndex: 0,
      keypressBuffer: new Array(keyBufLength)
    };
  }

  componentDidMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  }

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  }

  componentDidUpdate(prevProps: ArticleListProps) {
    // Reset scroll position when the enclosing key changes.
    if (prevProps.selectionKey === this.props.selectionKey) {
      return;
    }
    if (this.list) {
      this.list.scrollTo(0);
      // @ts-ignore
      this.list.forceUpdate();
    }
    this.setState({
      articleEntries: Array.from(this.props.articleEntries),
      scrollIndex: -1,
      keypressBuffer: new Array(keyBufLength)
    });
  }

  render() {
    if (this.state.articleEntries.length === 0) {
      return (
        <Container fixed className="GoliathArticleListContainer">
          <Box className="GoliathArticleListEmpty">
            <InboxIcon className="GoliathArticleListEmptyIcon"/>
            <Typography className="GoliathArticleListEmptyText">
              No unread articles
            </Typography>
          </Box>
        </Container>
      )
    } else {
      const articles = this.state.articleEntries;
      const renderIndex = Math.max(0, this.state.scrollIndex);
      const [article, title, favicon] = articles[renderIndex];

      // Impossible, but not statically, so lint shuts up.
      if (this.state.scrollIndex === -10) {
        return (
          <Box className="GoliathArticleListContainer">
            <ReactList
              ref={this.handleMounted}
              itemRenderer={(e) => this.renderArticle(articles, e)}
              length={articles.length}
              type='variable'/>
          </Box>
        )
      }


      return (
        <Container maxWidth={false} className="GoliathArticleListContainer">
          <Grid container spacing={3}>
            <Grid container item xs={4} wrap="nowrap">
              <div style={{
                overflowY: 'scroll',
                width: "100vh",
                height: "100vh",
                paddingTop: "15px",
                paddingRight: "15px"
              }}>
                <ReactList
                  ref={this.handleMounted}
                  itemRenderer={(e) => this.renderArticleListEntry(articles, e)}
                  length={articles.length}
                  type='uniform'/>
              </div>
            </Grid>
            <Grid item xs={8}>
              <SplitViewArticleCard
                key={article.id.toString()}
                article={article}
                title={title}
                favicon={favicon}
                isSelected={true}
                shouldRerender={() => ({})}/>
            </Grid>
          </Grid>
        </Container>
      )
    }
  }

  renderArticleListEntry(articles: ArticleListEntry[], index: number) {
    const [article, title, favicon] = articles[index];
    let t;

    if (!favicon) {
      t = <i className="fas fa-rss-square"/>
    } else {
      t = <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }

    let css;

    css = {
      borderBottom: "1px solid gray",
      padding: "10px",
    };

    if (index == this.state.scrollIndex) {
      css = {
        borderBottom: "1px solid gray",
        padding: "10px",
        background: "rgb(0.1, 0.1, 0.1)"
      };
    }

    return (
      // <Paper elevation={elevation}>
      <Grid zeroMinWidth container direction="column" style={css}>
        <Grid zeroMinWidth item xs>
          <Typography noWrap className="GoliathArticleTitle">
            {this.extractContent(article.title)}
          </Typography>
        </Grid>
        <Grid zeroMinWidth container item xs>
          <Grid item sx={{paddingRight: "10px"}} xs="auto">
            {t}
          </Grid>
          <Grid item zeroMinWidth xs>
            <Typography noWrap className="GoliathArticleFeedTitle">
              {this.extractContent(title)}
            </Typography>
          </Grid>
        </Grid>
        <Grid container item wrap="nowrap">
          <Grid item zeroMinWidth xs>
            <Typography noWrap className="GoliathArticleContent">
              {this.extractContent(article.html)}
            </Typography>
          </Grid>
        </Grid>
      </Grid>
      // </Paper>
    );
  }

  extractContent(html: string) {
    return new DOMParser()
      .parseFromString(html, "text/html")
      .documentElement.textContent;
  }

  renderArticle(articles: ArticleListEntry[], index: number) {
    const [article, title, favicon] = articles[index];

    return <Article
      key={article.id.toString()}
      article={article}
      title={title}
      favicon={favicon}
      isSelected={index === this.state.scrollIndex}
      shouldRerender={() => this.handleRerender()}/>
  }

  handleRerender() {
    if (this.list !== null) {
      // An element in the list has dynamically resized, so cached sizing
      // calculations are incorrect. Forcibly trigger a render again so that
      // the new size can be accounted for correctly.
      this.list.forceUpdate();
    }
  }

  handleKeyDown = (event: KeyboardEvent) => {
    // Ignore keypress events when some modifiers are also enabled to avoid
    // triggering on (e.g.) browser shortcuts. Shift is the exception here since
    // we do care about Shift+I.
    if (event.altKey || event.metaKey || event.ctrlKey) {
      return;
    }

    this.setState((prevState) => {
      let scrollIndex = prevState.scrollIndex;
      let articleEntries = Array.from(prevState.articleEntries);

      // Add new keypress to buffer, dropping the oldest entry.
      let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

      // If this sequence is fulfilled, reset the buffer and handle it.
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        this.props.selectAllCallback();
        keypressBuffer = new Array(keyBufLength);
      } else if (markAllRead.every((e, i) => e === keypressBuffer[i])) {
        this.props.handleMark(
          'read', this.props.selectionKey, this.props.selectionType);
        articleEntries.forEach((e: ArticleListEntry) => {
          const [article] = e;
          article.is_read = 1
        });
        keypressBuffer = new Array(keyBufLength);
      } else {
        switch (event.key) {
        case 'ArrowDown':
          event.preventDefault(); // fallthrough
        case 'j':
          scrollIndex = Math.min(
            prevState.scrollIndex + 1, this.state.articleEntries.length - 1);
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          scrollIndex = Math.max(prevState.scrollIndex - 1, 0);
          break;
        case 'v':
          // If trying to open an article before any articles are selected,
          // treat it like a scroll to the first article.
          if (scrollIndex === -1) {
            scrollIndex = 0;
          }

          const [article] = this.state.articleEntries[scrollIndex];
          window.open(article.url, '_blank');
          break;
        default:
          // No known key pressed, just ignore.
          break;
        }
      }
      return {
        articleEntries,
        keypressBuffer,
        scrollIndex
      };
    }, this.handleScroll);
  };

  handleMounted = (list: ReactList) => {
    this.list = list;
  };

  handleScroll() {
    // If this feed is empty, there's no list, so nothing to scroll.
    if (this.list) {
      // A scroll index of less than 0 is meaningless.
      if (this.state.scrollIndex < 0) {
        return;
      }

      // Animate scrolling using technique described by react-list author here:
      // https://github.com/coderiety/react-list/issues/79
      // @ts-ignore
      const scrollPos = this.list.getSpaceBefore(this.state.scrollIndex);

      scroll.scrollTo(scrollPos, {
        // The scrolling container is not trivial to figure out, but react-list
        // has already done the work to figure it out, so use it directly.
        // @ts-ignore
        container: this.list.scrollParent,
        isDynamic: true,
        duration: 400,
        smooth: "easeInOutQuad",
      });

      this.setState((prevState: ArticleListState) => {
        const articleEntries = Array.from(prevState.articleEntries);
        const entry = articleEntries[prevState.scrollIndex];
        const [article, , , feedId, folderId] = entry;

        if (!(article.is_read === 1)) {
          this.props.handleMark(
            'read', [article.id, feedId, folderId], SelectionType.Article);
          article.is_read = 1;
        }
        return {
          articleEntries
        };
      });
    }
  }
}