import { FeedId } from '../models/feed';
import { formatFriendly } from '../utils/dates';
import FeedIcon from './FeedIcon';
import { PeekDirection } from '../utils/useSwipeNavigate';

/** The little of an article that the pane below a pull has to show. */
export interface ArticlePeek {
  feedId: FeedId;
  feedTitle: string;
  favicon: string;
  date: Date;
  title: string;
}

export interface NextArticlePreviewProps {
  // Which way the pull is going, and so which edge this waits behind.
  direction: PeekDirection;
  peek: ArticlePeek | undefined;
}

/**
 * The article a pull is heading for, waiting off the edge it will arrive
 * from.
 *
 * It is not a badge or a hint: it is the top of the article itself, byline
 * and headline, drawn where the article will be. That is what makes the pull
 * one movement rather than a gesture followed by a replacement -- by the time
 * the pane has carried through, what is on screen is already the thing the
 * reader asked for, and only the body arrives late.
 *
 * It moves with the pull because the stylesheet adds the pull to its resting
 * offset, which is one pane clear of the screen; it has no state of its own.
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
      <div className="GoliathArticlePreviewByline">
        <FeedIcon
          favicon={peek.favicon}
          feedTitle={peek.feedTitle}
          feedId={peek.feedId}
          size={16}
        />
        <span className="GoliathArticlePreviewFeed">{peek.feedTitle}</span>
        <span className="GoliathArticlePreviewSep">·</span>
        <span className="GoliathArticlePreviewDate">
          {formatFriendly(peek.date)}
        </span>
      </div>
      <div className="GoliathArticlePreviewTitle">{peek.title}</div>
    </div>
  );
};

export default NextArticlePreview;
