terraform {
  required_providers {
    mreg = {
      version = "0.1.6"
      source  = "uio.no/usit/mreg"
    }
  }
}

provider "mreg" {
  serverurl = "https://mreg-test.example.com/"
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
    manual_ipaddress = "192.168.0.55"
  }
  host {
    name = "terraform-provider-test03.example.com"
  }
  contact = "your.email.address@example.com"
  comment = "Created by the Terraform provider for Mreg"
  network = "192.168.0.0/16"
  policies = "without_monitoring, backup_no_backup"
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
  network = "192.168.0.0/16"
}

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

# hosts with both IPv4 and IPv6
resource "mreg_hosts" "host_with_multiple_ips" {
  host {
    name = "terraform-provider-test04.example.com"
  }
  host {
    name = "terraform-provider-test05.example.com"
    manual_ipaddress = "192.168.0.243,fd12:3456:789a:1::1" # the manual_ipaddress field can have a comma-separated list of addresses
  }
  contact = "your.email.address@example.com"
  comment = "Created by the Terraform provider for Mreg"
  network = "192.168.0.0/16,fc00::/7"     # the network field can have a comma-separated list of networks
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

output "qux" {
  value = mreg_hosts.host_with_multiple_ips
}
