package nvrsnapshot

import "context"

type Repository interface {
	ListCandidates(ctx context.Context, selection Selection) ([]Candidate, error)
	UpsertSnapshot(ctx context.Context, snapshot Snapshot) error
}
