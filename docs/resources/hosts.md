---
page_title: "mreg_hosts Resource - terraform-provider-mreg"
subcategory: ""
description: |-
  
---

# Resource `mreg_hosts`





## Schema

### Required

- **contact** (String)
- **host** (Block List, Min: 1) (see [below for nested schema](#nestedblock--host))
- **network** (String)

### Optional

- **comment** (String)
- **id** (String) The ID of this resource.

<a id="nestedblock--host"></a>
### Nested Schema for `host`

Required:

- **name** (String)

Optional:

- **manual_ipaddress** (String)

Read-only:

- **comment** (String)
- **contact** (String)
- **ipaddress** (String)


