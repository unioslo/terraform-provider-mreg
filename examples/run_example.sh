#!/bin/bash
set -e
cd `dirname $0`
pushd .. >/dev/null
rm -f terraform-provider-mreg
go get -v
go build
#TODO after the provider is added to the registry, there's no need to copy the file here
rm -rf ~/.terraform.d/plugins/uio.no/usit/mreg/
mkdir -p ~/.terraform.d/plugins/uio.no/usit/mreg/0.1.5/linux_amd64
cp terraform-provider-mreg ~/.terraform.d/plugins/uio.no/usit/mreg/0.1.5/linux_amd64/
popd >/dev/null
rm -rf .terraform .terraform.lock.hcl terraform.tfstate crash.log
terraform init
terraform apply -auto-approve -parallelism=10
terraform plan -detailed-exitcode  # If there's a diff, the provider is not refreshing the state correctly
echo "Dropping you into a shell so you can inspect what was created...  exit when ready"
bash
terraform destroy -auto-approve
echo Everything works!
