package tools

import (
	"reflect"
	"strings"
	"testing"
)

func TestListServices(t *testing.T) {
	body := `{"data":{"services":{"nodes":[
 {"id":"a","name":"API","description":"d","escalationPolicyID":"ep1","maintenanceExpiresAt":"2026-11-01T13:00:00Z"},
 {"id":"b","name":"Web","description":"","escalationPolicyID":"ep2","maintenanceExpiresAt":"2026-11-01T11:00:00Z"},
 {"id":"c","name":"DB","description":"","escalationPolicyID":"ep3","maintenanceExpiresAt":null}],
 "pageInfo":{"endCursor":null,"hasNextPage":false}}}}`
	tl, calls := fakeTools(t, static(body))
	out, err := tl.listServices(ctx, ListServicesIn{Search: "x"})
	if err != nil {
		t.Fatal(err)
	}
	got := []bool{out.Services[0].IsMaintenance, out.Services[1].IsMaintenance, out.Services[2].IsMaintenance}
	if !reflect.DeepEqual(got, []bool{true, false, false}) {
		t.Fatalf("maintenance flags = %v", got)
	}
	if out.Services[0].EscalationPolicyID != "ep1" || out.PageInfo.HasNextPage {
		t.Fatalf("out = %+v", out)
	}
	if v := input(t, (*calls)[0]); !reflect.DeepEqual(v, map[string]any{"search": "x", "first": 15.0, "after": ""}) {
		t.Fatalf("vars = %v", v)
	}
}

func TestSearchUsers(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"users":{"nodes":[{"id":"u1","name":"Ann","email":"a@x.io","role":"admin"}],"pageInfo":{"endCursor":"c","hasNextPage":true}}}}`))
	out, err := tl.searchUsers(ctx, SearchUsersIn{Search: "ann", First: 200})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"users":[{"id":"u1","name":"Ann","email":"a@x.io","role":"admin"}],"page_info":{"end_cursor":"c","has_next_page":true}}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s", got)
	}
	if v := input(t, (*calls)[0]); v["first"] != 100.0 || v["search"] != "ann" {
		t.Fatalf("vars = %v", v)
	}
}

func TestGetUser(t *testing.T) {
	body := `{"data":{"user":{"id":"u1","name":"Ann","email":"a@x.io","role":"user",
 "contactMethods":[{"id":"cm1","name":"Phone","disabled":false,"dest":{"type":"builtin-twilio-sms","args":{"phone_number":"+1555"}}}],
 "onCallOverview":{"serviceAssignments":[{"serviceID":"s1","serviceName":"API","escalationPolicyID":"ep1","escalationPolicyName":"Default","stepNumber":1}]}}}}`
	tl, calls := fakeTools(t, static(body))
	out, err := tl.getUser(ctx, GetUserIn{ID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"user":{"id":"u1","name":"Ann","email":"a@x.io","role":"user"},` +
		`"contact_methods":[{"id":"cm1","name":"Phone","type":"builtin-twilio-sms","args":{"phone_number":"+1555"},"disabled":false}],` +
		`"on_call":[{"service_id":"s1","service_name":"API","escalation_policy_id":"ep1","escalation_policy_name":"Default","step":2}]}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if (*calls)[0].Vars["id"] != "u1" {
		t.Fatalf("vars = %v", (*calls)[0].Vars)
	}
}

func TestGetUser_Errors(t *testing.T) {
	tl, _ := fakeTools(t, static(`{"data":{"user":null}}`))
	if _, err := tl.getUser(ctx, GetUserIn{ID: "x"}); err == nil || !strings.Contains(err.Error(), "user not found: x") {
		t.Fatalf("err = %v", err)
	}
	if _, err := tl.getUser(ctx, GetUserIn{}); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestWhoAmI(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"user":{"id":"u1","name":"Ann","email":"a@x.io","role":"admin"}}}`))
	out, err := tl.whoami(ctx, WhoAmIIn{})
	if err != nil || out.ID != "u1" || out.Role != "admin" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if len((*calls)[0].Vars) != 0 {
		t.Fatalf("whoami must not pass an id, vars = %v", (*calls)[0].Vars)
	}
	tl, _ = fakeTools(t, static(`{"data":{"user":null}}`))
	if _, err := tl.whoami(ctx, WhoAmIIn{}); err == nil {
		t.Fatal("expected error when no current user")
	}
}

func TestWhoAmI_ViewerAndOwnerRoles(t *testing.T) {
	for _, role := range []string{"viewer", "owner"} {
		tl, _ := fakeTools(t, static(`{"data":{"user":{"id":"u1","name":"Ann","email":"a@x.io","role":"`+role+`"}}}`))
		out, err := tl.whoami(ctx, WhoAmIIn{})
		if err != nil || out.Role != role {
			t.Fatalf("role %s: out=%+v err=%v", role, out, err)
		}
	}
}
