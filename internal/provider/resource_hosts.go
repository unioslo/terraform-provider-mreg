package provider

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/fslock"
)

var seen []string

func resourceHosts() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHostsCreate,
		ReadContext:   resourceHostsRead,
		DeleteContext: resourceHostsDelete,
		Schema: map[string]*schema.Schema{
			"host": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"comment": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"contact": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"ipv4": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"ipv6": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"ipaddress": {
							Type:     schema.TypeList,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Computed: true,
						},
					},
				},
			},
			"network": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},
			"comment": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"contact": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"policies": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

/*func splitString(source string) []string {
	result := make([]string, 0)
	for _, s := range strings.Split(source, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}*/

func convertTerraformInputToListOfStrings(input interface{}) []string {
	result := make([]string, 0)
	a := input.([]interface{})
	for _, elem := range a {
		b := elem.(string)
		result = append(result, b)
	}
	return result
}

func resourceHostsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	apiClient := m.(apiClient)

	hosts := d.Get("host").([]interface{})
	comment := d.Get("comment").(string)
	contact := d.Get("contact").(string)
	networks := convertTerraformInputToListOfStrings(d.Get("network"))
	policies := convertTerraformInputToListOfStrings(d.Get("policies"))

	lockfile := "/tmp/terraform-provider-mreg-lockfile"
	lock := fslock.New(lockfile)
	if err := lock.Lock(); err != nil {
		return diag.Errorf("Unable to use lockfile %s: %s", lockfile, err.Error())
	}
	defer lock.Unlock()

	hostnames := make([]string, len(hosts))
	for i := range hosts {
		host := hosts[i].(map[string]interface{})
		hostname := host["name"].(string)
		hostnames[i] = hostname

		//TODO manual_ips := convertTerraformInputToListOfStrings(host["manual_ipaddress"])
		manual_ips := make([]string, 0)
		if s, ok := host["ipv4"].(string); ok && s != "" {
			manual_ips = append(manual_ips, s)
		}
		if s, ok := host["ipv6"].(string); ok && s != "" {
			manual_ips = append(manual_ips, s)
		}

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
				var ipaddress string
				retries := 0
				for {
					_, body, diags := apiClient.httpRequest("GET", "/api/v1/networks/"+networks[i]+"/first_unused", nil, http.StatusOK)
					if len(diags) > 0 {
						return diags
					}
					ipaddress = body.(string)
					actuallyUnused := true
					for _, v := range seen {
						if v == ipaddress {
							actuallyUnused = false
							break
						}
					}
					if actuallyUnused {
						seen = append(seen, ipaddress)
						break
					} else if retries > 100 {
						log.Fatalf("Unable to find enough unused addresses on network %s", networks[i])
					} else {
						time.Sleep(time.Second)
						retries++
					}
				}
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

		ipaddresses := make([]string, 0)
		for _, elem := range result["ipaddresses"].([]interface{}) {
			m := elem.(map[string]interface{})
			ipaddresses = append(ipaddresses, m["ipaddress"].(string))
		}

		// Update the ResourceData
		host["ipaddress"] = ipaddresses
		host["comment"] = comment
		host["contact"] = contact
		hosts[i] = host

		d.Set("host", hosts)
		d.SetId(hostname)
	}
	if err := d.Set("host", hosts); err != nil {
		return diag.FromErr(err)
	}
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

		ipaddresses := make([]string, 0)
		for _, elem := range result["ipaddresses"].([]interface{}) {
			m := elem.(map[string]interface{})
			ipaddresses = append(ipaddresses, m["ipaddress"].(string))
		}

		// Update the data model with data from Mreg
		host["comment"] = result["comment"]
		host["ipaddress"] = ipaddresses
		host["contact"] = result["contact"]
		hosts[i] = host
		hostnames = append(hostnames, hostname)
	}

	if err := d.Set("host", hosts); err != nil {
		return diag.FromErr(err)
	}
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
