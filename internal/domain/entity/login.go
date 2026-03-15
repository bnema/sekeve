package entity

type Login struct {
	Site     string `json:"site"`
	Username string `json:"username"`
	Password string `json:"password"`
	Notes    string `json:"notes,omitempty"`
}
