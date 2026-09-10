package models

import "testing"

// The root folder is presented under a name meant for people, so a user must
// not be able to give a folder of their own the same name: two folders
// answering to it, one of them where unfiled feeds live, is indistinguishable
// to whoever is reading the list.
func TestValidateFolderNameReservesTheRootDisplayName(t *testing.T) {
	for _, name := range []string{
		RootFolderDisplayName,
		"uncategorized",
		"UNCATEGORIZED",
		"  Uncategorized  ",
	} {
		if err := ValidateFolderName(name); err == nil {
			t.Errorf("ValidateFolderName(%q) allowed a reserved name", name)
		}
	}
}

// Everything else is the user's business, including the stored sentinel: a
// folder imported under that name merges into the root, which is long-standing
// behaviour rather than a collision.
func TestValidateFolderNameAllowsOrdinaryNames(t *testing.T) {
	for _, name := range []string{
		"Comics", "News", "Uncategorised", "Uncategorized things", "", RootFolder,
	} {
		if err := ValidateFolderName(name); err != nil {
			t.Errorf("ValidateFolderName(%q) = %v, want nil", name, err)
		}
	}
}
