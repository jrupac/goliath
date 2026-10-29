import { getFeedInitials, hashToSwatchIndex } from '../utils/helpers';

interface FeedIconProps {
  favicon: string;
  feedTitle: string;
  feedId: string;
  size?: number;
  alt?: string;
}

const FeedIcon = ({
  favicon,
  feedTitle,
  feedId,
  size = 16,
  alt = '',
}: FeedIconProps) => {
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
