package userdata

// Preferences are the CEO's, for every workspace (tygo.yaml).
type Preferences struct {
	Theme string `toml:"theme" json:"theme"` // system, dark or light
}
