package database

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"
)

// Notification represents a persisted job terminal event for the in-app feed.
type Notification struct {
	TenantId       string    `spanner:"TenantId"`
	NotificationId string    `spanner:"NotificationId"`
	JobId          string    `spanner:"JobId"`
	JobName        *string   `spanner:"JobName"`
	FinalStatus    string    `spanner:"FinalStatus"`
	ServiceTier    *string   `spanner:"ServiceTier"`
	AssignedService *string  `spanner:"AssignedService"`
	OccurredAt     time.Time `spanner:"OccurredAt"`
	ErrorMessage   *string   `spanner:"ErrorMessage"`
	IsRead         bool      `spanner:"IsRead"`
	CreatedAt      time.Time `spanner:"CreatedAt"`
}

var notificationColumns = []string{
	"TenantId", "NotificationId", "JobId", "JobName",
	"FinalStatus", "ServiceTier", "AssignedService",
	"OccurredAt", "ErrorMessage", "IsRead", "CreatedAt",
}

// InsertNotification persists a new notification row. It is idempotent: if a
// row with the same (TenantId, NotificationId) already exists it is skipped.
func (c *Client) InsertNotification(ctx context.Context, n *Notification) error {
	_, err := c.client.Apply(ctx, []*spanner.Mutation{
		spanner.InsertOrUpdate("Notifications",
			notificationColumns,
			[]interface{}{
				n.TenantId, n.NotificationId, n.JobId, n.JobName,
				n.FinalStatus, n.ServiceTier, n.AssignedService,
				n.OccurredAt, n.ErrorMessage, false, spanner.CommitTimestamp,
			},
		),
	})
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// ListNotifications returns notifications for a tenant ordered by newest first.
// limit ≤ 0 defaults to 20.
func (c *Client) ListNotifications(ctx context.Context, tenantID string, limit int32) ([]*Notification, error) {
	if limit <= 0 {
		limit = 20
	}

	stmt := spanner.Statement{
		SQL: `SELECT ` + columnList(notificationColumns) + `
		      FROM Notifications
		      WHERE TenantId = @tenantId
		      ORDER BY OccurredAt DESC
		      LIMIT @limit`,
		Params: map[string]interface{}{
			"tenantId": tenantID,
			"limit":    int64(limit),
		},
	}

	iter := c.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	var notifications []*Notification
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list notifications: %w", err)
		}
		var n Notification
		if err := row.ToStruct(&n); err != nil {
			return nil, fmt.Errorf("parse notification row: %w", err)
		}
		notifications = append(notifications, &n)
	}
	return notifications, nil
}

// CountUnread returns the number of unread notifications for a tenant.
func (c *Client) CountUnread(ctx context.Context, tenantID string) (int32, error) {
	stmt := spanner.Statement{
		SQL:    `SELECT COUNT(*) FROM Notifications WHERE TenantId = @tenantId AND IsRead = FALSE`,
		Params: map[string]interface{}{"tenantId": tenantID},
	}
	iter := c.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	row, err := iter.Next()
	if err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	var count int64
	if err := row.Columns(&count); err != nil {
		return 0, fmt.Errorf("scan unread count: %w", err)
	}
	return int32(count), nil
}

// AckNotification marks a single notification as read.
func (c *Client) AckNotification(ctx context.Context, tenantID, notificationID string) error {
	_, err := c.client.Apply(ctx, []*spanner.Mutation{
		spanner.Update("Notifications",
			[]string{"TenantId", "NotificationId", "IsRead"},
			[]interface{}{tenantID, notificationID, true},
		),
	})
	if err != nil {
		return fmt.Errorf("ack notification %s: %w", notificationID, err)
	}
	return nil
}

// columnList joins column names with ", " for use in SELECT.
func columnList(cols []string) string {
	result := ""
	for i, c := range cols {
		if i > 0 {
			result += ", "
		}
		result += c
	}
	return result
}
