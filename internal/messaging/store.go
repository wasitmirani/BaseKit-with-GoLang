package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the engine's view of the NestJS database: delivery_jobs, plus
// read-only lookups of channel accounts, campaigns and workspaces in
// product_records / tenants. It connects as the table owner, so it works across
// workspaces (row-level security applies to NestJS's per-request role only).
type Store struct {
	pool *pgxpool.Pool
	box  *SecretBox
}

func NewStore(pool *pgxpool.Pool, box *SecretBox) *Store { return &Store{pool: pool, box: box} }

// A job held longer than this belongs to a worker that died and is taken again
// (at-least-once: the provider may see that message twice).
const staleLease = 10 * time.Minute

// Claim leases up to n due jobs of active workspaces.
func (s *Store) Claim(ctx context.Context, worker string, n int) ([]Job, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE delivery_jobs j
		   SET status = 'sending', locked_at = now(), locked_by = $1, attempts = j.attempts + 1, updated_at = now()
		  FROM (
		    SELECT id FROM delivery_jobs
		     WHERE ((status = 'pending' AND next_attempt_at <= now())
		         OR (status = 'sending' AND locked_at < now() - make_interval(secs => $3)))
		       -- Suspended or sending-paused workspaces keep their jobs; they resume afterwards.
		       AND tenant_id IN (SELECT id FROM tenants WHERE status = 'active' AND sending_paused_at IS NULL)
		     ORDER BY next_attempt_at
		     LIMIT $2
		     FOR UPDATE SKIP LOCKED
		  ) due
		 WHERE j.id = due.id
		RETURNING j.id, j.tenant_id, j.message_id, j.campaign_id, j.account_id, j.channel, j.kind, j.payload, j.attempts`,
		worker, n, staleLease.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.TenantID, &j.MessageID, &j.CampaignID, &j.AccountID, &j.Channel, &j.Kind, &j.Payload, &j.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// Account loads and decrypts a channel account of the job's workspace.
func (s *Store) Account(ctx context.Context, tenantID, accountID string) (*Account, error) {
	var provider, status string
	var sealed *string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(data->>'provider', ''), COALESCE(data->>'status', ''), data->>'_credentials'
		  FROM product_records
		 WHERE tenant_id = $1 AND collection = 'channel-accounts' AND id = $2`,
		tenantID, accountID).Scan(&provider, &status, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, Fail("PROVIDER_NOT_CONNECTED", "The channel account was removed; reconnect it in Channels.", true)
	}
	if err != nil {
		return nil, err
	}
	key := providerKey(provider)
	if key == "" || sealed == nil {
		return nil, Fail("PROVIDER_NOT_CONNECTED", "The channel account has no usable credentials.", true)
	}
	if status != "connected" {
		return nil, Fail("PROVIDER_NOT_CONNECTED", fmt.Sprintf("%s is %s; reconnect it in Channels.", provider, status), true)
	}
	creds := Credentials{}
	if err := s.box.Open(*sealed, &creds); err != nil {
		return nil, Fail("CREDENTIALS_UNREADABLE", err.Error(), true)
	}
	return &Account{ID: accountID, Provider: key, Label: provider, Credentials: creds}, nil
}

// CampaignStopped reports whether the campaign was paused or cancelled after the job was queued.
func (s *Store) CampaignStopped(ctx context.Context, tenantID, campaignID string) (string, error) {
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(data->>'status', '') FROM product_records
		 WHERE tenant_id = $1 AND collection = 'campaigns' AND id = $2`,
		tenantID, campaignID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "deleted", nil
	}
	if err != nil {
		return "", err
	}
	if status == "paused" || status == "cancelled" {
		return status, nil
	}
	return "", nil
}

func historyJSON(e HistoryEntry) []byte {
	b, _ := json.Marshal([]HistoryEntry{e})
	return b
}

// Sent records success and tells NestJS (it also polls).
func (s *Store) Sent(ctx context.Context, job Job, worker string, r Result, at time.Time) error {
	res, _ := json.Marshal(r)
	tag, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs SET status = 'sent', result = $2, finished_at = $4, locked_at = NULL, updated_at = now()
		 WHERE id = $1 AND locked_by = $3 AND status = 'sending'`,
		job.ID, res, worker, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errLeaseLost
	}
	_, _ = s.pool.Exec(ctx, `SELECT pg_notify('delivery_results', $1)`, job.ID)
	return nil
}

// Failed records a final failure.
func (s *Store) Failed(ctx context.Context, job Job, worker string, f *Failure, at time.Time) error {
	res, _ := json.Marshal(f)
	tag, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs SET status = 'failed', result = $2, finished_at = $4, locked_at = NULL, updated_at = now()
		 WHERE id = $1 AND locked_by = $3 AND status = 'sending'`,
		job.ID, res, worker, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errLeaseLost
	}
	_, _ = s.pool.Exec(ctx, `SELECT pg_notify('delivery_results', $1)`, job.ID)
	return nil
}

// Retry puts the job back with a delay and notes why.
func (s *Store) Retry(ctx context.Context, job Job, worker string, f *Failure, next time.Time) error {
	res, _ := json.Marshal(f)
	entry := HistoryEntry{
		Status: "retrying", At: time.Now().UTC(),
		Detail: fmt.Sprintf("%s — retrying at %s (attempt %d of %d)", f.Message, next.UTC().Format(time.RFC3339), job.Attempts, MaxAttempts),
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs
		   SET status = 'pending', result = $2, next_attempt_at = $4, locked_at = NULL, locked_by = NULL,
		       history = history || $5::jsonb, updated_at = now()
		 WHERE id = $1 AND locked_by = $3 AND status = 'sending'`,
		job.ID, res, worker, next, historyJSON(entry))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errLeaseLost
	}
	return nil
}

// Release hands a job back untouched (rate limited or shutting down), without using up an attempt.
func (s *Store) Release(ctx context.Context, job Job, worker string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs SET status = 'pending', attempts = GREATEST(attempts - 1, 0), next_attempt_at = $3,
		       locked_at = NULL, locked_by = NULL, updated_at = now()
		 WHERE id = $1 AND locked_by = $2 AND status = 'sending'`,
		job.ID, worker, at)
	return err
}

// Stats are queue counts for /metrics.
func (s *Store) Stats(ctx context.Context) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT status, count(*) FROM delivery_jobs
		 WHERE status IN ('pending', 'sending') OR (applied_at IS NULL AND status IN ('sent', 'failed'))
		 GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{"pending": 0, "sending": 0, "sent": 0, "failed": 0}
	for rows.Next() {
		var st string
		var n int64
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

var errLeaseLost = errors.New("lease lost: another worker took the job")
