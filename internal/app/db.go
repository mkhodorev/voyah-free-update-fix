//nolint:noctx
package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const flashFailureJSONSetArgumentCount = 4

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
func removeUndownloadedPackagesInDB(dbConn *sql.DB, fix contextDBFix) error {
	if len(fix.PackageIDs) == 0 {
		return nil
	}

	packageIDsToDelete := append([]int(nil), fix.PackageIDs...)

	// Удаляем с конца массива к началу, чтобы индексы не съезжали.
	sort.Sort(sort.Reverse(sort.IntSlice(packageIDsToDelete)))

	removePaths := make([]string, 0, len(packageIDsToDelete)+2)

	for _, idx := range packageIDsToDelete {
		if idx < 0 {
			return fmt.Errorf("error while removing packages from ota_task: invalid package index: %d", idx)
		}

		removePaths = append(removePaths, fmt.Sprintf("$.packages_info[%d]", idx))
	}

	switch fix.Mode {
	case contextDBFixModeFlashFailure:
		return applyFlashFailureFix(dbConn, fix, removePaths)
	case contextDBFixModeUndownloaded:
		removePaths = append([]string{"$.flash_state", "$.schedule_state"}, removePaths...)

		return applyUndownloadedFix(dbConn, removePaths)
	case contextDBFixModeUnsupported:
		return fmt.Errorf("error while removing packages from ota_task: unsupported fix mode: %d", fix.Mode)
	}

	return fmt.Errorf("error while removing packages from ota_task: unknown fix mode: %d", fix.Mode)
}

func applyUndownloadedFix(dbConn *sql.DB, removePaths []string) error {
	placeholders, args := jsonRemoveArguments(removePaths)

	args = append(args,
		//nolint:misspell
		`{"download_type":0,"percents":90,"stage":"Retrive Packages"}`,
		`{"stage":"Download","state":"Process"}`,
	)

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

	return executeTaskDataUpdate(dbConn, query, args)
}

func applyFlashFailureFix(dbConn *sql.DB, fix contextDBFix, packagePaths []string) error {
	rollbackPaths, err := rollbackPathsToDelete(dbConn, fix.PackageECUs)
	if err != nil {
		return err
	}

	removePaths := make([]string, 0, len(packagePaths)+len(rollbackPaths)+1)
	removePaths = append(removePaths, "$.flash_state")
	removePaths = append(removePaths, packagePaths...)
	removePaths = append(removePaths, rollbackPaths...)

	placeholders, args := jsonRemoveArguments(removePaths)
	args = append(args,
		`{"stage":"Schedule","state":"Process"}`,
		`{"set_time":0,"stage":"Wait Set Time"}`,
		fix.ExpireTime,
		fix.TargetBaselineVersion,
	)

	query := fmt.Sprintf(`
UPDATE ota_task
SET taskData = json_set(
	json_remove(taskData, %s),
	'$.overall_state', json(?),
	'$.schedule_state', json(?),
	'$.expire_time', ?,
	'$.target_baseline_version', ?
)
WHERE "type" = 'task_info'
  AND json_valid(taskData)
  AND json_type(taskData, '$.packages_info') = 'array';
`, strings.Join(placeholders, ", "))

	return executeTaskDataUpdate(dbConn, query, args)
}

func rollbackPathsToDelete(dbConn *sql.DB, ecusToDelete []string) ([]string, error) {
	taskData, err := getOTATaskData(dbConn)
	if err != nil {
		return nil, err
	}

	var payload struct {
		RollbackVersions []struct {
			ECU string `json:"ecu"`
		} `json:"ecu_rollback_versions"`
	}
	if err := json.Unmarshal([]byte(taskData), &payload); err != nil {
		return nil, fmt.Errorf("error reading ecu_rollback_versions from ota_task: %w", err)
	}

	ecuSet := make(map[string]struct{}, len(ecusToDelete))
	for _, ecu := range ecusToDelete {
		ecuSet[ecu] = struct{}{}
	}

	indexes := make([]int, 0, len(ecusToDelete))

	for idx, rollbackVersion := range payload.RollbackVersions {
		if _, ok := ecuSet[rollbackVersion.ECU]; ok {
			indexes = append(indexes, idx)
		}
	}

	sort.Sort(sort.Reverse(sort.IntSlice(indexes)))

	paths := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		paths = append(paths, fmt.Sprintf("$.ecu_rollback_versions[%d]", idx))
	}

	return paths, nil
}

func jsonRemoveArguments(removePaths []string) ([]string, []any) {
	placeholders := make([]string, 0, len(removePaths))
	args := make([]any, 0, len(removePaths)+flashFailureJSONSetArgumentCount)

	for _, path := range removePaths {
		placeholders = append(placeholders, "?")
		args = append(args, path)
	}

	return placeholders, args
}

func executeTaskDataUpdate(dbConn *sql.DB, query string, args []any) error {
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
