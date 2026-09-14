// INPUT: Runtime-owned approval classification.
// OUTPUT: The authorization boundary shown to the host.
// POS: Approval metadata, never an authorization grant.
package permission

type Boundary string

const (
	BoundaryTool          Boundary = "tool"
	BoundarySandboxEscape Boundary = "sandbox_escape"
)
