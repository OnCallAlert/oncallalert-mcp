package tools

import (
	"strings"
	"testing"
)

func TestGetOnCall_Service(t *testing.T) {
	tl, calls := fakeTools(t, static(`{"data":{"service":{"onCallUsers":[{"userID":"u1","userName":"Ann","stepNumber":0},{"userID":"u2","userName":"Bob","stepNumber":0},{"userID":"u3","userName":"Cy","stepNumber":1}]}}}`))
	out, err := tl.getOnCall(ctx, GetOnCallIn{ServiceID: "svc"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"service_id":"svc","steps":[{"step":1,"users":[{"id":"u1","name":"Ann"},{"id":"u2","name":"Bob"}]},{"step":2,"users":[{"id":"u3","name":"Cy"}]}]}`
	if got := mustJSON(t, out); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if (*calls)[0].Vars["id"] != "svc" {
		t.Fatalf("vars = %v", (*calls)[0].Vars)
	}
}

func TestGetOnCall_PolicyUsesAssignedService(t *testing.T) {
	tl, calls := fakeTools(t, func(c call) string {
		if strings.Contains(c.Query, "escalationPolicy") {
			return `{"data":{"escalationPolicy":{"assignedTo":[{"id":"x","type":"user"},{"id":"svc9","type":"service"}]}}}`
		}
		return `{"data":{"service":{"onCallUsers":[{"userID":"u1","userName":"Ann","stepNumber":0}]}}}`
	})
	out, err := tl.getOnCall(ctx, GetOnCallIn{EscalationPolicyID: "ep"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || (*calls)[1].Vars["id"] != "svc9" || out.EscalationPolicyID != "ep" || len(out.Steps) != 1 {
		t.Fatalf("calls=%v out=%+v", *calls, out)
	}
	tl, _ = fakeTools(t, static(`{"data":{"escalationPolicy":{"assignedTo":[]}}}`))
	if _, err := tl.getOnCall(ctx, GetOnCallIn{EscalationPolicyID: "ep"}); err == nil {
		t.Fatal("expected error for unused policy")
	}
}

func TestGetOnCall_Schedule(t *testing.T) {
	body := `{"data":{"schedule":{"shifts":[{"userID":"u1","user":{"name":"Ann"}},{"userID":"u1","user":{"name":"Ann"}}]}}}`
	tl, calls := fakeTools(t, static(body))
	out, err := tl.getOnCall(ctx, GetOnCallIn{ScheduleID: "sch"})
	if err != nil {
		t.Fatal(err)
	}
	if got := mustJSON(t, out); got != `{"schedule_id":"sch","users":[{"id":"u1","name":"Ann"}]}` {
		t.Fatalf("got %s", got)
	}
	v := (*calls)[0].Vars
	if v["start"] != "2026-11-01T12:00:00Z" || v["end"] != "2026-11-01T12:00:01Z" {
		t.Fatalf("vars = %v", v)
	}

	out, err = tl.getOnCall(ctx, GetOnCallIn{ScheduleID: "sch", At: "2026-12-25"})
	if err != nil || out.At != "2026-12-25T00:00:00Z" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if v := (*calls)[1].Vars; v["start"] != "2026-12-25T00:00:00Z" {
		t.Fatalf("vars = %v", v)
	}
}

func TestGetOnCall_InvalidInput(t *testing.T) {
	tl, calls := fakeTools(t, static(`{}`))
	for name, in := range map[string]GetOnCallIn{
		"none":       {},
		"two":        {ServiceID: "a", ScheduleID: "b"},
		"at+service": {ServiceID: "a", At: "2026-01-01"},
		"bad at":     {ScheduleID: "b", At: "soon"},
	} {
		if _, err := tl.getOnCall(ctx, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("invalid input must not reach the server")
	}
}

func TestGetOnCall_NotFound(t *testing.T) {
	tl, _ := fakeTools(t, static(`{"data":{"service":null,"schedule":null}}`))
	for _, in := range []GetOnCallIn{{ServiceID: "a"}, {ScheduleID: "b"}} {
		if _, err := tl.getOnCall(ctx, in); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("%+v: err = %v", in, err)
		}
	}
}
