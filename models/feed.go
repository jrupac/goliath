package models

type Feed struct {
	Title       string `xml:"title,attr"`
	Description string `xml:"description,attr"`
	Url         string `xml:"xmlUrl,attr"`
}
