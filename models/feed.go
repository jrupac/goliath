package models

type Feed struct {
	Title       string `xml:"title,attr,omitempty"`
	Description string `xml:"description,attr,omitempty"`
	Url         string `xml:"xmlUrl,attr,omitempty"`
	Text        string `xml:"text,attr,omitempty"`
}

type Folder struct {
	Name string
}
