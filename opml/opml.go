package opml

import (
	"bytes"
	"encoding/xml"
	"github.com/jrupac/goliath/models"
	"golang.org/x/net/html/charset"
	"io/ioutil"
)

// ParseOpml open a file and returns a parsed OPML object.
func ParseOpml(filename string) (*models.Opml, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(f)
	d := xml.NewDecoder(r)
	d.CharsetReader = charset.NewReaderLabel

	p := new(models.Opml)
	err = d.Decode(&p)
	if err != nil {
		return nil, err
	}

	p.Body = convertToFolders(p.Body)
	return p, nil
}

// convertToFolders converts "outline" objects that are actually just folder names into Folder objects
func convertToFolders(outlines []models.Outline) []models.Outline {
	if len(outlines) == 0 {
		return outlines
	}

	var updatedBody []models.Outline
	for _, o := range outlines {
		// A Folder has a name, no URL associated with it
		if (o.Title != "" || o.Text != "") && o.Url == "" {
			var name string
			if o.Title != "" {
				name = o.Title
			} else {
				name = o.Text
			}

			var newOutline = models.Outline{
				Folder:   models.Folder{Name: name},
				Outlines: convertToFolders(o.Outlines),
			}
			updatedBody = append(updatedBody, newOutline)
		} else {
			o.Outlines = convertToFolders(o.Outlines)
			updatedBody = append(updatedBody, o)
		}
	}
	return updatedBody
}
