package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

func httpRequest(method, urlPath string, requestBody map[string]interface{}, expectedStatus int, apiClient apiClient) (bodyText string, body interface{}, diags diag.Diagnostics) {
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
	req.Header.Add("Authorization", "Token "+apiClient.Token)
	if reqBodyReader != nil {
		req.Header.Add("Content-Type", "application/json")
	}

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
