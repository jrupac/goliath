package models

import (
	"encoding/xml"
)

type Header struct {
	Title string `xml:"title"`
}

type Outline struct {
	Title       string    `xml:"title,attr,omitempty"`
	Description string    `xml:"description,attr,omitempty"`
	Url         string    `xml:"xmlUrl,attr,omitempty"`
	Text        string    `xml:"text,attr,omitempty"`
	Folders     []Outline `xml:"outline,omitempty"`
}

type OpmlInternal struct {
	XMLName xml.Name  `xml:"opml"`
	Header  Header    `xml:"head"`
	Body    []Outline `xml:"body>outline"`
}

type Opml struct {
	Header  Header
	Folders Folder
}
