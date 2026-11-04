package tools

import (
	"reflect"
	"strings"
	"testing"
)

const alertNode = `{"alertID":7,"summary":"disk full","status":"StatusUnacknowledged","serviceID":"s1","createdAt":"2026-11-01T10:00:00Z"}`

func TestListAlerts_Variables(t *testing.T) {
	cases := []struct {
		name string
		in   ListAlertsIn
		want map[string]any
	}{
		{"defaults", ListAlertsIn{}, map[string]any{"first": 15.0, "after": "", "search": "", "sort": "dateID"}},
		{"cap first", ListAlertsIn{First: 500}, map[string]any{"first": 100.0, "after": "", "search": "", "sort": "dateID"}},
		{"all filters", ListAlertsIn{ServiceID: "s1", Status: "acknowledged", Since: "2026-10-01", Until: "2026-11-01T00:00:00+02:00", Search: "x", First: 5, After: "cur"},
			map[string]any{"first": 5.0, "after": "cur", "search": "x", "sort": "dateID",
				"filterByServiceID": []any{"s1"}, "filterByStatus": []any{"StatusAcknowledged"},
				"notCreatedBefore": "2026-10-01T00:00:00Z", "createdBefore": "2026-10-31T22:00:00Z"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tl, calls := fakeTools(t, static(`{"data":{"alerts":{"nodes":[],"pageInfo":{"endCursor":null,"hasNextPage":false}}}}`))
			if _, err := tl.listAlerts(ctx, tc.in); err != nil {
				t.Fatal(err)
			}
			if got := input(t, (*calls)[0]); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("vars = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListAlerts_Output(t *testing.T) {
	tl, _ := fakeTools(t, static(`{"data":{"alerts":{"nodes":[`+alertNode+`],"pageInfo":{"endCursor":"c2","hasNextPage":true}}}}`))
	out, err := tl.listAlerts(ctx, ListAlertsIn{})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"alerts":[{"id":7,"summary":"disk full","status":"open","service_id":"s1","created_at":"2026-11-01T10:00:00Z"}],"page_info":{"end_cursor":"c2","has_next_page":true}}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestListAlerts_InvalidInput(t *testing.T) {
	tl, calls := fakeTools(t, static(`{}`))
	for _, in := range []ListAlertsIn{{Status: "bogus"}, {Since: "yesterday"}, {Until: "nope"}} {
		if _, err := tl.listAlerts(ctx, in); err == nil {
			t.Errorf("%+v: expected error", in)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("invalid input must not reach the server")
	}
}

func TestGetAlert(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"alert":{"alertID":7,"summary":"disk full","status":"StatusClosed","serviceID":"s1","createdAt":"2026-11-01T10:00:00Z","details":"d","service":{"name":"API"},"recentEvents":{"nodes":[{"timestamp":"2026-11-01T11:00:00Z","message":"closed"}]}}}}`))
	out, err := tl.getAlert(ctx, GetAlertIn{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].Vars["id"] != 7.0 || (*calls)[0].Vars["limit"] != 50.0 {
		t.Fatalf("vars = %v", (*calls)[0].Vars)
	}
	want := `{"alert":{"id":7,"summary":"disk full","status":"closed","service_id":"s1","created_at":"2026-11-01T10:00:00Z"},"details":"d","service_name":"API","events":[{"message":"closed","time":"2026-11-01T11:00:00Z"}]}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestGetAlert_NotFoundAndBadID(t *testing.T) {
	tl, _ := fakeTools(t, static(`{"data":{"alert":null}}`))
	if _, err := tl.getAlert(ctx, GetAlertIn{ID: 9}); err == nil || !strings.Contains(err.Error(), "alert not found: 9") {
		t.Fatalf("err = %v", err)
	}
	if _, err := tl.getAlert(ctx, GetAlertIn{}); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestSetAlertStatus(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"updateAlerts":[{"alertID":1},{"alertID":3}]}}`))
	out, err := tl.setAlertStatus(ctx, AlertIDsIn{IDs: []int{1, 2, 3}}, "StatusAcknowledged")
	if err != nil {
		t.Fatal(err)
	}
	if got := mustJSON(t, out); got != `{"updated_ids":[1,3],"skipped_ids":[2]}` {
		t.Fatalf("got %s", got)
	}
	in := input(t, (*calls)[0])
	if in["newStatus"] != "StatusAcknowledged" || !reflect.DeepEqual(in["alertIDs"], []any{1.0, 2.0, 3.0}) {
		t.Fatalf("vars = %v", in)
	}
}

func TestSetAlertStatus_NullResultAndLimits(t *testing.T) {
	tl, _ := fakeTools(t, static(`{"data":{"updateAlerts":null}}`))
	out, err := tl.setAlertStatus(ctx, AlertIDsIn{IDs: []int{5}}, "StatusClosed")
	if err != nil || mustJSON(t, out) != `{"updated_ids":[],"skipped_ids":[5]}` {
		t.Fatalf("out=%s err=%v", mustJSON(t, out), err)
	}
	if _, err := tl.setAlertStatus(ctx, AlertIDsIn{}, "StatusClosed"); err == nil {
		t.Error("empty ids: expected error")
	}
	if _, err := tl.setAlertStatus(ctx, AlertIDsIn{IDs: make([]int, 51)}, "StatusClosed"); err == nil {
		t.Error("51 ids: expected error")
	}
}

func TestEscalateAlert(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"escalateAlerts":[{"alertID":4}]}}`))
	out, err := tl.escalateAlert(ctx, EscalateAlertIn{ID: 4})
	if err != nil || out.EscalatedID != 4 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !reflect.DeepEqual((*calls)[0].Vars["ids"], []any{4.0}) {
		t.Fatalf("vars = %v", (*calls)[0].Vars)
	}
	tl, _ = fakeTools(t, static(`{"data":{"escalateAlerts":[]}}`))
	if _, err := tl.escalateAlert(ctx, EscalateAlertIn{ID: 4}); err == nil {
		t.Fatal("expected error when nothing was escalated")
	}
}

func TestCreateAlert(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"createAlert":`+alertNode+`}}`))
	out, err := tl.createAlert(ctx, CreateAlertIn{ServiceID: "s1", Summary: "disk full", Dedup: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Alert.ID != 7 || out.Alert.Status != "open" {
		t.Fatalf("out = %+v", out)
	}
	want := map[string]any{"serviceID": "s1", "summary": "disk full", "dedup": "k"}
	if got := input(t, (*calls)[0]); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars = %v, want %v", got, want)
	}
	if _, err := tl.createAlert(ctx, CreateAlertIn{Summary: "x"}); err == nil {
		t.Fatal("expected error without service_id")
	}
}

func TestMutations_PassServerErrors(t *testing.T) {
	tl, _ := fakeTools(t, static(`{"errors":[{"message":"forbidden: write scope required"}]}`))
	_, err := tl.createAlert(ctx, CreateAlertIn{ServiceID: "s", Summary: "x"})
	if err == nil || !strings.Contains(err.Error(), "write scope required") {
		t.Fatalf("err = %v", err)
	}
}
