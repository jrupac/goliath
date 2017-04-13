package models

type Folder struct {
	FolderId uint

	Name    string
	Feed    []Feed
	Folders []Folder
}
