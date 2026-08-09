// Package functest holds cross-domain functional tests that run every service against a real
// SQLite file, wired exactly the way cmd/backend/main.go wires it.
//
// The suite was written to make the GORM -> sqlc migration verifiable and is kept as the
// storage regression suite: it pins the observable behaviour of the business logic, the
// physical schema the current code produces, and the ability to read a database written by the
// pre-migration code (testdata/legacy, which is frozen input — see legacy_test.go).
//
// The package itself is intentionally empty; everything lives in _test.go files.
package functest
