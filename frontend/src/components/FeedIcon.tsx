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
        style={{
          borderRadius: '50%',
          objectFit: 'cover',
          display: 'block',
        }}
      />
    );
  }

  const initials = getFeedInitials(feedTitle);
  const swatchIndex = hashToSwatchIndex(feedId);
  const fontSize = Math.round(size * 0.45);

  return (
    <div
      className="GoliathFeedIcon"
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        background: `var(--swatch-${swatchIndex})`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
      }}
    >
      <span
        style={{
          color: 'white',
          fontWeight: 600,
          fontSize,
        }}
      >
        {initials}
      </span>
    </div>
  );
};

export default FeedIcon;
