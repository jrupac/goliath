package models

import (
	"fmt"
	"strings"
)

const (
	// RootFolder is the unique name of the root folder.
	// Any folders imported with this name will get merged with this folder.
	RootFolder = "<root>"

	// RootFolderDisplayName is what the root folder is called when a client is
	// shown it.
	//
	// A feed's folder is part of the key its articles are stored under, so
	// every feed is in one, and a feed in no folder of the user's own is in
	// this one. RootFolder is a sentinel written for the database; a client
	// given that name draws a folder called "<root>", so it is presented under
	// a name meant for people. Being user-visible makes it a name a user must
	// not also be able to give a folder of their own -- see ValidateFolderName.
	RootFolderDisplayName = "Uncategorized"
)

// ValidateFolderName reports whether a name may be given to a folder the user
// is creating.
//
// The only reserved name is the one the root folder is presented under. Two
// folders answering to it, one of them the place feeds go when they are filed
// nowhere, is indistinguishable to whoever is looking at the list.
//
// RootFolder itself is deliberately not reserved: a folder imported under that
// name merges into the root, which is long-standing behaviour rather than a
// collision.
func ValidateFolderName(name string) error {
	if strings.EqualFold(strings.TrimSpace(name), RootFolderDisplayName) {
		return fmt.Errorf("folder name %q is reserved for feeds that are in no other folder", name)
	}
	return nil
}

// Folder is a collection of feeds and subfolders.
type Folder struct {
	// Primary key
	ID int64
	// Data fields
	Name    string
	Feed    []Feed
	Folders []Folder
}
