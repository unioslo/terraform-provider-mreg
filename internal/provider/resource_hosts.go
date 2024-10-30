package provider

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/fslock"
)

func resourceHosts() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHostsCreate,
		ReadContext:   resourceHostsRead,
		DeleteContext: resourceHostsDelete,
		Schema: map[string]*schema.Schema{
			"host": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"comment": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"contact": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"manual_ipaddress": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"ipaddress": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"network": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"comment": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"contact": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"policies": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func splitString(source string) []string {
	result := make([]string, 0)
	for _, s := range strings.Split(source, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func resourceHostsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	apiClient := m.(apiClient)

	hosts := d.Get("host").([]interface{})
	comment := d.Get("comment").(string)
	contact := d.Get("contact").(string)
	networks := splitString(d.Get("network").(string))
	policies := splitString(d.Get("policies").(string))

	lock := fslock.New("terraform-provider-mreg-lockfile")
	lock.Lock()
	defer lock.Unlock()

	hostnames := make([]string, len(hosts))
	for i := range hosts {
		host := hosts[i].(map[string]interface{})
		hostname := host["name"].(string)
		hostnames[i] = hostname

		manual_ips := splitString(host["manual_ipaddress"].(string))

		// Allocate a new host object in Mreg
		postdata := map[string]interface{}{
			"name":    hostname,
			"contact": contact,
			"comment": comment,
		}
		if len(manual_ips) > 0 {
			postdata["ipaddress"] = manual_ips[0]
		} else if len(networks) > 0 {
			postdata["network"] = networks[0]
		}
		_, _, diags := apiClient.httpRequest("POST", "/api/v1/hosts/", postdata, http.StatusCreated)
		if len(diags) > 0 {
			return diags
		}

		// Retrieve the host by name to find Mreg's internal ID for the host
		_, body, diags := apiClient.httpRequest("GET", "/api/v1/hosts/"+url.QueryEscape(hostname), nil, http.StatusOK)
		if len(diags) > 0 {
			return diags
		}
		result := body.(map[string]interface{})
		host_id := int(result["id"].(float64))

		// Assign any additional ip addresses
		for i := 1; i < len(manual_ips); i++ {
			postdata := map[string]interface{}{
				"ipaddress": manual_ips[i],
				"host":      host_id,
			}
			_, _, diags := apiClient.httpRequest("POST", "/api/v1/ipaddresses/", postdata, http.StatusCreated)
			if len(diags) > 0 {
				return diags
			}
		}
		if len(manual_ips) == 0 {
			for i := 1; i < len(networks); i++ {
				// Find an unused address
				_, body, diags := apiClient.httpRequest("GET", "/api/v1/networks/"+networks[i]+"/first_unused", nil, http.StatusOK)
				if len(diags) > 0 {
					return diags
				}
				ipaddress := body.(string)
				postdata := map[string]interface{}{
					"ipaddress": ipaddress,
					"host":      host_id,
				}
				_, _, diags = apiClient.httpRequest("POST", "/api/v1/ipaddresses/", postdata, http.StatusCreated)
				if len(diags) > 0 {
					return diags
				}
			}
		}

		// Assign host policies, if any
		for _, p := range policies {
			postdata := map[string]interface{}{
				"name": hostname,
			}
			_, _, diags := apiClient.httpRequest("POST", "/api/v1/hostpolicy/roles/"+p+"/hosts/", postdata, http.StatusCreated)
			if len(diags) > 0 {
				return diags
			}
		}

		// Retrieve information about the host to find out which ip addresses it ended up with
		_, body, diags = apiClient.httpRequest("GET", "/api/v1/hosts/"+url.QueryEscape(hostname), nil, http.StatusOK)
		if len(diags) > 0 {
			return diags
		}
		result = body.(map[string]interface{})
		ipaddressesCommaSeparated := ""
		for _, elem := range result["ipaddresses"].([]interface{}) {
			m := elem.(map[string]interface{})
			if ipaddressesCommaSeparated == "" {
				ipaddressesCommaSeparated = m["ipaddress"].(string)
			} else {
				ipaddressesCommaSeparated = ipaddressesCommaSeparated + "," + m["ipaddress"].(string)
			}
		}

		// Update the ResourceData
		host["ipaddress"] = ipaddressesCommaSeparated
		host["comment"] = comment
		host["contact"] = contact
		hosts[i] = host

		d.Set("host", hosts)
		d.SetId(hostname)
	}
	d.Set("host", hosts)
	d.SetId(compoundId(hostnames))

	return diags

}

func resourceHostsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	apiClient := m.(apiClient)

	hosts, ok := d.Get("host").([]interface{})
	if !ok {
		var diags diag.Diagnostics
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  "Apparently the TF state doesn't contain any Mreg hosts",
			Detail:   "",
		})
		return diags
	}

	hostnames := make([]string, 0, len(hosts))
	for i := 0; i < len(hosts); i++ {
		host := hosts[i].(map[string]interface{})
		hostname := host["name"].(string)

		// Read information about this host from Mreg
		_, body, diags := apiClient.httpRequest("GET", "/api/v1/hosts/"+url.QueryEscape(hostname),
			nil, http.StatusOK)
		if len(diags) > 0 {
			return diags
		}
		result := body.(map[string]interface{})

		// make a comma-separated list of the IP address(es) in case there are many
		ipaddressesCommaSeparated := ""
		for _, elem := range result["ipaddresses"].([]interface{}) {
			m := elem.(map[string]interface{})
			if ipaddressesCommaSeparated == "" {
				ipaddressesCommaSeparated = m["ipaddress"].(string)
			} else {
				ipaddressesCommaSeparated = ipaddressesCommaSeparated + "," + m["ipaddress"].(string)
			}
		}

		// Update the data model with data from Mreg
		host["comment"] = result["comment"]
		host["ipaddress"] = ipaddressesCommaSeparated
		host["contact"] = result["contact"]
		hosts[i] = host

		hostnames = append(hostnames, hostname)
	}

	d.Set("host", hosts)
	d.SetId(compoundId(hostnames))

	return diag.Diagnostics{}
}

func resourceHostsDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	apiClient := m.(apiClient)
	d.SetId("")

	hosts, ok := d.Get("host").([]interface{})
	if !ok {
		return diag.Diagnostics{}
	}

	for i := 0; i < len(hosts); i++ {
		host := hosts[i].(map[string]interface{})
		hostname := host["name"].(string)

		// Delete this host from Mreg
		_, _, diags := apiClient.httpRequest("DELETE", "/api/v1/hosts/"+url.QueryEscape(hostname),
			nil, http.StatusNoContent)
		if len(diags) > 0 {
			return diags
		}
	}

	return diag.Diagnostics{}
}

// compoundId returns an id value that is unique for the given set of hostnames,
// and doesn't depend on the order.
func compoundId(hostnames []string) string {
	sort.Strings(hostnames)
	hash := md5.New()
	for _, s := range hostnames {
		hash.Write([]byte(s))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
