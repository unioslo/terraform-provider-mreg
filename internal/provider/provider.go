package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func init() {
	// Set descriptions to support markdown syntax, this will be used in document generation
	// and the language server.
	schema.DescriptionKind = schema.StringMarkdown

	// Customize the content of descriptions when output. For example you can add defaults on
	// to the exported descriptions if present.
	// schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
	// 	desc := s.Description
	// 	if s.Default != nil {
	// 		desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
	// 	}
	// 	return strings.TrimSpace(desc)
	// }
}

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"mreg_hosts":   resourceHosts(),
				"mreg_dns_srv": resourceSRV(),
			},
			DataSourcesMap: map[string]*schema.Resource{},
			Schema: map[string]*schema.Schema{
				"serverurl": &schema.Schema{
					Type:     schema.TypeString,
					Required: true,
				},
				"token": &schema.Schema{
					Type:      schema.TypeString,
					Optional:  true,
					Sensitive: true,
				},
				"username": &schema.Schema{
					Type:      schema.TypeString,
					Optional:  true,
					Sensitive: true,
				},
				"password": &schema.Schema{
					Type:      schema.TypeString,
					Optional:  true,
					Sensitive: true,
				},
			},
		}

		p.ConfigureContextFunc = configure(version, p)

		return p
	}
}

func configure(version string, p *schema.Provider) func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(c context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		client := apiClient{
			Serverurl: d.Get("serverurl").(string),
			Token:     d.Get("token").(string),
			Username:  d.Get("username").(string),
			Password:  d.Get("password").(string),
		}
		if client.Username != "" {
			client.login()
		}
		return client, nil
	}
}
