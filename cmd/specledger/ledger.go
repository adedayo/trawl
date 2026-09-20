package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ledger is one change folder and what its checklist says.
type ledger struct {
	ID       string
	Archived bool
	Open     int
	Done     int
}

// notArchivable lists changes that legitimately stay under changes/ with no
// open boxes. The reason is recorded beside each, because "it is an exception"
// is not a reason, and an unexplained exemption is how a guard becomes advice.
var notArchivable = map[string]string{
	"001-initial-build": "holds the live capability specs, so it is not archivable when its ledger closes",
}

// boxes counts the checklist items in a ledger.
//
// Ticked and unticked are counted separately because they mean different
// things here: a tick records a conclusion, which may be that the work landed
// or that a decision not to do it was taken in place. An open box is the only
// thing that means outstanding work.
func boxes(content string) (open, done int) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		switch {
		case strings.HasPrefix(trimmed, "- [ ]"):
			open++
		case strings.HasPrefix(trimmed, "- [x]"), strings.HasPrefix(trimmed, "- [X]"):
			done++
		}
	}
	return open, done
}

// scan reads every change folder under root and reports what is structurally
// wrong with each. Structural problems are returned rather than aggregated
// into an error so that one broken change does not hide the next.
func scan(root string) ([]ledger, []string, error) {
	active, activeProblems, err := scanDir(filepath.Join(root, "changes"), false)
	if err != nil {
		return nil, nil, err
	}
	archived, archivedProblems, err := scanDir(filepath.Join(root, "changes", "archive"), true)
	if err != nil {
		return nil, nil, err
	}

	all := append(active, archived...)
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	return all, append(activeProblems, archivedProblems...), nil
}

func scanDir(dir string, archived bool) ([]ledger, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	var ledgers []ledger
	var problems []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		id := e.Name()
		where := filepath.Join(dir, id)

		tasks := filepath.Join(where, "tasks.md")
		content, err := os.ReadFile(tasks)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s has no readable tasks.md: %v", where, err))
			continue
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			// The failure mode that prompted this command. An empty ledger is
			// worse than a missing one: it reads as a change with nothing
			// outstanding, and the proposal beside it goes on citing phases
			// that are no longer written down anywhere.
			problems = append(problems, fmt.Sprintf("%s is empty; a ledger asserting nothing reads as a change with nothing left to do", tasks))
			continue
		}

		open, done := boxes(string(content))
		if open+done == 0 && !archived {
			// Archived ledgers are allowed to be prose. 002 was superseded and
			// its ledger was rewritten as an explanation of why not to build
			// it, which is more use than nine ticked boxes. An *active* change
			// with no checklist is a different thing: it is a proposal that
			// never became a plan.
			problems = append(problems, fmt.Sprintf("%s contains no checklist items, so nothing about it can be outstanding or done", tasks))
		}
		if _, err := os.Stat(filepath.Join(where, "proposal.md")); err != nil {
			problems = append(problems, fmt.Sprintf("%s has no proposal.md, so nothing records why it exists", where))
		}

		ledgers = append(ledgers, ledger{ID: id, Archived: archived, Open: open, Done: done})
	}
	return ledgers, problems, nil
}

// validate reports the disagreements between a change's location and its
// contents. Each of these is a claim about state that the tree is making
// implicitly, and each can be wrong.
func validate(ledgers []ledger) []string {
	var problems []string

	seen := map[string]bool{}
	for _, l := range ledgers {
		if seen[l.ID] {
			// Archiving is `git mv`. An editor holding an unsaved buffer on a
			// moved file writes it back to the old path and recreates the
			// directory, so the change exists in both places and the active
			// tree quietly regrows a change that was closed.
			problems = append(problems, fmt.Sprintf(
				"%s exists under both changes/ and changes/archive/; archiving is `git mv`, and an editor with an unsaved buffer will recreate the source", l.ID))
			continue
		}
		seen[l.ID] = true
	}

	for _, l := range ledgers {
		switch {
		case l.Archived && l.Open > 0:
			problems = append(problems, fmt.Sprintf(
				"%s is archived with %d open item(s); an archived change reads as settled truth, so either finish them or tick them with the reasoning", l.ID, l.Open))
		case !l.Archived && l.Open == 0:
			if _, exempt := notArchivable[l.ID]; !exempt {
				problems = append(problems, fmt.Sprintf(
					"%s has no open items but is not archived; a closed ledger left in changes/ makes the active set read as larger than it is", l.ID))
			}
		}
	}

	sort.Strings(problems)
	return problems
}

// A STATUS.md row naming a change, followed by cells the command maintains.
// Anchored on the backticked identifier because that is how every table in the
// document names a change, and unanchored matching would rewrite prose.
var statusRow = regexp.MustCompile("^(\\|\\s*`([0-9]{3}-[a-z0-9-]+)`\\s*\\|)(.*)$")

// reconcile rewrites the numeric cells of STATUS.md from the ledgers, and
// reports any change the document and the tree disagree about the existence
// of.
//
// Only counts are rewritten. The prose in each row is the part a human wrote
// and is the part worth keeping; a tool that regenerated the whole table would
// destroy the reasoning in order to fix two integers.
func reconcile(status string, ledgers []ledger) (string, []string) {
	byID := map[string]ledger{}
	for _, l := range ledgers {
		byID[l.ID] = l
	}

	mentioned := map[string]bool{}
	lines := strings.Split(status, "\n")
	for i, line := range lines {
		m := statusRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := m[2]
		mentioned[id] = true
		l, known := byID[id]
		if !known {
			continue
		}
		lines[i] = m[1] + rewriteCounts(m[3], l)
	}

	var problems []string
	for _, l := range ledgers {
		if !mentioned[l.ID] {
			problems = append(problems, fmt.Sprintf(
				"%s is not named in STATUS.md; a change nobody indexed is invisible, and an invisible gap is indistinguishable from a decision not to build something", l.ID))
		}
	}
	for id := range mentioned {
		if _, known := byID[id]; !known {
			problems = append(problems, fmt.Sprintf("STATUS.md names %s, which has no change folder", id))
		}
	}
	sort.Strings(problems)

	return strings.Join(lines, "\n"), problems
}

// rewriteCounts replaces the leading numeric cells of a row's remainder.
//
// The tables are not uniform: "substantially delivered" carries open and done,
// "not started" carries only open, and the archived table carries neither.
// Rewriting whatever numeric cells are actually present, in order, keeps the
// three shapes working without the command needing to know which table it is
// in — which it could only learn by parsing headings, and would then get wrong
// the first time a heading was reworded.
func rewriteCounts(rest string, l ledger) string {
	cells := strings.Split(rest, "|")
	values := []int{l.Open, l.Done}
	next := 0
	for i, cell := range cells {
		if next >= len(values) {
			break
		}
		if _, err := strconv.Atoi(strings.TrimSpace(cell)); err != nil {
			break
		}
		cells[i] = padLike(cell, strconv.Itoa(values[next]))
		next++
	}
	return strings.Join(cells, "|")
}

// padLike keeps a rewritten cell's surrounding whitespace, so that a count
// changing from 9 to 10 does not reflow a table and bury the change in a diff
// of alignment.
func padLike(cell, value string) string {
	trimmed := strings.TrimSpace(cell)
	if trimmed == "" {
		return cell
	}
	idx := strings.Index(cell, trimmed)
	lead, tail := cell[:idx], cell[idx+len(trimmed):]
	if d := len(trimmed) - len(value); d > 0 {
		lead += strings.Repeat(" ", d)
	}
	// A count that grows a digit widens its column by one character. That is
	// left alone deliberately: eating the leading space instead would make the
	// cell touch the pipe, and markdown tables need not align anyway.
	return lead + value + tail
}
