import { forwardRef } from 'react';
import ExpandLessTwoToneIcon from '@mui/icons-material/ExpandLessTwoTone';
import ExpandMoreTwoToneIcon from '@mui/icons-material/ExpandMoreTwoTone';
import { PeekDirection } from '../utils/useSwipeNavigate';

/** The little of an article the strip needs to name it. */
export interface ArticlePeek {
  feedTitle: string;
  title: string;
}

export interface NextArticlePeekProps {
  // Which way the pull is going: towards the next article, the previous one,
  // or nowhere, which is every moment the reader is not pulling.
  direction: PeekDirection;
  // Whether letting go now would go there. The strip says so rather than
  // leaving the reader to judge it from how far the strip has opened.
  armed: boolean;
  feedTitle?: string;
  title?: string;
}

/**
 * What is on the other side of a pull, shown while the pull is being made.
 *
 * The strip rises out of the edge the reader is pulling from, and its own
 * height is how far the pull has got, so nothing else has to be drawn to say
 * so. It is named rather than hinted at: a chevron would say there is
 * something there, which the reader already knows, where the title answers
 * the question that decides whether to carry on.
 *
 * Its height is written by the gesture and not by React, which is why this
 * takes a ref and renders at no height of its own.
 */
const NextArticlePeek = forwardRef<HTMLDivElement, NextArticlePeekProps>(
  ({ direction, armed, feedTitle, title }, ref) => {
    const classes = ['GoliathNextArticlePeek'];
    if (direction === -1) {
      classes.push('GoliathNextArticlePeekAtStart');
    }
    if (armed) {
      classes.push('GoliathNextArticlePeekArmed');
    }

    return (
      <div className={classes.join(' ')} ref={ref} aria-hidden="true">
        <div className="GoliathNextArticlePeekText">
          {feedTitle && (
            <span className="GoliathNextArticlePeekFeed">{feedTitle}</span>
          )}
          <span className="GoliathNextArticlePeekTitle">
            {title ?? (direction === -1 ? 'Previous article' : 'Next article')}
          </span>
        </div>
        <span className="GoliathNextArticlePeekChevron">
          {direction === -1 ? (
            <ExpandMoreTwoToneIcon />
          ) : (
            <ExpandLessTwoToneIcon />
          )}
        </span>
      </div>
    );
  }
);

NextArticlePeek.displayName = 'NextArticlePeek';

export default NextArticlePeek;
