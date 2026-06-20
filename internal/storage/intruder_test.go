package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

func TestIntruderAttackLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	attack := &models.IntruderAttack{
		ID: NewID(), Name: "login fuzz", Scheme: "https", Host: "ex.com:443",
		Template: []byte("GET /?id=§1§ HTTP/1.1\r\n\r\n"), AttackType: "sniper",
		Config: []byte(`{"payloadSets":[]}`), Total: 3,
	}
	if err := db.CreateIntruderAttack(ctx, attack); err != nil {
		t.Fatalf("CreateIntruderAttack: %v", err)
	}

	// Record three results.
	for i := 0; i < 3; i++ {
		r := &models.IntruderResult{
			ID: NewID(), AttackID: attack.ID, Index: i, Payloads: []byte(`["a"]`),
			StatusCode: 200, Length: 100 + i, DurationMs: 5,
			RequestRaw: []byte("req"), ResponseRaw: []byte("resp"),
		}
		if err := db.AddIntruderResult(ctx, r); err != nil {
			t.Fatalf("AddIntruderResult: %v", err)
		}
	}
	if err := db.UpdateIntruderProgress(ctx, attack.ID, 3); err != nil {
		t.Fatalf("UpdateIntruderProgress: %v", err)
	}
	if err := db.SetIntruderStatus(ctx, attack.ID, "completed"); err != nil {
		t.Fatalf("SetIntruderStatus: %v", err)
	}

	got, err := db.GetIntruderAttack(ctx, attack.ID)
	if err != nil {
		t.Fatalf("GetIntruderAttack: %v", err)
	}
	if got.Status != "completed" || got.Completed != 3 || string(got.Template) == "" {
		t.Errorf("attack state = %+v", got)
	}

	results, err := db.ListIntruderResults(ctx, attack.ID)
	if err != nil {
		t.Fatalf("ListIntruderResults: %v", err)
	}
	if len(results) != 3 || results[0].Index != 0 || results[2].Index != 2 {
		t.Fatalf("results = %d, want 3 ordered by index", len(results))
	}
	// List omits raw bytes; the single-result fetch includes them.
	full, err := db.GetIntruderResult(ctx, results[0].ID)
	if err != nil {
		t.Fatalf("GetIntruderResult: %v", err)
	}
	if string(full.ResponseRaw) != "resp" {
		t.Errorf("raw response = %q, want resp", full.ResponseRaw)
	}

	list, err := db.ListIntruderAttacks(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListIntruderAttacks = %d (err %v), want 1", len(list), err)
	}

	// Deleting the attack cascades to its results.
	if err := db.DeleteIntruderAttack(ctx, attack.ID); err != nil {
		t.Fatalf("DeleteIntruderAttack: %v", err)
	}
	if results, _ := db.ListIntruderResults(ctx, attack.ID); len(results) != 0 {
		t.Errorf("results not cascade-deleted: %d remain", len(results))
	}
}

func TestReconcileRunningAttacksOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reconcile.db")
	ctx := context.Background()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	attack := &models.IntruderAttack{
		ID: NewID(), Name: "orphan", Scheme: "http", Host: "h", AttackType: "sniper",
		Config: []byte("{}"), Status: "running",
	}
	if err := db.CreateIntruderAttack(ctx, attack); err != nil {
		t.Fatalf("CreateIntruderAttack: %v", err)
	}
	_ = db.Close()

	// Reopening simulates a restart after a crash; the orphaned 'running' row
	// must be reconciled to 'stopped' so it is not perpetually in-flight.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	got, err := db2.GetIntruderAttack(ctx, attack.ID)
	if err != nil {
		t.Fatalf("GetIntruderAttack: %v", err)
	}
	if got.Status != "stopped" {
		t.Errorf("status after reopen = %q, want stopped", got.Status)
	}
}
