package models

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// DefaultProjectID is the project every mockway instance is seeded with.
// It matches the SCW_DEFAULT_PROJECT_ID that infrafactory's Layer 2
// cloudEnv exports, so scenarios that never mention a project keep
// working unchanged.
const DefaultProjectID = "00000000-0000-0000-0000-000000000000"

// ProjectNotEmptyError reports an attempt to delete a project that still
// owns resources. Real Scaleway refuses this, and mirroring the refusal
// is the point: it turns a bad destroy ordering into a Layer 2 failure
// that costs seconds, instead of a Layer 3 failure that leaves billable
// resources behind.
//
// Blockers are "<collection>/<id>" strings. They are the whole value of
// the error -- "cannot delete: dependents exist" tells an operator
// nothing about which teardown step went wrong.
type ProjectNotEmptyError struct {
	ProjectID string
	Blockers  []string
}

func (e *ProjectNotEmptyError) Error() string {
	return fmt.Sprintf("project %s still owns %d resource(s): %s",
		e.ProjectID, len(e.Blockers), strings.Join(e.Blockers, ", "))
}

// Is reports ErrConflict so existing writeDomainError call sites degrade
// to a 409 rather than a 500 if they do not know about this type.
func (e *ProjectNotEmptyError) Is(target error) bool { return target == ErrConflict }
