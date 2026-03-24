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
	"345": graph.User{
		ID:          "345",
		DisplayName: "The Burned Man",
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
			{
				ID:   "420",
				Text: "test comment 2",
				User: User{
					Id:    "345",
					Name:  "Joshua Graham",
					Email: "jgraham@microsoft.com",
				},
			},
		},
	}

	task.AddUsers(testUsers)
	require.Equal(t, "Harald", task.CompletedBy.Name)
	require.Equal(t, "hhardrada@microsoft.com", task.CompletedBy.Email)
	require.Equal(t, "123", task.Comments[0].User.Id)
	require.Equal(t, "The Burned Man", task.Comments[1].User.Name)
	require.Equal(t, "jgraham@microsoft.com", task.Comments[1].User.Email)
}
