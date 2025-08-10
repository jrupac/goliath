import React, { ReactNode, useEffect, useState } from 'react';
import {
  Box,
  Card,
  CardHeader,
  IconButton,
  Skeleton,
  Stack,
  Tooltip,
} from '@mui/material';
import {
  fetchReadability,
  formatFriendly,
  formatFull,
  makeAbsolute,
} from '../utils/helpers';
import RssFeedOutlinedIcon from '@mui/icons-material/RssFeedOutlined';
import BookmarkTwoToneIcon from '@mui/icons-material/BookmarkTwoTone';
import { ArticleView } from '../models/article';
import CheckCircleOutlineTwoToneIcon from '@mui/icons-material/CheckCircleOutlineTwoTone';
import { FaviconCls } from '../models/feed';

export interface ArticleProps {
  article: ArticleView;
  title: string;
  favicon: FaviconCls | undefined;
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

  const toggleParseContent = () => {
    // If already showing parsed content, disable showing it.
    if (state.showParsed) {
      setState({ ...state, showParsed: false });
      return;
    }

    // If already parsed before, just enabling showing it.
    if (state.parsed !== null) {
      setState({ ...state, showParsed: true });
      return;
    }

    // It's okay if this state change doesn't happen fast enough, it'll just get
    // reset lower down anyway.
    setState({ ...state, loading: true });

    const url = makeAbsolute('/cache?url=' + encodeURI(props.article.url));
    fetchReadability(url)
      .then((content) => {
        setState({
          parsed: content,
          showParsed: true,
          loading: false,
        });
      })
      .catch((e) => {
        console.log('Could not parse URL %s: %s', url, e);
        setState({
          ...state,
          showParsed: false,
          loading: false,
        });
      });
  };

  const handleKeyDown = (event: KeyboardEvent) => {
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
  };

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const date = new Date(props.article.creationTime * 1000);
  const feedTitle = props.title;

  let headerClass = '';

  if (props.article.isRead) {
    headerClass = 'GoliathArticleHeaderRead';
  }

  const renderFavicon = (): ReactNode => {
    const favicon: string | undefined = props.favicon?.GetFavicon();
    if (!favicon || favicon === '') {
      return <RssFeedOutlinedIcon fontSize="small" />;
    } else {
      return <img src={`data:${favicon}`} height={16} width={16} alt="" />;
    }
  };

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
      <Box className="GoliathSplitViewArticleCardActionBar">
        <Box className="GoliathArticleFeed">
          {renderFavicon()}
          <p className="GoliathArticleFeedTitle">{feedTitle}</p>
        </Box>
        <Box className="GoliathHeaderActionButtons">
          <Tooltip title="Save article">
            <IconButton
              aria-label="save article"
              className="GoliathButton"
              size="small"
            >
              <BookmarkTwoToneIcon />
            </IconButton>
          </Tooltip>
          <Tooltip title="Mark article read">
            <IconButton
              aria-label="mark as read"
              onClick={() => props.onMarkArticleRead()}
              className="GoliathButton"
              size="small"
            >
              <CheckCircleOutlineTwoToneIcon />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>
      <Box className="GoliathSplitViewArticleContainer">
        <Card elevation={0} className="GoliathHeaderContainer">
          <CardHeader
            disableTypography={true}
            className={`GoliathArticleHeader ${headerClass}`}
            title={
              <div>
                <Box className="GoliathArticleTitle">
                  <a
                    target="_blank"
                    rel="noopener noreferrer"
                    href={props.article.url}
                  >
                    <div>
                      <div
                        dangerouslySetInnerHTML={{
                          __html: props.article.title,
                        }}
                      />
                    </div>
                  </a>
                </Box>
              </div>
            }
            subheader={
              <Box className="GoliathArticleSubheader">
                <Box className="GoliathArticleMeta">
                  <Tooltip title={formatFull(date)}>
                    <Box className="GoliathArticleDate">
                      {formatFriendly(date)}
                    </Box>
                  </Tooltip>
                </Box>
              </Box>
            }
          />
        </Card>
        <div className="GoliathSplitViewArticleContent GoliathArticleContentStyling">
          {renderContent()}
        </div>
      </Box>
    </Stack>
  );
};

export default ArticleCard;
