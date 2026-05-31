package store

import (
	"time"

	"github.com/everscribe/ai-assisted-setup-demo/models"
)

// Seed populates the store with two organizations, four users (two
// admins, two members), a handful of resources, and a mix of access
// requests in every state. Called once on server start so the demo has
// realistic content immediately. The IDs are fixed so the frontend's
// quick-pick login is stable across restarts.
func (s *Store) Seed() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
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
		{ID: "r_acme_stripe", Name: "stripe-live-dashboard", Description: "Live-mode Stripe console — refund + chargeback ops", Sensitivity: models.SensitivityRestricted, OrganizationID: "org_acme", RegisteredBy: "u_ada", CreatedAt: yesterday},
		{ID: "r_globex_k8s", Name: "k8s-prod-cluster", Description: "Production EKS cluster (kubectl admin)", Sensitivity: models.SensitivityRestricted, OrganizationID: "org_globex", RegisteredBy: "u_grace", CreatedAt: yesterday},
		{ID: "r_globex_pagerduty", Name: "pagerduty-admin", Description: "PagerDuty account admin (rotation + escalation policies)", Sensitivity: models.SensitivityConfidential, OrganizationID: "org_globex", RegisteredBy: "u_grace", CreatedAt: yesterday},
	}
	for _, r := range resources {
		s.resources[r.ID] = r
		s.resourcesByOrg[r.OrganizationID] = append(s.resourcesByOrg[r.OrganizationID], r.ID)
	}

	// --- Access requests across all four states ---
	requests := []*models.AccessRequest{
		{
			ID: "req_acme_1", ResourceID: "r_acme_pg", OrganizationID: "org_acme",
			RequestedBy: "u_linus", Reason: "Debug a stuck payment intent (#PAY-4821).",
			Status: models.StatusPending, CreatedAt: now,
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
}
