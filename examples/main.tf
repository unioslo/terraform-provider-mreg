terraform {
  required_providers {
    mreg = {
      version = "0.1.4"
      source  = "uio.no/usit/mreg"
    }
  }
}

provider "mreg" {
  serverurl = "https://mreg-test01.example.com/"
  token     = "1234567890ABCDEF" # replace with actual token
}

resource "mreg_hosts" "my_hosts" {
  # You can supply more than one host in one resource
  host {
    name = "terraform-provider-test01.example.com"
  }
  host {
    name = "terraform-provider-test02.example.com"
    # You can also manually pick an ip address instead of getting assigned a free one
    manual_ipaddress = "192.0.2.55"
  }
  host {
    name = "terraform-provider-test03.example.com"
  }
  contact = "your.email.address@example.com"
  comment = "Created by the Terraform provider for Mreg"
  network = "192.0.2.0/24"
}

locals {
  hostnames = toset(["test01.terraform-provider-test.example.com", "test02.terraform-provider-test.example.com"])
}

resource "mreg_hosts" "loop_hosts" {
  # You can loop through a set of hostnames like this
  for_each = local.hostnames
  host {
    name = each.key
  }
  contact = "your.email.address@example.com"
  comment = "Created by the Terraform provider for Mreg"

resource "mreg_hosts" "metahosts" {
  # hosts without IP addresses
  host {
    name = "terraform-provider-meta01.example.com"
  }
  host {
    name = "terraform-provider-meta02.example.com"
  }
  contact = "your.email.address@example.com"
  comment = "Created by the Terraform provider for Mreg"
}

# Here's how to create SRV records
resource "mreg_dns_srv" "srv" {
  depends_on  = [mreg_hosts.loop_hosts]
  for_each    = local.hostnames
  target_host = each.key
  service     = "mysql"
  proto       = "tcp"
  name        = "terraform-provider-test.example.com"
  priority    = 0
  weight      = 5
  port        = 3306
}

output "foo" {
  value = mreg_hosts.my_hosts
}

output "bar" {
  value = mreg_hosts.loop_hosts
}

output "baz" {
  value = mreg_dns_srv.srv
}
