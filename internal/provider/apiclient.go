package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

type apiClient struct {
	Serverurl string
	Token     string
	Username  string
	Password  string
}

func (c apiClient) UrlWithoutSlash() string {
	return strings.TrimSuffix(c.Serverurl, "/")
}

func (apiClient apiClient) httpRequest(method, urlPath string, requestBody map[string]interface{}, expectedStatus int) (bodyText string, body interface{}, diags diag.Diagnostics) {
	// Turn the request body structure into JSON
	var reqBodyReader io.Reader
	if requestBody != nil {
		reqbody, err := json.Marshal(requestBody)
		if err != nil {
			diags = diag.FromErr(err)
			return
		}
		reqBodyReader = bytes.NewReader(reqbody)
	}

	// Set up the request
	url := apiClient.UrlWithoutSlash() + urlPath
	req, err := http.NewRequest(method, url, reqBodyReader)
	if err != nil {
		diags = diag.FromErr(err)
		return
	}
	if apiClient.Token != "" {
		req.Header.Add("Authorization", "Token "+apiClient.Token)
	}
	if reqBodyReader != nil {
		req.Header.Add("Content-Type", "application/json")
	}
	req.Header.Add("User-Agent", "Terraform provider for Mreg")

	// Perform the request
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		diags = diag.FromErr(err)
		return
	}
	defer response.Body.Close()

	// Read the response body
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		diags = diag.FromErr(err)
		return
	}
	bodyText = string(responseBody)

	// Check the status code
	if response.StatusCode != expectedStatus {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Got an error message from the MREG API",
			Detail: fmt.Sprintf("%s %s\nrequest body: %s\nresponse: http status %d\n%s",
				req.Method, url, requestBody, response.StatusCode, bodyText),
		})
		return
	}

	// Unmarshal the response if it is JSON
	if response.Header.Get("Content-Type") == "application/json" {
		err = json.Unmarshal(responseBody, &body)
		if err != nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  err.Error(),
				Detail:   string(responseBody),
			})
			return
		}
	}

	return
}

func (c *apiClient) login() diag.Diagnostics {
	bodyText, body, diags := c.httpRequest("POST", "/api/token-auth/", map[string]interface{}{
		"username": c.Username,
		"password": c.Password,
	}, http.StatusOK)
	if diags != nil {
		return diags
	}
	data, ok := body.(map[string]interface{})
	if !ok {
		return diag.Errorf("The Mreg token-auth endpoint returned an unexpected result: %s", bodyText)
	}
	c.Token, ok = data["token"].(string)
	if !ok {
		return diag.Errorf("The Mreg token-auth endpoint returned an unexpected result: %s", bodyText)
	}
	return nil
}
