import React from 'react';
import { getFeedInitials, hashToSwatchIndex } from '../utils/helpers';

interface FeedIconProps {
  favicon: string;
  feedTitle: string;
  feedId: string;
  size?: number;
  alt?: string;
}

const FeedIcon: React.FC<FeedIconProps> = ({
  favicon,
  feedTitle,
  feedId,
  size = 16,
  alt = '',
}) => {
  if (favicon) {
    return (
      <img
        src={favicon}
        width={size}
        height={size}
        alt={alt}
        className="GoliathFeedIconImg"
      />
    );
  }

  const initials = getFeedInitials(feedTitle);
  const swatchIndex = hashToSwatchIndex(feedId);

  return (
    <div className={`GoliathFeedIcon GoliathFeedIconSwatch${swatchIndex}`}>
      <span>{initials}</span>
    </div>
  );
};

export default FeedIcon;
