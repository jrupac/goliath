import React, { memo, useCallback, useMemo, useState } from 'react';
import { Tooltip, Typography } from '@mui/material';
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord';
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked';
import { extractText, formatFriendly, formatFull } from '../utils/helpers';
import { ArticleId, ArticleView } from '../models/article';
import { FaviconCls } from '../models/feed';
import ImagePreview from './ImagePreview';
import FeedIcon from './FeedIcon';
import { SelectionType } from '../utils/types';
import StarIcon from '@mui/icons-material/Star';
import StarBorderIcon from '@mui/icons-material/StarBorder';

export interface ArticleListEntryProps {
  articleView: ArticleView;
  favicon: FaviconCls | undefined;
  feedTitle: string;
  feedId: string;
  selected: boolean;
  showPreviews: boolean;
  onSelect?: (id: ArticleId) => void;
  onToggleRead?: (id: ArticleId) => void;
  onToggleSave?: (id: ArticleId) => void;
  selectionType?: SelectionType;
}

const ArticleListEntry: React.FC<ArticleListEntryProps> = memo(
  function ArticleListEntry({
    articleView,
    favicon,
    feedTitle,
    feedId,
    selected,
    showPreviews,
    onSelect,
    onToggleRead,
    onToggleSave,
    selectionType,
  }: ArticleListEntryProps) {
    const [dotHovered, setDotHovered] = useState(false);

    const extractedTitle = useMemo(
      () => extractText(articleView.title) || '',
      [articleView.title]
    );

    const extractedContent = useMemo(
      () => extractText(articleView.html) || '',
      [articleView.html]
    );

    const date = useMemo(
      () => new Date(articleView.creationTime * 1000),
      [articleView.creationTime]
    );

    const isSavedStream = selectionType === SelectionType.Saved;
    const isDimmed = isSavedStream ? !articleView.isSaved : articleView.isRead;

    const handleToggleAction = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        if (isSavedStream) {
          onToggleSave?.(articleView.id);
        } else {
          onToggleRead?.(articleView.id);
        }
      },
      [isSavedStream, onToggleRead, onToggleSave, articleView.id]
    );

    const handleSelect = useCallback(
      () => onSelect?.(articleView.id),
      [onSelect, articleView.id]
    );

    // Swap dot style on hover as click affordance: unread shows filled, read shows hollow.
    // On hover, invert: read shows filled (affordance to mark unread), unread shows hollow.
    const showFilledDot = articleView.isRead ? dotHovered : !dotHovered;
    const showFilledStar = articleView.isSaved ? !dotHovered : dotHovered;

    const extraClasses: string[] = ['GoliathArticleCard'];
    if (isDimmed) {
      extraClasses.push('GoliathArticleCardDimmed');
    } else {
      extraClasses.push('GoliathArticleCardBright');
    }
    if (selected) {
      extraClasses.push('GoliathArticleCardSelected');
    }

    return (
      <div
        className={extraClasses.join(' ')}
        onClick={handleSelect}
      >
        {/* Row 1 — source */}
        <div className="GoliathArticleCardSource">
          <span className="GoliathArticleCardIconSlot">
            <span
              className="GoliathArticleCardDot"
              onClick={handleToggleAction}
              onMouseEnter={() => setDotHovered(true)}
              onMouseLeave={() => setDotHovered(false)}
            >
              {isSavedStream ? (
                showFilledStar ? (
                  <StarIcon fontSize="small" data-testid="StarIcon" />
                ) : (
                  <StarBorderIcon
                    fontSize="small"
                    data-testid="StarBorderIcon"
                  />
                )
              ) : showFilledDot ? (
                <FiberManualRecordIcon
                  fontSize="small"
                  data-testid="FiberManualRecordIcon"
                />
              ) : (
                <RadioButtonUncheckedIcon
                  fontSize="small"
                  data-testid="RadioButtonUncheckedIcon"
                />
              )}
            </span>
            <span
              className="GoliathArticleCardFavicon"
            >
              <FeedIcon
                favicon={favicon?.GetFavicon() || ''}
                feedTitle={feedTitle}
                feedId={feedId}
                size={16}
              />
            </span>
          </span>
          <span className="GoliathArticleCardFeedName">{feedTitle}</span>
          <Tooltip title={formatFull(date)}>
            <span className="GoliathArticleCardTime">
              {formatFriendly(date)}
            </span>
          </Tooltip>
        </div>

        {/* Body area containing text content and preview image side-by-side */}
        <div className="GoliathArticleCardBody">
          <div className="GoliathArticleCardText">
            {/* Row 2 — title */}
            <div className="GoliathArticleCardTitle">
              {extractedTitle}
            </div>

            {/* Row 3 — content */}
            <div className="GoliathArticleCardContent">
              <Typography className="GoliathArticleCardSnippet">
                {extractedContent}
              </Typography>
            </div>
          </div>
          {showPreviews && <ImagePreview article={articleView} />}
        </div>
      </div>
    );
  }
);

export default ArticleListEntry;
