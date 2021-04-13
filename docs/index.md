---
page_title: "Provider: Mreg"
subcategory: ""
description: |-
  The Mreg provider provides resources to interact with [Mreg](https://github.com/unioslo/mreg/) through the API.
---

# Mreg Provider

The Mreg provider provides resources to interact with [Mreg](https://github.com/unioslo/mreg/) through the API.

## Example usage

    provider "mreg" {
        serverurl = "https://mreg.example.com/"
        token     = "1234567890ABCDEF"
    }

#### Alternatively, supply a username and password:

    provider "mreg" {
        serverurl = "https://mreg.example.com/"
        username  = "bob"
        password  = "secret123"
    }

### Required configuration options

- **serverurl** (String)

### Either a token or a username and password is required

- **token** (String, Sensitive)
- **username** (String, Sensitive)
- **password** (String, Sensitive)
