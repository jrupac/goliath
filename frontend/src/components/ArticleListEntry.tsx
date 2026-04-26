import React, { memo, useCallback, useMemo, useState } from 'react';
import { Chip, Paper, Typography } from '@mui/material';
import Grid from '@mui/material/Grid';
import { extractText, formatFriendly } from '../utils/helpers';
import RssFeedIcon from '@mui/icons-material/RssFeed';
import CheckCircleTwoToneIcon from '@mui/icons-material/CheckCircleTwoTone';
import CheckTwoToneIcon from '@mui/icons-material/CheckTwoTone';
import { ArticleId, ArticleView } from '../models/article';
import { FaviconCls } from '../models/feed';
import ImagePreview from './ImagePreview';

export interface ArticleListEntryProps {
  articleView: ArticleView;
  favicon: FaviconCls | undefined;
  selected: boolean;
  showPreviews: boolean;
  onSelect?: (id: ArticleId) => void;
  onToggleRead?: (id: ArticleId) => void;
}

const ArticleListEntry: React.FC<ArticleListEntryProps> = memo(
  function ArticleListEntry({
    articleView,
    favicon,
    selected,
    showPreviews,
    onSelect,
    onToggleRead,
  }: ArticleListEntryProps) {
    const [isHovered, setIsHovered] = useState(false);

    const extractedTitle: string = useMemo(() => {
      return extractText(articleView.title) || '';
    }, [articleView.title]);
    const extractedContent: string = useMemo(() => {
      return extractText(articleView.html) || '';
    }, [articleView.html]);

    const renderMeta = () => {
      const date = new Date(articleView.creationTime * 1000);
      return (
        <Chip
          size="small"
          className="GoliathArticleListMetaChip"
          label={formatFriendly(date)}
        />
      );
    };

    const faviconSrc = favicon?.GetFavicon();

    const handleToggleRead = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onToggleRead?.(articleView.id);
      },
      [onToggleRead, articleView.id]
    );

    let elevation = 3;
    const extraClasses = ['GoliathArticleListBase'];
    if (articleView.isRead) {
      extraClasses.push('GoliathArticleListRead');
      elevation = selected ? 10 : 0;
    }
    if (selected) {
      extraClasses.push('GoliathArticleListSelected');
    }

    return (
      <Paper
        elevation={elevation}
        square
        className={extraClasses.join(' ')}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
        onClick={onSelect ? () => onSelect(articleView.id) : undefined}
      >
        <Grid container direction="column" className="GoliathArticleListGrid">
          <Grid sx={{ minWidth: 0 }} className="GoliathArticleListTitleGrid">
            <Grid
              container
              wrap="nowrap"
              alignItems="center"
              spacing={1}
              className="GoliathArticleListTitleRow"
            >
              <Grid className="GoliathArticleListFaviconContainer">
                {isHovered ? (
                  articleView.isRead ? (
                    <CheckTwoToneIcon
                      fontSize="small"
                      className="GoliathArticleListToggleIcon"
                      onClick={handleToggleRead}
                    />
                  ) : (
                    <CheckCircleTwoToneIcon
                      fontSize="small"
                      className="GoliathArticleListToggleIcon"
                      onClick={handleToggleRead}
                    />
                  )
                ) : faviconSrc ? (
                  <img
                    src={faviconSrc}
                    height={16}
                    width={16}
                    alt=""
                    className="GoliathArticleListFaviconImg"
                  />
                ) : (
                  <RssFeedIcon fontSize="small" />
                )}
              </Grid>
              <Grid item xs>
                <Typography
                  noWrap
                  className="GoliathArticleListTitleType"
                >
                  <a
                    target="_blank"
                    rel="noopener noreferrer"
                    href={articleView.url}
                  >
                    {extractedTitle}
                  </a>
                </Typography>
              </Grid>
            </Grid>
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
