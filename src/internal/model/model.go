// Package model defines the core data types that the git log parser produces
// and every analysis consumes: a flat sequence of Modification records (one per
// commit/file pair) plus the run Options that shape parsing and analysis.
//
// These are pure data types with no behavior. Output shaping (snake_case JSON
// keys, CSV headers) is the concern of the analysis and output layers, so the
// structs deliberately carry no serialization tags.
package model

// Modification is a single (commit, file) change record: the uniform shape the
// parser reduces every log entry to and that all analyses read.
type Modification struct {
	// Entity is the changed file path (the module under analysis).
	Entity string
	// Rev is the commit short hash identifying the logical change set.
	Rev string
	// Date is the commit date in canonical YYYY-MM-dd form.
	Date string
	// Author is the committer name; may be remapped to a team downstream.
	Author string
	// Message is the commit subject; "-" when absent (stock 3-field log).
	Message string
	// LocAdded is the number of lines added; 0 for binary files.
	LocAdded int
	// LocDeleted is the number of lines deleted; 0 for binary files.
	LocDeleted int
	// Binary reports whether git recorded "-"/"-" numstat for a binary file.
	Binary bool
	// HasLoc reports whether numstat data was present. It guards the churn and
	// ownership analyses and is always true for the git2 format.
	HasLoc bool
}

// Options carries run-time options shared across parsing and analysis. Later
// tickets extend it with additional fields; today it holds only the input
// encoding.
type Options struct {
	// InputEncoding is the log's character encoding; the empty value means the
	// UTF-8 default.
	InputEncoding string
}
