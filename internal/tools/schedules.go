package tools

import (
	"context"
	"errors"
	"time"
)

// ListSchedulesIn is the input of list_schedules.
type ListSchedulesIn struct {
	Search string `json:"search,omitempty" jsonschema:"Optional search term to filter schedules by name or description."`
	First  int    `json:"first,omitempty" jsonschema:"Maximum number of schedules to return (default 15, max 100)."`
	After  string `json:"after,omitempty" jsonschema:"Pagination cursor (page_info.end_cursor)."`
}

// ScheduleDTO is a schedule as returned to MCP clients.
type ScheduleDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TimeZone    string `json:"time_zone"`
}

type gqlScheduleBase struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TimeZone    string `json:"timeZone"`
}

func (b gqlScheduleBase) dto() ScheduleDTO {
	return ScheduleDTO{ID: b.ID, Name: b.Name, Description: b.Description, TimeZone: b.TimeZone}
}

// ListSchedulesOut is the output of list_schedules.
type ListSchedulesOut struct {
	Schedules []ScheduleDTO `json:"schedules"`
	PageInfo  PageInfo      `json:"page_info"`
}

// GetScheduleIn is the input of get_schedule.
type GetScheduleIn struct {
	ID    string `json:"id" jsonschema:"ID of the schedule."`
	Start string `json:"start,omitempty" jsonschema:"Start of the window (default now). RFC 3339 with zone (2026-11-01T09:00:00Z) or a date (2026-11-01, UTC)."`
	End   string `json:"end,omitempty" jsonschema:"End of the window (default 7 days after start). Same formats as start."`
}

// ShiftDTO is an on-call shift.
type ShiftDTO struct {
	UserID    string `json:"user_id"`
	Start     string `json:"start_time"`
	End       string `json:"end_time"`
	Truncated bool   `json:"truncated"`
}

// RuleDTO is one schedule rule: a time window on some weekdays for a rotation or user.
type RuleDTO struct {
	TargetID      string `json:"target_id"`
	TargetType    string `json:"target_type"`
	TargetName    string `json:"target_name"`
	Start         string `json:"start"`
	End           string `json:"end"`
	WeekdayFilter []bool `json:"weekday_filter"` // 7 items, Sunday first
}

// OverrideDTO is a user override.
type OverrideDTO struct {
	ID           string `json:"id"`
	Start        string `json:"start_time"`
	End          string `json:"end_time"`
	AddUserID    string `json:"add_user_id"`
	RemoveUserID string `json:"remove_user_id"`
}

// GetScheduleOut is the output of get_schedule.
type GetScheduleOut struct {
	Schedule  ScheduleDTO   `json:"schedule"`
	Start     string        `json:"start"`
	End       string        `json:"end"`
	Shifts    []ShiftDTO    `json:"shifts"`
	Rules     []RuleDTO     `json:"rules"`
	Overrides []OverrideDTO `json:"overrides"`
}

const listSchedulesQuery = `query($i: ScheduleSearchOptions) {
  schedules(input: $i) {
    nodes { id name description timeZone }
    pageInfo { endCursor hasNextPage }
  }
}`

const getScheduleQuery = `query($id: ID!, $start: ISOTimestamp!, $end: ISOTimestamp!, $ov: UserOverrideSearchOptions) {
  schedule(id: $id) {
    id name description timeZone
    shifts(start: $start, end: $end) { userID start end truncated }
    targets { target { id type name } rules { start end weekdayFilter } }
  }
  userOverrides(input: $ov) { nodes { id start end addUserID removeUserID } }
}`

func (t *Tools) registerSchedules(r *registry) {
	add(r, "list_schedules", "List on-call schedules with search filtering and pagination.", false, t.listSchedules)
	add(r, "get_schedule", "Get details, shift assignments, schedule rules, and overrides for a schedule in a time window (default: next 7 days).", false, t.getSchedule)
	t.registerOverrides(r)
}

func (t *Tools) listSchedules(ctx context.Context, in ListSchedulesIn) (ListSchedulesOut, error) {
	vars := map[string]any{"i": map[string]any{"search": in.Search, "first": clampFirst(in.First), "after": in.After}}
	var data struct {
		Schedules struct {
			Nodes    []gqlScheduleBase `json:"nodes"`
			PageInfo gqlPage           `json:"pageInfo"`
		} `json:"schedules"`
	}
	if err := t.gql.Do(ctx, listSchedulesQuery, vars, &data); err != nil {
		return ListSchedulesOut{}, err
	}
	nodes := make([]ScheduleDTO, 0, len(data.Schedules.Nodes))
	for _, n := range data.Schedules.Nodes {
		nodes = append(nodes, n.dto())
	}
	return ListSchedulesOut{Schedules: nodes, PageInfo: data.Schedules.PageInfo.convert()}, nil
}

// timeRange reads the optional start and end. start defaults to now, end to start + 7 days.
func (t *Tools) timeRange(startStr, endStr string) (start, end time.Time, err error) {
	start = t.now()
	if startStr != "" {
		if start, err = parseTime("start", startStr); err != nil {
			return
		}
	}
	end = start.Add(7 * 24 * time.Hour)
	if endStr != "" {
		if end, err = parseTime("end", endStr); err != nil {
			return
		}
	}
	if !end.After(start) {
		err = errors.New("end must be after start")
	}
	return
}

type gqlSchedule struct {
	gqlScheduleBase
	Shifts []struct {
		UserID    string `json:"userID"`
		Start     string `json:"start"`
		End       string `json:"end"`
		Truncated bool   `json:"truncated"`
	} `json:"shifts"`
	Targets []struct {
		Target struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"target"`
		Rules []struct {
			Start         string `json:"start"`
			End           string `json:"end"`
			WeekdayFilter []bool `json:"weekdayFilter"`
		} `json:"rules"`
	} `json:"targets"`
}

func (t *Tools) getSchedule(ctx context.Context, in GetScheduleIn) (GetScheduleOut, error) {
	if in.ID == "" {
		return GetScheduleOut{}, errRequired("id")
	}
	start, end, err := t.timeRange(in.Start, in.End)
	if err != nil {
		return GetScheduleOut{}, err
	}
	s, e := start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)
	vars := map[string]any{
		"id": in.ID, "start": s, "end": e,
		"ov": map[string]any{"scheduleID": in.ID, "start": s, "end": e, "first": maxFirst},
	}
	var data struct {
		Schedule      *gqlSchedule `json:"schedule"`
		UserOverrides struct {
			Nodes []struct {
				ID           string `json:"id"`
				Start        string `json:"start"`
				End          string `json:"end"`
				AddUserID    string `json:"addUserID"`
				RemoveUserID string `json:"removeUserID"`
			} `json:"nodes"`
		} `json:"userOverrides"`
	}
	if err := t.gql.Do(ctx, getScheduleQuery, vars, &data); err != nil {
		return GetScheduleOut{}, err
	}
	if data.Schedule == nil {
		return GetScheduleOut{}, notFound("schedule", in.ID)
	}
	out := GetScheduleOut{Schedule: data.Schedule.dto(), Start: s, End: e,
		Shifts: []ShiftDTO{}, Rules: []RuleDTO{}, Overrides: []OverrideDTO{}}
	for _, sh := range data.Schedule.Shifts {
		out.Shifts = append(out.Shifts, ShiftDTO{UserID: sh.UserID, Start: sh.Start, End: sh.End, Truncated: sh.Truncated})
	}
	for _, tg := range data.Schedule.Targets {
		for _, r := range tg.Rules {
			wf := r.WeekdayFilter
			if wf == nil {
				wf = []bool{}
			}
			out.Rules = append(out.Rules, RuleDTO{TargetID: tg.Target.ID, TargetType: tg.Target.Type, TargetName: tg.Target.Name,
				Start: r.Start, End: r.End, WeekdayFilter: wf})
		}
	}
	for _, o := range data.UserOverrides.Nodes {
		out.Overrides = append(out.Overrides, OverrideDTO{ID: o.ID, Start: o.Start, End: o.End, AddUserID: o.AddUserID, RemoveUserID: o.RemoveUserID})
	}
	return out, nil
}
