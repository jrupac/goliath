import ArticleCard from './ArticleCard';
import React, {
  ReactElement,
  useCallback,
  useEffect,
  useRef,
  useState
} from "react";
import ReactList from 'react-list';
import {animateScroll} from 'react-scroll';
import {
  ArticleImagePreview,
  ArticleListView,
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {Box, Container, Divider, Grid} from "@mui/material";
import SplitViewArticleCard from "./SplitViewArticleCard";
import SplitViewArticleListEntry from "./SplitViewArticleListEntry";
import LRUCache from "lru-cache";
import {DoneAllRounded} from "@mui/icons-material";
import smartcrop from "smartcrop";

import {ArticleId, ArticleView} from "../models/article";

const goToAllSequence = ['g', 'a'];
const markAllReadSequence = ['Shift', 'I'];
const keyBufLength = 2;

export interface ArticleListProps {
  articleEntriesCls: ArticleView[];
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  selectAllCallback: () => void;
  handleMark: (mark: MarkState, entity: SelectionKey, type: SelectionType) => void;
}

interface ArticleListState {
  articleEntries: ArticleView[];
  articleImagePreviews: LRUCache<ArticleId, ArticleImagePreview>;
  showPreviews: boolean;
  scrollIndex: number;
  smoothScroll: boolean;
  keypressBuffer: Array<string>;
  articleViewToggleState: ArticleListView;
}

const ArticleList: React.FC<ArticleListProps> = ({
  articleEntriesCls,
  selectionKey,
  selectionType,
  selectAllCallback,
  handleMark,
}) => {
  const listRef = useRef<ReactList | null>(null);
  const inflightPreview = useRef<Set<ArticleId>>(new Set<ArticleId>());
  const [state, setState] = useState<ArticleListState>({
    articleEntries: articleEntriesCls,
    articleImagePreviews: new LRUCache<ArticleId, ArticleImagePreview>({
      max: 100
    }),
    showPreviews: false,
    scrollIndex: 0,
    smoothScroll: true,
    keypressBuffer: new Array(keyBufLength),
    articleViewToggleState: ArticleListView.Split
  });

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    // Ignore keypress events when some modifiers are also enabled to avoid
    // triggering on (e.g.) browser shortcuts. Shift is the exception here since
    // we do care about Shift+I.
    if (event.altKey || event.metaKey || event.ctrlKey) {
      return;
    }

    setState((prevState) => {
      let showPreviews = prevState.showPreviews;
      let smoothScroll = prevState.smoothScroll;
      let scrollIndex = prevState.scrollIndex;
      let articleEntries = Array.from(prevState.articleEntries);
      let articleViewToggleState = prevState.articleViewToggleState;
      const articleView: ArticleView = prevState.articleEntries[scrollIndex];

      // Add new keypress to buffer, dropping the oldest entry.
      let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

      // If this sequence is fulfilled, reset the buffer and handle it.
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        selectAllCallback();
        keypressBuffer = new Array(keyBufLength);
      } else if (markAllReadSequence.every((e, i) => e === keypressBuffer[i])) {
        handleMark(
          'read', selectionKey, selectionType);
        articleEntries.forEach((e: ArticleView) => e.is_read = 1);
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
            prevState.scrollIndex + 1, state.articleEntries.length - 1);
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          scrollIndex = Math.max(prevState.scrollIndex - 1, 0);
          break;
        case 'p':
          showPreviews = !showPreviews;
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

          window.open(articleView.url, '_blank');
          break;
        default:
          // No known key pressed, just ignore.
          break;
        }
      }

      if (!(articleView.is_read === 1)) {
        handleMark(
          'read', [articleView.id, articleView.feed_id, articleView.folder_id],
          SelectionType.Article);
        articleView.is_read = 1;
      }

      return {
        ...prevState,
        showPreviews: showPreviews,
        smoothScroll: smoothScroll,
        scrollIndex: scrollIndex,
        articleEntries: articleEntries,
        keypressBuffer: keypressBuffer,
        articleViewToggleState: articleViewToggleState
      };
    });
  }, [
    selectAllCallback, handleMark, selectionKey, selectionType,
    state.articleEntries, state.showPreviews, state.smoothScroll,
    state.scrollIndex, state.articleViewToggleState, state.keypressBuffer,
  ]);

  const generateImagePreview = useCallback(async (article: ArticleView) => {
    // Already generated preview for this article, so nothing to do.
    if (state.articleImagePreviews.has(article.id)) {
      return;
    }

    // There's already an inflight request for this article, so nothing to do.
    if (inflightPreview.current.has(article.id)) {
      return;
    }
    inflightPreview.current.add(article.id);

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

      setState((prevState) => {
        prevState.articleImagePreviews.set(article.id, imgPreview);
        inflightPreview.current.delete(article.id)
        return {
          ...prevState,
          articleImagePreviews: prevState.articleImagePreviews
        };
      });
    }
  }, [state.articleImagePreviews, inflightPreview, state.articleEntries]);

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  const prevSelectionKey = useRef<SelectionKey>(selectionKey);

  useEffect(() => {
    // Reset scroll position when the enclosing key changes.
    if (selectionKey === prevSelectionKey.current) {
      return;
    }
    if (listRef.current) {
      listRef.current.scrollTo(0);
      // @ts-ignore
      listRef.current.forceUpdate();
    }
    setState((prevState) => ({
      ...prevState,
      articleEntries: Array.from(articleEntriesCls),
      scrollIndex: 0,
      keypressBuffer: new Array(keyBufLength)
    }));
    prevSelectionKey.current = selectionKey;
  }, [selectionKey, articleEntriesCls]);

  const handleRerender = useCallback(() => {
    if (listRef.current !== null) {
      // An element in the list has dynamically resized, so cached sizing
      // calculations are incorrect. Forcibly trigger a render again so that
      // the new size can be accounted for correctly.
      listRef.current.forceUpdate();
    }
  }, [listRef]);

  const renderArticle = useCallback((index: number): ReactElement => {
    const articleView: ArticleView = state.articleEntries[index];

    return <ArticleCard
      key={articleView.id}
      article={articleView}
      title={articleView.feed_title}
      favicon={articleView.favicon}
      isSelected={index === state.scrollIndex}
      shouldRerender={handleRerender}
    />;
  }, [
    state.articleEntries, state.scrollIndex, handleRerender,
    state.articleViewToggleState
  ]);

  const renderSplitViewArticleListEntry = useCallback((index: number): ReactElement => {
    const articleView: ArticleView = state.articleEntries[index];
    if (state.showPreviews) {
      generateImagePreview(articleView).then();
    }

    return <SplitViewArticleListEntry
      key={articleView.id}
      articleView={articleView}
      preview={state.articleImagePreviews.get(articleView.id)}
      selected={index === state.scrollIndex}
    />;
  }, [
    state.articleEntries,
    state.scrollIndex, state.articleImagePreviews,
    state.showPreviews, generateImagePreview,
    state.articleViewToggleState
  ]);

  const handleMounted = useCallback((list: ReactList) => {
    listRef.current = list;
  }, []);

  useEffect(() => {
    if (listRef.current) {
      // Animate scrolling using technique described by react-list author here:
      // https://github.com/coderiety/react-list/issues/79
      // @ts-ignore
      const scrollPos = listRef.current.getSpaceBefore(state.scrollIndex);

      // The scrolling container is not trivial to figure out, but react-list
      // has already done the work to figure it out, so use it directly.
      if (state.smoothScroll) {
        animateScroll.scrollTo(scrollPos, {
          // @ts-ignore
          container: listRef.current.scrollParent,
          isDynamic: false,
          smooth: 'linear',
          duration: 100
        });
      } else {
        // @ts-ignore
        listRef.current.scrollParent.scrollTo({
          top: scrollPos,
          behavior: 'instant'
        });
      }
    }
  }, [state.scrollIndex, state.smoothScroll, listRef]);

  if (state.articleEntries.length === 0) {
    return (
      <Container fixed className="GoliathArticleListContainer">
        <Box className="GoliathArticleListEmpty">
          <DoneAllRounded className="GoliathArticleListEmptyIcon"/>
        </Box>
      </Container>
    );
  } else {
    const articles = state.articleEntries;
    const renderIndex = Math.max(0, state.scrollIndex);
    const articleView = articles[renderIndex];

    if (state.articleViewToggleState === ArticleListView.Combined) {
      return (
        <Box className="GoliathArticleListBox">
          <ReactList
            ref={handleMounted}
            itemRenderer={renderArticle}
            length={articles.length}
            type='variable'
          />
        </Box>
      );
    }

    return (
      <Container
        maxWidth={false}
        className="GoliathSplitViewArticleListContainer"
      >
        <Grid container wrap="nowrap">
          <Grid container item xs={4}>
            <Box
              className="GoliathSplitViewArticleListBox"
              sx={{
                height: '100vh',
                overflowY: 'auto',
                width: '100%',
              }}
            >
              <ReactList
                ref={handleMounted}
                itemRenderer={renderSplitViewArticleListEntry}
                length={articles.length}
                threshold={500}
                useStaticSize={true}
                type='uniform'
              />
            </Box>
          </Grid>
          <Grid item xs='auto'>
            <Divider orientation="vertical"/>
          </Grid>
          <Grid item xs className="GoliathSplitViewArticleOuter">
            <SplitViewArticleCard
              key={articleView.id}
              article={articleView}
              title={articleView.feed_title}
              favicon={articleView.favicon}
              isSelected={true}
            />
          </Grid>
        </Grid>
      </Container>
    );
  }
};

export default ArticleList;