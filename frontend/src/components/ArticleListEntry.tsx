import React, { memo, useMemo } from 'react';
import { Avatar, Chip, Paper, Typography } from '@mui/material';
import Grid from '@mui/material/Grid';
import { extractText, formatFriendly } from '../utils/helpers';
import RssFeedIcon from '@mui/icons-material/RssFeed';
import { ArticleView } from '../models/article';
import { FaviconCls } from '../models/feed';
import ImagePreview from './ImagePreview';

export interface ArticleListEntryProps {
  articleView: ArticleView;
  favicon: FaviconCls | undefined;
  selected: boolean;
  showPreviews: boolean;
}

const ArticleListEntry: React.FC<ArticleListEntryProps> = memo(
  function ArticleListEntry({
    articleView,
    favicon,
    selected,
    showPreviews,
  }: ArticleListEntryProps) {
    const extractedTitle: string = useMemo(() => {
      return extractText(articleView.title) || '';
    }, [articleView.title]);
    const extractedContent: string = useMemo(() => {
      return extractText(articleView.html) || '';
    }, [articleView.html]);

    const renderMeta = () => {
      const date = new Date(articleView.creationTime * 1000);
      if (favicon && favicon.GetFavicon()) {
        return (
          <Chip
            size="small"
            className="GoliathArticleListMetaChip"
            avatar={
              <Avatar
                src={`data:${favicon.GetFavicon()}`}
                alt={extractedTitle}
              />
            }
            label={formatFriendly(date)}
          />
        );
      } else {
        return (
          <Chip
            size="small"
            className="GoliathArticleListMetaChip"
            icon={<RssFeedIcon />}
            label={formatFriendly(date)}
          />
        );
      }
    };

    let elevation = 3;
    const extraClasses = ['GoliathArticleListBase'];
    if (selected) {
      elevation = 10;
      extraClasses.push('GoliathArticleListSelected');
    } else if (articleView.isRead) {
      elevation = 0;
      extraClasses.push('GoliathArticleListRead');
    }

    return (
      <Paper elevation={elevation} square className={extraClasses.join(' ')}>
        <Grid container direction="column" className="GoliathArticleListGrid">
          <Grid sx={{ minWidth: 0 }} className="GoliathArticleListTitleGrid">
            <Typography noWrap className="GoliathArticleListTitleType">
              {extractedTitle}
            </Typography>
          </Grid>
          <Grid sx={{ minWidth: 0 }} size="grow">
            <div className="GoliathArticleListMeta">{renderMeta()}</div>
          </Grid>
          <Grid container wrap="nowrap" className="GoliathArticleListContent">
            {showPreviews && (
              <Grid size="auto">
                <ImagePreview article={articleView} />
              </Grid>
            )}
            <Grid
              sx={{ minWidth: 0 }}
              size="grow"
              className="GoliathArticleContentPreviewGrid"
            >
              <Typography className="GoliathArticleContentPreview">
                {extractedContent}
              </Typography>
            </Grid>
          </Grid>
        </Grid>
      </Paper>
    );
  }
);

export default ArticleListEntry;
