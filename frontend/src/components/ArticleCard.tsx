import React, {
  ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { Box, IconButton, Skeleton, Stack, Tooltip } from '@mui/material';
import { formatFriendly, formatFull, scopeArticleHtml } from '../utils/helpers';
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
import ArrowBackTwoToneIcon from '@mui/icons-material/ArrowBackTwoTone';
import ExpandLessTwoToneIcon from '@mui/icons-material/ExpandLessTwoTone';
import ExpandMoreTwoToneIcon from '@mui/icons-material/ExpandMoreTwoTone';
import { useOverlayScrollbar } from '../utils/useOverlayScrollbar';

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
  modalOpen?: boolean;
  onBack?: () => void;
  onPrev?: () => void;
  onNext?: () => void;
  isMobile?: boolean;
  showMobileLayout?: boolean;
}

interface ArticleState {
  showParsed: boolean;
  loading: boolean;
}

const ArticleCard: React.FC<ArticleProps> = ({
  modalOpen = false,
  ...props
}: ArticleProps) => {
  const {
    fetchApi,
    handleUpdateArticleParsed,
    article,
    onBack,
    onPrev,
    onNext,
    isMobile,
    showMobileLayout,
  } = props;
  const useMobileLayout = showMobileLayout ?? isMobile;

  const [state, setState] = useState<ArticleState>({
    showParsed: false,
    loading: false,
  });

  // This component is deliberately not keyed on the article id: keying it
  // would remount the whole card — roughly forty MUI elements — on every
  // article change, where reconciling is far cheaper. The cost of reconciling
  // is that per-article state has to be reset explicitly. Adjusting state
  // during render is React's documented pattern for this; it re-runs this
  // component immediately, without committing the stale state.
  const [renderedArticleId, setRenderedArticleId] = useState(article.id);
  if (renderedArticleId !== article.id) {
    setRenderedArticleId(article.id);
    setState({ showParsed: false, loading: false });
  }

  // The scrollbar is drawn as an overlay rather than natively, so that the
  // title bar can span the full width of the card — see useOverlayScrollbar.
  const contentScrollRef = useRef<HTMLDivElement | null>(null);
  const articleContentRef = useRef<HTMLDivElement | null>(null);
  const titleBarRef = useRef<HTMLDivElement | null>(null);
  const {
    railRef,
    thumbRef,
    sync: syncScrollbar,
    onRailPointerDown,
    onThumbPointerDown,
  } = useOverlayScrollbar(contentScrollRef, articleContentRef, titleBarRef);

  // Likewise, the scroll container is reused across articles, so a new article
  // would otherwise open at the previous one's scroll offset. Reset before
  // paint so the jump is never visible.
  useLayoutEffect(() => {
    if (contentScrollRef.current) {
      contentScrollRef.current.scrollTop = 0;
    }
    // In the same layout pass as the reset, not in a scroll handler: the
    // scroll event this fires is delivered asynchronously, which would leave
    // the thumb painted at the previous article's offset for a frame.
    syncScrollbar();
  }, [article.id, syncScrollbar]);

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

  // Handler map for article view keybindings — held in a ref so the effect
  // below can register once and still dispatch to the newest closure. Updated
  // in an effect rather than during render, which would be a render side
  // effect (react(refs)); effects still run before any key event can fire.
  const articleViewHandlersRef = useRef<Record<string, () => void>>({
    toggleReaderMode: toggleParseContent,
  });
  useEffect(() => {
    articleViewHandlersRef.current.toggleReaderMode = toggleParseContent;
  });

  useEffect(() => {
    if (modalOpen || !props.isSelected) {
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
  }, [modalOpen, props.isSelected]);

  const date = new Date(props.article.creationTime * 1000);
  const feedTitle = props.title;
  const faviconSrc = props.favicon?.GetFavicon() || '';

  const getArticleContent = (): string => {
    const rawContent = state.showParsed
      ? props.article.parsed || ''
      : props.article.html;
    return scopeArticleHtml(rawContent, props.article.id);
  };

  const handleArticleContentClick = useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      const target = event.target as HTMLElement;
      const anchor = target.closest('a');
      if (!anchor) return;

      const href = anchor.getAttribute('href');
      if (href && href.startsWith('#')) {
        event.preventDefault();
        const targetId = href.substring(1);
        const container = event.currentTarget;
        const targetEl = container.querySelector(`[id="${targetId}"]`);
        if (targetEl) {
          // 'start' rather than 'nearest': a footnote below the fold should
          // come to the top of the article, not to the bottom edge, which is
          // where the minimal scroll would leave it.
          targetEl.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
      }
    },
    []
  );

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
      return (
        <div
          dangerouslySetInnerHTML={{ __html: getArticleContent() }}
          onClick={handleArticleContentClick}
        />
      );
    }
  };

  return (
    <Stack className="GoliathArticleCardColumn">
      {/* Topmost bar (feed name, date, buttons) */}
      <Box className="GoliathArticleByline">
        <Box className="GoliathArticleBylineInfo">
          {!useMobileLayout && onBack && (
            <IconButton
              aria-label="back to list"
              onClick={onBack}
              className="GoliathButton"
              size="small"
              sx={{ mr: 1 }}
            >
              <ArrowBackTwoToneIcon />
            </IconButton>
          )}
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
          {!useMobileLayout && onPrev && (
            <Tooltip title="Scroll up / Previous article">
              <IconButton
                aria-label="previous article"
                onClick={onPrev}
                className="GoliathButton"
                size="small"
              >
                <ExpandLessTwoToneIcon />
              </IconButton>
            </Tooltip>
          )}
          {!useMobileLayout && onNext && (
            <Tooltip title="Scroll down / Next article">
              <IconButton
                aria-label="next article"
                onClick={onNext}
                className="GoliathButton"
                size="small"
              >
                <ExpandMoreTwoToneIcon />
              </IconButton>
            </Tooltip>
          )}
          {!useMobileLayout && (
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
          )}
          {!useMobileLayout && (
            <Tooltip
              title={props.article.isSaved ? 'Unsave article' : 'Save article'}
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
          )}
          {!useMobileLayout && props.selectionType !== SelectionType.Saved && (
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
          )}
        </Box>
      </Box>

      {/* Article scroll area */}
      <Box className="GoliathSplitViewArticleContainer" ref={contentScrollRef}>
        {/* Frosted title bar. It lives inside the scroller and sticks to its
            top, so content scrolls under it and is blurred by it while the
            scrollbar, which is outside the content box, stays crisp. */}
        <Box className="GoliathArticleTitleBar" ref={titleBarRef}>
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
        <div
          ref={articleContentRef}
          className="GoliathSplitViewArticleContent GoliathArticleContentStyling"
        >
          {renderContent()}
        </div>
      </Box>

      {/* Scrollbar for the container above. The rail spans the whole scroller,
          but its track starts below the title bar — see useOverlayScrollbar.
          Hidden until the first sync finds something to scroll. */}
      <Box
        className="GoliathArticleScrollbar"
        ref={railRef}
        onPointerDown={onRailPointerDown}
        aria-hidden="true"
        hidden
      >
        <Box
          className="GoliathArticleScrollbarThumb"
          ref={thumbRef}
          onPointerDown={onThumbPointerDown}
        />
      </Box>

      {/* Mobile Bottom Navigation Bar */}
      {useMobileLayout && (
        <Box className="GoliathMobileBottomBar">
          <IconButton
            aria-label="back to list"
            onClick={onBack}
            className="GoliathButton"
            size="small"
          >
            <ArrowBackTwoToneIcon />
          </IconButton>
          <IconButton
            aria-label="previous article"
            onClick={onPrev}
            disabled={!onPrev}
            className="GoliathButton"
            size="small"
          >
            <ExpandLessTwoToneIcon />
          </IconButton>
          <IconButton
            aria-label="next article"
            onClick={onNext}
            disabled={!onNext}
            className="GoliathButton"
            size="small"
          >
            <ExpandMoreTwoToneIcon />
          </IconButton>
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
          <IconButton
            aria-label="save article"
            className="GoliathButton"
            size="small"
            onClick={props.onToggleSave}
          >
            {props.article.isSaved ? <StarIcon /> : <StarBorderIcon />}
          </IconButton>
          {props.selectionType !== SelectionType.Saved && (
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
          )}
        </Box>
      )}
    </Stack>
  );
};

export default React.memo(ArticleCard);
