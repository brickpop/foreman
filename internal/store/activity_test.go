package store

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// activityFixture: orion on ledger (stopped), then atlas on ledger and on
// relay, with messages and actions at the given times (stored text, as Add
// writes it).
func activityFixture(t *testing.T, rows ...[3]string) *DB {
	t.Helper()
	db := open(t)
	for _, d := range []string{"orion@ledger", "atlas@ledger", "atlas@relay"} {
		team, project, _ := strings.Cut(d, "@")
		if _, err := db.Deployments().Deploy(ctx, team, project); err != nil {
			t.Fatal(err)
		}
		if d == "orion@ledger" { // a project has one team at a time
			if err := db.Deployments().Stop(ctx, d, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, r := range rows { // {deployment, "m" or "a", created_at}
		var err error
		if r[1] == "m" {
			_, err = db.sql.Exec(`INSERT INTO team_messages (deployment_id, ref, from_instance, to_instance, kind, body, created_at)
				VALUES (?, ?, 'mara', 'tove', 'ask', ?, ?)`, r[0], fmt.Sprintf("msg-%d", i), fmt.Sprintf("#%d", i), r[2])
		} else {
			_, err = db.sql.Exec(`INSERT INTO team_actions (deployment_id, instance, tool, branch, repos, created_at)
				VALUES (?, 'mara', 'switch', ?, '[]', ?)`, r[0], fmt.Sprintf("#%d", i), r[2])
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// labels names a page's items by their text (#i, the row they were made from).
func labels(p ActivityPage) []string {
	var out []string
	for _, it := range p.Items {
		if it.Message != nil {
			out = append(out, "m"+it.Message.Body)
		} else {
			out = append(out, "a"+it.Action.Branch)
		}
	}
	return out
}

// Newest first across both tables, of the deployment asked for only, by real
// time: RFC 3339 with trimmed nanoseconds doesn't sort as text (".5Z" after
// ".25Z", "05Z" after "05.5Z").
func TestActivityIsNewestFirstAcrossTables(t *testing.T) {
	db := activityFixture(t,
		[3]string{"atlas@ledger", "m", "2026-10-17T10:00:05Z"},        // #0
		[3]string{"atlas@ledger", "a", "2026-10-17T10:00:05.5Z"},      // #1
		[3]string{"atlas@ledger", "a", "2026-10-17T10:00:05.25Z"},     // #2
		[3]string{"atlas@relay", "m", "2026-10-17T10:00:09Z"},         // #3: another project
		[3]string{"orion@ledger", "m", "2026-10-17T10:00:08Z"},        // #4: another team, before
		[3]string{"atlas@ledger", "m", "2026-10-17T09:59:59.999999Z"}, // #5
	)
	p, err := db.Activity().Page(ctx, "atlas@ledger", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := labels(p), []string{"a#1", "a#2", "m#0", "m#5"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
	if p.Next != "" {
		t.Errorf("a next page after everything: %q", p.Next)
	}
	if p.Items[0].Action.DeploymentID != "atlas@ledger" || p.Items[2].Message.Ref != "msg-0" {
		t.Errorf("rows not read whole: %+v %+v", p.Items[0].Action, p.Items[2].Message)
	}
}

// Pages follow each other with no gap and no repeat, also through items at
// the same instant in both tables.
func TestActivityPages(t *testing.T) {
	var rows [][3]string
	for i := range 7 {
		table := []string{"m", "a"}[i%2]
		rows = append(rows, [3]string{"atlas@ledger", table, "2026-10-17T10:00:00Z"}) // all at once
	}
	db := activityFixture(t, rows...)
	var seen []string
	before := ""
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("never ended")
		}
		p, err := db.Activity().Page(ctx, "atlas@ledger", before, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Items) > 3 {
			t.Fatalf("%d items in a page of 3", len(p.Items))
		}
		seen = append(seen, labels(p)...)
		if p.Next == "" {
			break
		}
		before = p.Next
	}
	if len(seen) != 7 {
		t.Fatalf("saw %v", seen)
	}
	sorted := slices.Clone(seen)
	slices.Sort(sorted)
	if len(slices.Compact(sorted)) != 7 {
		t.Errorf("repeats: %v", seen)
	}
}

func TestActivityRefusals(t *testing.T) {
	db := activityFixture(t, [3]string{"atlas@ledger", "m", "2026-10-17T10:00:00Z"})
	for _, c := range []string{"x", "1:z:2", "a:m:2", "1:m:b", "1:m"} {
		if _, err := db.Activity().Page(ctx, "atlas@ledger", c, 50); !errors.Is(err, ErrBadCursor) {
			t.Errorf("cursor %q: %v", c, err)
		}
	}
	if p, err := db.Activity().Page(ctx, "nobody@nowhere", "", 50); err != nil || len(p.Items) != 0 {
		t.Errorf("an unknown deployment: %+v %v", p, err)
	}
}
