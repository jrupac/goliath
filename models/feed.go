package models

type Feed struct {
	Id       int
	FolderId int

	Title       string
	Description string
	Url         string
	Text        string
}
