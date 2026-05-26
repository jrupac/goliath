import React, {
  ReactElement,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
// Import CJS package with a fallback for Vite 8's stricter CJS interop.
// When the `legacy.inconsistentCjsInterop` flag is eventually removed,
// this pattern continues to work without changes.
import * as ReactListModule from 'react-list';
const ReactList = ReactListModule.default || ReactListModule;
import type ReactListType from 'react-list';
import { animateScroll } from 'react-scroll';
import {
  MarkState,
  NavigationDirection,
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
import { Keybindings, getTinykeysSequence } from '../utils/keybindings';
import { keybindRegistry } from '../utils/keybindRegistry';
import { DoneAllRounded } from '@mui/icons-material';

import { ArticleId, ArticleView } from '../models/article';
import ExpandLessTwoToneIcon from '@mui/icons-material/ExpandLessTwoTone';
import ExpandMoreTwoToneIcon from '@mui/icons-material/ExpandMoreTwoTone';
import DoneAllTwoTone from '@mui/icons-material/DoneAllTwoTone';
import HotelClassTwoTone from '@mui/icons-material/HotelClassTwoTone';
import HotelClassRounded from '@mui/icons-material/HotelClassRounded';
import { FaviconCls, FeedId } from '../models/feed';

export interface ArticleListProps {
  articleEntriesCls: ArticleView[];
  faviconMap: Map<FeedId, FaviconCls>;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  selectAllCallback: () => void;
  selectUnreadCallback: () => void;
  handleMark: (
    mark: MarkState,
    entity: SelectionKey,
    type: SelectionType
  ) => void;
  buildTimestamp: string;
  buildHash: string;
  threshold?: number;
  navigateToAdjacentEntry?: (direction: NavigationDirection) => void;
  showKeybindingsModal?: boolean;
  clearReadCallback?: (selectedArticleId: ArticleId | null) => void;
  selectSavedCallback?: () => void;
}

const ArticleList: React.FC<ArticleListProps> = ({
  articleEntriesCls,
  faviconMap,
  selectionKey,
  selectionType,
  selectAllCallback,
  selectUnreadCallback,
  selectSavedCallback,
  handleMark,
  buildTimestamp,
  buildHash,
  threshold = 500,
  navigateToAdjacentEntry,
  showKeybindingsModal = false,
  clearReadCallback,
}) => {
  const listRef = useRef<ReactListType | null>(null);
  const selectionKeyRef = useRef<SelectionKey>(selectionKey);

  const [selectedArticleId, setSelectedArticleId] = useState<ArticleId | null>(
    null
  );
  const [showPreviews, setShowPreviews] = useState<boolean>(true);
  const [smoothScroll, setSmoothScroll] = useState<boolean>(true);

  // Unique key for ReactList so it remounts on selection change,
  // resetting its internal scroll position.
  const reactListKey = `${selectionType}-${
    Array.isArray(selectionKey) ? selectionKey.join('-') : selectionKey
  }`;

  const scrollIndex = useMemo(() => {
    // If no article is selected, or the list is empty, default to the top.
    if (!selectedArticleId || articleEntriesCls.length === 0) {
      return 0;
    }
    const index = articleEntriesCls.findIndex(
      (a) => a.id === selectedArticleId
    );
    // If the selected article is not in the new list, default to the top.
    return index === -1 ? 0 : index;
  }, [selectedArticleId, articleEntriesCls]);

  const handleClickArticle = useCallback((articleId: ArticleId) => {
    setSelectedArticleId(articleId);
  }, []);

  const handleMarkAllRead = useCallback(() => {
    handleMark(MarkState.Read, selectionKey, selectionType);
  }, [handleMark, selectionKey, selectionType]);

  const handleMarkArticleRead = useCallback(
    (index: number) => {
      if (selectionType === SelectionType.Saved) {
        return;
      }
      if (index >= 0 && index <= articleEntriesCls.length - 1) {
        const articleView: ArticleView = articleEntriesCls[index];
        if (articleView && !articleView.isRead) {
          handleMark(
            MarkState.Read,
            [articleView.id, articleView.feedId, articleView.folderId],
            SelectionType.Article
          );
        }
      }
    },
    [articleEntriesCls, handleMark, selectionType]
  );

  const handleToggleArticleSave = useCallback(
    (id: ArticleId) => {
      const articleView = articleEntriesCls.find((a) => a.id === id);
      if (!articleView) return;
      const newState = articleView.isSaved
        ? MarkState.Unsaved
        : MarkState.Saved;
      handleMark(
        newState,
        [articleView.id, articleView.feedId, articleView.folderId],
        SelectionType.Article
      );
    },
    [articleEntriesCls, handleMark]
  );

  const handleMarkAllSavedToggle = useCallback(() => {
    const anySaved = articleEntriesCls.some((a) => a.isSaved);
    const newState = anySaved ? MarkState.Unsaved : MarkState.Saved;
    handleMark(newState, selectionKey, selectionType);
  }, [handleMark, selectionKey, selectionType, articleEntriesCls]);

  const handleToggleArticleRead = useCallback(
    (id: ArticleId) => {
      const articleView = articleEntriesCls.find((a) => a.id === id);
      if (!articleView) return;
      const newState = articleView.isRead ? MarkState.Unread : MarkState.Read;
      handleMark(
        newState,
        [articleView.id, articleView.feedId, articleView.folderId],
        SelectionType.Article
      );
    },
    [articleEntriesCls, handleMark]
  );

  const handleSelectByIndex = useCallback(
    (index: number) => {
      const newIndex = Math.max(
        0,
        Math.min(index, articleEntriesCls.length - 1)
      );

      const newSelectedArticle = articleEntriesCls[newIndex];
      if (newSelectedArticle) {
        setSelectedArticleId(newSelectedArticle.id);
      }
    },
    [articleEntriesCls]
  );

  const handleScrollUp = useCallback(() => {
    if (scrollIndex <= 0) {
      navigateToAdjacentEntry?.(NavigationDirection.Prev);
      return;
    }
    handleSelectByIndex(scrollIndex - 1);
  }, [handleSelectByIndex, scrollIndex, navigateToAdjacentEntry]);

  const handleScrollDown = useCallback(() => {
    handleMarkArticleRead(scrollIndex);
    if (scrollIndex >= articleEntriesCls.length - 1) {
      navigateToAdjacentEntry?.(NavigationDirection.Next);
      return;
    }
    handleSelectByIndex(scrollIndex + 1);
  }, [
    handleMarkArticleRead,
    handleSelectByIndex,
    scrollIndex,
    navigateToAdjacentEntry,
    articleEntriesCls.length,
  ]);

  const handleOpenArticle = useCallback(
    (index: number) => {
      // If opening an article when no article is selected, default to opening
      // the first article.
      index = Math.max(0, index);

      // If the index is beyond the end of the list, do nothing.
      if (index > articleEntriesCls.length - 1) {
        return;
      }

      const articleView = articleEntriesCls[index];
      if (articleView && articleView.url) {
        window.open(articleView.url, '_blank');
        handleMarkArticleRead(index);
      }

      // This only has an effect if no article is selected. In which case it's
      // treated as a scroll to the first article.
      handleSelectByIndex(index);
    },
    [handleMarkArticleRead, handleSelectByIndex, articleEntriesCls]
  );

  // Handler map for article list keybindings — held in a ref so its
  // identity stays stable across renders.
  const articleListHandlersRef = useRef<Record<string, () => void>>({
    scrollDown: handleScrollDown,
    scrollUp: handleScrollUp,
    openInTab: () => handleOpenArticle(scrollIndex),
    togglePreviews: () => setShowPreviews((prev) => !prev),
    toggleSmoothScroll: () => setSmoothScroll((prev) => !prev),
    goAll: selectAllCallback,
    goUnread: selectUnreadCallback,
    markAllRead: handleMarkAllRead,
    clearRead: () => {
      if (
        selectionType === SelectionType.Unread ||
        selectionType === SelectionType.Folder ||
        selectionType === SelectionType.Feed ||
        selectionType === SelectionType.Saved
      ) {
        const activeId = articleEntriesCls[scrollIndex]?.id ?? null;
        clearReadCallback?.(activeId);
      }
    },
    toggleSave: () => {
      const selectedArticle = articleEntriesCls[scrollIndex];
      if (selectedArticle) {
        handleToggleArticleSave(selectedArticle.id);
      }
    },
    goSaved: () => {
      selectSavedCallback?.();
    },
  });
  // Keep the ref up to date when handlers change.
  articleListHandlersRef.current.scrollDown = handleScrollDown;
  articleListHandlersRef.current.scrollUp = handleScrollUp;
  articleListHandlersRef.current.goAll = selectAllCallback;
  articleListHandlersRef.current.goUnread = selectUnreadCallback;
  articleListHandlersRef.current.goSaved = () => selectSavedCallback?.();
  articleListHandlersRef.current.markAllRead = handleMarkAllRead;
  articleListHandlersRef.current.openInTab = () =>
    handleOpenArticle(scrollIndex);
  articleListHandlersRef.current.clearRead = () => {
    if (
      selectionType === SelectionType.Unread ||
      selectionType === SelectionType.Folder ||
      selectionType === SelectionType.Feed ||
      selectionType === SelectionType.Saved
    ) {
      const activeId = articleEntriesCls[scrollIndex]?.id ?? null;
      clearReadCallback?.(activeId);
    }
  };
  articleListHandlersRef.current.toggleSave = () => {
    const selectedArticle = articleEntriesCls[scrollIndex];
    if (selectedArticle) {
      handleToggleArticleSave(selectedArticle.id);
    }
  };

  useEffect(() => {
    if (showKeybindingsModal) {
      keybindRegistry.unregister('articleList');
      return;
    }

    const keymap: Record<string, (event: KeyboardEvent) => void> = {};
    Keybindings.articleList.forEach((kb) => {
      const sequence = getTinykeysSequence(kb);
      keymap[sequence] = (event: KeyboardEvent) => {
        const handler = articleListHandlersRef.current[kb.handlerKey];
        if (handler) {
          event.preventDefault();
          handler();
        }
      };
    });

    keybindRegistry.register('articleList', keymap);

    return () => {
      keybindRegistry.unregister('articleList');
    };
  }, [showKeybindingsModal]);

  useEffect(() => {
    // When the selection key changes, find the first article and select it.
    if (selectionKey !== selectionKeyRef.current) {
      if (selectionType === SelectionType.All) {
        // "All" stream: select first article in list
        if (articleEntriesCls.length > 0) {
          setSelectedArticleId(articleEntriesCls[0].id);
        } else {
          setSelectedArticleId(null);
        }
      } else {
        const firstUnreadArticle = articleEntriesCls.find(
          (article) => !article.isRead
        );

        if (firstUnreadArticle) {
          setSelectedArticleId(firstUnreadArticle.id);
        } else if (articleEntriesCls.length > 0) {
          // If all are read, default to the top.
          setSelectedArticleId(articleEntriesCls[0].id);
        } else {
          // If the list is empty, select nothing.
          setSelectedArticleId(null);
        }
      }
    }

    // Update ref
    selectionKeyRef.current = selectionKey;
  }, [selectionKey, selectionType, articleEntriesCls]); // selectionType needed for All-stream vs Unread-stream logic

  const renderArticleListEntry = useCallback(
    (index: number): ReactElement => {
      const articleView: ArticleView = articleEntriesCls[index];

      // Defensive check, though ReactList should only call with valid indices
      if (!articleView) {
        // This should ideally not happen if the length is managed correctly
        return <></>;
      }

      return (
        <ArticleListEntry
          key={articleView.id}
          articleView={articleView}
          favicon={faviconMap.get(articleView.feedId)}
          feedTitle={articleView.feedTitle}
          feedId={articleView.feedId}
          selected={index === scrollIndex}
          showPreviews={showPreviews}
          onSelect={handleClickArticle}
          onToggleRead={handleToggleArticleRead}
          selectionType={selectionType}
          onToggleSave={handleToggleArticleSave}
        />
      );
    },
    [
      articleEntriesCls,
      scrollIndex,
      showPreviews,
      faviconMap,
      handleClickArticle,
      handleToggleArticleRead,
      handleToggleArticleSave,
      selectionType,
    ]
  );

  const handleMounted = useCallback((list: ReactListType | null) => {
    listRef.current = list;
  }, []);

  useEffect(() => {
    if (listRef.current && articleEntriesCls.length > 0) {
      // Animate scrolling using technique described by react-list author here:
      // https://github.com/coderiety/react-list/issues/79
      // @ts-ignore
      const scrollPos = listRef.current.getSpaceBefore(scrollIndex);

      // The scrolling container is not trivial to figure out, but `react-list`
      // has already done the work to figure it out, so use it directly.
      const doScroll = () => {
        // @ts-ignore
        if (!listRef.current?.scrollParent) return;
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
      };

      // Defer the scroll to the next animation frame so react-list
      // recalculates positions after the articles array changes.
      requestAnimationFrame(doScroll);
    }
  }, [scrollIndex, smoothScroll, articleEntriesCls.length, selectionType]);

  const renderEmpty = () => (
    <Container fixed className="GoliathArticleListContainer">
      <Box className="GoliathArticleListEmpty">
        {selectionType === SelectionType.Saved ? (
          <HotelClassRounded className="GoliathArticleListEmptyIcon" />
        ) : (
          <DoneAllRounded className="GoliathArticleListEmptyIcon" />
        )}
        <Box className="GoliathFooter">
          Goliath RSS
          <br />
          Built at: {buildTimestamp}
          <br />
          {buildHash}
        </Box>
      </Box>
    </Container>
  );

  if (articleEntriesCls.length === 0) {
    return renderEmpty();
  } else {
    const articles = articleEntriesCls;
    // The scrollIndex is now derived and memoized, but we still need to
    // guard against it being out of bounds during a brief render cycle.
    const renderIndex = Math.min(scrollIndex, articles.length - 1);
    const articleView = articles[renderIndex];

    if (!articleView) {
      return renderEmpty();
    }

    return (
      <Container
        maxWidth={false}
        className="GoliathSplitViewArticleListContainer"
      >
        <Grid container wrap="nowrap" size="grow">
          <Stack className="GoliathArticleListColumn">
            <Box className="GoliathSplitViewArticleListActionBar">
              {selectionType === SelectionType.Saved ? (
                <Tooltip
                  title={
                    articleEntriesCls.some((a) => a.isSaved)
                      ? 'Unsave all'
                      : 'Save all'
                  }
                >
                  <IconButton
                    aria-label="mark all as unsaved"
                    onClick={() => handleMarkAllSavedToggle()}
                    className="GoliathButton"
                    size="small"
                  >
                    <HotelClassTwoTone />
                  </IconButton>
                </Tooltip>
              ) : (
                <Tooltip title="Mark all as read">
                  <IconButton
                    aria-label="mark all as read"
                    onClick={() => handleMarkAllRead()}
                    className="GoliathButton"
                    size="small"
                  >
                    <DoneAllTwoTone />
                  </IconButton>
                </Tooltip>
              )}
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
                key={reactListKey}
                ref={handleMounted}
                itemRenderer={renderArticleListEntry}
                length={articles.length}
                threshold={threshold}
                useStaticSize={true}
                type="uniform"
              />
            </Box>
          </Stack>
          <Grid className="GoliathSplitViewArticleOuter" size="grow">
            <ArticleCard
              key={articleView.id}
              article={articleView}
              title={articleView.feedTitle}
              favicon={faviconMap.get(articleView.feedId)}
              feedId={articleView.feedId}
              isSelected={true}
              onMarkArticleRead={() => handleToggleArticleRead(articleView.id)}
              onToggleSave={() => handleToggleArticleSave(articleView.id)}
              selectionType={selectionType}
              showKeybindingsModal={showKeybindingsModal}
            />
          </Grid>
        </Grid>
      </Container>
    );
  }
};

export default ArticleList;
