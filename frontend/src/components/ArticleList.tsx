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
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {Box, Container, Grid, Stack} from "@mui/material";
import ArticleCard from "./ArticleCard";
import ArticleListEntry from "./ArticleListEntry";
import LRUCache from "lru-cache";
import {DoneAllRounded} from "@mui/icons-material";
import smartcrop from "smartcrop";

import {ArticleId, ArticleView} from "../models/article";
import ExpandLessTwoToneIcon from "@mui/icons-material/ExpandLessTwoTone";
import ExpandMoreTwoToneIcon from "@mui/icons-material/ExpandMoreTwoTone";
import CheckCircleOutlineTwoToneIcon
  from '@mui/icons-material/CheckCircleOutlineTwoTone';
import {FaviconCls, FeedId} from "../models/feed";

const goToAllSequence = ['g', 'a'];
const markAllReadSequence = ['Shift', 'I'];
const keyBufLength = 2;

export interface ArticleListProps {
  articleEntriesCls: ArticleView[];
  faviconMap: Map<FeedId, FaviconCls>;
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
}

const ArticleList: React.FC<ArticleListProps> = ({
  articleEntriesCls,
  faviconMap,
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
      const articleView: ArticleView = prevState.articleEntries[scrollIndex];
      let shouldMarkRead: boolean = false;

      // Add new keypress to buffer, dropping the oldest entry.
      let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

      // If this sequence is fulfilled, reset the buffer and handle it.
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        selectAllCallback();
        // Reset key buffer
        keypressBuffer = new Array(keyBufLength);
      } else if (markAllReadSequence.every((e, i) => e === keypressBuffer[i])) {
        handleMark(MarkState.Read, selectionKey, selectionType);
        articleEntries = articleEntries.map((e: ArticleView): ArticleView => {
          return {...e, isRead: true}
        });
        // Reset key buffer
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
            prevState.scrollIndex + 1, prevState.articleEntries.length - 1);
          shouldMarkRead = true;
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          scrollIndex = Math.max(prevState.scrollIndex - 1, 0);
          break;
        case 'p':
          showPreviews = !showPreviews;
          break;
        case 'v':
          // If trying to open an article before any articles are selected,
          // treat it like a scroll to the first article.
          if (
            prevState.scrollIndex === -1 &&
            prevState.articleEntries.length > 0) {
            scrollIndex = 0;
          }

          if (articleView) {
            window.open(articleView.url, '_blank');
            shouldMarkRead = true;
          }
          break;
        default:
          // No known key pressed, just ignore.
          break;
        }
      }

      if (articleView && !articleView.isRead && shouldMarkRead) {
        handleMark(
          MarkState.Read,
          [articleView.id, articleView.feedId, articleView.folderId],
          SelectionType.Article);
        // Immutably change the article read status
        if (
          prevState.scrollIndex >= 0 &&
          prevState.scrollIndex < articleEntries.length) {
          articleEntries[prevState.scrollIndex] = {...articleView, isRead: true}
        }
      }

      return {
        ...prevState,
        showPreviews: showPreviews,
        smoothScroll: smoothScroll,
        scrollIndex: scrollIndex,
        articleEntries: articleEntries,
        keypressBuffer: keypressBuffer,
      };
    });
  }, [
    selectAllCallback, handleMark, selectionKey, selectionType,
    state.articleEntries,
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
      inflightPreview.current.delete(article.id); // Ensure inflight is cleared
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
          } else {
            reject();
          }
        }).catch(() => {
          /* Ignore errors */
          reject();
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

    // Returned image was invalid, so just mark it complete.
    if (height <= 0) {
      inflightPreview.current.delete(article.id);
      return;
    }

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
        return null;
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
      const imagePreviews = prevState.articleImagePreviews;
      imagePreviews.set(article.id, imgPreview);
      inflightPreview.current.delete(article.id)

      return {
        ...prevState,
        articleImagePreviews: imagePreviews
      };
    });

  }, [state.articleImagePreviews, inflightPreview]);

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  const mergeArticleLists = useCallback(
    (scrollIndex: number, oldList: ArticleView[], newList: ArticleView[]): [number, ArticleView[]] => {
      const anchorArticleId: ArticleId = oldList[scrollIndex]?.id;
      const propArticles: ArticleView[] = newList;

      const propArticleIds = new Set(
        propArticles.map((a: ArticleView): ArticleId => a.id));
      // Keep articles that are marked read and don't appear in the new list
      const locallyReadToKeep: ArticleView[] = oldList.filter(
        (localArticle: ArticleView): boolean => localArticle.isRead &&
          !propArticleIds.has(localArticle.id)
      );

      const mergedArticles: ArticleView[] = [];
      let propPtr: number = 0;
      let localReadPtr: number = 0;

      while (propPtr < propArticles.length &&
      localReadPtr < locallyReadToKeep.length) {
        // Compare by creationTime (higher is newer, so comes first in descending sort)
        if (propArticles[propPtr].creationTime >=
          locallyReadToKeep[localReadPtr].creationTime) {
          mergedArticles.push(propArticles[propPtr++]);
        } else {
          mergedArticles.push(locallyReadToKeep[localReadPtr++]);
        }
      }
      // Append any remaining articles from either list. Only one of these lists
      // will be non-empty, so don't need to merge further.
      while (propPtr < propArticles.length) {
        mergedArticles.push(propArticles[propPtr++]);
      }
      while (localReadPtr < locallyReadToKeep.length) {
        mergedArticles.push(locallyReadToKeep[localReadPtr++]);
      }

      let newScrollIndex: number = 0;

      if (mergedArticles.length > 0) {
        if (anchorArticleId) {
          const newIndexOfAnchor: number = mergedArticles.findIndex(
            (a: ArticleView): boolean => a.id === anchorArticleId);
          if (newIndexOfAnchor !== -1) {
            newScrollIndex = newIndexOfAnchor;
          } else {
            // Anchor article is no longer in the merged list (e.g., it was
            // unread and removed by the parent). Just try to stay near the old
            // numerical position, clamped to new list bounds.
            newScrollIndex = Math.min(scrollIndex, mergedArticles.length - 1);
          }
        } else {
          // No previous anchor. Scroll to the top of the new list.
          newScrollIndex = 0;
        }
      }

      return [newScrollIndex, mergedArticles];
    }, []);


  const prevSelectionKey = useRef<SelectionKey>(selectionKey);
  const prevArticleEntriesCls = useRef<ArticleView[]>(articleEntriesCls);

  useEffect(() => {
    if (selectionKey !== prevSelectionKey.current) {
      console.log("Selection key changed: " + selectionKey)

      // The selection key has changed, so reset everything
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
    } else if (articleEntriesCls !== prevArticleEntriesCls.current) {
      console.log("Article entries changed")

      // The parent list of articles has changed despite the selection key being
      // the same. Use that list but also merge in recently read articles to
      // preserve scrollback history until the selection key changes.
      setState((prevState) => {
        const [newScrollIndex, newArticles] = mergeArticleLists(
          prevState.scrollIndex, prevState.articleEntries, articleEntriesCls);
        return {
          ...prevState,
          scrollIndex: newScrollIndex,
          articleEntries: newArticles
        }
      });
    }

    // Update refs
    prevSelectionKey.current = selectionKey;
    prevArticleEntriesCls.current = articleEntriesCls;
  }, [selectionKey, articleEntriesCls]);

  const renderArticleListEntry = useCallback((index: number): ReactElement => {
    const articleView: ArticleView = state.articleEntries[index];

    // Defensive check, though ReactList should only call with valid indices
    if (!articleView) {
      // This should ideally not happen if the length is managed correctly
      return <></>;
    }

    if (state.showPreviews) {
      generateImagePreview(articleView).then();
    }

    return <ArticleListEntry
      key={articleView.id}
      articleView={articleView}
      favicon={faviconMap.get(articleView.feedId)}
      preview={state.articleImagePreviews.get(articleView.id)}
      selected={index === state.scrollIndex}
    />;
  }, [
    state.articleEntries,
    state.scrollIndex, state.articleImagePreviews,
    state.showPreviews, generateImagePreview, faviconMap
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

      // The scrolling container is not trivial to figure out, but `react-list`
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

    return (
      <Container
        maxWidth={false}
        className="GoliathSplitViewArticleListContainer"
      >
        <Grid container wrap="nowrap">
          <Stack className="GoliathArticleListColumn">
            <Box className="GoliathSplitViewArticleListActionBar">
              <CheckCircleOutlineTwoToneIcon/>
              <div className="GoliathActionBarSpacer"></div>
              <ExpandLessTwoToneIcon/>
              <ExpandMoreTwoToneIcon/>
            </Box>
            <Box className="GoliathSplitViewArticleListBox">
              <ReactList
                ref={handleMounted}
                itemRenderer={renderArticleListEntry}
                length={articles.length}
                threshold={500}
                useStaticSize={true}
                type='uniform'
              />
            </Box>
          </Stack>
          <Grid item xs className="GoliathSplitViewArticleOuter">
            <ArticleCard
              key={articleView.id}
              article={articleView}
              title={articleView.feedTitle}
              favicon={faviconMap.get(articleView.feedId)}
              isSelected={true}
            />
          </Grid>
        </Grid>
      </Container>
    );
  }
};

export default ArticleList;