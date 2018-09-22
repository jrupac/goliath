package models

const (
	// Name of unique root folder.
	RootFolder = "<root>"
)

// Folder is a collection of feeds and subfolders.
type Folder struct {
	ID int64

	Name    string
	Feed    []Feed
	Folders []Folder
}
