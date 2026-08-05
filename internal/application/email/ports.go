// Package email defines application ports for provider template discovery.
// Admin HTTP/OpenAPI wiring comes later; adapters implement these ports now.
package email

import "context"

// TemplateSummary is a provider template list entry.
type TemplateSummary struct {
	ID     string
	Name   string
	Alias  string
	Status string
}

// TemplateDiscovery lists Resend templates and extracts variable names.
type TemplateDiscovery interface {
	ListTemplates(ctx context.Context) ([]TemplateSummary, error)
	GetTemplateVariables(ctx context.Context, idOrAlias string) ([]string, error)
}
