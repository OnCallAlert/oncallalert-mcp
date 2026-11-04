package tools

import (
	"context"
	"fmt"
)

// maxAlertIDs caps ack_alert and resolve_alert to keep one call small.
const maxAlertIDs = 50

// AlertIDsIn is the input of ack_alert and resolve_alert.
type AlertIDsIn struct {
	IDs []int `json:"ids" jsonschema:"Alert IDs, 1 to 50."`
}

// UpdateAlertsOut is the output of ack_alert and resolve_alert.
type UpdateAlertsOut struct {
	UpdatedIDs []int `json:"updated_ids"`
	SkippedIDs []int `json:"skipped_ids"`
}

// EscalateAlertIn is the input of escalate_alert.
type EscalateAlertIn struct {
	ID int `json:"id" jsonschema:"Alert ID."`
}

// EscalateAlertOut is the output of escalate_alert.
type EscalateAlertOut struct {
	EscalatedID int `json:"escalated_id"`
}

// CreateAlertIn is the input of create_alert.
type CreateAlertIn struct {
	ServiceID string `json:"service_id" jsonschema:"Service ID."`
	Summary   string `json:"summary" jsonschema:"Short summary."`
	Details   string `json:"details,omitempty" jsonschema:"Optional details (Markdown)."`
	Dedup     string `json:"dedup,omitempty" jsonschema:"Optional dedup key. An open alert with the same key is reused. Defaults to one built from summary and details."`
}

// CreateAlertOut is the output of create_alert.
type CreateAlertOut struct {
	Alert AlertDTO `json:"alert"`
}

const (
	updateAlertsMutation = `mutation($i: UpdateAlertsInput!) { updateAlerts(input: $i) { alertID } }`
	escalateMutation     = `mutation($ids: [Int!]) { escalateAlerts(input: $ids) { alertID } }`
	createAlertMutation  = `mutation($i: CreateAlertInput!) { createAlert(input: $i) { ` + alertFields + ` } }`
)

func (t *Tools) registerAlertWrites(r *registry) {
	add(r, "ack_alert", "Acknowledge open alerts. Changes live data. Alerts that are not open are skipped.", true,
		func(ctx context.Context, in AlertIDsIn) (UpdateAlertsOut, error) {
			return t.setAlertStatus(ctx, in, "StatusAcknowledged")
		})
	add(r, "resolve_alert", "Close (resolve) alerts. Changes live data. Alerts that are already closed are skipped.", true,
		func(ctx context.Context, in AlertIDsIn) (UpdateAlertsOut, error) {
			return t.setAlertStatus(ctx, in, "StatusClosed")
		})
	add(r, "escalate_alert", "Escalate an alert to the next step of its escalation policy now. Changes live data.", true, t.escalateAlert)
	add(r, "create_alert", "Create an alert on a service, which pages whoever is on call. Changes live data. An open alert with the same dedup key is reused instead.", true, t.createAlert)
}

func (t *Tools) setAlertStatus(ctx context.Context, in AlertIDsIn, status string) (UpdateAlertsOut, error) {
	if len(in.IDs) == 0 {
		return UpdateAlertsOut{}, fmt.Errorf("ids must be a non-empty list of alert IDs")
	}
	if len(in.IDs) > maxAlertIDs {
		return UpdateAlertsOut{}, fmt.Errorf("ids: at most %d per call, got %d", maxAlertIDs, len(in.IDs))
	}
	vars := map[string]any{"i": map[string]any{"alertIDs": in.IDs, "newStatus": status}}
	var data struct {
		UpdateAlerts []struct {
			AlertID int `json:"alertID"`
		} `json:"updateAlerts"`
	}
	if err := t.gql.Do(ctx, updateAlertsMutation, vars, &data); err != nil {
		return UpdateAlertsOut{}, err
	}
	updated := make([]int, 0, len(data.UpdateAlerts))
	for _, a := range data.UpdateAlerts {
		updated = append(updated, a.AlertID)
	}
	return UpdateAlertsOut{UpdatedIDs: updated, SkippedIDs: missingIDs(in.IDs, updated)}, nil
}

// missingIDs returns the IDs in want that are not in got, in order.
func missingIDs(want, got []int) []int {
	seen := make(map[int]bool, len(got))
	for _, id := range got {
		seen[id] = true
	}
	res := []int{}
	for _, id := range want {
		if !seen[id] {
			res = append(res, id)
		}
	}
	return res
}

func (t *Tools) escalateAlert(ctx context.Context, in EscalateAlertIn) (EscalateAlertOut, error) {
	if in.ID <= 0 {
		return EscalateAlertOut{}, errRequired("id")
	}
	var data struct {
		EscalateAlerts []struct {
			AlertID int `json:"alertID"`
		} `json:"escalateAlerts"`
	}
	if err := t.gql.Do(ctx, escalateMutation, map[string]any{"ids": []int{in.ID}}, &data); err != nil {
		return EscalateAlertOut{}, err
	}
	for _, a := range data.EscalateAlerts {
		if a.AlertID == in.ID {
			return EscalateAlertOut{EscalatedID: in.ID}, nil
		}
	}
	return EscalateAlertOut{}, fmt.Errorf("alert %d was not escalated: it may not exist or may not be open", in.ID)
}

func (t *Tools) createAlert(ctx context.Context, in CreateAlertIn) (CreateAlertOut, error) {
	if in.ServiceID == "" || in.Summary == "" {
		return CreateAlertOut{}, fmt.Errorf("service_id and summary are required")
	}
	input := map[string]any{"serviceID": in.ServiceID, "summary": in.Summary}
	if in.Details != "" {
		input["details"] = in.Details
	}
	if in.Dedup != "" {
		input["dedup"] = in.Dedup
	}
	var data struct {
		CreateAlert *gqlAlert `json:"createAlert"`
	}
	if err := t.gql.Do(ctx, createAlertMutation, map[string]any{"i": input}, &data); err != nil {
		return CreateAlertOut{}, err
	}
	if data.CreateAlert == nil {
		return CreateAlertOut{}, fmt.Errorf("create alert: the server returned no alert")
	}
	return CreateAlertOut{Alert: data.CreateAlert.dto()}, nil
}
