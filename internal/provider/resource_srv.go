package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceSRV() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSRVCreate,
		ReadContext:   resourceSRVRead,
		DeleteContext: resourceSRVDelete,
		Schema: map[string]*schema.Schema{
			"target_host": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"service": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"proto": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"priority": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"weight": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"port": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceSRVCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	apiClient := m.(apiClient)

	// Find the host ID in Mreg by looking up the hostname
	_, body, diags := apiClient.httpRequest("GET", "/api/v1/hosts/"+url.QueryEscape(d.Get("target_host").(string)), nil, http.StatusOK)
	if len(diags) > 0 {
		return diags
	}
	result := body.(map[string]interface{})
	hostID := int(result["id"].(float64)) // Go always turns JSON numbers into float64 values

	// assemble the "_service._proto.name."-part of the SRV record
	serviceProtoName := fmt.Sprintf("_%s._%s.%s.", d.Get("service").(string), d.Get("proto").(string), d.Get("name").(string))

	// Create a new SRV record
	postdata := map[string]interface{}{
		"name":     serviceProtoName,
		"priority": d.Get("priority"),
		"weight":   d.Get("weight"),
		"port":     d.Get("port"),
		"host":     hostID,
	}
	_, _, diags = apiClient.httpRequest("POST", "/api/v1/srvs/", postdata, http.StatusCreated)
	if len(diags) > 0 {
		return diags
	}

	d.SetId(fmt.Sprintf("%s|%d|%d|%d|%d", serviceProtoName, d.Get("priority").(int), d.Get("weight").(int), d.Get("port").(int), hostID))
	return diag.Diagnostics{}
}

func resourceSRVRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return diag.Diagnostics{}
}

func resourceSRVDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	apiClient := m.(apiClient)

	// assemble the "_service._proto.name."-part of the SRV record
	serviceProtoName := fmt.Sprintf("_%s._%s.%s.", d.Get("service").(string), d.Get("proto").(string), d.Get("name").(string))

	// Find the host ID in Mreg by looking up the hostname
	_, body, diags := apiClient.httpRequest("GET", "/api/v1/hosts/"+url.QueryEscape(d.Get("target_host").(string)), nil, http.StatusOK)
	if len(diags) > 0 {
		return diags
	}
	bodyMap := body.(map[string]interface{})
	hostID := int(bodyMap["id"].(float64)) // Go always turns JSON numbers into float64 values

	// Find all SRV records of that particular type, for that particular host
	_, body, diags = apiClient.httpRequest("GET", fmt.Sprintf("/api/v1/srvs/?host=%d&name=%s", hostID, url.QueryEscape(serviceProtoName)), nil, http.StatusOK)
	if len(diags) > 0 {
		return diags
	}

	bodyMap = body.(map[string]interface{})
	list := bodyMap["results"].([]interface{})

	// Look through the SRV records for one that matches the one I'm trying to delete, and extract the ID
	priority := d.Get("priority").(int)
	weight := d.Get("weight").(int)
	port := d.Get("port").(int)
	var srvId int
	for _, r := range list {
		m := r.(map[string]interface{})
		if serviceProtoName == m["name"].(string) && priority == int(m["priority"].(float64)) &&
			weight == int(m["weight"].(float64)) && port == int(m["port"].(float64)) {
			srvId = int(m["id"].(float64))
			break
		}
	}
	if srvId == 0 {
		// Not found, but that's no problem really
		d.SetId("")
		var diags diag.Diagnostics
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  "Apparently the host doesn't have the SRV record in Mreg",
			Detail:   serviceProtoName + " , " + d.Get("target_host").(string),
		})
		return diags
	}

	// Delete the SRV record
	_, _, diags = apiClient.httpRequest("DELETE", fmt.Sprintf("/api/v1/srvs/%d", srvId), nil, http.StatusNoContent)
	if len(diags) > 0 {
		return diags
	}

	d.SetId("")
	return diag.Diagnostics{}
}
