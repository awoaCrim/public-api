package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRequestSnapshotPermissionIsNotDelegatable guards the root-only access
// contract. Request bodies are not exposed through the editable authz catalog;
// the API route enforces the root role directly.
func TestRequestSnapshotPermissionIsNotDelegatable(t *testing.T) {
	for _, resource := range Catalog() {
		assert.NotEqual(t, "request_snapshot", resource.Resource)
	}
}
