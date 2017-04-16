package models

type Folder struct {
	Id int64

	Name    string
	Feed    []Feed
	Folders []Folder
}
