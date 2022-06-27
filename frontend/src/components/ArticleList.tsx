import Article from './ArticleCard';
import React from "react";
import ReactList from 'react-list';
import {animateScroll as scroll} from 'react-scroll';
import {
  ArticleImagePreview,
  ArticleListEntry,
  ArticleListView,
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {Box, Container, Grid, Typography} from "@mui/material";
import InboxIcon from '@mui/icons-material/Inbox';
import SplitViewArticleCard from "./SplitViewArticleCard";

const goToAllSequence = ['g', 'a'];
const markAllReadSequence = ['Shift', 'I'];
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
  articleImagePreviews: ArticleImagePreview[];

  scrollIndex: number;
  keypressBuffer: Array<string>;
  articleViewToggleState: ArticleListView
}

export default class ArticleList extends React.Component<ArticleListProps, ArticleListState> {
  list: ReactList | null = null;

  constructor(props: ArticleListProps) {
    super(props);
    this.state = {
      articleEntries: props.articleEntries,
      articleImagePreviews: new Array(props.articleEntries.length),
      scrollIndex: 0,
      keypressBuffer: new Array(keyBufLength),
      articleViewToggleState: ArticleListView.Combined
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
      articleImagePreviews: new Array(this.props.articleEntries.length),
      scrollIndex: 0,
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

      if (this.state.articleViewToggleState === ArticleListView.Combined) {
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
                height: "100vh"
              }}>
                <ReactList
                  ref={this.handleMounted}
                  itemRenderer={(e) => this.renderArticleListEntry(e)}
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

  async generateImagePreview(index: number) {
    // Already generated preview for this article, so nothing to do.
    if (this.state.articleImagePreviews[index] !== undefined) {
      return;
    }

    const minPixelSize = 100;
    const p: Promise<any>[] = [];
    const [article] = this.state.articleEntries[index];
    const images = new DOMParser()
      .parseFromString(article.html, "text/html").images;

    if (images === undefined) {
      return;
    }

    let limit = Math.min(5, images.length)
    for (let j = 0; j < limit; j++) {
      const img = new Image();
      img.src = images[j].src;

      p.push(new Promise((resolve, reject) => {
        img.decode().then(() => {
          if (img.height >= minPixelSize && img.width >= minPixelSize) {
            resolve([img.height, img.src]);
          }
          reject();
        }).catch(() => {
        });
      }));
    }

    const results = await Promise.allSettled(p);
    let height = 0;
    let src: string;

    results.forEach((result) => {
      if (result.status === 'fulfilled') {
        const [imgHeight, imgSrc] = result.value;
        if (imgHeight > height) {
          height = imgHeight;
          src = imgSrc;
        }
      }
    })

    if (height > 0) {
      this.setState((prevState) => {
        const imgPrevs = Array.from(prevState.articleImagePreviews);
        imgPrevs[index] = (
          <img className="GoliathArticleListImagePreview" src={src}/>);
        return {articleImagePreviews: imgPrevs}
      });
    }

  }

  renderArticleListEntry(index: number) {
    const articles = this.state.articleEntries
    const articlePrevs = this.state.articleImagePreviews;

    // We want this to be async, but add the .then() to quiet the warning.
    this.generateImagePreview(index).then();

    const [article, title, favicon] = articles[index];
    let faviconImg;

    if (!favicon) {
      faviconImg = <i className="fas fa-rss-square"/>
    } else {
      faviconImg = <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }

    let css: Record<string, string> = {
      borderBottom: "1px solid gray",
      padding: "10px",
    };

    if (index == this.state.scrollIndex) {
      css['background'] = "rgb(20, 20, 20)";
    }

    return (
      <Grid container direction="column" style={css} key={index}>
        <Grid zeroMinWidth item className="GoliathArticleListEntryContainer">
          <Typography noWrap className="GoliathArticleListTitle">
            {this.extractContent(article.title)}
          </Typography>
        </Grid>
        <Grid zeroMinWidth container item xs>
          <Grid item sx={{paddingRight: "10px"}} xs="auto">
            {faviconImg}
          </Grid>
          <Grid item zeroMinWidth xs>
            <Typography noWrap className="GoliathArticleFeedTitle">
              {this.extractContent(title)}
            </Typography>
          </Grid>
        </Grid>
        <Grid container item wrap="nowrap">
          <Grid item xs='auto'>
            {articlePrevs[index]}
          </Grid>
          <Grid item zeroMinWidth xs style={{height: '100px'}}>
            <Typography className="GoliathArticleContentPreview">
              {this.extractContent(article.html)}
            </Typography>
          </Grid>
        </Grid>
      </Grid>
    );
  }

  extractContent(html: string): string | null {
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
      let articleViewToggleState = prevState.articleViewToggleState;

      // Add new keypress to buffer, dropping the oldest entry.
      let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

      // If this sequence is fulfilled, reset the buffer and handle it.
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        this.props.selectAllCallback();
        keypressBuffer = new Array(keyBufLength);
      } else if (markAllReadSequence.every((e, i) => e === keypressBuffer[i])) {
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
        case 's':
          if (articleViewToggleState === ArticleListView.Combined) {
            articleViewToggleState = ArticleListView.Split;
          } else {
            articleViewToggleState = ArticleListView.Combined;
          }
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
        articleEntries: articleEntries,
        keypressBuffer: keypressBuffer,
        scrollIndex: scrollIndex,
        articleViewToggleState: articleViewToggleState
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
          articleEntries: articleEntries
        };
      });
    }
  }
}