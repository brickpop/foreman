package home

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// The config files the CEO owns, all TOML: project.toml
// (wiki/product/models/project-config.md), team files
// (wiki/product/models/team-definition.md, souls:
// wiki/product/models/agent-soul.md) and settings.toml
// (wiki/product/models/global-settings.md).
//
// Parsing never fails as a whole: what is wrong comes back as Problems, one
// per field, so a half-edited file still shows up with its errors.

// ProjectConfig is project.toml.
type ProjectConfig struct {
	Description  string   `toml:"description,omitempty" json:"description"`
	Checks       string   `toml:"checks,omitempty" json:"checks"`
	AlwaysAskCEO []string `toml:"always_ask_ceo,omitempty" json:"alwaysAskCeo"`
	// RecursiveSubmodules also checks out the submodules of submodules in the
	// agents' worktrees.
	RecursiveSubmodules bool `toml:"recursive_submodules,omitempty" json:"recursiveSubmodules"`
	// LandingPrefixes are the names a lead may propose for a branch its merge
	// lands on (request_merge's land), made only when the CEO merges:
	// DefaultLandingPrefixes when the file names none, nothing with [].
	LandingPrefixes []string `toml:"landing_prefixes,omitempty" json:"landingPrefixes"`
}

// DefaultLandingPrefixes are the landing prefixes of a project.toml without
// landing_prefixes (the CEO, 2026-11-11).
var DefaultLandingPrefixes = []string{"f/", "feat/", "fix/"}

// TeamConfig is a team file, teams/<team>.toml.
type TeamConfig struct {
	Team     string          `toml:"team" json:"team"`
	Name     string          `toml:"name" json:"name"`
	Purpose  string          `toml:"purpose,omitempty" json:"purpose"`
	Lead     string          `toml:"lead" json:"lead"`
	Playbook []PlaybookStep  `toml:"playbook,omitempty" json:"playbook"`
	Escalate []Escalation    `toml:"escalate,omitempty" json:"escalate"`
	Agents   map[string]Soul `toml:"agents" json:"agents"`
	// Order is the agents in the file's order, which a map (and so JSON) loses.
	Order []string `toml:"-" json:"order"`
}

// PlaybookStep is one `{ who = "text" }` step.
type PlaybookStep map[string]string

// Escalation is one row of the escalation table.
type Escalation struct {
	When string `toml:"when" json:"when"`
	To   string `toml:"to" json:"to"`
}

// Soul is one agent of a team.
type Soul struct {
	Name        string   `toml:"name" json:"name"`
	Role        string   `toml:"role" json:"role"`
	Personality string   `toml:"personality,omitempty" json:"personality"`
	Skills      []string `toml:"skills,omitempty" json:"skills"`
	Instruction string   `toml:"instruction,omitempty" json:"instruction"`
}

// Settings is settings.toml. Absent fields keep their defaults.
type Settings struct {
	Port              int    `toml:"port" json:"port"`
	MaxSessions       int    `toml:"max_sessions" json:"maxSessions"`
	Engine            string `toml:"engine" json:"engine"`
	StuckAfterMinutes int    `toml:"stuck_after_minutes" json:"stuckAfterMinutes"`
	LogCapMB          int    `toml:"log_cap_mb" json:"logCapMb"`
	// ForemanFolder holds what the CEO owns (Home.Folder): Foreman's HQ, the
	// projects, the agents' worktrees. Read at start only, and never through
	// the API: not served, kept on a save (the CEO, 2026-11-15 and 11-16).
	ForemanFolder string        `toml:"foreman_folder" json:"-"`
	Notifications Notifications `toml:"notifications" json:"notifications"`
	// The look of the UI and its terminal (Settings → Notifications & look).
	Theme  string `toml:"theme" json:"theme"`   // system, dark or light
	Accent string `toml:"accent" json:"accent"` // one of Accents: the UI's accent color (its styles.css)
	// TerminalFont is one of TerminalFonts; the UI ships each but system.
	TerminalFont     string `toml:"terminal_font" json:"terminalFont"`
	TerminalFontSize int    `toml:"terminal_font_size" json:"terminalFontSize"` // px, MinFontSize to MaxFontSize
}

var themes = []string{"system", "dark", "light"}

// Accents are the UI's accent colors, the default first (the UI's accents
// lists the same ids).
var Accents = []string{"blue", "green", "purple", "teal", "orange", "pink", "amber"}

// TerminalFonts are the terminal's fonts (the UI's terminalFonts lists the
// same ids); system is the operating system's monospace.
var TerminalFonts = []string{"jetbrains-mono", "fira-code", "source-code-pro", "system"}

// The terminal's font sizes, in px.
const (
	MinFontSize = 10
	MaxFontSize = 24
)

// Notifications are the browser notification switches.
type Notifications struct {
	NeedsMe        bool `toml:"needs_me" json:"needsMe"`
	Finishes       bool `toml:"finishes" json:"finishes"`
	MergeRequested bool `toml:"merge_requested" json:"mergeRequested"`
}

// DefaultSettings are the settings while settings.toml doesn't exist.
func DefaultSettings() Settings {
	return Settings{
		Port: 8888, MaxSessions: 12, Engine: "podman",
		StuckAfterMinutes: 20, LogCapMB: 50,
		ForemanFolder: "~/foreman",
		Notifications: Notifications{NeedsMe: true, Finishes: true, MergeRequested: true},
		Theme:         "system", Accent: Accents[0], TerminalFont: TerminalFonts[0], TerminalFontSize: 13,
	}
}

var (
	slugRule  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	snakeCase = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
)

// Version identifies a file's content: writes carry the version they read.
func Version(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// problems collects validation messages.
type problems []string

func (p *problems) add(format string, a ...any) { *p = append(*p, fmt.Sprintf(format, a...)) }

// decode parses data into v, reporting a syntax error or unknown keys (which a
// save would otherwise drop silently), but those retired says an older file
// may still hold: those load, ignored, and the next save drops them.
func decode(data []byte, v any, retired ...func(toml.Key) bool) (toml.MetaData, problems) {
	md, err := toml.Decode(string(data), v)
	if err != nil {
		return md, problems{err.Error()}
	}
	var p problems
	for _, key := range md.Undecoded() {
		if slices.ContainsFunc(retired, func(r func(toml.Key) bool) bool { return r(key) }) {
			continue
		}
		p.add("%s: not a field Foreman knows (misspelled, or left from an older version); remove it", key)
	}
	return md, p
}

// ParseProject parses a project.toml: the project's rules, which travel with
// its HQ (its name and repos are this machine's, in the database).
func ParseProject(data []byte) (ProjectConfig, []string) {
	var c ProjectConfig
	md, p := decode(data, &c)
	// Retired keys every project.toml of their time has: say what replaced them.
	for _, r := range []struct{ key, note string }{
		{"main", "each code repo has its own default branch now (Repos)"}, // before 2026-10-21
		{"merge_to_main", "only the CEO merges, always"},                  // before 2026-11-02; it only ever took "ceo"
	} {
		if md.IsDefined(r.key) {
			p = slices.DeleteFunc(p, func(s string) bool { return strings.HasPrefix(s, r.key+": ") })
			p.add("%s: %s; remove this line", r.key, r.note)
		}
	}
	c.AlwaysAskCEO = normalize(c.AlwaysAskCEO)
	if !md.IsDefined("landing_prefixes") {
		c.LandingPrefixes = slices.Clone(DefaultLandingPrefixes)
	}
	for _, prefix := range c.LandingPrefixes {
		// A prefix names a folder of branches: never an agent's.
		if !strings.HasSuffix(prefix, "/") || prefix == "/" || strings.HasPrefix(prefix, "wt/") || strings.HasPrefix(prefix, "lead/") {
			p.add("landing_prefixes: %q is not a folder of branches of yours (like \"f/\"; never wt/ or lead/)", prefix)
		}
	}
	if strings.ContainsAny(c.Description, "\n") {
		p.add("description: one line")
	}
	return c, p
}

// ParseTeam parses the team file of team id. Rules across teams (agent ids
// and names unique across teams) are checked when the home loads.
func ParseTeam(id string, data []byte) (TeamConfig, []string) {
	var c TeamConfig
	md, p := decode(data, &c, retiredLook)
	c.Order = agentOrder(md)
	c.Playbook, c.Escalate = normalize(c.Playbook), normalize(c.Escalate)
	if c.Agents == nil {
		c.Agents = map[string]Soul{}
	}
	for id, s := range c.Agents {
		s.Skills = normalize(s.Skills)
		c.Agents[id] = s
	}
	p = append(p, c.validate(id)...)
	return c, p
}

// retiredLook is a soul's look: its avatar, removed on 2026-11-02 (the CEO:
// "it only makes things ugly, and complex").
func retiredLook(k toml.Key) bool { return len(k) >= 3 && k[0] == "agents" && k[2] == "look" }

func (c TeamConfig) validate(id string) problems {
	var p problems
	switch {
	case c.Team == "":
		p.add("team: required")
	case c.Team != id:
		p.add("team: %q must equal the file name %q", c.Team, id)
	case !slugRule.MatchString(c.Team):
		p.add("team: %q must be a slug (lowercase, digits, hyphens)", c.Team)
	}
	if strings.TrimSpace(c.Name) == "" {
		p.add("name: required")
	}
	if len(c.Agents) == 0 {
		p.add("agents: at least the lead")
	}
	if _, ok := c.Agents[c.Lead]; !ok {
		p.add("lead: %q is not an agent of the team", c.Lead)
	}
	for _, step := range c.Playbook {
		if len(step) != 1 {
			p.add("playbook: each step is one `{ who = \"text\" }`")
		}
	}
	for i, e := range c.Escalate {
		if strings.TrimSpace(e.When) == "" || strings.TrimSpace(e.To) == "" {
			p.add("escalate[%d]: when and to are required", i)
		}
	}
	named := map[string]string{} // a name → the first agent with it
	for _, aid := range sortedKeys(c.Agents) {
		p = append(p, c.Agents[aid].validate("agents."+aid, aid)...)
		name := strings.TrimSpace(c.Agents[aid].Name)
		if other, ok := named[name]; ok && name != "" {
			p.add("agents.%s.name: %q is also %s's name", aid, name, other)
		} else {
			named[name] = aid
		}
	}
	return p
}

func (s Soul) validate(at, id string) problems {
	var p problems
	if !slugRule.MatchString(id) {
		p.add("%s: the agent id must be a slug (lowercase, digits, hyphens)", at)
	}
	if strings.TrimSpace(s.Name) == "" {
		p.add("%s.name: required", at)
	}
	if !snakeCase.MatchString(s.Role) {
		p.add("%s.role: %q must be a role in snake_case (team_lead, engineer, …)", at, s.Role)
	}
	seen := map[string]bool{}
	for _, skill := range s.Skills {
		if seen[skill] {
			p.add("%s.skills: %q twice", at, skill)
		}
		seen[skill] = true
	}
	return p
}

// ParseSettings parses settings.toml over the defaults. A look the UI
// doesn't have (a hand edit gone wrong, a font no longer offered) reads as
// its default, its problem still reported: the UI applies the look as it
// comes.
func ParseSettings(data []byte) (Settings, []string) {
	s, def := DefaultSettings(), DefaultSettings()
	_, p := decode(data, &s)
	p = append(p, s.validate()...)
	if !slices.Contains(themes, s.Theme) {
		s.Theme = def.Theme
	}
	if !slices.Contains(Accents, s.Accent) {
		s.Accent = def.Accent
	}
	if !slices.Contains(TerminalFonts, s.TerminalFont) {
		s.TerminalFont = def.TerminalFont
	}
	if s.TerminalFontSize < MinFontSize || s.TerminalFontSize > MaxFontSize {
		s.TerminalFontSize = def.TerminalFontSize
	}
	return s, p
}

func (s Settings) validate() problems {
	var p problems
	if s.Port < 1 || s.Port > 65535 {
		p.add("port: %d is not a port", s.Port)
	}
	if s.MaxSessions < 1 || s.MaxSessions > 64 {
		p.add("max_sessions: must be 1 to 64")
	}
	if s.Engine != "podman" && s.Engine != "direct" {
		p.add(`engine: must be "podman" or "direct"`)
	}
	if s.StuckAfterMinutes <= 0 {
		p.add("stuck_after_minutes: must be more than 0")
	}
	if s.LogCapMB <= 0 {
		p.add("log_cap_mb: must be more than 0")
	}
	if f := s.ForemanFolder; f != "~" && !strings.HasPrefix(f, "~/") && !filepath.IsAbs(f) {
		p.add("foreman_folder: %q must be an absolute path or start with ~/", f)
	}
	if !slices.Contains(themes, s.Theme) {
		p.add("theme: %q is not one of %s", s.Theme, strings.Join(themes, ", "))
	}
	if !slices.Contains(Accents, s.Accent) {
		p.add("accent: %q is not one of %s", s.Accent, strings.Join(Accents, ", "))
	}
	if !slices.Contains(TerminalFonts, s.TerminalFont) {
		p.add("terminal_font: %q is not one of %s", s.TerminalFont, strings.Join(TerminalFonts, ", "))
	}
	if s.TerminalFontSize < MinFontSize || s.TerminalFontSize > MaxFontSize {
		p.add("terminal_font_size: %d is not %d to %d", s.TerminalFontSize, MinFontSize, MaxFontSize)
	}
	return p
}

// encode renders v as TOML. A save rewrites the whole file: comments written
// by hand don't survive it; Foreman's HQ keeps a team file's earlier versions.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = "" // flush left, like a file written by hand
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// normalize makes absent lists and maps empty rather than nil, so the JSON
// the UI reads always has a list where its types say so.
func normalize[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// agentOrder is the agent ids in the order the file declares them.
func agentOrder(md toml.MetaData) []string {
	order := []string{}
	for _, k := range md.Keys() {
		if len(k) == 2 && k[0] == "agents" {
			order = append(order, k[1])
		}
	}
	return order
}

// encodeTeam encodes a team file with its agents in c.Order; the encoder
// alone would sort them. Agents missing from Order follow, sorted.
func encodeTeam(c TeamConfig) ([]byte, error) {
	agents := c.Agents
	c.Agents = nil
	out, err := encode(c)
	if err != nil {
		return nil, err
	}
	ids := slices.DeleteFunc(slices.Clone(c.Order), func(id string) bool { _, ok := agents[id]; return !ok })
	for _, id := range sortedKeys(agents) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		soul := agents[id]
		chunk, err := encode(map[string]map[string]Soul{"agents": {id: soul}})
		if err != nil {
			return nil, err
		}
		// Each chunk opens with its parent table, [agents], which [agents.<id>]
		// already implies.
		out = append(append(out, '\n'), strings.TrimPrefix(string(chunk), "[agents]\n")...)
	}
	return out, nil
}

// Roster is the team's agent ids as the team presents them: the lead first,
// then the file's order (agents it doesn't name follow, sorted).
func (c TeamConfig) Roster() []string {
	ids := []string{}
	add := func(id string) {
		if _, ok := c.Agents[id]; ok && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	add(c.Lead)
	for _, id := range c.Order {
		add(id)
	}
	for _, id := range sortedKeys(c.Agents) {
		add(id)
	}
	return ids
}
