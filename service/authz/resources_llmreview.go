package authz

const (
	// ResourceLLMReview guards the LLM compliance review management surface
	// (task list, details with payload/snippet content, retry, manual
	// creation). There is intentionally no default grant: only the root
	// superuser and roles explicitly granted llm_review.read can inspect
	// review records.
	ResourceLLMReview = "llm_review"
)

var (
	// LLMReviewRead allows viewing and managing review tasks after secondary
	// (2FA/passkey) verification with scope llm_review.read.
	LLMReviewRead = Permission{Resource: ResourceLLMReview, Action: ActionRead}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceLLMReview,
		LabelKey: "LLM Review",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Manage LLM review tasks",
				DescriptionKey: "View compliance review records, details and queue status, and retry or create review tasks after secure verification.",
				// Empty DefaultRoles: no built-in role receives this action.
				// The root superuser remains implicitly allowed; every other
				// role must be granted it explicitly.
				DefaultRoles: []string{},
			},
		},
	})
}
