package models

import "encoding/xml"

type Header struct {
	Title string `xml:"title"`
}

type Outline struct {
	Feed
	Outlines []Outline `xml:"outline,omitempty"`

	// Do not directly unmarshal into this object. This is populated afterwards.
	Folder `xml:"-"`
}

type Opml struct {
	XMLName xml.Name  `xml:"opml"`
	Header  Header    `xml:"head"`
	Body    []Outline `xml:"body>outline"`
}
