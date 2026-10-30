/**
 * How many articles the refresh just brought in, drawn beside the unread
 * count for as long as the caller keeps a count on the prop.
 *
 * Announced as well as drawn: it appears and disappears on its own, which is
 * exactly the kind of change a reader not looking at the sidebar would
 * otherwise miss.
 */
const NewArticlePill = ({ count }: { count: number }) => (
  <div className="GoliathSidebarPillNew" role="status">
    <span className="GoliathVisuallyHidden">
      {count === 1 ? '1 new article' : `${count} new articles`}
    </span>
    <span aria-hidden="true">+{count}</span>
  </div>
);

export default NewArticlePill;
