package ssm

import (
	"context"
	"time"
)

// Parameter represents an SSM parameter.
type Parameter struct {
	Name         string
	Value        string
	Type         string
	Version      int64
	LastModified time.Time
}

// ClientOptions configures the SSM client.
type ClientOptions struct {
	Region   string
	Profile  string
	Endpoint string // For LocalStack
}

// ListOptions configures parameter listing.
type ListOptions struct {
	Path      string
	Recursive bool
	Decrypt   bool
}

// Client defines the interface for SSM operations.
type Client interface {
	ListParameters(ctx context.Context, opts ListOptions) ([]Parameter, error)
	GetAccountID(ctx context.Context) (string, error)
	GetParameters(ctx context.Context, names []string, decrypt bool) ([]Parameter, error)
	PutParameter(ctx context.Context, name, value, paramType string, overwrite bool) error
}

// NewClient creates a new SSM client.
func NewClient(ctx context.Context, opts ClientOptions) (Client, error) {
	return newAWSClient(ctx, opts)
}
