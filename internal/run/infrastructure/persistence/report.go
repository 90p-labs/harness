package persistence

import (
	"context"
	"time"

	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	selectTransitions = "SELECT run_id, at, from_phase, event, to_phase, reason, rework_class FROM run_transitions WHERE run_id = ? ORDER BY at, id"
	selectRepeatClass = "SELECT t.rework_class, count(*) AS n FROM run_transitions t JOIN runs r ON r.id = t.run_id " +
		"WHERE r.repo = ? AND t.event IN (?, ?) AND t.rework_class <> '' GROUP BY t.rework_class HAVING count(*) >= ? ORDER BY n DESC, t.rework_class"

	minRepeatCount = 2
)

func (r SQLRunRepository) Transitions(ctx context.Context, runID string) ([]domain.Transitioned, error) {
	rows, err := r.db.QueryContext(ctx, selectTransitions, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var found []domain.Transitioned
	for rows.Next() {
		var row domain.Transitioned
		var at time.Time
		if err := rows.Scan(&row.RunID, &at, &row.From, &row.Event, &row.To, &row.Reason, &row.Class); err != nil {
			return nil, err
		}
		row.At = at.UTC()
		found = append(found, row)
	}
	return found, rows.Err()
}

func (r SQLRunRepository) RepeatClasses(ctx context.Context, repo string) ([]domain.ClassCount, error) {
	rows, err := r.db.QueryContext(ctx, selectRepeatClass, repo, string(domain.EventRequestRework), string(domain.EventReworkMinor), minRepeatCount)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var counts []domain.ClassCount
	for rows.Next() {
		var count domain.ClassCount
		if err := rows.Scan(&count.Class, &count.Count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}
	return counts, rows.Err()
}
