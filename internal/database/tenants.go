package database

import (
	"context"
	"fmt"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"
)

// InsertTenant creates a new tenant
func (c *Client) InsertTenant(ctx context.Context, tenantID, name string) error {
	_, err := c.client.Apply(ctx, []*spanner.Mutation{
		spanner.Insert("Tenants",
			[]string{"TenantId", "Name", "CreatedAt", "UpdatedAt"},
			[]interface{}{tenantID, name, spanner.CommitTimestamp, spanner.CommitTimestamp},
		),
	})
	if err != nil {
		return fmt.Errorf("failed to insert tenant: %w", err)
	}
	return nil
}

// GetTenant retrieves a tenant by ID
func (c *Client) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	row, err := c.client.Single().ReadRow(ctx, "Tenants",
		spanner.Key{tenantID},
		[]string{"TenantId", "Name", "CreatedAt", "UpdatedAt"},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	var tenant Tenant
	if err := row.ToStruct(&tenant); err != nil {
		return nil, fmt.Errorf("failed to parse tenant: %w", err)
	}

	return &tenant, nil
}

// ListTenants returns all tenants
func (c *Client) ListTenants(ctx context.Context) ([]*Tenant, error) {
	stmt := spanner.Statement{
		SQL: `SELECT TenantId, Name, CreatedAt, UpdatedAt FROM Tenants ORDER BY CreatedAt DESC`,
	}

	iter := c.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	var tenants []*Tenant
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate tenants: %w", err)
		}

		var tenant Tenant
		if err := row.ToStruct(&tenant); err != nil {
			return nil, fmt.Errorf("failed to parse tenant: %w", err)
		}
		tenants = append(tenants, &tenant)
	}

	return tenants, nil
}

// DeleteTenant removes a tenant and all its jobs (CASCADE)
func (c *Client) DeleteTenant(ctx context.Context, tenantID string) error {
	_, err := c.client.Apply(ctx, []*spanner.Mutation{
		spanner.Delete("Tenants", spanner.Key{tenantID}),
	})
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}
	return nil
}
