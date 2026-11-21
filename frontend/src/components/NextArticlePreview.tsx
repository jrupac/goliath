import { FeedId } from '../models/feed';
import FeedIcon from './FeedIcon';
import { PeekDirection } from '../utils/useSwipeNavigate';

/** The little of an article that the pane behind a pull has to show. */
export interface ArticlePeek {
  feedId: FeedId;
  feedTitle: string;
  favicon: string;
  title: string;
}

export interface NextArticlePreviewProps {
  // Which way the pull is going, and so which edge this waits behind.
  direction: PeekDirection;
  peek: ArticlePeek | undefined;
}

/**
 * The article a pull is heading for, waiting behind the one being read.
 *
 * It names that article and stops there: which feed, by its mark, and the
 * headline in a line or two. Drawing the article's own byline and title
 * instead would be a near-match for what lands a moment later, and a
 * near-match is worse than a plain difference -- every pull would end in the
 * type resettling by a few pixels. This is its own thing, so there is
 * nothing for the eye to compare.
 *
 * It takes no part in the gesture. The stylesheet decides where it waits and
 * whether the pull moves it or merely uncovers it.
 */
const NextArticlePreview = ({ direction, peek }: NextArticlePreviewProps) => {
  if (direction === 0 || peek === undefined) {
    return null;
  }

  return (
    <div
      className={
        direction === -1
          ? 'GoliathArticlePreview GoliathArticlePreviewBefore'
          : 'GoliathArticlePreview'
      }
      aria-hidden="true"
    >
      <div className="GoliathArticlePreviewCard">
        <FeedIcon
          favicon={peek.favicon}
          feedTitle={peek.feedTitle}
          feedId={peek.feedId}
          size={18}
        />
        <span className="GoliathArticlePreviewTitle">{peek.title}</span>
      </div>
    </div>
  );
};

export default NextArticlePreview;
