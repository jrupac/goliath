const describeCount = (count: number): string => {
  if (count === 0) {
    return 'No new articles';
  }
  return count === 1 ? '1 new article' : `${count} new articles`;
};

const labelCount = (count: number): string =>
  count === 0 ? 'Nothing new' : `${count} new`;

export interface NewArticleNoteProps {
  // What the last refresh brought in, or null before there has been one.
  // Zero is not null: a refresh that found nothing is the answer to having
  // asked, and saying so is the point.
  count: number | null;
  // Whether that report is still current. The text outlives it, so that it
  // does not disappear ahead of the element holding it.
  shown: boolean;
}

/**
 * What the last refresh brought in, reported in the list's own header.
 *
 * A refresh lands the reader at the top of the unread stream, so the header
 * is where the eye already is when the result arrives; the same number drawn
 * in the sidebar has to be caught in the moment to be read at all. It also
 * sits ahead of the bar's spacer, which absorbs its width, so nothing else in
 * the bar moves as the note comes and goes.
 *
 * The region is rendered whether or not there is anything to report, since a
 * live region that appears together with its text is not reliably announced.
 * Its announcement is cleared when the report lapses, so that the next one
 * reads as new even if it says the same thing.
 */
const NewArticleNote = ({ count, shown }: NewArticleNoteProps) => {
  const current = shown && count !== null;
  return (
    <div
      className={
        current
          ? 'GoliathNewArticleNote GoliathNewArticleNoteVisible'
          : 'GoliathNewArticleNote'
      }
      role="status"
    >
      <span className="GoliathVisuallyHidden">
        {current ? describeCount(count) : ''}
      </span>
      <span aria-hidden="true">{count === null ? '' : labelCount(count)}</span>
    </div>
  );
};

export default NewArticleNote;
