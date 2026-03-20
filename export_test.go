package main

import (
	"testing"

	"github.com/alamo-ds/msgraph/graph"
	"github.com/stretchr/testify/require"
)

var testUsers = users{
	"abc": graph.User{
		ID:                "abc",
		DisplayName:       "Harald",
		UserPrincipalName: "hhardrada@microsoft.com",
	},
	"frejya@microsoft.com": graph.User{
		ID:                "123",
		DisplayName:       "Frejya",
		UserPrincipalName: "frejya@microsoft.com",
	},
}

func TestTask_AddUsers(t *testing.T) {
	task := &Task{
		CompletedBy: User{
			Id:    "abc",
			Name:  "Sven",
			Email: "sven@microsoft.com",
		},
		Comments: []Comment{
			{
				ID:   "69",
				Text: "test comment",
				User: User{
					Email: "frejya@microsoft.com",
				},
			},
		},
	}

	task.AddUsers(testUsers)
	require.Equal(t, "Harald", task.CompletedBy.Name)
	require.Equal(t, "hhardrada@microsoft.com", task.CompletedBy.Email)
	require.Equal(t, "123", task.Comments[0].User.Id)
}
