import ArticleCard from './ArticleCard';
import React from "react";
import ReactList from 'react-list';
import {animateScroll as scroll} from 'react-scroll';
import {
  Article,
  ArticleId,
  ArticleImagePreview,
  ArticleListEntry,
  ArticleListView,
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {Box, Container, Divider, Grid, Typography} from "@mui/material";
import SplitViewArticleCard from "./SplitViewArticleCard";
import SplitViewArticleListEntry from "./SplitViewArticleListEntry";
import LRUCache from "lru-cache";
import {DoneAllRounded} from "@mui/icons-material";
import smartcrop from "smartcrop";

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
  articleImagePreviews: LRUCache<ArticleId, ArticleImagePreview>;

  scrollIndex: number;
  smoothScroll: boolean;
  keypressBuffer: Array<string>;
  articleViewToggleState: ArticleListView
}

export default class ArticleList extends React.Component<ArticleListProps, ArticleListState> {
  list: ReactList | null = null;
  inflightPreview: Set<ArticleId> = new Set<ArticleId>();

  constructor(props: ArticleListProps) {
    super(props);
    this.state = {
      articleEntries: props.articleEntries,
      articleImagePreviews: new LRUCache<ArticleId, ArticleImagePreview>({
        max: 100
      }),
      scrollIndex: 0,
      smoothScroll: false,
      keypressBuffer: new Array(keyBufLength),
      articleViewToggleState: ArticleListView.Split
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
      scrollIndex: 0,
      keypressBuffer: new Array(keyBufLength)
    });
  }

  render() {
    if (this.state.articleEntries.length === 0) {
      return (
        <Container fixed className="GoliathArticleListContainer">
          <Box className="GoliathArticleListEmpty">
            <DoneAllRounded className="GoliathArticleListEmptyIcon"/>
            <Typography className="GoliathArticleListEmptyText">
              All done!
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
          <Box className="GoliathArticleListBox">
            <ReactList
              ref={this.handleMounted}
              itemRenderer={(e) => this.renderArticle(e)}
              length={articles.length}
              type='variable'/>
          </Box>
        )
      }

      return (
        <Container
          maxWidth={false}
          className="GoliathSplitViewArticleListContainer">
          <Grid container>
            <Grid container item xs={4}>
              <Box className="GoliathSplitViewArticleListBox">
                <ReactList
                  ref={this.handleMounted}
                  itemRenderer={(e) => this.renderSplitViewArticleListEntry(e)}
                  length={articles.length}
                  threshold={500}
                  type='uniform'/>
              </Box>
            </Grid>
            <Grid item xs='auto'>
              <Divider orientation="vertical"/>
            </Grid>
            <Grid item xs className="GoliathSplitViewArticleOuter">
              <SplitViewArticleCard
                key={article.id.toString()}
                article={article}
                title={title}
                favicon={favicon}
                isSelected={true}/>
            </Grid>
          </Grid>
        </Container>
      )
    }
  }

  renderSplitViewArticleListEntry(index: number) {
    const articleEntry = this.state.articleEntries[index];
    const [article] = articleEntry;
    this.generateImagePreview(article).then();

    return <SplitViewArticleListEntry
      key={article.id.toString()}
      article={articleEntry}
      preview={this.state.articleImagePreviews.get(article.id)}
      selected={index === this.state.scrollIndex}/>
  }

  renderArticle(index: number) {
    const [article, title, favicon] = this.state.articleEntries[index];

    return <ArticleCard
      key={article.id.toString()}
      article={article}
      title={title}
      favicon={favicon}
      isSelected={index === this.state.scrollIndex}
      shouldRerender={() => this.handleRerender()}/>
  }

  async generateImagePreview(article: Article) {
    // Already generated preview for this article, so nothing to do.
    if (this.state.articleImagePreviews.has(article.id)) {
      return;
    }

    // There's already an inflight request for this article, so nothing to do.
    if (this.inflightPreview.has(article.id)) {
      return;
    }
    this.inflightPreview.add(article.id);

    const minPixelSize = 100, imgFetchLimit = 5;
    const p: Promise<any>[] = [];
    const images = new DOMParser()
      .parseFromString(article.html, "text/html").images;

    if (images === undefined) {
      return;
    }

    let limit = Math.min(imgFetchLimit, images.length)
    for (let j = 0; j < limit; j++) {
      const img = new Image();
      img.src = images[j].src;

      p.push(new Promise((resolve, reject) => {
        img.decode().then(() => {
          if (img.height >= minPixelSize && img.width >= minPixelSize) {
            resolve([img.height, img.width, img.src]);
          }
          reject();
        }).catch(() => {
          /* Ignore errors */
        });
      }));
    }

    const results = await Promise.allSettled(p);
    let height = 0, width = 0, src = "";

    results.forEach((result) => {
      if (result.status === 'fulfilled') {
        const [imgHeight, imgWidth, imgSrc] = result.value;
        if (imgHeight > height) {
          height = imgHeight;
          width = imgWidth;
          src = imgSrc;
        }
      }
    })

    if (height > 0) {
      const crop = await fetch(src)
        .then((f) => f.blob())
        .then(createImageBitmap)
        .then((i) => smartcrop.crop(i, {
          minScale: 0.001,
          height: minPixelSize,
          width: minPixelSize,
          ruleOfThirds: false
        }))
        .catch(() => {
          /* Ignore errors */
        });

      let imgPreview: ArticleImagePreview;

      if (crop) {
        imgPreview = {
          src: src,
          x: crop.topCrop.x,
          y: crop.topCrop.y,
          origWidth: width,
          width: crop.topCrop.width,
          height: crop.topCrop.height,
        }
      } else {
        // Finding a good crop didn't work, so just show the original image
        // This will be scaled appropriately when shown.
        imgPreview = {
          src: src,
          x: 0,
          y: 0,
          origWidth: width,
          width: width,
          height: height,
        }
      }

      this.setState((prevState) => {
        prevState.articleImagePreviews.set(article.id, imgPreview);
        return {
          articleImagePreviews: prevState.articleImagePreviews
        }
      }, () => this.inflightPreview.delete(article.id));
    }
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
      let smoothScroll = prevState.smoothScroll;
      let scrollIndex = prevState.scrollIndex;
      let articleEntries = Array.from(prevState.articleEntries);
      let articleViewToggleState = prevState.articleViewToggleState;
      const [article] = this.state.articleEntries[scrollIndex];

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
        case 'f':
          smoothScroll = !smoothScroll;
          break;
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

          window.open(article.url, '_blank');
          break;
        default:
          // No known key pressed, just ignore.
          break;
        }
      }

      const entry = articleEntries[prevState.scrollIndex];
      const [, , , feedId, folderId] = entry;

      if (!(article.is_read === 1)) {
        this.props.handleMark(
          'read', [article.id, feedId, folderId], SelectionType.Article);
        article.is_read = 1;
      }

      return {
        smoothScroll: smoothScroll,
        scrollIndex: scrollIndex,
        articleEntries: articleEntries,
        keypressBuffer: keypressBuffer,
        articleViewToggleState: articleViewToggleState
      };
    }, () => {
      if (this.list) {
        if (this.state.smoothScroll) {
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
            duration: 500,
            smooth: "easeInSine",
          });
        } else {
          this.list.scrollTo(this.state.scrollIndex);
        }
      }
    });
  };

  handleMounted = (list: ReactList) => {
    this.list = list;
  };
}