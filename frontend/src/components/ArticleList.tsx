import React, {
  ReactElement,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import ReactList from 'react-list';
import { animateScroll } from 'react-scroll';
import { MarkState, SelectionKey, SelectionType } from '../utils/types';
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
import { DoneAllRounded } from '@mui/icons-material';

import { ArticleId, ArticleView } from '../models/article';
import ExpandLessTwoToneIcon from '@mui/icons-material/ExpandLessTwoTone';
import ExpandMoreTwoToneIcon from '@mui/icons-material/ExpandMoreTwoTone';
import CheckCircleOutlineTwoToneIcon from '@mui/icons-material/CheckCircleOutlineTwoTone';
import { FaviconCls, FeedId } from '../models/feed';

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
  buildTimestamp: string;
  buildHash: string;
  threshold?: number;
}

const ArticleList: React.FC<ArticleListProps> = ({
  articleEntriesCls,
  faviconMap,
  selectionKey,
  selectionType,
  selectAllCallback,
  handleMark,
  buildTimestamp,
  buildHash,
  threshold = 500,
}) => {
  const listRef = useRef<ReactList | null>(null);
  const selectionKeyRef = useRef<SelectionKey>(selectionKey);

  const [selectedArticleId, setSelectedArticleId] = useState<ArticleId | null>(
    null
  );
  const [keypressBuffer, setKeypressBuffer] = useState<string[]>(
    new Array(keyBufLength)
  );
  const [showPreviews, setShowPreviews] = useState<boolean>(true);
  const [smoothScroll, setSmoothScroll] = useState<boolean>(true);

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

  const handleMarkAllRead = useCallback(() => {
    handleMark(MarkState.Read, selectionKey, selectionType);
  }, [handleMark, selectionKey, selectionType]);

  const handleMarkArticleRead = useCallback(
    (index: number) => {
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
    handleSelectByIndex(scrollIndex - 1);
  }, [handleSelectByIndex, scrollIndex]);

  const handleScrollDown = useCallback(() => {
    // If the previous scroll index pointed at a valid article, mark it read
    handleMarkArticleRead(scrollIndex);
    handleSelectByIndex(scrollIndex + 1);
  }, [handleMarkArticleRead, handleSelectByIndex, scrollIndex]);

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

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      // Ignore keypress events when some modifiers are also enabled to avoid
      // triggering on (e.g.) browser shortcuts. Shift is the exception here
      // since we do care about Shift+I.
      if (event.altKey || event.metaKey || event.ctrlKey) {
        return;
      }

      setKeypressBuffer((prevBuffer) => {
        // Add new keypress to buffer, dropping the oldest entry.
        const newBuffer = [...prevBuffer.slice(1), event.key];
        if (goToAllSequence.every((e, i) => e === newBuffer[i])) {
          selectAllCallback();
          return new Array(keyBufLength);
        } else if (markAllReadSequence.every((e, i) => e === newBuffer[i])) {
          handleMarkAllRead();
          return new Array(keyBufLength);
        }
        return newBuffer;
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
          handleOpenArticle(scrollIndex);
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
      scrollIndex,
    ]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  useEffect(() => {
    // When the selection key changes, find the first unread article and select it.
    if (selectionKey !== selectionKeyRef.current) {
      // TODO: This logic will need to change when we support saved items.
      // If the stream filer changes, we should always go to the top (index 0).
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

    // Update ref
    selectionKeyRef.current = selectionKey;
  }, [selectionKey, articleEntriesCls]);

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
          selected={index === scrollIndex}
          showPreviews={showPreviews}
        />
      );
    },
    [articleEntriesCls, scrollIndex, showPreviews, faviconMap]
  );

  const handleMounted = useCallback((list: ReactList) => {
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
  }, [scrollIndex, smoothScroll, articleEntriesCls.length]);

  const renderEmpty = () => (
    <Container fixed className="GoliathArticleListContainer">
      <Box className="GoliathArticleListEmpty">
        <DoneAllRounded className="GoliathArticleListEmptyIcon" />
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
                threshold={threshold}
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
