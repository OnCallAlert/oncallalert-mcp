package tools

import (
	"context"
	"fmt"
)

// alertEventLimit is how many recent events get_alert returns.
const alertEventLimit = 50

// Alert status names shown to MCP clients, matching the web UI.
var (
	gqlStatusByName = map[string]string{
		"open":         "StatusUnacknowledged",
		"acknowledged": "StatusAcknowledged",
		"closed":       "StatusClosed",
	}
	statusNameByGQL = map[string]string{
		"StatusUnacknowledged": "open",
		"StatusAcknowledged":   "acknowledged",
		"StatusClosed":         "closed",
	}
)

// AlertDTO is an alert as returned to MCP clients.
type AlertDTO struct {
	ID        int    `json:"id"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	ServiceID string `json:"service_id"`
	CreatedAt string `json:"created_at"`
}

type gqlAlert struct {
	AlertID   int    `json:"alertID"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	ServiceID string `json:"serviceID"`
	CreatedAt string `json:"createdAt"`
}

func (a gqlAlert) dto() AlertDTO {
	status, ok := statusNameByGQL[a.Status]
	if !ok {
		status = a.Status
	}
	return AlertDTO{ID: a.AlertID, Summary: a.Summary, Status: status, ServiceID: a.ServiceID, CreatedAt: a.CreatedAt}
}

const alertFields = `alertID summary status serviceID createdAt`

// ListAlertsIn is the input of list_alerts.
type ListAlertsIn struct {
	ServiceID string `json:"service_id,omitempty" jsonschema:"Only alerts for this service ID."`
	Status    string `json:"status,omitempty" jsonschema:"Only alerts with this status: open (not yet acknowledged), acknowledged or closed."`
	Since     string `json:"since,omitempty" jsonschema:"Only alerts created at or after this time. RFC 3339 with zone (2026-11-01T09:00:00Z) or a date (2026-11-01, UTC)."`
	Until     string `json:"until,omitempty" jsonschema:"Only alerts created before this time. Same formats as since."`
	Search    string `json:"search,omitempty" jsonschema:"Match against summary, alert ID or service name."`
	First     int    `json:"first,omitempty" jsonschema:"Maximum number of alerts to return (default 15, max 100)."`
	After     string `json:"after,omitempty" jsonschema:"Pagination cursor (page_info.end_cursor). Filters come from the cursor."`
}

// ListAlertsOut is the output of list_alerts.
type ListAlertsOut struct {
	Alerts   []AlertDTO `json:"alerts"`
	PageInfo PageInfo   `json:"page_info"`
}

const listAlertsQuery = `query($i: AlertSearchOptions) {
  alerts(input: $i) {
    nodes { ` + alertFields + ` }
    pageInfo { endCursor hasNextPage }
  }
}`

// GetAlertIn is the input of get_alert.
type GetAlertIn struct {
	ID int `json:"id" jsonschema:"Alert ID."`
}

// AlertEvent is one alert log entry.
type AlertEvent struct {
	Message string `json:"message"`
	Time    string `json:"time"`
}

// GetAlertOut is the output of get_alert.
type GetAlertOut struct {
	Alert       AlertDTO     `json:"alert"`
	Details     string       `json:"details"`
	ServiceName string       `json:"service_name"`
	Events      []AlertEvent `json:"events"`
}

const getAlertQuery = `query($id: Int!, $limit: Int) {
  alert(id: $id) {
    ` + alertFields + ` details
    service { name }
    recentEvents(input: {limit: $limit}) { nodes { timestamp message } }
  }
}`

func (t *Tools) registerAlerts(r *registry) {
	add(r, "list_alerts", "List alerts, newest first. Filter by service, status, creation time or search text.", false, t.listAlerts)
	add(r, "get_alert", "Get one alert with its service name and its most recent events (newest first).", false, t.getAlert)
	t.registerAlertWrites(r)
}

func (t *Tools) listAlerts(ctx context.Context, in ListAlertsIn) (ListAlertsOut, error) {
	opts := map[string]any{"first": clampFirst(in.First), "after": in.After, "search": in.Search, "sort": "dateID"}
	if in.ServiceID != "" {
		opts["filterByServiceID"] = []string{in.ServiceID}
	}
	if in.Status != "" {
		s, ok := gqlStatusByName[in.Status]
		if !ok {
			return ListAlertsOut{}, fmt.Errorf("status must be open, acknowledged or closed, got %q", in.Status)
		}
		opts["filterByStatus"] = []string{s}
	}
	since, err := optTime("since", in.Since)
	if err != nil {
		return ListAlertsOut{}, err
	}
	until, err := optTime("until", in.Until)
	if err != nil {
		return ListAlertsOut{}, err
	}
	if since != "" {
		opts["notCreatedBefore"] = since
	}
	if until != "" {
		opts["createdBefore"] = until
	}

	var data struct {
		Alerts struct {
			Nodes    []gqlAlert `json:"nodes"`
			PageInfo gqlPage    `json:"pageInfo"`
		} `json:"alerts"`
	}
	if err := t.gql.Do(ctx, listAlertsQuery, map[string]any{"i": opts}, &data); err != nil {
		return ListAlertsOut{}, err
	}
	return ListAlertsOut{Alerts: alertDTOs(data.Alerts.Nodes), PageInfo: data.Alerts.PageInfo.convert()}, nil
}

func alertDTOs(in []gqlAlert) []AlertDTO {
	out := make([]AlertDTO, 0, len(in))
	for _, a := range in {
		out = append(out, a.dto())
	}
	return out
}

func (t *Tools) getAlert(ctx context.Context, in GetAlertIn) (GetAlertOut, error) {
	if in.ID <= 0 {
		return GetAlertOut{}, errRequired("id")
	}
	var data struct {
		Alert *struct {
			gqlAlert
			Details string `json:"details"`
			Service *struct {
				Name string `json:"name"`
			} `json:"service"`
			RecentEvents struct {
				Nodes []struct {
					Timestamp string `json:"timestamp"`
					Message   string `json:"message"`
				} `json:"nodes"`
			} `json:"recentEvents"`
		} `json:"alert"`
	}
	vars := map[string]any{"id": in.ID, "limit": alertEventLimit}
	if err := t.gql.Do(ctx, getAlertQuery, vars, &data); err != nil {
		return GetAlertOut{}, err
	}
	a := data.Alert
	if a == nil {
		return GetAlertOut{}, notFound("alert", fmt.Sprint(in.ID))
	}
	out := GetAlertOut{Alert: a.dto(), Details: a.Details, Events: []AlertEvent{}}
	if a.Service != nil {
		out.ServiceName = a.Service.Name
	}
	for _, e := range a.RecentEvents.Nodes {
		out.Events = append(out.Events, AlertEvent{Message: e.Message, Time: e.Timestamp})
	}
	return out, nil
}
