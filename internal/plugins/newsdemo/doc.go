// Package newsplugin provides the shared runtime and data-model helpers used by
// the built-in modular news sample plugins.
//
// It is no longer exposed as a standalone built-in plugin. The runnable
// built-in sample surface is now composed from:
//   - news-demo-content
//   - news-demo-governance
//   - news-demo-archive
//   - news-demo-ops
//
// This package now mainly owns:
//   - shared App/runtime wiring
//   - shared data models and indexing
//   - shared governance/archive/ops helper logic
//   - runtime path and sync support
package newsplugin
