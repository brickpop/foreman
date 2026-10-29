package home

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
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
	MergeToMain  string   `toml:"merge_to_main,omitempty" json:"mergeToMain"`
	AlwaysAskCEO []string `toml:"always_ask_ceo,omitempty" json:"alwaysAskCeo"`
	// RecursiveSubmodules also checks out the submodules of submodules in the
	// agents' worktrees.
	RecursiveSubmodules bool  `toml:"recursive_submodules,omitempty" json:"recursiveSubmodules"`
	Forge               Forge `toml:"forge,omitempty" json:"forge"`
}

// Forge is where the CEO can create PRs.
type Forge struct {
	Kind        string `toml:"kind,omitempty" json:"kind"`
	URL         string `toml:"url,omitempty" json:"url"`
	ProjectRepo string `toml:"project_repo,omitempty" json:"projectRepo"`
}

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
	Look        Look     `toml:"look" json:"look"`
}

// Look is the avatar (wiki/product/design/avatar.md).
type Look struct {
	Color     string `toml:"color" json:"color"`
	Skin      string `toml:"skin" json:"skin"`
	Hair      string `toml:"hair" json:"hair"`
	HairColor string `toml:"hair_color" json:"hairColor"`
	Eyes      string `toml:"eyes" json:"eyes"`
	Beard     string `toml:"beard" json:"beard"`
}

// Settings is settings.toml. Absent fields keep their defaults.
type Settings struct {
	Listen            string                  `toml:"listen" json:"listen"`
	Port              int                     `toml:"port" json:"port"`
	MaxSessions       int                     `toml:"max_sessions" json:"maxSessions"`
	Engine            string                  `toml:"engine" json:"engine"`
	StuckAfterMinutes int                     `toml:"stuck_after_minutes" json:"stuckAfterMinutes"`
	LogCapMB          int                     `toml:"log_cap_mb" json:"logCapMb"`
	Forges            map[string]ForgeAccount `toml:"forges,omitempty" json:"forges"`
	GlobalForge       GlobalForge             `toml:"global_forge" json:"globalForge"`
	ProjectsFolder    string                  `toml:"projects_folder" json:"projectsFolder"`
	Notifications     Notifications           `toml:"notifications" json:"notifications"`
	// Instructions are the CEO's standing instructions for every agent, added
	// to each session prompt (wiki/product/architecture/session-prompt.md).
	// Written as a multi-line string (encodeSettings).
	Instructions string `toml:"instructions,omitempty" json:"instructions"`
}

// ForgeAccount is a forge account's host or owner; its token is a secret.
type ForgeAccount struct {
	URL string `toml:"url" json:"url"`
}

// GlobalForge is the forge hosting the global repo (teams and settings).
type GlobalForge struct {
	Kind string `toml:"kind" json:"kind"`
	URL  string `toml:"url,omitempty" json:"url"`
}

// Notifications are the browser notification switches.
type Notifications struct {
	NeedsMe        bool `toml:"needs_me" json:"needsMe"`
	Finishes       bool `toml:"finishes" json:"finishes"`
	MergeRequested bool `toml:"merge_requested" json:"mergeRequested"`
}

// DefaultSettings are the settings while settings.toml doesn't exist.
func DefaultSettings() Settings {
	return Settings{
		Listen: "local", Port: 8888, MaxSessions: 12, Engine: "podman",
		StuckAfterMinutes: 20, LogCapMB: 50,
		ProjectsFolder: "~/workspace/foreman",
		GlobalForge:    GlobalForge{Kind: "none"},
		Notifications:  Notifications{NeedsMe: true, Finishes: true, MergeRequested: true},
	}
}

var (
	slug       = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	snakeCase  = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	forgeKinds = []string{"github", "forgejo", "gitea", "none"}
	hairs      = []string{"short", "long", "curly", "bun", "bald"}
	eyes       = []string{"dots", "glasses", "shades", "sleepy"}
	beards     = []string{"none", "stubble", "full", "mustache"}
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
// save would otherwise drop silently).
func decode(data []byte, v any) (toml.MetaData, problems) {
	md, err := toml.Decode(string(data), v)
	if err != nil {
		return md, problems{err.Error()}
	}
	var p problems
	for _, key := range md.Undecoded() {
		p.add("%s: not a field Foreman knows (misspelled, or left from an older version); remove it", key)
	}
	return md, p
}

// ParseProject parses a project.toml: the project's rules, which travel with
// its project repo (its name and repos are this machine's, in the database).
// Defaults are applied (merge_to_main = "ceo", forge none).
func ParseProject(data []byte) (ProjectConfig, []string) {
	var c ProjectConfig
	md, p := decode(data, &c)
	if md.IsDefined("main") {
		// Every project.toml before 2026-10-21 has it: say what replaced it.
		p = slices.DeleteFunc(p, func(s string) bool { return strings.HasPrefix(s, "main: ") })
		p.add("main: each code repo has its own main branch now (Project settings → Code repos); remove this line")
	}
	if c.MergeToMain == "" {
		c.MergeToMain = "ceo"
	}
	if c.Forge.Kind == "" {
		c.Forge.Kind = "none"
	}
	c.AlwaysAskCEO = normalize(c.AlwaysAskCEO)
	if strings.ContainsAny(c.Description, "\n") {
		p.add("description: one line")
	}
	if c.MergeToMain != "ceo" {
		p.add(`merge_to_main: must be "ceo"`)
	}
	if !slices.Contains(forgeKinds, c.Forge.Kind) {
		p.add("forge.kind: %q is not one of %s", c.Forge.Kind, strings.Join(forgeKinds, ", "))
	} else if c.Forge.Kind != "none" {
		checkURL(&p, "forge.url", c.Forge.URL)
		checkURL(&p, "forge.project_repo", c.Forge.ProjectRepo)
	}
	return c, p
}

func checkURL(p *problems, field, value string) {
	if value == "" {
		p.add("%s: required when a forge is set", field)
		return
	}
	if u, err := url.Parse(value); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		p.add("%s: %q is not an http(s) URL", field, value)
	}
}

// ParseTeam parses the team file of team id. Rules across teams (agent ids
// and names unique across teams) are checked when the home loads.
func ParseTeam(id string, data []byte) (TeamConfig, []string) {
	var c TeamConfig
	md, p := decode(data, &c)
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

func (c TeamConfig) validate(id string) problems {
	var p problems
	switch {
	case c.Team == "":
		p.add("team: required")
	case c.Team != id:
		p.add("team: %q must equal the file name %q", c.Team, id)
	case !slug.MatchString(c.Team):
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
	for _, aid := range sortedKeys(c.Agents) {
		p = append(p, c.Agents[aid].validate("agents."+aid, aid)...)
	}
	return p
}

func (s Soul) validate(at, id string) problems {
	var p problems
	if !slug.MatchString(id) {
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
	// The look is filled in when an agent is hired; a hand-written soul may
	// leave it out, but a value that is there must be valid.
	for field, v := range map[string]struct {
		value   string
		allowed []string
	}{"hair": {s.Look.Hair, hairs}, "eyes": {s.Look.Eyes, eyes}, "beard": {s.Look.Beard, beards}} {
		if v.value != "" && !slices.Contains(v.allowed, v.value) {
			p.add("%s.look.%s: %q is not one of %s", at, field, v.value, strings.Join(v.allowed, ", "))
		}
	}
	return p
}

// ParseSettings parses settings.toml over the defaults.
func ParseSettings(data []byte) (Settings, []string) {
	s := DefaultSettings()
	_, p := decode(data, &s)
	p = append(p, s.validate()...)
	return s, p
}

func (s Settings) validate() problems {
	var p problems
	if s.Listen != "local" && s.Listen != "lan" {
		p.add(`listen: must be "local" or "lan"`)
	}
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
	if f := s.ProjectsFolder; f != "~" && !strings.HasPrefix(f, "~/") && !filepath.IsAbs(f) {
		p.add("projects_folder: %q must be an absolute path or start with ~/", f)
	}
	for _, k := range sortedKeys(s.Forges) {
		if k == "none" || !slices.Contains(forgeKinds, k) {
			p.add("forges.%s: not a forge kind", k)
		}
	}
	if !slices.Contains(forgeKinds, s.GlobalForge.Kind) {
		p.add("global_forge.kind: %q is not one of %s", s.GlobalForge.Kind, strings.Join(forgeKinds, ", "))
	} else if s.GlobalForge.Kind != "none" {
		checkURL(&p, "global_forge.url", s.GlobalForge.URL)
	}
	return p
}

// encode renders v as TOML. A save rewrites the whole file: comments written
// by hand don't survive it; the global repo keeps the earlier versions.
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

// encodeSettings renders settings.toml, the instructions as a multi-line
// string, so their Markdown reads as written: the TOML library writes every
// string on one line, with \n escapes.
func encodeSettings(s Settings) ([]byte, error) {
	text := s.Instructions
	s.Instructions = ""
	out, err := encode(s)
	if err != nil || text == "" {
		return out, err
	}
	key := []byte("instructions = " + multiline(text) + "\n")
	// A top-level key goes before the first table, or it would be the table's.
	if i := bytes.Index(out, []byte("\n[")); i >= 0 {
		return slices.Concat(out[:i+1], key, out[i+1:]), nil
	}
	return append(out, key...), nil
}

// multiline is s as a TOML multi-line basic string: backslashes and the
// control characters TOML refuses escaped, and quotes too wherever they could
// close it.
func multiline(s string) string {
	var b strings.Builder
	b.WriteString("\"\"\"\n")
	for i, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"' && (strings.HasPrefix(s[i:], `"""`) || i == len(s)-1):
			b.WriteString(`\"`)
		case r < 0x20 && r != '\n' && r != '\t' || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(`"""`)
	return b.String()
}
