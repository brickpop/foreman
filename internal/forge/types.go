// Package forge is how Foreman talks to code forges (GitHub, Forgejo, Gitea):
// PRs, syncs and token checks (wiki/product/architecture/forge-adapters.md).
// For now it holds only the wire types; the adapters come with the Git epic.
package forge

// Account is a forge's connection, for Settings → Forge accounts (tygo.yaml).
// Its host is in the workspace settings (forges); its token is a secret, never
// sent back.
type Account struct {
	Kind     string `json:"kind"` // github, forgejo or gitea
	TokenSet bool   `json:"tokenSet"`
}
