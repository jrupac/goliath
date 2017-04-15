package models

type Folder struct {
	Id int

	Name    string
	Feed    []Feed
	Folders []Folder
}
