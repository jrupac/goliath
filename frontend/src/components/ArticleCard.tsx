import React, { ReactNode, useCallback, useEffect, useState } from 'react';
import { Box, IconButton, Skeleton, Stack, Tooltip } from '@mui/material';
import {
  fetchReadability,
  formatFriendly,
  formatFull,
  makeAbsolute,
} from '../utils/helpers';
import FeedIcon from './FeedIcon';
import BookmarkBorderIcon from '@mui/icons-material/BookmarkBorder';
import BookmarkIcon from '@mui/icons-material/Bookmark';
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline';
import CheckCircleTwoToneIcon from '@mui/icons-material/CheckCircleTwoTone';
import ChromeReaderModeOutlinedIcon from '@mui/icons-material/ChromeReaderModeOutlined';
import ChromeReaderModeIcon from '@mui/icons-material/ChromeReaderMode';
import { ArticleView } from '../models/article';
import { FaviconCls } from '../models/feed';

export interface ArticleProps {
  article: ArticleView;
  title: string;
  favicon: FaviconCls | undefined;
  feedId: string;
  isSelected: boolean;
  onMarkArticleRead: () => void;
}

interface ArticleState {
  parsed: string | null;
  showParsed: boolean;
  loading: boolean;
}

const ArticleCard: React.FC<ArticleProps> = (props: ArticleProps) => {
  const [state, setState] = useState<ArticleState>({
    parsed: null,
    showParsed: false,
    loading: false,
  });

  const toggleParseContent = useCallback(() => {
    setState((prevState) => {
      // If already showing parsed content, disable showing it.
      if (prevState.showParsed) {
        return { ...prevState, showParsed: false };
      }

      // If already parsed before, just enabling showing it.
      if (prevState.parsed !== null) {
        return { ...prevState, showParsed: true };
      }

      // It's okay if this state change doesn't happen fast enough, it'll just
      // get reset lower down anyway.
      const url = makeAbsolute('/cache?url=' + encodeURI(props.article.url));
      fetchReadability(url)
        .then((content) => {
          setState((innerPrevState) => ({
            ...innerPrevState,
            parsed: content,
            showParsed: true,
            loading: false,
          }));
        })
        .catch((e) => {
          console.log('Could not parse URL %s: %s', url, e);
          setState((innerPrevState) => ({
            ...innerPrevState,
            showParsed: false,
            loading: false,
          }));
        });
      return { ...prevState, loading: true };
    });
  }, [props.article.url]);

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      // Ignore all key events unless this is the selected article.
      if (!props.isSelected) {
        return;
      }

      // Ignore keypress events when some modifiers are also enabled to avoid
      // triggering on (e.g.) browser shortcuts.
      if (event.altKey || event.metaKey || event.ctrlKey || event.shiftKey) {
        return;
      }

      if (event.key === 'm') {
        toggleParseContent();
      }
    },
    [props.isSelected, toggleParseContent]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  const date = new Date(props.article.creationTime * 1000);
  const feedTitle = props.title;
  const faviconSrc = props.favicon?.GetFavicon() || '';

  const getArticleContent = (): string => {
    if (state.showParsed) {
      // This field is checked for non-nullity before being set.
      return state.parsed!;
    }
    return props.article.html;
  };

  const renderContent = (): ReactNode => {
    if (state.loading) {
      return (
        <Box>
          <Skeleton variant="text" animation="wave" />
          <Skeleton variant="text" animation="wave" />
          <Skeleton variant="text" animation="wave" />
        </Box>
      );
    } else {
      return <div dangerouslySetInnerHTML={{ __html: getArticleContent() }} />;
    }
  };

  return (
    <Stack className="GoliathArticleCardColumn">
      {/* Toolbar — fixed, no feed identity */}
      <Box className="GoliathArticleToolbar">
        <Tooltip title="Save article">
          <IconButton
            aria-label="save article"
            className="GoliathButton"
            size="small"
            onClick={() => {}}
          >
            {props.article.isSaved ? <BookmarkIcon /> : <BookmarkBorderIcon />}
          </IconButton>
        </Tooltip>
        <Tooltip
          title={props.article.isRead ? 'Mark as unread' : 'Mark as read'}
        >
          <IconButton
            aria-label="mark as read"
            onClick={() => props.onMarkArticleRead()}
            className="GoliathButton"
            size="small"
          >
            {props.article.isRead ? (
              <CheckCircleOutlineIcon />
            ) : (
              <CheckCircleTwoToneIcon />
            )}
          </IconButton>
        </Tooltip>
        <Tooltip title="Reader mode (m)">
          <IconButton
            aria-label="reader mode"
            className={`GoliathButton${state.showParsed ? ' GoliathReaderModeActive' : ''}`}
            size="small"
            onClick={toggleParseContent}
          >
            {state.showParsed ? (
              <ChromeReaderModeIcon />
            ) : (
              <ChromeReaderModeOutlinedIcon />
            )}
          </IconButton>
        </Tooltip>
      </Box>

      {/* Article scroll area */}
      <Box className="GoliathSplitViewArticleContainer">
        {/* Sticky header: byline + title with blur backdrop */}
        <Box className="GoliathArticleHeaderSticky">
          {/* Byline row */}
          <Box className="GoliathArticleByline">
            <FeedIcon
              favicon={faviconSrc}
              feedTitle={feedTitle}
              feedId={props.feedId}
              size={16}
            />
            <span className="GoliathArticleBylineText">{feedTitle}</span>
            <span className="GoliathArticleBylineSep">·</span>
            <Tooltip title={formatFull(date)}>
              <span className="GoliathArticleBylineDate">
                {formatFriendly(date)}
              </span>
            </Tooltip>
          </Box>

          {/* Article title */}
          <h1 className="GoliathArticleTitle">
            <a
              target="_blank"
              rel="noopener noreferrer"
              href={props.article.url}
            >
              <span dangerouslySetInnerHTML={{ __html: props.article.title }} />
            </a>
          </h1>
        </Box>

        {/* Article content */}
        <div className="GoliathSplitViewArticleContent GoliathArticleContentStyling">
          {renderContent()}
        </div>
      </Box>
    </Stack>
  );
};

export default React.memo(ArticleCard);
