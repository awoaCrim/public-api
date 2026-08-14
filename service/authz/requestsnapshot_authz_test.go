package authz

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestSnapshotReadHasNoDefaultAdminGrant guards the security contract:
// request_snapshot.read must never be part of any built-in role baseline. Only
// the root superuser (implicit) and explicitly granted operators may read
// captured request bodies.
func TestRequestSnapshotReadHasNoDefaultAdminGrant(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	// Root is a superuser role: allowed implicitly even without a policy row.
	assert.True(t, Can(1, common.RoleRootUser, RequestSnapshotRead))

	// The admin role has no baseline grant for the snapshot permission.
	assert.False(t, Can(2, common.RoleAdminUser, RequestSnapshotRead))
	assert.False(t, Can(3, common.RoleCommonUser, RequestSnapshotRead))

	// The permission is registered and exposed through the catalog.
	found := false
	for _, resource := range Catalog() {
		if resource.Resource != ResourceRequestSnapshot {
			continue
		}
		for _, action := range resource.Actions {
			if action.Action == ActionRead {
				assert.Empty(t, action.DefaultRoles, "request_snapshot.read must have no default roles")
				found = true
			}
		}
	}
	assert.True(t, found, "request_snapshot resource must be registered")
}
