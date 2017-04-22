package opml

import (
	"bytes"
	"encoding/xml"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"

	"golang.org/x/net/html/charset"
	"io/ioutil"
)

// ParseOpml open a file and returns a parsed OPML object.
func ParseOpml(filename string) (*models.Opml, error) {
	log.Infof("Loading OPML file from %s", filename)

	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(f)
	d := xml.NewDecoder(r)
	d.CharsetReader = charset.NewReaderLabel

	oi := new(models.OpmlInternal)
	err = d.Decode(&oi)
	if err != nil {
		return nil, err
	}

	return createOpmlObject(oi), nil
}

func createOpmlObject(oi *models.OpmlInternal) *models.Opml {
	return &models.Opml{
		Header:  oi.Header,
		Folders: parseOutline(oi.Body),
	}
}

// parseOutline converts "outline" objects into Folder and Feed objects.
func parseOutline(children []models.Outline) models.Folder {
	folder := models.Folder{
		Name: "<root>",
	}

	for _, o := range children {
		if o.Url != "" {
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
				Url:         o.Url,
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
