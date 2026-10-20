package userdata

// Preferences are the CEO's, on this machine (tygo.yaml).
type Preferences struct {
	Theme string `toml:"theme" json:"theme"` // system, dark or light
	// TerminalFont is one of TerminalFonts; the UI ships each but system.
	TerminalFont     string `toml:"terminal_font" json:"terminalFont"`
	TerminalFontSize int    `toml:"terminal_font_size" json:"terminalFontSize"` // px, MinFontSize to MaxFontSize
}
