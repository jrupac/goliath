import React, {
  ReactElement,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import ReactList from 'react-list';
import { animateScroll } from 'react-scroll';
import {
  ArticleImagePreview,
  MarkState,
  SelectionKey,
  SelectionType,
} from '../utils/types';
import {
  Box,
  Container,
  Grid,
  IconButton,
  Stack,
  Tooltip,
} from '@mui/material';
import ArticleCard from './ArticleCard';
import ArticleListEntry from './ArticleListEntry';
import LRUCache from 'lru-cache';
import { DoneAllRounded } from '@mui/icons-material';

import { ArticleCls, ArticleId, ArticleView } from '../models/article';
import ExpandLessTwoToneIcon from '@mui/icons-material/ExpandLessTwoTone';
import ExpandMoreTwoToneIcon from '@mui/icons-material/ExpandMoreTwoTone';
import CheckCircleOutlineTwoToneIcon from '@mui/icons-material/CheckCircleOutlineTwoTone';
import { FaviconCls, FeedId } from '../models/feed';
import { getPreviewImage } from '../utils/helpers';

const goToAllSequence = ['g', 'a'];
const markAllReadSequence = ['Shift', 'I'];
const keyBufLength = 2;

export interface ArticleListProps {
  articleEntriesCls: ArticleView[];
  faviconMap: Map<FeedId, FaviconCls>;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  selectAllCallback: () => void;
  handleMark: (
    mark: MarkState,
    entity: SelectionKey,
    type: SelectionType
  ) => void;
}

interface ArticleListState {
  articleEntries: ArticleView[];
  scrollIndex: number;
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
  const inflightPreviewRef = useRef<Set<ArticleId>>(new Set<ArticleId>());
  const selectionKeyRef = useRef<SelectionKey>(selectionKey);
  const articleEntriesClsRef = useRef<ArticleView[]>(articleEntriesCls);

  const [state, setState] = useState<ArticleListState>({
    articleEntries: articleEntriesCls,
    scrollIndex: 0,
    keypressBuffer: new Array(keyBufLength),
  });
  const [showPreviews, setShowPreviews] = useState<boolean>(false);
  const [smoothScroll, setSmoothScroll] = useState<boolean>(true);
  const [articleImagePreviews, setArticleImagePreviews] = useState<
    LRUCache<ArticleId, ArticleImagePreview>
  >(new LRUCache<ArticleId, ArticleImagePreview>({ max: 100 }));

  const handleAddImagePreview = useCallback(
    (k: ArticleId, v: ArticleImagePreview) => {
      setArticleImagePreviews((prevCache) => {
        const newCache = new LRUCache<ArticleId, ArticleImagePreview>({
          max: prevCache.max,
        });
        for (const [ok, ov] of prevCache.entries()) {
          newCache.set(ok, ov);
        }
        newCache.set(k, v);
        return newCache;
      });
    },
    []
  );

  const handleMarkAllRead = useCallback(() => {
    setState((prevState) => {
      handleMark(MarkState.Read, selectionKey, selectionType);

      // Immutably mark all articles read
      const articleEntries = prevState.articleEntries.map(
        (e: ArticleView): ArticleView => {
          return { ...e, isRead: true };
        }
      );
      return {
        ...prevState,
        articleEntries: articleEntries,
      };
    });
  }, [handleMark, selectionKey, selectionType]);

  const handleMarkArticleRead = useCallback(
    (index: number) => {
      setState((prevState) => {
        let articleEntries = prevState.articleEntries;

        if (index >= 0 && index <= prevState.articleEntries.length - 1) {
          const articleView: ArticleView = prevState.articleEntries[index];
          if (articleView && !articleView.isRead) {
            handleMark(
              MarkState.Read,
              [articleView.id, articleView.feedId, articleView.folderId],
              SelectionType.Article
            );

            // Immutably change the article read status
            articleEntries = Array.from(prevState.articleEntries);
            articleEntries[index] = { ...articleView, isRead: true };
          }
        }

        return {
          ...prevState,
          articleEntries: articleEntries,
        };
      });
    },
    [handleMark]
  );

  const handleScrollTo = useCallback((index: number) => {
    setState((prevState) => {
      const newIndex = Math.max(
        0,
        Math.min(index, prevState.articleEntries.length - 1)
      );
      return {
        ...prevState,
        scrollIndex: newIndex,
      };
    });
  }, []);

  const handleScrollUp = useCallback(() => {
    handleScrollTo(state.scrollIndex - 1);
  }, [state.scrollIndex]);

  const handleScrollDown = useCallback(() => {
    // If the previous scroll index pointed at a valid article, mark it read
    handleMarkArticleRead(state.scrollIndex);
    handleScrollTo(state.scrollIndex + 1);
  }, [state.scrollIndex]);

  const handleOpenArticle = useCallback(
    (index: number) => {
      // If opening an article when no article is selected, default to opening
      // the first article.
      index = Math.max(0, index);

      // If the index is beyond the end of the list, do nothing.
      if (index > state.articleEntries.length - 1) {
        return;
      }

      const articleView = state.articleEntries[index];
      if (articleView && articleView.url) {
        window.open(articleView.url, '_blank');
        handleMarkArticleRead(index);
      }

      // This only has an effect if no article is selected. In which case it's
      // treated as a scroll to the first article.
      handleScrollTo(index);
    },
    [state.articleEntries]
  );

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      // Ignore keypress events when some modifiers are also enabled to avoid
      // triggering on (e.g.) browser shortcuts. Shift is the exception here
      // since we do care about Shift+I.
      if (event.altKey || event.metaKey || event.ctrlKey) {
        return;
      }

      setState((prevState) => {
        // Add new keypress to buffer, dropping the oldest entry.
        let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

        // If this sequence is fulfilled, reset the buffer and handle it.
        if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
          selectAllCallback();
          keypressBuffer = new Array(keyBufLength);
        } else if (
          markAllReadSequence.every((e, i) => e === keypressBuffer[i])
        ) {
          handleMarkAllRead();
          keypressBuffer = new Array(keyBufLength);
        }

        return {
          ...prevState,
          keypressBuffer: keypressBuffer,
        };
      });

      switch (event.key) {
        case 'f':
          setSmoothScroll((smoothScroll) => !smoothScroll);
          break;
        case 'ArrowDown':
          event.preventDefault(); // fallthrough
        case 'j':
          handleScrollDown();
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          handleScrollUp();
          break;
        case 'p':
          setShowPreviews((showPreviews) => !showPreviews);
          break;
        case 'v':
          handleOpenArticle(state.scrollIndex);
          break;
        default:
          // No known key pressed, just ignore.
          break;
      }
    },
    [
      selectAllCallback,
      handleMarkAllRead,
      handleScrollDown,
      handleScrollUp,
      handleOpenArticle,
      state.scrollIndex,
    ]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  const generateImagePreview = useCallback(
    async (article: ArticleView) => {
      // Already generated preview for this article, so nothing to do.
      if (articleImagePreviews.has(article.id)) {
        return;
      }

      // There's already an inflight request for this article, so nothing to do.
      if (inflightPreviewRef.current.has(article.id)) {
        return;
      }

      inflightPreviewRef.current.add(article.id);
      const imgPreview = await getPreviewImage(article);
      inflightPreviewRef.current.delete(article.id);

      if (imgPreview) {
        handleAddImagePreview(article.id, imgPreview);
      }
    },
    [articleImagePreviews, handleAddImagePreview]
  );

  const mergeArticleLists = useCallback(
    (
      scrollIndex: number,
      oldList: ArticleView[],
      newList: ArticleView[]
    ): [number, ArticleView[]] => {
      const anchorArticleId: ArticleId = oldList[scrollIndex]?.id;
      const finalArticlesMap = new Map<ArticleId, ArticleView>();
      const oldIdSet = new Set<ArticleId>();
      const newIdSet = new Set<ArticleId>();

      // First add in all the new articles
      for (const article of newList) {
        finalArticlesMap.set(article.id, article);
        newIdSet.add(article.id);
      }

      // Add in previously read articles from the current (old) list
      for (const article of oldList) {
        if (article.isRead) {
          if (!finalArticlesMap.has(article.id)) {
            finalArticlesMap.set(article.id, article);
          }
        }
        oldIdSet.add(article.id);
      }

      // It's only considered a change when there's a new element that was not
      // part of our previous set. Removal of previously read items does not
      // constitute a meaningful change in props.
      const changed = newIdSet.difference(oldIdSet).size > 0;

      const mergedArticles = Array.from(finalArticlesMap.values()).sort(
        ArticleCls.ArticleViewComparator
      );

      let newScrollIndex: number = 0;

      if (mergedArticles.length > 0) {
        // We might need to re-anchor the currently selected article in case
        // we didn't meaningful change props but the position changed anyway.
        if (anchorArticleId && !changed) {
          const newIndexOfAnchor: number = mergedArticles.findIndex(
            (a: ArticleView): boolean => a.id === anchorArticleId
          );
          if (newIndexOfAnchor !== -1) {
            // Anchor article is still in the list. Scroll to it.
            newScrollIndex = newIndexOfAnchor;
          } else {
            // Anchor article is no longer in the merged list. Scroll to top.
            newScrollIndex = 0;
          }
        } else {
          // No previous anchor or list changed. Scroll to the top of the new
          // list.
          newScrollIndex = 0;
        }
      }

      return [newScrollIndex, mergedArticles];
    },
    []
  );

  useEffect(() => {
    if (selectionKey !== selectionKeyRef.current) {
      console.log('Selection key changed: ' + selectionKey);

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
        keypressBuffer: new Array(keyBufLength),
      }));
    } else if (articleEntriesCls !== articleEntriesClsRef.current) {
      console.log('Article entries changed');

      // The parent list of articles has changed despite the selection key being
      // the same. Use that list but also merge in recently read articles to
      // preserve scrollback history until the selection key changes.
      setState((prevState) => {
        const [newScrollIndex, newArticles] = mergeArticleLists(
          prevState.scrollIndex,
          prevState.articleEntries,
          articleEntriesCls
        );
        return {
          ...prevState,
          scrollIndex: newScrollIndex,
          articleEntries: newArticles,
        };
      });
    }

    // Update refs
    selectionKeyRef.current = selectionKey;
    articleEntriesClsRef.current = articleEntriesCls;
  }, [selectionKey, articleEntriesCls, mergeArticleLists]);

  const renderArticleListEntry = useCallback(
    (index: number): ReactElement => {
      const articleView: ArticleView = state.articleEntries[index];

      // Defensive check, though ReactList should only call with valid indices
      if (!articleView) {
        // This should ideally not happen if the length is managed correctly
        return <></>;
      }

      if (showPreviews) {
        generateImagePreview(articleView).then();
      }

      const preview = showPreviews
        ? articleImagePreviews.get(articleView.id)
        : undefined;

      return (
        <ArticleListEntry
          key={articleView.id}
          articleView={articleView}
          favicon={faviconMap.get(articleView.feedId)}
          preview={preview}
          selected={index === state.scrollIndex}
        />
      );
    },
    [
      state.articleEntries,
      state.scrollIndex,
      articleImagePreviews,
      showPreviews,
      generateImagePreview,
      faviconMap,
    ]
  );

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
      if (smoothScroll) {
        animateScroll.scrollTo(scrollPos, {
          // @ts-ignore
          container: listRef.current.scrollParent,
          isDynamic: false,
          smooth: 'linear',
          duration: 100,
        });
      } else {
        // @ts-ignore
        listRef.current.scrollParent.scrollTo({
          top: scrollPos,
          behavior: 'instant',
        });
      }
    }
  }, [state.scrollIndex, smoothScroll]);

  if (state.articleEntries.length === 0) {
    return (
      <Container fixed className="GoliathArticleListContainer">
        <Box className="GoliathArticleListEmpty">
          <DoneAllRounded className="GoliathArticleListEmptyIcon" />
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
              <Tooltip title="Mark all as read">
                <IconButton
                  aria-label="mark all as read"
                  onClick={() => handleMarkAllRead()}
                  className="GoliathButton"
                  size="small"
                >
                  <CheckCircleOutlineTwoToneIcon />
                </IconButton>
              </Tooltip>
              <div className="GoliathActionBarSpacer"></div>
              <Tooltip title="Scroll up">
                <IconButton
                  aria-label="scroll up"
                  onClick={() => handleScrollUp()}
                  className="GoliathButton"
                  size="small"
                >
                  <ExpandLessTwoToneIcon />
                </IconButton>
              </Tooltip>
              <Tooltip title="Scroll down">
                <IconButton
                  aria-label="scroll down"
                  onClick={() => handleScrollDown()}
                  className="GoliathButton"
                  size="small"
                >
                  <ExpandMoreTwoToneIcon />
                </IconButton>
              </Tooltip>
            </Box>
            <Box className="GoliathSplitViewArticleListBox">
              <ReactList
                ref={handleMounted}
                itemRenderer={renderArticleListEntry}
                length={articles.length}
                threshold={500}
                useStaticSize={true}
                type="uniform"
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
              onMarkArticleRead={() => handleMarkArticleRead(renderIndex)}
            />
          </Grid>
        </Grid>
      </Container>
    );
  }
};

export default ArticleList;
