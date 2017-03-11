package models

type Header struct {
	Title string `xml:"title"`
}

type Opml struct {
	Header Header `xml:"head"`
	Feeds  []Feed `xml:"body>outline"`
}
