package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/effort"
	"github.com/andreswebs/codelens/internal/model"
)

// communicationRow is one output row of the communication analysis: a directed
// author->peer pair with Shared (entities both touched), Average (the ceil of
// their mean touched-entity counts), and Strength (Shared as a percentage of
// Average). Both directions of a pair are emitted, so a symmetric pair yields
// two rows differing only in which author is the subject.
type communicationRow struct {
	Author   string `json:"author"`
	Peer     string `json:"peer"`
	Shared   int    `json:"shared"`
	Average  int    `json:"average"`
	Strength int    `json:"strength"`
}

func init() {
	Register(communicationDescriptor())
}

// communicationDescriptor is the registered contract for the communication
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func communicationDescriptor() Descriptor {
	return Descriptor{
		Name:    "communication",
		Summary: "Heuristic communication strength between author pairs",
		RowSchema: []Column{
			{Name: "author", Type: "string", Desc: "subject author of the pair"},
			{Name: "peer", Type: "string", Desc: "co-working author"},
			{Name: "shared", Type: "int", Desc: "entities both authors have worked on"},
			{Name: "average", Type: "int", Desc: "ceil of the mean entity count of the two authors"},
			{Name: "strength", Type: "int", Desc: "shared as a percentage of average, 0-100"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runCommunication,
	}
}

// authorPair is a directed pair of authors used as a frequency-map key. A
// self-pair (Me == Peer) carries an author's total touched-entity count.
type authorPair struct {
	Me   string
	Peer string
}

// runCommunication scores how strongly each pair of authors co-works, porting
// code-maat's communication analysis. For every entity it takes the distinct
// authors and forms all ordered pairs with replacement (selections), then
// counts how many entities each pair co-occurs in. A self-pair (a, a) therefore
// counts the entities author a touched. For each distinct directed pair of
// different authors, average is the ceil of the mean of the two authors'
// touched-entity counts and strength is shared as a percentage of that average
// (truncated). Rows sort by strength, then author, then peer, all descending,
// matching the original's reverse sort on [strength author] with a deterministic
// peer tie-break.
func runCommunication(mods []model.Modification, _ Opts) (any, error) {
	freq := make(map[authorPair]int)
	for _, e := range effort.ByEntity(mods) {
		authors := make([]string, len(e.Authors))
		for i, a := range e.Authors {
			authors[i] = a.Author
		}
		for _, me := range authors {
			for _, peer := range authors {
				freq[authorPair{Me: me, Peer: peer}]++
			}
		}
	}

	rows := make([]communicationRow, 0, len(freq))
	for pair, shared := range freq {
		if pair.Me == pair.Peer {
			continue
		}
		myCount := freq[authorPair{Me: pair.Me, Peer: pair.Me}]
		peerCount := freq[authorPair{Me: pair.Peer, Peer: pair.Peer}]
		average := calc.Ceil(calc.Average(myCount, peerCount))
		strength := calc.TruncInt(calc.Percentage(float64(shared) / float64(average)))
		rows = append(rows, communicationRow{
			Author:   pair.Me,
			Peer:     pair.Peer,
			Shared:   shared,
			Average:  average,
			Strength: strength,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Strength != rows[j].Strength {
			return rows[i].Strength > rows[j].Strength
		}
		if rows[i].Author != rows[j].Author {
			return rows[i].Author > rows[j].Author
		}
		return rows[i].Peer > rows[j].Peer
	})

	return rows, nil
}
