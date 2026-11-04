package tools

import (
	"context"
	"time"
)

// ListServicesIn is the input of list_services.
type ListServicesIn struct {
	Search string `json:"search,omitempty" jsonschema:"Optional search term to filter services by name or description."`
	First  int    `json:"first,omitempty" jsonschema:"Maximum number of services to return (default 15, max 100)."`
	After  string `json:"after,omitempty" jsonschema:"Pagination cursor (page_info.end_cursor)."`
}

// ServiceDTO is a service as returned to MCP clients.
type ServiceDTO struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	EscalationPolicyID string `json:"escalation_policy_id"`
	IsMaintenance      bool   `json:"is_maintenance"`
}

// ListServicesOut is the output of list_services.
type ListServicesOut struct {
	Services []ServiceDTO `json:"services"`
	PageInfo PageInfo     `json:"page_info"`
}

const listServicesQuery = `query($i: ServiceSearchOptions) {
  services(input: $i) {
    nodes { id name description escalationPolicyID maintenanceExpiresAt }
    pageInfo { endCursor hasNextPage }
  }
}`

func (t *Tools) registerServices(r *registry) {
	add(r, "list_services", "List services with their escalation policy IDs, with search filtering and pagination.", false, t.listServices)
}

func (t *Tools) listServices(ctx context.Context, in ListServicesIn) (ListServicesOut, error) {
	vars := map[string]any{"i": map[string]any{"search": in.Search, "first": clampFirst(in.First), "after": in.After}}
	var data struct {
		Services struct {
			Nodes []struct {
				ID                   string  `json:"id"`
				Name                 string  `json:"name"`
				Description          string  `json:"description"`
				EscalationPolicyID   string  `json:"escalationPolicyID"`
				MaintenanceExpiresAt *string `json:"maintenanceExpiresAt"`
			} `json:"nodes"`
			PageInfo gqlPage `json:"pageInfo"`
		} `json:"services"`
	}
	if err := t.gql.Do(ctx, listServicesQuery, vars, &data); err != nil {
		return ListServicesOut{}, err
	}
	now := t.now()
	out := ListServicesOut{Services: make([]ServiceDTO, 0, len(data.Services.Nodes)), PageInfo: data.Services.PageInfo.convert()}
	for _, n := range data.Services.Nodes {
		out.Services = append(out.Services, ServiceDTO{
			ID: n.ID, Name: n.Name, Description: n.Description, EscalationPolicyID: n.EscalationPolicyID,
			IsMaintenance: inFuture(n.MaintenanceExpiresAt, now),
		})
	}
	return out, nil
}

func inFuture(ts *string, now time.Time) bool {
	if ts == nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, *ts)
	return err == nil && t.After(now)
}
