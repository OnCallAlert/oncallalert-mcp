package tools

import (
	"context"
	"errors"
)

// SearchUsersIn is the input of search_users.
type SearchUsersIn struct {
	Search string `json:"search,omitempty" jsonschema:"Match against user names (email is not searched)."`
	First  int    `json:"first,omitempty" jsonschema:"Maximum number of users to return (default 15, max 100)."`
	After  string `json:"after,omitempty" jsonschema:"Pagination cursor (page_info.end_cursor)."`
}

// UserDTO is a user as returned to MCP clients.
type UserDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// SearchUsersOut is the output of search_users.
type SearchUsersOut struct {
	Users    []UserDTO `json:"users"`
	PageInfo PageInfo  `json:"page_info"`
}

// GetUserIn is the input of get_user.
type GetUserIn struct {
	ID string `json:"id" jsonschema:"User ID."`
}

// ContactMethodDTO is a contact method as returned to MCP clients.
type ContactMethodDTO struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Args     map[string]string `json:"args"`
	Disabled bool              `json:"disabled"`
}

// OnCallAssignment is one service escalation step a user is on call for.
type OnCallAssignment struct {
	ServiceID            string `json:"service_id"`
	ServiceName          string `json:"service_name"`
	EscalationPolicyID   string `json:"escalation_policy_id"`
	EscalationPolicyName string `json:"escalation_policy_name"`
	Step                 int    `json:"step"` // 1 = first step
}

// GetUserOut is the output of get_user.
type GetUserOut struct {
	User           UserDTO            `json:"user"`
	ContactMethods []ContactMethodDTO `json:"contact_methods"`
	OnCall         []OnCallAssignment `json:"on_call"`
}

// WhoAmIIn is the (empty) input of whoami.
type WhoAmIIn struct{}

// WhoAmIOut is the output of whoami.
type WhoAmIOut = UserDTO

const searchUsersQuery = `query($i: UserSearchOptions) {
  users(input: $i) {
    nodes { id name email role }
    pageInfo { endCursor hasNextPage }
  }
}`

const getUserQuery = `query($id: ID) {
  user(id: $id) {
    id name email role
    contactMethods { id name disabled dest { type args } }
    onCallOverview {
      serviceAssignments { serviceID serviceName escalationPolicyID escalationPolicyName stepNumber }
    }
  }
}`

const whoamiQuery = `query { user { id name email role } }`

func (t *Tools) registerUsers(r *registry) {
	add(r, "search_users", "Search users by name, sorted by name.", false, t.searchUsers)
	add(r, "get_user", "Get a user with their contact methods and the services they are on call for now.", false, t.getUser)
	add(r, "whoami", "The user this MCP server acts as (the owner of the API token), with their role.", false, t.whoami)
}

func (t *Tools) searchUsers(ctx context.Context, in SearchUsersIn) (SearchUsersOut, error) {
	vars := map[string]any{"i": map[string]any{"search": in.Search, "first": clampFirst(in.First), "after": in.After}}
	var data struct {
		Users struct {
			Nodes    []UserDTO `json:"nodes"`
			PageInfo gqlPage   `json:"pageInfo"`
		} `json:"users"`
	}
	if err := t.gql.Do(ctx, searchUsersQuery, vars, &data); err != nil {
		return SearchUsersOut{}, err
	}
	users := data.Users.Nodes
	if users == nil {
		users = []UserDTO{}
	}
	return SearchUsersOut{Users: users, PageInfo: data.Users.PageInfo.convert()}, nil
}

func (t *Tools) getUser(ctx context.Context, in GetUserIn) (GetUserOut, error) {
	if in.ID == "" {
		return GetUserOut{}, errRequired("id")
	}
	var data struct {
		User *struct {
			UserDTO
			ContactMethods []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Disabled bool   `json:"disabled"`
				Dest     struct {
					Type string            `json:"type"`
					Args map[string]string `json:"args"`
				} `json:"dest"`
			} `json:"contactMethods"`
			OnCallOverview struct {
				ServiceAssignments []struct {
					ServiceID            string `json:"serviceID"`
					ServiceName          string `json:"serviceName"`
					EscalationPolicyID   string `json:"escalationPolicyID"`
					EscalationPolicyName string `json:"escalationPolicyName"`
					StepNumber           int    `json:"stepNumber"`
				} `json:"serviceAssignments"`
			} `json:"onCallOverview"`
		} `json:"user"`
	}
	if err := t.gql.Do(ctx, getUserQuery, map[string]any{"id": in.ID}, &data); err != nil {
		return GetUserOut{}, err
	}
	u := data.User
	if u == nil {
		return GetUserOut{}, notFound("user", in.ID)
	}
	out := GetUserOut{User: u.UserDTO, ContactMethods: []ContactMethodDTO{}, OnCall: []OnCallAssignment{}}
	for _, cm := range u.ContactMethods {
		args := cm.Dest.Args
		if args == nil {
			args = map[string]string{}
		}
		out.ContactMethods = append(out.ContactMethods, ContactMethodDTO{ID: cm.ID, Name: cm.Name, Type: cm.Dest.Type, Args: args, Disabled: cm.Disabled})
	}
	for _, a := range u.OnCallOverview.ServiceAssignments {
		out.OnCall = append(out.OnCall, OnCallAssignment{
			ServiceID: a.ServiceID, ServiceName: a.ServiceName,
			EscalationPolicyID: a.EscalationPolicyID, EscalationPolicyName: a.EscalationPolicyName, Step: a.StepNumber + 1,
		})
	}
	return out, nil
}

func (t *Tools) whoami(ctx context.Context, _ WhoAmIIn) (WhoAmIOut, error) {
	var data struct {
		User *UserDTO `json:"user"`
	}
	if err := t.gql.Do(ctx, whoamiQuery, nil, &data); err != nil {
		return WhoAmIOut{}, err
	}
	if data.User == nil {
		return WhoAmIOut{}, errors.New("the server returned no current user for this token")
	}
	return *data.User, nil
}
