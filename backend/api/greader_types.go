package api

type greaderTokenType struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

type greaderHandlelogin struct {
	SID  string `json:"SID"`
	LSID string `json:"LSID"`
	Auth string `json:"Auth"`
}

type greaderUserInfo struct {
	UserId   string `json:"userId"`
	Username string `json:"userName"`
}

type greaderCategory struct {
	Id    string `json:"id"`
	Label string `json:"label"`
}

type greaderSubscription struct {
	Title         string            `json:"title"`
	FirstItemMsec string            `json:"firstitemmsec"`
	HtmlUrl       string            `json:"htmlUrl"`
	IconUrl       string            `json:"iconUrl"`
	SortId        string            `json:"sortid"`
	Id            string            `json:"id"`
	Categories    []greaderCategory `json:"categories"`
}

type greaderSubscriptionList struct {
	Subscriptions []greaderSubscription `json:"subscriptions"`
}

type greaderCanonical struct {
	Href string `json:"href"`
}

type greaderContent struct {
	Direction string `json:"direction,omitempty"`
	Content   string `json:"content"`
}

type greaderOrigin struct {
	StreamId string `json:"streamId"`
	Title    string `json:"title"`
	HtmlUrl  string `json:"htmlUrl"`
}

type greaderItemContent struct {
	CrawlTimeMsec string             `json:"crawlTimeMsec"`
	TimestampUsec string             `json:"timestampUsec"`
	Id            string             `json:"id"`
	Categories    []string           `json:"categories"`
	Title         string             `json:"title"`
	Published     int64              `json:"published"`
	Canonical     []greaderCanonical `json:"canonical"`
	Alternate     []greaderCanonical `json:"alternate"`
	Summary       greaderContent     `json:"summary"`
	Origin        greaderOrigin      `json:"origin"`
}

type greaderItemRef struct {
	Id              string   `json:"id"`
	DirectStreamIds []string `json:"directStreamIds"`
	TimestampUsec   string   `json:"timestampUsec"`
}

type greaderStreamItemIds struct {
	Items    []greaderItemContent `json:"items"`
	ItemRefs []greaderItemRef     `json:"itemRefs"`
}

type greaderStreamItemsContents struct {
	Id      string               `json:"id"`
	Updated int64                `json:"updated"`
	Items   []greaderItemContent `json:"items"`
}
