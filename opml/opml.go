package opml

import (
	"bytes"
	"encoding/xml"
	"github.com/jrupac/goliath/models"
	"golang.org/x/net/html/charset"
	"io/ioutil"
)

func ParseOpml(filename string) (*models.Opml, error) {

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(file)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel

	parsedOpml := new(models.Opml)
	err = decoder.Decode(&parsedOpml)
	if err != nil {
		return nil, err
	}

	return parsedOpml, nil
}
