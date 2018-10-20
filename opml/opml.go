package opml

import (
	"bytes"
	"encoding/xml"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"golang.org/x/net/html/charset"
	"io/ioutil"
	"os"
	"syscall"
	"time"
)

type header struct {
	Title       string `xml:"title"`
	DateCreated string `xml:"dateCreated"`
}

type outline struct {
	Title       string    `xml:"title,attr,omitempty"`
	Description string    `xml:"description,attr,omitempty"`
	URL         string    `xml:"xmlUrl,attr,omitempty"`
	Text        string    `xml:"text,attr,omitempty"`
	Folders     []outline `xml:"outline,omitempty"`
}

type internalOpmlType struct {
	XMLName xml.Name  `xml:"opml"`
	Header  header    `xml:"head"`
	Body    []outline `xml:"body>outline"`
}

// Opml is a nested tree of folders.
type Opml struct {
	Header  header
	Folders models.Folder
}

func createOpmlObject(oi *internalOpmlType) *Opml {
	return &Opml{
		Header:  oi.Header,
		Folders: parseOutline(oi.Body),
	}
}

// parseOutline converts "outline" objects into Folder and Feed objects.
func parseOutline(children []outline) models.Folder {
	folder := models.Folder{
		Name: models.RootFolder,
	}

	for _, o := range children {
		if o.URL != "" {
			// This entity has a URL so we presume it is a Feed and has no children.
			var name string
			switch {
			case o.Title != "":
				name = o.Title
			case o.Text != "":
				name = o.Text
			default:
				name = "<untitled>"
			}

			feed := models.Feed{
				Title:       name,
				Description: o.Description,
				URL:         o.URL,
			}
			folder.Feed = append(folder.Feed, feed)
		} else {
			// This entity has no URL so we presume it is a Folder.
			var name string
			switch {
			case o.Title != "":
				name = o.Title
			case o.Text != "":
				name = o.Text
			default:
				name = "<untitled>"
			}

			child := parseOutline(o.Folders)
			child.Name = name
			folder.Folders = append(folder.Folders, child)
		}
	}

	return folder
}

// parseFolders converts a rooted folder into a list of outline objects.
func parseFolders(folder models.Folder) []outline {
	var outlines []outline

	for _, feed := range folder.Feed {
		outlines = append(outlines, outline{Text: feed.Title, URL: feed.URL})
	}

	for _, child := range folder.Folders {
		o := outline{Text: child.Name}
		o.Folders = parseFolders(child)
		outlines = append(outlines, o)
	}

	return outlines
}

// ParseOpml open a file and returns a parsed OPML object.
func ParseOpml(filename string) (*Opml, error) {
	log.Infof("Loading OPML file from %s", filename)

	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(f)
	d := xml.NewDecoder(r)
	d.CharsetReader = charset.NewReaderLabel

	oi := new(internalOpmlType)
	err = d.Decode(&oi)
	if err != nil {
		return nil, err
	}

	return createOpmlObject(oi), nil
}

// ExportOpml exports the given folder (with associated feeds and child folders)
// to a file of the given filename in OPML format.
// The file is created with 0777 mode if it does not exist.
func ExportOpml(tree *models.Folder, filename string) error {
	// OPML specifies that time fields conform to RFC822.
	exportTime := time.Now().Format(time.RFC822)
	export := &internalOpmlType{}
	export.Header = header{Title: "Goliath Feed Export", DateCreated: exportTime}
	export.Body = parseFolders(*tree)

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, syscall.S_IRWXU|syscall.S_IRWXG|syscall.S_IRWXO)
	if err != nil {
		return err
	}
	defer f.Close()

	e := xml.NewEncoder(f)
	e.Indent("", "  ")
	return e.Encode(export)
}
