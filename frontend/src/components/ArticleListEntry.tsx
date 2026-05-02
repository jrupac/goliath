import React, { memo, useCallback, useMemo, useState } from 'react';
import { Typography } from '@mui/material';
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord';
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked';
import { extractText, formatFriendly } from '../utils/helpers';
import { ArticleId, ArticleView } from '../models/article';
import { FaviconCls } from '../models/feed';
import ImagePreview from './ImagePreview';

export interface ArticleListEntryProps {
  articleView: ArticleView;
  favicon: FaviconCls | undefined;
  feedTitle: string;
  feedId: string;
  selected: boolean;
  showPreviews: boolean;
  onSelect?: (id: ArticleId) => void;
  onToggleRead?: (id: ArticleId) => void;
}

const ArticleListEntry: React.FC<ArticleListEntryProps> = memo(
  function ArticleListEntry({
    articleView,
    favicon,
    feedTitle,
    selected,
    showPreviews,
    onSelect,
    onToggleRead,
  }: ArticleListEntryProps) {
    const [cardHovered, setCardHovered] = useState(false);
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

    const handleToggleRead = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onToggleRead?.(articleView.id);
      },
      [onToggleRead, articleView.id]
    );

    const handleSelect = useCallback(
      () => onSelect?.(articleView.id),
      [onSelect, articleView.id]
    );

    // Swap dot style on hover as click affordance: unread shows filled, read shows hollow.
    // On hover, invert: read shows filled (affordance to mark unread), unread shows hollow.
    const showFilledDot = articleView.isRead ? dotHovered : !dotHovered;

    // Show favicon by default; swap to dot on card hover.
    const faviconSrc = favicon?.GetFavicon();

    const extraClasses: string[] = ['GoliathArticleCard'];
    if (articleView.isRead) {
      extraClasses.push('GoliathArticleCardRead');
    } else {
      extraClasses.push('GoliathArticleCardUnread');
    }
    if (selected) {
      extraClasses.push(
        articleView.isRead
          ? 'GoliathArticleCardReadSelected'
          : 'GoliathArticleCardUnreadSelected'
      );
    }

    return (
      <div
        className={extraClasses.join(' ')}
        onClick={handleSelect}
        onMouseEnter={() => setCardHovered(true)}
        onMouseLeave={() => setCardHovered(false)}
      >
        {/* Row 1 — source */}
        <div className="GoliathArticleCardSource">
          <span className="GoliathArticleCardIconSlot">
            {cardHovered || !faviconSrc ? (
              <span
                className="GoliathArticleCardDot"
                onClick={handleToggleRead}
                onMouseEnter={() => setDotHovered(true)}
                onMouseLeave={() => setDotHovered(false)}
              >
                {showFilledDot ? (
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
            ) : (
              <img src={faviconSrc} alt="" />
            )}
          </span>
          <span className="GoliathArticleCardFeedName">{feedTitle}</span>
          <span className="GoliathArticleCardTime">{formatFriendly(date)}</span>
        </div>

        {/* Row 2 — title */}
        <div className="GoliathArticleCardTitle">
          <a target="_blank" rel="noopener noreferrer" href={articleView.url}>
            {extractedTitle}
          </a>
        </div>

        {/* Row 3 — content */}
        <div className="GoliathArticleCardContent">
          <Typography className="GoliathArticleCardSnippet">
            {extractedContent}
          </Typography>
          {showPreviews && <ImagePreview article={articleView} />}
        </div>
      </div>
    );
  }
);

export default ArticleListEntry;
