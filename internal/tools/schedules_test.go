package tools

import (
	"reflect"
	"strings"
	"testing"
)

func TestListSchedules(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"schedules":{"nodes":[{"id":"s1","name":"Primary","description":"d","timeZone":"UTC"}],"pageInfo":{"endCursor":"c","hasNextPage":true}}}}`))
	out, err := tl.listSchedules(ctx, ListSchedulesIn{Search: "pri", First: 3, After: "a"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schedules":[{"id":"s1","name":"Primary","description":"d","time_zone":"UTC"}],"page_info":{"end_cursor":"c","has_next_page":true}}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s", got)
	}
	if got := input(t, (*calls)[0]); !reflect.DeepEqual(got, map[string]any{"search": "pri", "first": 3.0, "after": "a"}) {
		t.Fatalf("vars = %v", got)
	}
}

const scheduleBody = `{"data":{
 "schedule":{"id":"s1","name":"Primary","description":"d","timeZone":"UTC",
  "shifts":[{"userID":"u1","start":"2026-11-01T12:00:00Z","end":"2026-11-02T12:00:00Z","truncated":true}],
  "targets":[{"target":{"id":"r1","type":"rotation","name":"Weekly"},"rules":[{"start":"00:00","end":"00:00","weekdayFilter":[true,true,true,true,true,true,true]}]}]},
 "userOverrides":{"nodes":[{"id":"o1","start":"2026-11-02T00:00:00Z","end":"2026-11-03T00:00:00Z","addUserID":"u2","removeUserID":""}]}}}`

func TestGetSchedule(t *testing.T) {
	tl, calls := fakeTools(t, static(scheduleBody))
	out, err := tl.getSchedule(ctx, GetScheduleIn{ID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schedule":{"id":"s1","name":"Primary","description":"d","time_zone":"UTC"},"start":"2026-11-01T12:00:00Z","end":"2026-11-08T12:00:00Z",` +
		`"shifts":[{"user_id":"u1","start_time":"2026-11-01T12:00:00Z","end_time":"2026-11-02T12:00:00Z","truncated":true}],` +
		`"rules":[{"target_id":"r1","target_type":"rotation","target_name":"Weekly","start":"00:00","end":"00:00","weekday_filter":[true,true,true,true,true,true,true]}],` +
		`"overrides":[{"id":"o1","start_time":"2026-11-02T00:00:00Z","end_time":"2026-11-03T00:00:00Z","add_user_id":"u2","remove_user_id":""}]}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	v := (*calls)[0].Vars
	ov := v["ov"].(map[string]any)
	if v["id"] != "s1" || v["start"] != "2026-11-01T12:00:00Z" || ov["scheduleID"] != "s1" || ov["first"] != 100.0 {
		t.Fatalf("vars = %v", v)
	}
}

func TestGetSchedule_RangeAndErrors(t *testing.T) {
	tl, calls := fakeTools(t, static(scheduleBody))
	if _, err := tl.getSchedule(ctx, GetScheduleIn{ID: "s1", Start: "2026-11-01", End: "2026-11-03"}); err != nil {
		t.Fatal(err)
	}
	if v := (*calls)[0].Vars; v["start"] != "2026-11-01T00:00:00Z" || v["end"] != "2026-11-03T00:00:00Z" {
		t.Fatalf("vars = %v", v)
	}
	for name, in := range map[string]GetScheduleIn{
		"no id": {}, "end before start": {ID: "s", Start: "2026-11-05", End: "2026-11-01"}, "bad start": {ID: "s", Start: "x"},
	} {
		if _, err := tl.getSchedule(ctx, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	tl, _ = fakeTools(t, static(`{"data":{"schedule":null,"userOverrides":{"nodes":[]}}}`))
	if _, err := tl.getSchedule(ctx, GetScheduleIn{ID: "zz"}); err == nil || !strings.Contains(err.Error(), "schedule not found: zz") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateUserOverride(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"createUserOverride":{"id":"o9","start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00Z","addUserID":"u2","removeUserID":"u1"}}}`))
	out, err := tl.createUserOverride(ctx, CreateUserOverrideIn{ScheduleID: "s1", Start: "2026-11-02T10:00:00+01:00", End: "2026-11-02T18:00:00+01:00", AddUserID: "u2", RemoveUserID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"o9","schedule_id":"s1","start_time":"2026-11-02T09:00:00Z","end_time":"2026-11-02T17:00:00Z","add_user_id":"u2","remove_user_id":"u1"}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s", got)
	}
	wantVars := map[string]any{"scheduleID": "s1", "start": "2026-11-02T09:00:00Z", "end": "2026-11-02T17:00:00Z", "addUserID": "u2", "removeUserID": "u1"}
	if got := input(t, (*calls)[0]); !reflect.DeepEqual(got, wantVars) {
		t.Fatalf("vars = %v", got)
	}
}

func TestCreateUserOverride_InvalidInput(t *testing.T) {
	tl, calls := fakeTools(t, static(`{}`))
	ok := CreateUserOverrideIn{ScheduleID: "s", Start: "2026-11-02", End: "2026-11-03", AddUserID: "u"}
	for name, mod := range map[string]func(*CreateUserOverrideIn){
		"no schedule": func(i *CreateUserOverrideIn) { i.ScheduleID = "" },
		"no end":      func(i *CreateUserOverrideIn) { i.End = "" },
		"no users":    func(i *CreateUserOverrideIn) { i.AddUserID = "" },
		"bad start":   func(i *CreateUserOverrideIn) { i.Start = "tomorrow" },
	} {
		in := ok
		mod(&in)
		if _, err := tl.createUserOverride(ctx, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("invalid input must not reach the server")
	}
}
