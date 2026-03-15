package entity

type EntryType int

const (
	EntryTypeUnspecified EntryType = iota
	EntryTypeLogin
	EntryTypeSecret
	EntryTypeNote
)

func (t EntryType) String() string {
	switch t {
	case EntryTypeLogin:
		return "login"
	case EntryTypeSecret:
		return "secret"
	case EntryTypeNote:
		return "note"
	default:
		return "unspecified"
	}
}

func ParseEntryType(s string) EntryType {
	switch s {
	case "login":
		return EntryTypeLogin
	case "secret":
		return EntryTypeSecret
	case "note":
		return EntryTypeNote
	default:
		return EntryTypeUnspecified
	}
}
