package models

const (
	// RootFolder is the unique name of the root folder.
	// Any folders imported with this name will get merged with this folder.
	RootFolder = "<root>"
)

// Folder is a collection of feeds and subfolders.
type Folder struct {
	// Primary key
	ID int64
	// Data fields
	Name    string
	Feed    []Feed
	Folders []Folder
}
