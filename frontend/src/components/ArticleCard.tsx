import React, {
  ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import { Box, IconButton, Skeleton, Stack, Tooltip } from '@mui/material';
import { formatFriendly, formatFull } from '../utils/helpers';
import FeedIcon from './FeedIcon';
import { Keybindings, getTinykeysSequence } from '../utils/keybindings';
import { keybindRegistry } from '../utils/keybindRegistry';
import StarIcon from '@mui/icons-material/Star';
import StarBorderIcon from '@mui/icons-material/StarBorder';
import { SelectionType } from '../utils/types';
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline';
import CheckCircleTwoToneIcon from '@mui/icons-material/CheckCircleTwoTone';
import ChromeReaderModeOutlinedIcon from '@mui/icons-material/ChromeReaderModeOutlined';
import ChromeReaderModeIcon from '@mui/icons-material/ChromeReaderMode';
import { ArticleId, ArticleView } from '../models/article';
import { FaviconCls, FeedId } from '../models/feed';
import { FolderId } from '../models/folder';
import { FetchAPI } from '../api/interface';

export interface ArticleProps {
  fetchApi: FetchAPI;
  handleUpdateArticleParsed: (
    articleId: ArticleId,
    feedId: FeedId,
    folderId: FolderId,
    parsed: string
  ) => void;
  article: ArticleView;
  title: string;
  favicon: FaviconCls | undefined;
  feedId: string;
  isSelected: boolean;
  onMarkArticleRead: () => void;
  onToggleSave: () => void;
  selectionType: SelectionType;
  showKeybindingsModal?: boolean;
}

interface ArticleState {
  showParsed: boolean;
  loading: boolean;
}

const ArticleCard: React.FC<ArticleProps> = ({
  showKeybindingsModal = false,
  ...props
}: ArticleProps) => {
  const { fetchApi, handleUpdateArticleParsed, article } = props;

  const [state, setState] = useState<ArticleState>({
    showParsed: false,
    loading: false,
  });

  const toggleParseContent = useCallback(() => {
    setState((prevState) => {
      // If already showing parsed content, disable showing it.
      if (prevState.showParsed) {
        return { ...prevState, showParsed: false };
      }

      // If already parsed in global state, just enable showing it.
      if (article.parsed !== null) {
        return { ...prevState, showParsed: true };
      }

      fetchApi
        .ParseFullArticle(article.id)
        .then((content) => {
          handleUpdateArticleParsed(
            article.id,
            article.feedId,
            article.folderId,
            content
          );
          setState((innerPrevState) => ({
            ...innerPrevState,
            showParsed: true,
            loading: false,
          }));
        })
        .catch((e) => {
          console.log('Could not parse article %s: %s', article.id, e);
          setState((innerPrevState) => ({
            ...innerPrevState,
            showParsed: false,
            loading: false,
          }));
        });
      return { ...prevState, loading: true };
    });
  }, [
    article.id,
    article.feedId,
    article.folderId,
    article.parsed,
    fetchApi,
    handleUpdateArticleParsed,
  ]);

  // Handler map for article view keybindings — held in a ref so its
  // identity stays stable across renders.
  const articleViewHandlersRef = useRef<Record<string, () => void>>({
    toggleReaderMode: toggleParseContent,
  });
  // Keep the ref up to date when toggleParseContent changes.
  articleViewHandlersRef.current.toggleReaderMode = toggleParseContent;

  useEffect(() => {
    if (showKeybindingsModal || !props.isSelected) {
      keybindRegistry.unregister('articleCard');
      return;
    }

    const keymap: Record<string, (event: KeyboardEvent) => void> = {};
    Keybindings.articleView.forEach((kb) => {
      const sequence = getTinykeysSequence(kb);
      keymap[sequence] = (event: KeyboardEvent) => {
        const handler = articleViewHandlersRef.current[kb.handlerKey];
        if (handler) {
          event.preventDefault();
          handler();
        }
      };
    });

    keybindRegistry.register('articleCard', keymap);

    return () => {
      keybindRegistry.unregister('articleCard');
    };
  }, [showKeybindingsModal, props.isSelected]);

  const date = new Date(props.article.creationTime * 1000);
  const feedTitle = props.title;
  const faviconSrc = props.favicon?.GetFavicon() || '';

  const getArticleContent = (): string => {
    if (state.showParsed) {
      return props.article.parsed || '';
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
      {/* Article scroll area */}
      <Box className="GoliathSplitViewArticleContainer">
        {/* Overlay in the gap between top and sticky header */}
        <Box className="GoliathArticleOverlay" />
        {/* Sticky header: byline + title with blur backdrop */}
        <Box className="GoliathArticleHeaderSticky">
          {/* Byline row */}
          <Box className="GoliathArticleByline">
            <Box className="GoliathArticleBylineInfo">
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
            <Box className="GoliathArticleBylineActions">
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
              <Tooltip
                title={
                  props.article.isSaved ? 'Unsave article' : 'Save article'
                }
              >
                <IconButton
                  aria-label="save article"
                  className="GoliathButton"
                  size="small"
                  onClick={props.onToggleSave}
                >
                  {props.article.isSaved ? <StarIcon /> : <StarBorderIcon />}
                </IconButton>
              </Tooltip>
              {props.selectionType !== SelectionType.Saved && (
                <Tooltip
                  title={
                    props.article.isRead ? 'Mark as unread' : 'Mark as read'
                  }
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
              )}
            </Box>
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
