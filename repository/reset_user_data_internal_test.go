package repository

import "testing"

// Reset promises every row. A table that quietly opts out of that is
// how a mock becomes nondeterministic between scenarios.
//
// Internal test because the leak is only observable below the public
// API: Reset deletes the servers, so every route that could read
// user_data 404s on the missing server whether or not the rows
// survived. An external test asserting "GET 404s after reset" passes
// either way -- it did, and a mutation check caught that it proved
// nothing.
//
// The rows really do orphan rather than cascade: Reset deletes with
// foreign-key checks disabled, so ON DELETE CASCADE never fires.
func TestResetClearsServerUserData(t *testing.T) {
	repo, err := New(":memory:")
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	server, err := repo.CreateServer("fr-par-1", map[string]any{
		"name": "ud-reset", "commercial_type": "DEV1-S", "image": "ubuntu_jammy",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	serverID, _ := server["id"].(string)
	if err := repo.SetServerUserData(serverID, "cloud-init", "#cloud-config\n"); err != nil {
		t.Fatalf("set user data: %v", err)
	}

	var before int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM instance_server_user_data`).Scan(&before); err != nil {
		t.Fatalf("count before: %v", err)
	}
	if before != 1 {
		t.Fatalf("fixture did not store user_data: got %d rows", before)
	}

	if err := repo.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	var after int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM instance_server_user_data`).Scan(&after); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != 0 {
		t.Fatalf("reset left %d user_data row(s) behind; add the table to Reset's list", after)
	}
}
