import React, {
  ReactElement,
  useCallback,
  useEffect,
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

import { ArticleView } from '../models/article';
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

interface ArticleListState {
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
  buildTimestamp,
  buildHash,
  threshold = 500,
}) => {
  const listRef = useRef<ReactList | null>(null);
  const selectionKeyRef = useRef<SelectionKey>(selectionKey);

  const [state, setState] = useState<ArticleListState>({
    scrollIndex: 0,
    keypressBuffer: new Array(keyBufLength),
  });
  const [showPreviews, setShowPreviews] = useState<boolean>(true);
  const [smoothScroll, setSmoothScroll] = useState<boolean>(true);

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

  const handleScrollTo = useCallback(
    (index: number) => {
      const newIndex = Math.max(
        0,
        Math.min(index, articleEntriesCls.length - 1)
      );

      // If the index is unchanged, do nothing.
      if (newIndex === state.scrollIndex) {
        return;
      }

      setState((prevState) => {
        return {
          ...prevState,
          scrollIndex: newIndex,
        };
      });
    },
    [articleEntriesCls.length, state.scrollIndex]
  );

  const handleScrollUp = useCallback(() => {
    handleScrollTo(state.scrollIndex - 1);
  }, [handleScrollTo, state.scrollIndex]);

  const handleScrollDown = useCallback(() => {
    // If the previous scroll index pointed at a valid article, mark it read
    handleMarkArticleRead(state.scrollIndex);
    handleScrollTo(state.scrollIndex + 1);
  }, [handleMarkArticleRead, handleScrollTo, state.scrollIndex]);

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
      handleScrollTo(index);
    },
    [handleMarkArticleRead, handleScrollTo, articleEntriesCls]
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

  useEffect(() => {
    // When the selection key changes, reset the scroll index to the top.
    if (selectionKey !== selectionKeyRef.current) {
      console.log('Selection key changed: ' + selectionKey);
      handleScrollTo(0);
      if (listRef.current) {
        listRef.current.scrollTo(0);
        // @ts-ignore
        listRef.current.forceUpdate();
      }
    }

    // Update ref
    selectionKeyRef.current = selectionKey;
  }, [selectionKey, handleScrollTo]);

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
          selected={index === state.scrollIndex}
          showPreviews={showPreviews}
        />
      );
    },
    [articleEntriesCls, state.scrollIndex, showPreviews, faviconMap]
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
    let renderIndex = Math.max(0, state.scrollIndex);

    // If the scrollIndex is out of bounds for the new list, clamp it to the
    // last valid index. This can happen for one render cycle when switching
    // from a long list to a short one.
    if (renderIndex >= articles.length) {
      renderIndex = Math.max(0, articles.length - 1);
    }

    const articleView = articles[renderIndex];

    // As a final guard, if there are no articles for some reason after all
    // this, render the empty state.
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
