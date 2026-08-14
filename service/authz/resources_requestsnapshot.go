package authz

const (
	// ResourceRequestSnapshot is the permission resource guarding captured
	// request bodies. There is intentionally no default grant: only the root
	// superuser and operators who are explicitly granted request_snapshot.read
	// can decrypt and view request snapshots.
	ResourceRequestSnapshot = "request_snapshot"
)

var (
	// RequestSnapshotRead allows reading a captured request body after
	// secondary (2FA/passkey) verification with scope request_snapshot.read.
	RequestSnapshotRead = Permission{Resource: ResourceRequestSnapshot, Action: ActionRead}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceRequestSnapshot,
		LabelKey: "Request Snapshots",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read request snapshots",
				DescriptionKey: "View the captured body of a request after secure verification.",
				// Empty DefaultRoles: no built-in role receives this action.
				// The root superuser remains implicitly allowed; every other
				// role must be granted it explicitly.
				DefaultRoles: []string{},
			},
		},
	})
}
