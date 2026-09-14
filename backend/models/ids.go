package models

// ArticleId, FeedId and FolderId identify an article, feed and folder. All
// three are integers from the same database, so as plain int64s any one could
// be passed where another was meant and the mistake would compile, then read or
// change the wrong rows, or none. As distinct types, an ID can only become
// another kind by an explicit conversion, and those conversions are where a
// number arriving from outside -- a request parameter, an RPC field -- is
// decided to be one kind rather than another.
//
// Each has int64 as its underlying type, so database/sql binds and scans them
// as it would an int64. Driver helpers that special-case []int64, such as
// array parameters, need the slice converted first.
type (
	ArticleId int64
	FeedId    int64
	FolderId  int64
)
