package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	_, err = db.ExecContext(context.Background(), `CREATE TABLE ota_task (type TEXT, taskData TEXT);`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return db
}

func insertTaskDataRow(t *testing.T, db *sql.DB, taskData string) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO ota_task(type, taskData) VALUES('task_info', ?);`,
		taskData,
	)
	if err != nil {
		t.Fatalf("failed to insert row: %v", err)
	}
}

func extractPackageECUs(t *testing.T, db *sql.DB) []string {
	t.Helper()

	var taskData string

	row := db.QueryRowContext(
		context.Background(),
		`SELECT taskData FROM ota_task WHERE type = 'task_info'`,
	)

	if err := row.Scan(&taskData); err != nil {
		t.Fatalf("failed to read taskData: %v", err)
	}

	var parsed struct {
		Packages []struct {
			ECU string `json:"ecu"`
		} `json:"packages_info"`
	}
	if err := json.Unmarshal([]byte(taskData), &parsed); err != nil {
		t.Fatalf("failed to unmarshal taskData: %v", err)
	}

	out := make([]string, 0, len(parsed.Packages))
	for _, p := range parsed.Packages {
		out = append(out, p.ECU)
	}

	return out
}

func TestRemoveUndownloadedPackagesInDB(t *testing.T) {
	t.Parallel()

	t.Run("empty ids is no-op", func(t *testing.T) {
		t.Parallel()

		db := newTestDB(t)
		defer db.Close()

		insertTaskDataRow(t, db, `{"packages_info":[{"ecu":"A"}]}`)

		if err := removeUndownloadedPackagesInDB(db, nil); err != nil {
			t.Fatalf("expected nil error for empty ids, got %v", err)
		}

		ecus := extractPackageECUs(t, db)
		if !reflect.DeepEqual(ecus, []string{"A"}) {
			t.Fatalf("unexpected packages after no-op: %v", ecus)
		}
	})

	t.Run("removes indexes in descending-safe way", func(t *testing.T) {
		t.Parallel()

		db := newTestDB(t)
		defer db.Close()

		insertTaskDataRow(t, db, `{"packages_info":[{"ecu":"A"},{"ecu":"B"},{"ecu":"C"},{"ecu":"D"}]}`)

		if err := removeUndownloadedPackagesInDB(db, []int{1, 3}); err != nil {
			t.Fatalf("removeUndownloadedPackagesInDB returned error: %v", err)
		}

		ecus := extractPackageECUs(t, db)

		want := []string{"A", "C"}
		if !reflect.DeepEqual(ecus, want) {
			t.Fatalf("unexpected packages after delete: got %v, want %v", ecus, want)
		}
	})

	t.Run("negative index returns error", func(t *testing.T) {
		t.Parallel()

		db := newTestDB(t)
		defer db.Close()

		insertTaskDataRow(t, db, `{"packages_info":[{"ecu":"A"}]}`)

		err := removeUndownloadedPackagesInDB(db, []int{-1})
		if err == nil {
			t.Fatal("expected error for negative index")
		}

		if !strings.Contains(err.Error(), "invalid package index") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("no rows updated returns error", func(t *testing.T) {
		t.Parallel()

		db := newTestDB(t)
		defer db.Close()

		err := removeUndownloadedPackagesInDB(db, []int{0})
		if err == nil {
			t.Fatal("expected error when no rows were updated")
		}

		if !strings.Contains(err.Error(), "no rows were updated") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestGetOTATaskData(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	defer db.Close()

	insertTaskDataRow(t, db, `{"task_id":42}`)

	got, err := getOTATaskData(db)
	if err != nil {
		t.Fatalf("getOTATaskData returned error: %v", err)
	}

	if got != `{"task_id":42}` {
		t.Fatalf("unexpected taskData: %s", got)
	}
}
