package workflow

import (
	"context"
	"errors"
	"log/slog"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/store"
)

// The pipeline: work items on their way to the lead branch, then merge
// requests on their way to main (wiki/product/features/pipeline-and-merges.md).
// Work items move on hand-offs, deliveries and integration
// (wiki/product/architecture/protocol.md#work-items-and-merge-requests).

// Pipeline is a project's work items and merge requests, for Changes.
type Pipeline struct {
	WorkItems     []store.WorkItem     `json:"workItems"`
	MergeRequests []store.MergeRequest `json:"mergeRequests"`
}

// Pipeline lists a project's work items and merge requests.
func (w *Workflow) Pipeline(ctx context.Context, project string) (Pipeline, error) {
	items, err := w.WorkItems.ForProject(ctx, project)
	if err != nil {
		return Pipeline{}, err
	}
	mrs, err := w.MergeRequests.ForProject(ctx, project)
	return Pipeline{WorkItems: items, MergeRequests: mrs}, err
}

// Delivered moves a hand-off's work item to "lead reviewing" once Messaging
// has typed it into the lead's session.
func (w *Workflow) Delivered(ctx context.Context, msg store.TeamMessage) {
	if msg.Kind != "handoff" || msg.State != store.MessageDelivered || msg.ArtifactRef == "" {
		return
	}
	item, err := w.WorkItems.Find(ctx, msg.DeploymentID, msg.ArtifactRef)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("a hand-off's work item", "deployment", msg.DeploymentID, "branch", msg.ArtifactRef, "err", err)
		}
		return
	}
	if item.Stage == store.StageHanded {
		item.Stage = store.StageReviewing
		w.save(ctx, item)
	}
}

// handed records a hand-off of branch: its work item, new or back at
// "handed to lead", whatever stage it was at (a change request, a plain ask,
// moves nothing; the next hand-off does).
func (t tool) handed(ctx context.Context, branch string) {
	item, err := t.w.WorkItems.Find(ctx, t.c.Deployment.ID, branch)
	switch {
	case errors.Is(err, store.ErrNotFound):
		item = store.WorkItem{DeploymentID: t.c.Deployment.ID, AuthorInstance: t.c.Instance.ID, Branch: branch}
	case err != nil:
		slog.Warn("a hand-off's work item", "deployment", t.c.Deployment.ID, "branch", branch, "err", err)
		return
	}
	item.Stage, item.Note = store.StageHanded, ""
	t.w.save(ctx, item)
}

// integrate marks "integrated" every handed work item whose branch head one
// of the lead's lead/<team>/* branches contains, in every repo that has the
// branch. Called after the lead's merge and commit.
func (t tool) integrate(ctx context.Context) {
	items, err := t.w.WorkItems.ForDeployment(ctx, t.c.Deployment.ID)
	if err != nil {
		slog.Warn("work items", "deployment", t.c.Deployment.ID, "err", err)
		return
	}
	for _, item := range items {
		if item.Stage != store.StageHanded && item.Stage != store.StageReviewing {
			continue
		}
		if in, err := t.contained(ctx, item.Branch); err != nil {
			slog.Warn("integration", "branch", item.Branch, "err", err)
		} else if in {
			item.Stage = store.StageIntegrated
			t.w.save(ctx, item)
		}
	}
}

// contained reports whether a lead branch contains branch in every repo that
// has it (and there is at least one).
func (t tool) contained(ctx context.Context, branch string) (bool, error) {
	prefix := "lead/" + t.c.Team.ID + "/"
	found := false
	for _, r := range t.worktrees() {
		if !t.w.Git.HasBranch(ctx, r.dir, branch) {
			continue
		}
		found = true
		leads, err := t.w.Git.Branches(ctx, r.dir, prefix)
		if err != nil {
			return false, err
		}
		in := false
		for _, lead := range leads {
			if in, err = t.w.Git.Contains(ctx, r.dir, lead, branch); err != nil {
				return false, err
			} else if in {
				break
			}
		}
		if !in {
			return false, nil
		}
	}
	return found, nil
}

// save keeps a work item and announces it.
func (w *Workflow) save(ctx context.Context, item store.WorkItem) {
	saved, err := w.WorkItems.Save(ctx, item)
	if err != nil {
		slog.Warn("keeping a work item", "deployment", item.DeploymentID, "branch", item.Branch, "err", err)
		return
	}
	if w.Publish != nil {
		w.Publish(events.Event{Type: "pipeline.changed", Data: saved})
	}
}
