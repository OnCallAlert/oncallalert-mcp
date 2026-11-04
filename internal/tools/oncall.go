package tools

import (
	"context"
	"errors"
	"time"
)

// GetOnCallIn is the input of get_oncall.
type GetOnCallIn struct {
	ServiceID          string `json:"service_id,omitempty" jsonschema:"Service ID."`
	EscalationPolicyID string `json:"escalation_policy_id,omitempty" jsonschema:"Escalation policy ID."`
	ScheduleID         string `json:"schedule_id,omitempty" jsonschema:"Schedule ID."`
	At                 string `json:"at,omitempty" jsonschema:"Schedules only: a past or future time to look up instead of now. RFC 3339 with zone (2026-11-01T09:00:00Z) or a date (2026-11-01, UTC)."`
}

// UserRef is a user ID and name.
type UserRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OnCallStep is the users on call in one escalation step.
type OnCallStep struct {
	Step  int       `json:"step"` // 1 = first step
	Users []UserRef `json:"users"`
}

// GetOnCallOut is the output of get_oncall. Only the fields for the requested target are set.
type GetOnCallOut struct {
	ServiceID          string       `json:"service_id,omitempty"`
	EscalationPolicyID string       `json:"escalation_policy_id,omitempty"`
	ScheduleID         string       `json:"schedule_id,omitempty"`
	At                 string       `json:"at,omitempty"`
	Steps              []OnCallStep `json:"steps,omitempty"`
	Users              []UserRef    `json:"users,omitempty"`
}

const (
	serviceOnCallQuery  = `query($id: ID!) { service(id: $id) { onCallUsers { userID userName stepNumber } } }`
	policyServicesQuery = `query($id: ID!) { escalationPolicy(id: $id) { assignedTo { id type } } }`
	scheduleOnCallQuery = `query($id: ID!, $start: ISOTimestamp!, $end: ISOTimestamp!) {
  schedule(id: $id) { shifts(start: $start, end: $end) { userID user { name } } }
}`
)

func (t *Tools) registerOnCall(r *registry) {
	add(r, "get_oncall", "Who is on call. Give exactly one of service_id, escalation_policy_id or schedule_id. "+
		"For a service or escalation policy: the users on call now, per escalation step. "+
		"For a schedule: the users on call now, or at the time given in `at`.", false, t.getOnCall)
}

func (t *Tools) getOnCall(ctx context.Context, in GetOnCallIn) (GetOnCallOut, error) {
	n := 0
	for _, v := range []string{in.ServiceID, in.EscalationPolicyID, in.ScheduleID} {
		if v != "" {
			n++
		}
	}
	if n != 1 {
		return GetOnCallOut{}, errors.New("give exactly one of service_id, escalation_policy_id or schedule_id")
	}
	if in.At != "" && in.ScheduleID == "" {
		return GetOnCallOut{}, errors.New("at is supported for schedule_id only; services and escalation policies have current on-call data only")
	}
	switch {
	case in.ServiceID != "":
		steps, err := t.serviceSteps(ctx, in.ServiceID)
		return GetOnCallOut{ServiceID: in.ServiceID, Steps: steps}, err
	case in.EscalationPolicyID != "":
		steps, err := t.policySteps(ctx, in.EscalationPolicyID)
		return GetOnCallOut{EscalationPolicyID: in.EscalationPolicyID, Steps: steps}, err
	}
	return t.scheduleOnCall(ctx, in)
}

func (t *Tools) serviceSteps(ctx context.Context, id string) ([]OnCallStep, error) {
	var data struct {
		Service *struct {
			OnCallUsers []struct {
				UserID     string `json:"userID"`
				UserName   string `json:"userName"`
				StepNumber int    `json:"stepNumber"`
			} `json:"onCallUsers"`
		} `json:"service"`
	}
	if err := t.gql.Do(ctx, serviceOnCallQuery, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	if data.Service == nil {
		return nil, notFound("service", id)
	}
	steps := []OnCallStep{}
	for _, u := range data.Service.OnCallUsers {
		if len(steps) == 0 || steps[len(steps)-1].Step != u.StepNumber+1 {
			steps = append(steps, OnCallStep{Step: u.StepNumber + 1, Users: []UserRef{}})
		}
		last := &steps[len(steps)-1]
		last.Users = append(last.Users, UserRef{ID: u.UserID, Name: u.UserName})
	}
	return steps, nil
}

// policySteps finds a service that uses the policy and reads its on-call users,
// because the API exposes on-call users per service, not per policy.
func (t *Tools) policySteps(ctx context.Context, id string) ([]OnCallStep, error) {
	var data struct {
		EscalationPolicy *struct {
			AssignedTo []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"assignedTo"`
		} `json:"escalationPolicy"`
	}
	if err := t.gql.Do(ctx, policyServicesQuery, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	if data.EscalationPolicy == nil {
		return nil, notFound("escalation policy", id)
	}
	for _, a := range data.EscalationPolicy.AssignedTo {
		if a.Type == "service" {
			return t.serviceSteps(ctx, a.ID)
		}
	}
	return nil, errors.New("this escalation policy is not used by any service, so the API cannot tell who is on call for it")
}

func (t *Tools) scheduleOnCall(ctx context.Context, in GetOnCallIn) (GetOnCallOut, error) {
	at := t.now()
	if in.At != "" {
		var err error
		if at, err = parseTime("at", in.At); err != nil {
			return GetOnCallOut{}, err
		}
	}
	vars := map[string]any{
		"id":    in.ScheduleID,
		"start": at.UTC().Format(time.RFC3339),
		"end":   at.Add(time.Second).UTC().Format(time.RFC3339),
	}
	var data struct {
		Schedule *struct {
			Shifts []struct {
				UserID string `json:"userID"`
				User   *struct {
					Name string `json:"name"`
				} `json:"user"`
			} `json:"shifts"`
		} `json:"schedule"`
	}
	if err := t.gql.Do(ctx, scheduleOnCallQuery, vars, &data); err != nil {
		return GetOnCallOut{}, err
	}
	if data.Schedule == nil {
		return GetOnCallOut{}, notFound("schedule", in.ScheduleID)
	}
	out := GetOnCallOut{ScheduleID: in.ScheduleID, Users: []UserRef{}}
	if in.At != "" {
		out.At = at.Format(time.RFC3339)
	}
	seen := map[string]bool{}
	for _, s := range data.Schedule.Shifts {
		if seen[s.UserID] {
			continue
		}
		seen[s.UserID] = true
		ref := UserRef{ID: s.UserID}
		if s.User != nil {
			ref.Name = s.User.Name
		}
		out.Users = append(out.Users, ref)
	}
	return out, nil
}
