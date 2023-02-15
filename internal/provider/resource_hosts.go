package provider

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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

func resourceHostsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	apiClient := m.(apiClient)

	hosts := d.Get("host").([]interface{})
	comment := d.Get("comment").(string)
	contact := d.Get("contact").(string)
	network := d.Get("network").(string)
	policies_string := d.Get("policies").(string)
	policies := make([]string, 0)
	if policies_string != "" {
		for _, s := range strings.Split(policies_string, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				policies = append(policies, s)
			}
		}
	}

	lock := fslock.New("terraform-provider-mreg-lockfile")
	lock.Lock()
	defer lock.Unlock()

	hostnames := make([]string, len(hosts))
	for i := range hosts {
		host := hosts[i].(map[string]interface{})
		hostname := host["name"].(string)
		hostnames[i] = hostname

		var ipaddress string

		manual_ip := host["manual_ipaddress"].(string)
		if manual_ip != "" {
			ipaddress = manual_ip
		} else {
			if network != "" {
				// Find a free IP address in Mreg
				body, _, diags := apiClient.httpRequest(
					"GET", fmt.Sprintf("/api/v1/networks/%s/first_unused", url.QueryEscape(network)),
					nil, http.StatusOK)
				if len(diags) > 0 {
					return diags
				}

				ipaddress = strings.Trim(body, "\"")
			}
		}

		// Allocate a new host object in Mreg
		postdata := map[string]interface{}{
			"name":    hostname,
			"contact": contact,
			"comment": comment,
		}
		// Only add the ipaddress parameter if the host is supposed to have an IP address, or it will fail
		if ipaddress != "" {
			postdata["ipaddress"] = ipaddress
		}
		_, _, diags := apiClient.httpRequest("POST", "/api/v1/hosts/", postdata, http.StatusCreated)
		if len(diags) > 0 {
			return diags
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

		// Update the ResourceData
		host["ipaddress"] = ipaddress
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

		// Update the data model with data from Mreg
		host["comment"] = result["comment"]
		host["ipaddress"] = GetStringFromData(result, "ipaddresses.0.ipaddress")
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

// GetStringFromData lets you specify a path to the value that you want
// (e.g. "aaa.bbb.ccc") and have it extracted from the data structure.
func GetStringFromData(v interface{}, path string) string {
	for _, key := range strings.Split(path, ".") {
		iKey, err := strconv.ParseInt(key, 10, 32)
		if err == nil {
			// If the key is a number, we assume the structure is an array
			arr, ok := v.([]interface{})
			if !ok || int64(len(arr)) <= iKey {
				return ""
			}
			v = arr[iKey]
		} else {
			// If the key isn't a number, we assume the structure is a map
			m, ok := v.(map[string]interface{})
			if !ok {
				return ""
			}
			v = m[key]
		}
	}
	return fmt.Sprintf("%v", v)
}
