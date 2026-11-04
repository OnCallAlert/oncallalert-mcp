package tools

import (
	"context"
	"errors"
	"time"
)

// CreateUserOverrideIn is the input of create_user_override.
type CreateUserOverrideIn struct {
	ScheduleID   string `json:"schedule_id" jsonschema:"Target schedule ID."`
	Start        string `json:"start" jsonschema:"Override start time. RFC 3339 with zone (2026-11-01T09:00:00Z) or a date (2026-11-01, UTC)."`
	End          string `json:"end" jsonschema:"Override end time. Same formats as start."`
	AddUserID    string `json:"add_user_id,omitempty" jsonschema:"User ID to add as on-call during the override."`
	RemoveUserID string `json:"remove_user_id,omitempty" jsonschema:"User ID to remove during the override period. With add_user_id this replaces that user."`
}

// CreateUserOverrideOut is the output of create_user_override.
type CreateUserOverrideOut struct {
	ID           string `json:"id"`
	ScheduleID   string `json:"schedule_id"`
	Start        string `json:"start_time"`
	End          string `json:"end_time"`
	AddUserID    string `json:"add_user_id"`
	RemoveUserID string `json:"remove_user_id"`
}

const createOverrideMutation = `mutation($i: CreateUserOverrideInput!) {
  createUserOverride(input: $i) { id start end addUserID removeUserID }
}`

func (t *Tools) registerOverrides(r *registry) {
	add(r, "create_user_override", "Create a user override on a schedule for a time range. Changes live data. Give add_user_id, remove_user_id or both.", true, t.createUserOverride)
}

func (t *Tools) createUserOverride(ctx context.Context, in CreateUserOverrideIn) (CreateUserOverrideOut, error) {
	if in.ScheduleID == "" {
		return CreateUserOverrideOut{}, errRequired("schedule_id")
	}
	if in.Start == "" || in.End == "" {
		return CreateUserOverrideOut{}, errors.New("start and end times are required")
	}
	if in.AddUserID == "" && in.RemoveUserID == "" {
		return CreateUserOverrideOut{}, errors.New("give add_user_id, remove_user_id or both")
	}
	start, err := parseTime("start", in.Start)
	if err != nil {
		return CreateUserOverrideOut{}, err
	}
	end, err := parseTime("end", in.End)
	if err != nil {
		return CreateUserOverrideOut{}, err
	}
	input := map[string]any{
		"scheduleID": in.ScheduleID,
		"start":      start.UTC().Format(time.RFC3339),
		"end":        end.UTC().Format(time.RFC3339),
	}
	if in.AddUserID != "" {
		input["addUserID"] = in.AddUserID
	}
	if in.RemoveUserID != "" {
		input["removeUserID"] = in.RemoveUserID
	}
	var data struct {
		CreateUserOverride *struct {
			ID           string `json:"id"`
			Start        string `json:"start"`
			End          string `json:"end"`
			AddUserID    string `json:"addUserID"`
			RemoveUserID string `json:"removeUserID"`
		} `json:"createUserOverride"`
	}
	if err := t.gql.Do(ctx, createOverrideMutation, map[string]any{"i": input}, &data); err != nil {
		return CreateUserOverrideOut{}, err
	}
	o := data.CreateUserOverride
	if o == nil {
		return CreateUserOverrideOut{}, errors.New("create user override: the server returned no override")
	}
	return CreateUserOverrideOut{ID: o.ID, ScheduleID: in.ScheduleID, Start: o.Start, End: o.End, AddUserID: o.AddUserID, RemoveUserID: o.RemoveUserID}, nil
}
