package main

import (
	"testing"
	"time"

	"github.com/alamo-ds/msgraph/graph"
	"github.com/s-hammon/p"
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

func TestNewTaskFromGraph(t *testing.T) {
	now := time.Now().UTC()
	gt := graph.Task{
		ID:                   "task-1",
		PlanID:               "plan-1",
		BucketID:             "bucket-1",
		Title:                "Test Task",
		PercentComplete:      100,
		StartDateTime:        now,
		CreatedDateTime:      now,
		DueDateTime:          now,
		CompletedDateTime:    now,
		ConversationThreadID: "thread-1",
		AppliedCategories:    map[string]bool{"category1": true, "category2": true},
		CompletedBy:          graph.IdentitySet{User: graph.Identity{ID: "user-1", DisplayName: "User One"}},
		CreatedBy:            graph.IdentitySet{User: graph.Identity{ID: "user-2", DisplayName: "User Two"}},
		Assignments: map[string]graph.Assignment{
			"user-3": {AssignedBy: graph.IdentitySet{User: graph.Identity{ID: "user-4", DisplayName: "User Four"}}},
		},
	}

	task := NewTaskFromGraph(gt)
	require.True(t, task.Completed)
	require.ElementsMatch(t, []string{"category1", "category2"}, task.Labels)
	require.Equal(t, User{Id: "user-1", Name: "User One"}, task.CompletedBy)
	require.Equal(t, User{Id: "user-2", Name: "User Two"}, task.CreatedBy)
	require.True(t, now.Before(task.SnapshotDateTime))
}

func TestTask_AddDetails(t *testing.T) {
	task := Task{}
	now := time.Now()
	details := graph.TaskDetails{
		References: map[string]graph.ExternalReference{
			"ref-1": {
				Alias:                "document.pdf",
				Type:                 "pdf",
				LastModifiedDateTime: now,
			},
		},
		Checklist: map[string]graph.ChecklistItem{
			"check-1": {Title: "Step 1", IsChecked: true, LastModifiedDateTime: now},
		},
	}

	task.AddDetails(details)
	require.Len(t, task.Attachments, 1)
	require.Equal(t, "document.pdf", task.Attachments[0].Name)
	require.Equal(t, "pdf", task.Attachments[0].Type)
	require.Len(t, task.ChecklistItems, 1)
	require.Equal(t, "Step 1", task.ChecklistItems[0].Title)
	require.True(t, task.ChecklistItems[0].IsChecked)
}

func TestUsers(t *testing.T) {
	u := make(users)
	gu := graph.User{ID: "user-123", UserPrincipalName: "test@microsoft.com", DisplayName: "Test User"}

	u.add(gu)

	require.Equal(t, 2, len(u))
	require.ElementsMatch(t, []string{"user-123", "test@microsoft.com"}, p.Keys(u))

	got1 := u.get(&User{Id: "user-123"})
	require.Equal(t, "test@microsoft.com", got1.UserPrincipalName)
	got2 := u.get(&User{Email: "test@microsoft.com"})
	require.Equal(t, "Test User", got2.DisplayName)
	got3 := u.get(&User{Id: "unknown"})
	require.Empty(t, got3)
}
