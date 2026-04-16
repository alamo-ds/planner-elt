package msgraph

import (
	"encoding/json"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Group struct {
	ID                            string      `json:"id"`
	DeletedDateTime               time.Time   `json:"deletedDateTime"`
	Classification                string      `json:"classification"`
	CreatedDateTime               time.Time   `json:"createdDateTime"`
	Description                   string      `json:"description"`
	DisplayName                   string      `json:"displayName"`
	ExpirationDateTime            time.Time   `json:"expirationDateTime"`
	GroupTypes                    []string    `json:"groupTypes"`
	IsAssignableToRole            bool        `json:"isAssignableToRole"`
	Mail                          string      `json:"mail"`
	MailEnabled                   bool        `json:"mailEnabled"`
	MailNickname                  string      `json:"mailNickname"`
	MembershipRule                string      `json:"membershipRule"`
	MembershipRuleProcessingState string      `json:"membershipRuleProcessingState"`
	OnPremisesDomainName          string      `json:"onPremisesDomainName"`
	OnPremisesLastSyncDateTime    time.Time   `json:"onPremisesLastSyncDateTime"`
	OnPremisesNetBiosName         string      `json:"onPremisesNetBiosName"`
	OnPremisesSamAccountName      string      `json:"onPremisesSamAccountName"`
	OnPremisesSecurityIdentifier  string      `json:"onPremisesSecurityIdentifier"`
	OnPremisesSyncEnabled         bool        `json:"onPremisesSyncEnabled"`
	PreferredDataLocation         string      `json:"preferredDataLocation"`
	PreferredLanguage             string      `json:"preferredLanguage"`
	ProxyAddresses                []string    `json:"proxyAddresses"`
	RenewedDateTime               time.Time   `json:"renewedDateTime"`
	ResourceBehaviorOptions       []string    `json:"resourceBehaviorOptions"`
	ResourceProvisioningOptions   []string    `json:"resourceProvisioningOptions"`
	SecurityEnabled               bool        `json:"securityEnabled"`
	SecurityIdentifier            string      `json:"securityIdentifier"`
	Theme                         string      `json:"theme"`
	UniqueName                    string      `json:"uniqueName"`
	Visibility                    string      `json:"visibility"`
	OnPremisesProvisioningErrors  []string    `json:"onPremisesProvisioningErrors"`
	ServiceProvisioningErrors     []time.Time `json:"serviceProvisioningErrors"`
}

type Bucket struct {
	OdataEtag string `json:"@odata.etag"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	OrderHint string `json:"orderHint"`
	PlanID    string `json:"planId"`
}

type Conversation struct {
	CcResipients          Recipient   `json:"ccRecipients,omitzero"`
	HasAttachments        bool        `json:"hasAttachments,omitempty"`
	ID                    string      `json:"id,omitempty"`
	IsLocked              bool        `json:"isLocked,omitempty"`
	LastDeliveredDateTime time.Time   `json:"lastDeliveredDateTime,omitzero"`
	Preview               string      `json:"preview,omitempty"`
	Topic                 string      `json:"topic,omitempty"`
	ToRecipients          []Recipient `json:"toRecipients,omitempty"`
	UniqueSenders         []string    `json:"uniqueSenders,omitempty"`
	Posts                 []Post      `json:"posts,omitempty"`
}

type Identity struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

type IdentitySet struct {
	User        Identity `json:"user"`
	Application Identity `json:"application"`
}

type Recipient struct {
	EmailAddress EmailAddress `json:"emailAddress"`
}

type EmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type Post struct {
	OdataEtag            string      `json:"@odata.etag,omitempty"`
	ID                   string      `json:"id,omitempty"`
	TaskID               string      `json:"taskId"`
	ChangeKey            string      `json:"changeKey,omitempty"`
	ConversationID       string      `json:"conversationId,omitempty"`
	ConversationThreadID string      `json:"conversationThreadId,omitempty"`
	HasAttachments       bool        `json:"hasAttachments,omitempty"`
	Categories           []string    `json:"categories,omitempty"`
	CreatedDateTime      time.Time   `json:"createdDateTime,omitzero"`
	LastModifiedDateTime time.Time   `json:"lastModifiedDateTime,omitzero"`
	ReceivedDateTime     time.Time   `json:"receivedDateTime,omitzero"`
	From                 Recipient   `json:"from,omitzero"`
	Sender               Recipient   `json:"sender,omitzero"`
	Body                 ItemBody    `json:"body,omitzero"`
	NewParticipants      []Recipient `json:"newParticipants,omitzero"`
}

func (p Post) RawBody() string {
	// NOTE: doing this with Post instead of ItemBody, since I'm not sure
	// if this logic applies to all ItemBody with type HTML
	return p.Body.rawBody()
}

func (b ItemBody) rawBody() string {
	switch strings.ToLower(b.ContentType) {
	default:
		return b.Content
	case "string":
		return strings.TrimSpace(b.Content)
	case "html":
		return extractRawText(b.Content)
	}
}

func extractRawText(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return ""
	}

	var (
		body     *html.Node
		findBody func(*html.Node)
	)

	// First get the body node, usually the 2nd tag in the response.
	findBody = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "body" {
			body = node
			return
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}

	findBody(doc)
	if body == nil {
		return ""
	}

	// Next, traverse the body node until the inner-most div is found.
	// This actually contains the text node: for the latest comment, this
	// is usually the second div. For all others, it is usualy the third.
	// If no such div is found, return an empty string.
	for {
		var next *html.Node

		for c := body.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "div" {
				next = c
				break
			}
		}

		if next == nil {
			break
		}

		body = next
	}

	// Lastly, extract the content. Usually this is just a raw string.
	// A table node seems to accompany it, but it is ignored.
	var content strings.Builder
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			content.WriteString(c.Data)
		}
	}

	return content.String()
}

type ItemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type Plan struct {
	OdataEtag       string        `json:"@odata.etag"`
	CreatedDateTime time.Time     `json:"createdDateTime"`
	Owner           string        `json:"owner"`
	Title           string        `json:"title"`
	ID              string        `json:"id"`
	CreatedBy       IdentitySet   `json:"createdBy"`
	Container       PlanContainer `json:"container"`
}

type PlanContainer struct {
	ContainerID string `json:"containerId"`
	Type        string `json:"type"`
	URL         string `json:"url"`
}

type Task struct {
	OdataEtag                string                `json:"@odata.etag"`
	PlanID                   string                `json:"planId"`
	BucketID                 string                `json:"bucketId"`
	Title                    string                `json:"title"`
	OrderHint                string                `json:"orderHint"`
	AssigneePriority         string                `json:"assigneePriority"`
	PercentComplete          int                   `json:"percentComplete"`
	StartDateTime            time.Time             `json:"startDateTime"`
	CreatedDateTime          time.Time             `json:"createdDateTime"`
	DueDateTime              time.Time             `json:"dueDateTime"`
	HasDescription           bool                  `json:"hasDescription"`
	PreviewType              string                `json:"previewType"`
	CompletedDateTime        time.Time             `json:"completedDateTime"`
	ReferenceCount           int                   `json:"referenceCount"`
	ChecklistItemCount       int                   `json:"checklistItemCount"`
	ActiveChecklistItemCount int                   `json:"activeChecklistItemCount"`
	ConversationThreadID     string                `json:"conversationThreadId"`
	Priority                 int                   `json:"priority"`
	ID                       string                `json:"id"`
	CreatedBy                IdentitySet           `json:"createdBy"`
	CompletedBy              IdentitySet           `json:"completedBy"`
	AppliedCategories        map[string]bool       `json:"appliedCategories"`
	Assignments              map[string]Assignment `json:"assignments"`
}

type Assignment struct {
	OrderHint        string      `json:"orderHint"`
	AssignedBy       IdentitySet `json:"assignedBy"`
	AssignedDateTime time.Time   `json:"assignedDateTime"`
}

type TaskDetails struct {
	OdataEtag   string                       `json:"@odata.etag"`
	ID          string                       `json:"id"`
	TaskID      string                       `json:"taskId"`
	Description string                       `json:"description"`
	PreviewType string                       `json:"previewType"`
	Checklist   map[string]ChecklistItem     `json:"checklist"`
	References  map[string]ExternalReference `json:"references"`
}

type ChecklistItem struct {
	Title                string      `json:"title"`
	IsChecked            bool        `json:"isChecked"`
	OrderHint            string      `json:"orderHint"`
	LastModifiedBy       IdentitySet `json:"lastModifiedBy"`
	LastModifiedDateTime time.Time   `json:"lastModifiedByDateTime"`
}

type ExternalReference struct {
	Alias                string      `json:"alias"`
	LastModifiedBy       IdentitySet `json:"lastModifiedBy"`
	LastModifiedDateTime time.Time   `json:"lastModifiedByDateTime"`
	PreviewPriority      string      `json:"previewPriority"`
	Type                 string      `json:"type"`
}

// TODO: add other fields. For simplicity, I only added what I needed for
// a specific project.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Mail        string `json:"mail"`
	// NOTE: below is what should be used to search User by email, not "mail"
	UserPrincipalName     string    `json:"userPrincipalName"`
	CreationType          string    `json:"creationType"`
	Department            string    `json:"department"`
	GivenName             string    `json:"givenName"`
	AccountEnabled        bool      `json:"accountEnabled"`
	CreatedDateTime       time.Time `json:"createdDateTime,omitzero"`
	DeletedDateTime       time.Time `json:"deletedDateTime,omitzero"`
	EmployeeLeaveDateTime time.Time `json:"employeeLeaveDateTime,omitzero"`
	// TODO: the key matches, but for some reason not getting values. Investigate why.
	AssignedLicenses []AssignedLicense `json:"assignedLicenses"`
}

type AssignedLicense struct {
	SkuID         string   `json:"skuId"`
	DisabledPlans []string `json:"disabledPlans"`
}

type BatchRequest struct {
	Id     string `json:"id"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

type BatchResponse struct {
	Responses ResponseObject `json:"responses"`
}

type ResponseObject struct {
	Id      string            `json:"id"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}
