// Package churn holds the shared aggregation helpers for the churn family of
// analyses (absolute-churn, author-churn, entity-churn, entity-ownership, and
// the main-developer analyses). Every churn analysis needs loc metrics, so the
// package centralizes the metrics guard, the group-sum reduction, and the
// per-(entity, author) contribution roll-up, keeping the numeric behavior
// (binary rows count as zero loc; commits are distinct revisions) in one place.
package churn

import (
	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// ErrMissingMetrics reports that the log carries no modification metrics, so a
// churn analysis has nothing to sum. The git2 log always includes numstat; this
// guards a message-only log (exit code 3, input error).
var ErrMissingMetrics = terr.New(
	"missing_metrics",
	3,
	"generate the log with --numstat (see `codelens print-log-command`)",
	"the VCS data has no modification metrics",
)

// RequireLoc returns ErrMissingMetrics when mods contains modifications but
// none of them carry loc data. An empty slice is not a metrics error: absence
// of data is handled upstream (empty_log), not here.
func RequireLoc(mods []model.Modification) error {
	for _, m := range mods {
		if m.HasLoc {
			return nil
		}
	}
	if len(mods) == 0 {
		return nil
	}
	return ErrMissingMetrics
}

// GroupChurn is the churn of one group (a date, author, or entity): lines added
// and deleted summed over the group, and the number of distinct revisions that
// touched it.
type GroupChurn struct {
	Group   string
	Added   int
	Deleted int
	Commits int
}

// SumByGroup partitions mods by key and, per group, sums added and deleted
// lines and counts the distinct revisions. Binary rows carry zero loc (the
// parser normalizes "-"/"-" to 0) so they add nothing to the loc sums while
// still counting toward the commit total. Groups come back in ascending key
// order for deterministic downstream sorting and truncation.
func SumByGroup(mods []model.Modification, key func(model.Modification) string) []GroupChurn {
	groups := calc.GroupBy(mods, key)
	out := make([]GroupChurn, 0, len(groups))
	for _, g := range groups {
		var added, deleted int
		revs := make([]string, 0, len(g.Items))
		for _, m := range g.Items {
			added += m.LocAdded
			deleted += m.LocDeleted
			revs = append(revs, m.Rev)
		}
		out = append(out, GroupChurn{
			Group:   g.Key,
			Added:   added,
			Deleted: deleted,
			Commits: len(calc.Distinct(revs)),
		})
	}
	return out
}

// AuthorContrib is one author's contribution to an entity: the lines they added
// and deleted, summed across their revisions of that entity.
type AuthorContrib struct {
	Author  string
	Added   int
	Deleted int
}

// EntityContribs is an entity paired with the per-author contributions to it.
type EntityContribs struct {
	Entity   string
	Contribs []AuthorContrib
}

// ByEntityAuthorContrib groups mods by entity and, within each entity, by
// author, summing added and deleted lines per (entity, author). Both the entity
// and the author levels are returned in ascending key order, so ownership-style
// analyses built on top (entity-ownership, main-developer) produce stable
// output and can break max-contributor ties by ascending author.
func ByEntityAuthorContrib(mods []model.Modification) []EntityContribs {
	entities := calc.GroupBy(mods, func(m model.Modification) string { return m.Entity })
	out := make([]EntityContribs, 0, len(entities))
	for _, e := range entities {
		authors := calc.GroupBy(e.Items, func(m model.Modification) string { return m.Author })
		contribs := make([]AuthorContrib, 0, len(authors))
		for _, a := range authors {
			var added, deleted int
			for _, m := range a.Items {
				added += m.LocAdded
				deleted += m.LocDeleted
			}
			contribs = append(contribs, AuthorContrib{Author: a.Key, Added: added, Deleted: deleted})
		}
		out = append(out, EntityContribs{Entity: e.Key, Contribs: contribs})
	}
	return out
}
