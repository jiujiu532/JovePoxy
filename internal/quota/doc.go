// Package quota owns OpenCode control-plane quota capabilities:
//
//   - accounts.go / account_types.go: encrypted account credential CRUD
//   - parse.go: dashboard HTML window parsing (pure)
//   - scrape.go: concurrent dashboard fetch
//   - workspace.go: workspace connectivity probe
//
// Chat hot path must never import this package for routing decisions.
package quota
