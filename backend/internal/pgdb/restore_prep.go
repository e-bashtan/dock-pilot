package pgdb

import (
	"fmt"
	"strings"
)

// restorePrep decides drop/create steps before applying a SQL dump.
// existsInCluster is whether the target database already exists in Postgres
// (not only in the panel registry).
func restorePrep(existsInCluster, createDB, dropExisting bool) (drop, create bool, err error) {
	if existsInCluster && !dropExisting {
		return false, false, fmt.Errorf(
			"%w: database already exists — enable drop existing to replace it, or choose another name",
			ErrInvalidInput,
		)
	}
	if dropExisting {
		return true, true, nil
	}
	if createDB || !existsInCluster {
		return false, true, nil
	}
	return false, false, nil
}

func clusterHasDatabase(names []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
