package memory

import (
	"context"
	"time"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
)

// Seed populates the store with two organizations, four seeded users, a
// handful of resources, access requests in every state, and comments.
// IDs are fixed so the frontend's quick-pick login is stable across
// restarts.
//
// This is also the ordering fixture: r_acme_pg's three requests and
// req_acme_1's three comments each have one pair tied on created_at, so
// the id tiebreak has something to decide, and both groups are listed
// here in an order wrong for the declared sort, so serving insertion
// order fails.
func (s *Store) Seed(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Fixed instants, not now-relative offsets, so two independently seeded
	// stores agree exactly and the contract can assert created_at directly.
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	earlier := now.Add(-2 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)

	// --- Organizations ---
	orgs := []*models.Organization{
		{ID: "org_acme", Name: "Acme Corp", CreatedAt: yesterday},
		{ID: "org_globex", Name: "Globex", CreatedAt: yesterday},
	}
	for _, o := range orgs {
		s.orgs[o.ID] = o
	}

	// --- Users ---
	users := []*models.User{
		{ID: "u_ada", Name: "Ada Lovelace", Email: "ada@acme.test", Role: models.RoleAdmin, OrganizationID: "org_acme", CreatedAt: yesterday},
		{ID: "u_linus", Name: "Linus Torvalds", Email: "linus@acme.test", Role: models.RoleMember, OrganizationID: "org_acme", CreatedAt: yesterday},
		{ID: "u_grace", Name: "Grace Hopper", Email: "grace@globex.test", Role: models.RoleAdmin, OrganizationID: "org_globex", CreatedAt: yesterday},
		{ID: "u_alan", Name: "Alan Turing", Email: "alan@globex.test", Role: models.RoleMember, OrganizationID: "org_globex", CreatedAt: yesterday},
	}
	for _, u := range users {
		s.users[u.ID] = u
		s.usersByOrg[u.OrganizationID] = append(s.usersByOrg[u.OrganizationID], u.ID)
	}

	// --- Resources ---
	resources := []*models.Resource{
		{ID: "r_acme_pg", Name: "production-postgres", Description: "Primary customer database (us-east-1)", Sensitivity: models.SensitivityRestricted, OrganizationID: "org_acme", RegisteredBy: "u_ada", CreatedAt: yesterday},
		{ID: "r_acme_s3", Name: "audit-logs-bucket", Description: "S3 bucket for archived audit logs", Sensitivity: models.SensitivityConfidential, OrganizationID: "org_acme", RegisteredBy: "u_ada", CreatedAt: yesterday},
		{ID: "r_acme_stripe", Name: "stripe-live-dashboard", Description: "Live-mode Stripe console - refund + chargeback ops", Sensitivity: models.SensitivityRestricted, OrganizationID: "org_acme", RegisteredBy: "u_ada", CreatedAt: yesterday},
		{ID: "r_globex_k8s", Name: "k8s-prod-cluster", Description: "Production EKS cluster (kubectl admin)", Sensitivity: models.SensitivityRestricted, OrganizationID: "org_globex", RegisteredBy: "u_grace", CreatedAt: yesterday},
		{ID: "r_globex_pagerduty", Name: "pagerduty-admin", Description: "PagerDuty account admin (rotation + escalation policies)", Sensitivity: models.SensitivityConfidential, OrganizationID: "org_globex", RegisteredBy: "u_grace", CreatedAt: yesterday},
	}
	for _, r := range resources {
		s.resources[r.ID] = r
		s.resourcesByOrg[r.OrganizationID] = append(s.resourcesByOrg[r.OrganizationID], r.ID)
	}

	// --- Access requests across all four states ---
	//
	// The first three all sit on r_acme_pg and are listed newest-tie
	// first on purpose. Served order is req_acme_1, req_acme_4,
	// req_acme_5: req_acme_1 and req_acme_4 tie on created_at and split
	// on id ascending, and req_acme_5 is older so it sorts last.
	requests := []*models.AccessRequest{
		{
			ID: "req_acme_4", ResourceID: "r_acme_pg", OrganizationID: "org_acme",
			RequestedBy: "u_linus", Reason: "Re-run last quarter's revenue backfill.",
			Status:    models.StatusDenied,
			DecidedBy: "u_ada", DecidedAt: &now,
			CreatedAt: now,
		},
		{
			ID: "req_acme_1", ResourceID: "r_acme_pg", OrganizationID: "org_acme",
			RequestedBy: "u_linus", Reason: "Debug a stuck payment intent (#PAY-4821).",
			Status: models.StatusPending, CreatedAt: now,
		},
		{
			ID: "req_acme_5", ResourceID: "r_acme_pg", OrganizationID: "org_acme",
			RequestedBy: "u_linus", Reason: "Verify the read replica lag alert.",
			Status:    models.StatusApproved,
			DecidedBy: "u_ada", DecidedAt: &now,
			CreatedAt: earlier,
		},
		{
			ID: "req_acme_2", ResourceID: "r_acme_stripe", OrganizationID: "org_acme",
			RequestedBy: "u_linus", Reason: "Process a chargeback for customer ACME-203.",
			Status:    models.StatusApproved,
			DecidedBy: "u_ada", DecidedAt: &earlier,
			CreatedAt: earlier.Add(-time.Hour),
		},
		{
			ID: "req_acme_3", ResourceID: "r_acme_s3", OrganizationID: "org_acme",
			RequestedBy: "u_linus", Reason: "Pull logs for the SOC2 evidence locker.",
			Status:    models.StatusDenied,
			DecidedBy: "u_ada", DecidedAt: &yesterday,
			CreatedAt: yesterday.Add(-time.Hour),
		},
		{
			ID: "req_globex_1", ResourceID: "r_globex_k8s", OrganizationID: "org_globex",
			RequestedBy: "u_alan", Reason: "Roll a hotfix for the on-call escalation bug.",
			Status:    models.StatusApproved,
			DecidedBy: "u_grace", DecidedAt: &earlier,
			CreatedAt: earlier.Add(-30 * time.Minute),
		},
		{
			ID: "req_globex_2", ResourceID: "r_globex_pagerduty", OrganizationID: "org_globex",
			RequestedBy: "u_alan", Reason: "Update the after-hours escalation policy.",
			Status:    models.StatusRevoked,
			DecidedBy: "u_grace", DecidedAt: &yesterday,
			RevokedBy: "u_grace", RevokedAt: &earlier,
			CreatedAt: yesterday.Add(-2 * time.Hour),
		},
	}
	for _, req := range requests {
		s.requests[req.ID] = req
		s.requestsByRes[req.ResourceID] = append(s.requestsByRes[req.ResourceID], req.ID)
	}

	// --- Comments ---
	//
	// The three on req_acme_1 are listed out of order on purpose. Served
	// order is c_acme_1, c_acme_2, c_acme_3: c_acme_1 is oldest, and
	// c_acme_2 and c_acme_3 tie on created_at and split on id ascending.
	comments := []*models.Comment{
		{
			ID: "c_acme_3", RequestID: "req_acme_1", OrganizationID: "org_acme",
			AuthorID: "u_linus", Body: "Will drop the grant as soon as PAY-4821 closes.",
			CreatedAt: now,
		},
		{
			ID: "c_acme_2", RequestID: "req_acme_1", OrganizationID: "org_acme",
			AuthorID: "u_ada", Body: "Confirmed with the on-call lead. Approving for 24 hours.",
			CreatedAt: now,
		},
		{
			ID: "c_acme_1", RequestID: "req_acme_1", OrganizationID: "org_acme",
			AuthorID: "u_linus", Body: "Scope is the payments schema, read-only.",
			CreatedAt: earlier,
		},
		{
			ID: "c_globex_1", RequestID: "req_globex_2", OrganizationID: "org_globex",
			AuthorID: "u_grace", Body: "Revoked once the rotation change landed.",
			CreatedAt: earlier,
		},
	}
	for _, cm := range comments {
		s.comments[cm.ID] = cm
		s.commentsByReq[cm.RequestID] = append(s.commentsByReq[cm.RequestID], cm.ID)
	}

	return nil
}
