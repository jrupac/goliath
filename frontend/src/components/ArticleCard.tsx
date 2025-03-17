import React, {ReactNode, useEffect, useState} from "react";
import {
  Box,
  Card,
  CardContent,
  CardHeader,
  Skeleton,
  Tooltip
} from "@mui/material";
import {
  fetchReadability,
  formatFriendly,
  formatFull,
  makeAbsolute
} from "../utils/helpers";
import RssFeedOutlinedIcon from "@mui/icons-material/RssFeedOutlined";
import {ArticleView} from "../models/article";


export interface ArticleProps {
  article: ArticleView;
  title: string;
  favicon: string;
  isSelected: boolean;
  shouldRerender: () => void;
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
      setState({
        ...state,
        showParsed: false
      });
      props.shouldRerender()
      return;
    }

    // If already parsed before, just enabling showing it.
    if (state.parsed !== null) {
      setState({
        ...state,
        showParsed: true
      });
      props.shouldRerender()
      return;
    }

    // It's okay if this state change or re-render doesn't happen fast enough,
    // it'll just get reset lower down anyway.
    setState({
      ...state,
      loading: true
    });
    props.shouldRerender()

    const url = makeAbsolute("/cache?url=" + encodeURI(props.article.url));
    fetchReadability(url).then((content) => {
      setState({
        parsed: content,
        showParsed: true,
        loading: false
      });
      props.shouldRerender()
    }).catch((e) => {
      console.log("Could not parse URL %s: %s", url, e);
      setState({
        ...state,
        showParsed: false,
        loading: false
      });
      props.shouldRerender()
    })
  }

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
  }

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);


  const date = new Date(props.article.created_on_time * 1000);
  const feedTitle = props.title;

  let headerClass = '';
  let elevation;

  if (props.isSelected) {
    elevation = 8;
  } else if (props.article.is_read === 1) {
    headerClass = 'GoliathArticleHeaderRead';
    elevation = 0;
  } else {
    elevation = 2;
  }

  const renderFavicon = (): ReactNode => {
    const favicon = props.favicon;
    if (!favicon) {
      return <RssFeedOutlinedIcon fontSize="small"/>
    } else {
      return <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }
  }

  const getArticleContent = (): string => {
    if (state.showParsed) {
      // This field is checked for non-nullity before being set.
      return state.parsed!;
    }
    return props.article.html;
  }

  const renderContent = (): ReactNode => {
    if (state.loading) {
      return <Box>
        <Skeleton variant="text" animation="wave"/>
        <Skeleton variant="text" animation="wave"/>
        <Skeleton variant="text" animation="wave"/>
      </Box>;
    } else {
      return <div
        dangerouslySetInnerHTML={{__html: getArticleContent()}}/>;
    }
  }

  return (
    <Box className="GoliathArticleOuter">
      <Card elevation={elevation}>
        <CardHeader
          className={`GoliathArticleHeader ${headerClass}`}
          title={
            <Box className="GoliathArticleTitle">
              <a
                target="_blank"
                rel="noopener noreferrer"
                href={props.article.url}>
                <div>
                  <div
                    dangerouslySetInnerHTML={{__html: props.article.title}}/>
                </div>
              </a>
            </Box>
          }
          subheader={
            <Box className="GoliathArticleMeta">
              <Box className="GoliathArticleFeed">
                {renderFavicon()}
                <p className="GoliathArticleFeedTitle">{feedTitle}</p>
              </Box>
              <Tooltip
                title={formatFull(date)}>
                <Box className="GoliathArticleDate">
                  {formatFriendly(date)}
                </Box>
              </Tooltip>
            </Box>
          }/>
        <CardContent
          className="GoliathArticleContent GoliathArticleContentStyling">
          {renderContent()}
        </CardContent>
      </Card>
    </Box>
  )
};

export default ArticleCard;