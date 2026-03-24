package main

import (
	"time"

	"github.com/alamo-ds/msgraph/graph"
	"github.com/s-hammon/p"
)

// TODO: label enums??
type Task struct {
	ID                   string          `json:"id"`
	PlanID               string          `json:"planId"`
	BucketID             string          `json:"bucketId"`
	Name                 string          `json:"name"`
	Notes                string          `json:"notes"`
	ConversationThreadID string          `json:"conversationThreadId"`
	Completed            bool            `json:"completed"`
	StartDateTime        time.Time       `json:"startDateTime"`
	CreatedDateTime      time.Time       `json:"createdDateTime"`
	DueDateTime          time.Time       `json:"dueDateTime"`
	CompletedDateTime    time.Time       `json:"completedDateTime"`
	SnapshotDateTime     time.Time       `json:"snapshotDateTime"`
	CompletedBy          User            `json:"completedBy"`
	CreatedBy            User            `json:"createdBy"`
	Labels               []string        `json:"labels"`
	Attachments          []Attachment    `json:"attachments"`
	ChecklistItems       []ChecklistItem `json:"checklistItems"`
	Comments             []Comment       `json:"comments"`
	AssignedTo           User            `json:"assignedTo"`
	AssignedBy           User            `json:"assignedBy"`
}

func NewTaskFromGraph(task graph.Task) *Task {
	var assignedBy, assignedTo User

	for id, assignment := range task.Assignments {
		if !p.IsZero(assignment) {
			assignedTo = User{Id: id}
			assignedBy = NewUserFromIdentitySet(assignment.AssignedBy)
			break
		}
	}

	return &Task{
		ID:                   task.ID,
		PlanID:               task.PlanID,
		BucketID:             task.BucketID,
		Name:                 task.Title,
		Completed:            task.PercentComplete == 100,
		StartDateTime:        task.StartDateTime,
		CreatedDateTime:      task.CreatedDateTime,
		DueDateTime:          task.DueDateTime,
		CompletedDateTime:    task.CompletedDateTime,
		SnapshotDateTime:     time.Now().UTC(),
		ConversationThreadID: task.ConversationThreadID,
		Labels:               p.Keys(task.AppliedCategories),
		CompletedBy:          NewUserFromIdentitySet(task.CompletedBy),
		CreatedBy:            NewUserFromIdentitySet(task.CreatedBy),
		AssignedTo:           assignedTo,
		AssignedBy:           assignedBy,
	}
}

func (t *Task) AddDetails(details graph.TaskDetails) *Task {
	t.Notes = details.Description

	t.Attachments = make([]Attachment, 0, len(details.References))
	for ref, attachment := range details.References {
		t.Attachments = append(t.Attachments, Attachment{
			Ref:                  ref,
			Name:                 attachment.Alias,
			Type:                 attachment.Type,
			LastModifiedDateTime: attachment.LastModifiedDateTime,
			LastModifiedBy:       NewUserFromIdentitySet(attachment.LastModifiedBy),
		})
	}

	t.ChecklistItems = make([]ChecklistItem, 0, len(details.Checklist))
	for id, item := range details.Checklist {
		t.ChecklistItems = append(t.ChecklistItems, ChecklistItem{
			ID:                   id,
			Title:                item.Title,
			IsChecked:            item.IsChecked,
			LastModifiedDateTime: item.LastModifiedDateTime,
			LastModifiedBy:       NewUserFromIdentitySet(item.LastModifiedBy),
		})
	}

	return t
}

func (t *Task) AddComments(posts []graph.Post) *Task {
	t.Comments = make([]Comment, 0, len(posts))
	for _, post := range posts {
		t.Comments = append(t.Comments, NewCommentFromGraph(post))
	}

	return t
}

type users map[string]graph.User

// Add both the ID and principal name (email) to the list of users.
func (u users) add(user graph.User) {
	if user.ID != "" {
		u[user.ID] = user
	}
	if user.UserPrincipalName != "" {
		u[user.UserPrincipalName] = user
	}
}

// get the MS Graph user for a given user by id, or email if no ID is
// provided.
func (u users) get(user *User) graph.User {
	ret, ok := u[user.Id]
	if !ok {
		ret, ok = u[user.Email]
		if !ok {
			return graph.User{}
		}
	}

	return ret
}

// Map struct fields using a map of user IDs to Users obtained from
// MS Graph.
func (t *Task) AddUsers(users users) *Task {
	t.CompletedBy.AppendFromGraph(users)
	t.CreatedBy.AppendFromGraph(users)
	t.AssignedTo.AppendFromGraph(users)
	t.AssignedBy.AppendFromGraph(users)

	for i := range t.Comments {
		t.Comments[i].User.AppendFromGraph(users)
	}

	return t
}

type User struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *User) AppendFromGraph(users users) {
	user := users.get(u)
	u.Id = p.Coalesce(user.ID, u.Id)
	u.Email = p.Coalesce(user.UserPrincipalName, u.Email)
	u.Name = p.Coalesce(user.DisplayName, u.Name)
}

func NewUserFromIdentitySet(identity graph.IdentitySet) User {
	return User{
		Id:   identity.User.ID,
		Name: identity.User.DisplayName,
	}
}

type Attachment struct {
	Ref                  string    `json:"ref"`
	Name                 string    `json:"name"`
	Type                 string    `json:"type"`
	LastModifiedDateTime time.Time `json:"lastModifiedDateTime"`
	LastModifiedBy       User      `json:"lastModifiedBy"`
}

type ChecklistItem struct {
	ID                   string    `json:"id"`
	Title                string    `json:"title"`
	IsChecked            bool      `json:"isChecked"`
	LastModifiedDateTime time.Time `json:"lastModifiedDateTime"`
	LastModifiedBy       User      `json:"lastModifiedBy"`
}

type Comment struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	UpdatedAt time.Time `json:"updatedAt"`
	User      User      `json:"user"`
}

func NewCommentFromGraph(post graph.Post) Comment {
	return Comment{
		// NOTE: this is probably fine, given that there doesn't appear to
		// be a patch method for posts.
		ID:        post.ChangeKey,
		Text:      post.RawBody(),
		UpdatedAt: post.LastModifiedDateTime,
		// TODO: re-evaluate this
		User: User{
			Name:  post.Sender.EmailAddress.Name,
			Email: post.Sender.EmailAddress.Address,
		},
	}
}
