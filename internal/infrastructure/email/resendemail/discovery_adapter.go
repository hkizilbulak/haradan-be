package resendemail

import (
	"context"

	appemail "github.com/hkizilbulak/haradan-be/internal/application/email"
)

// Discovery adapts Sender to appemail.TemplateDiscovery.
type Discovery struct {
	Sender *Sender
}

var _ appemail.TemplateDiscovery = Discovery{}

// ListTemplates implements appemail.TemplateDiscovery.
func (d Discovery) ListTemplates(ctx context.Context) ([]appemail.TemplateSummary, error) {
	if d.Sender == nil {
		return nil, dependencyError()
	}
	raw, err := d.Sender.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appemail.TemplateSummary, 0, len(raw))
	for _, t := range raw {
		out = append(out, appemail.TemplateSummary{
			ID: t.ID, Name: t.Name, Alias: t.Alias, Status: t.Status,
		})
	}
	return out, nil
}

// GetTemplateVariables implements appemail.TemplateDiscovery.
func (d Discovery) GetTemplateVariables(ctx context.Context, idOrAlias string) ([]string, error) {
	if d.Sender == nil {
		return nil, dependencyError()
	}
	return d.Sender.GetTemplateVariables(ctx, idOrAlias)
}
