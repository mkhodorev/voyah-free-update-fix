//nolint:noctx
package app

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func connectToDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", localContextDBTmpFile)
	if err != nil {
		return nil, fmt.Errorf("error opening context.db: %w", err)
	}

	row := db.QueryRow("PRAGMA integrity_check;")

	var integrityResult string
	if err := row.Scan(&integrityResult); err != nil {
		db.Close()

		return nil, fmt.Errorf("error checking database integrity: %w", err)
	}

	if integrityResult != "ok" {
		db.Close()

		return nil, fmt.Errorf("database integrity check failed: %s", integrityResult)
	}

	return db, nil
}

func getOTATaskData(dbConn *sql.DB) (string, error) {
	var taskData string

	row := dbConn.QueryRow("SELECT taskData FROM ota_task WHERE type = 'task_info'")
	if err := row.Scan(&taskData); err != nil {
		return "", fmt.Errorf("error querying ota_task: %w", err)
	}

	return taskData, nil
}

//nolint:mnd
func removeUndownloadedPackagesInDB(dbConn *sql.DB, packageIDsToDelete []int) error {
	if len(packageIDsToDelete) == 0 {
		return nil
	}

	// Удаляем с конца массива к началу, чтобы индексы не съезжали.
	sort.Sort(sort.Reverse(sort.IntSlice(packageIDsToDelete)))

	removePaths := make([]string, 0, len(packageIDsToDelete)+2)
	removePaths = append(removePaths, "$.flash_state", "$.schedule_state")

	for _, idx := range packageIDsToDelete {
		if idx < 0 {
			return fmt.Errorf("error while removing packages from ota_task: invalid package index: %d", idx)
		}

		removePaths = append(removePaths, fmt.Sprintf("$.packages_info[%d]", idx))
	}

	placeholders := make([]string, 0, len(removePaths))
	args := make([]any, 0, len(removePaths)+2)

	for _, path := range removePaths {
		placeholders = append(placeholders, "?")
		args = append(args, path)
	}

	args = append(args,
		//nolint:misspell
		`{"download_type":0,"percents":90,"stage":"Retrive Packages"}`,
		`{"stage":"Download","state":"Process"}`,
	)

	//nolint:gosec
	query := fmt.Sprintf(`
UPDATE ota_task
SET taskData = json_set(
	json_remove(taskData, %s),
	'$.download_state', json(?),
	'$.overall_state', json(?)
)
WHERE "type" = 'task_info'
  AND json_valid(taskData)
  AND json_type(taskData, '$.packages_info') = 'array';
`, strings.Join(placeholders, ", "))

	result, err := dbConn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error removing packages from ota_task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error while removing packages from ota_task: error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("error while removing packages from ota_task: no rows were updated")
	}

	return nil
}
