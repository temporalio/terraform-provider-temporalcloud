---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "temporalcloud_namespaces Data Source - terraform-provider-temporalcloud"
subcategory: ""
description: |-
  
---

# temporalcloud_namespaces (Data Source)



## Example Usage

```terraform
terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

data "temporalcloud_namespaces" "my_namespaces" {}

output "namespaces" {
  value = data.temporalcloud_namespaces.my_namespaces.namespaces
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Read-Only

- `id` (String) The ID of this resource.
- `namespaces` (Attributes List) (see [below for nested schema](#nestedatt--namespaces))

<a id="nestedatt--namespaces"></a>
### Nested Schema for `namespaces`

Optional:

- `certificate_filters` (Attributes List) A list of filters to apply to client certificates when initiating a connection Temporal Cloud. If present, connections will only be allowed from client certificates whose distinguished name properties match at least one of the filters. (see [below for nested schema](#nestedatt--namespaces--certificate_filters))
- `codec_server` (Attributes) A codec server is used by the Temporal Cloud UI to decode payloads for all users interacting with this namespace, even if the workflow history itself is encrypted. (see [below for nested schema](#nestedatt--namespaces--codec_server))
- `custom_search_attributes` (Map of String) The custom search attributes to use for the namespace.
- `last_modified_time` (String) The date and time when the namespace was last modified. Will not be set if the namespace has never been modified.
- `private_connectivities` (Attributes List) The private connectivities for the namespace, if any. (see [below for nested schema](#nestedatt--namespaces--private_connectivities))

Read-Only:

- `accepted_client_ca` (String) The Base64-encoded CA cert in PEM format that clients use when authenticating with Temporal Cloud.
- `active_region` (String) The currently active region for the namespace.
- `created_time` (String) The date and time when the namespace was created.
- `endpoints` (Attributes) The endpoints for the namespace. (see [below for nested schema](#nestedatt--namespaces--endpoints))
- `id` (String) The unique identifier of the namespace across all Temporal Cloud tenants.
- `limits` (Attributes) The limits set on the namespace currently. (see [below for nested schema](#nestedatt--namespaces--limits))
- `name` (String) The name of the namespace.
- `regions` (List of String) The list of regions that this namespace is available in. If more than one region is specified, this namespace is "global" which is currently a preview feature with restricted access. Please reach out to Temporal support for more information on this feature.
- `retention_days` (Number) The number of days to retain workflow history. Any changes to the retention period will be applied to all new running workflows.
- `state` (String) The current state of the namespace.

<a id="nestedatt--namespaces--certificate_filters"></a>
### Nested Schema for `namespaces.certificate_filters`

Read-Only:

- `common_name` (String) The certificate's common name.
- `organization` (String) The certificate's organization.
- `organizational_unit` (String) The certificate's organizational unit.
- `subject_alternative_name` (String) The certificate's subject alternative name (or SAN).


<a id="nestedatt--namespaces--codec_server"></a>
### Nested Schema for `namespaces.codec_server`

Read-Only:

- `endpoint` (String) The endpoint of the codec server.
- `include_cross_origin_credentials` (Boolean) If true, Temporal Cloud will include cross-origin credentials in requests to the codec server.
- `pass_access_token` (Boolean) If true, Temporal Cloud will pass the access token to the codec server upon each request.


<a id="nestedatt--namespaces--private_connectivities"></a>
### Nested Schema for `namespaces.private_connectivities`

Optional:

- `aws_private_link_info` (Attributes) The AWS PrivateLink info. This will only be set for namespaces whose cloud provider is AWS. (see [below for nested schema](#nestedatt--namespaces--private_connectivities--aws_private_link_info))

Read-Only:

- `region` (String) The id of the region where the private connectivity applies.

<a id="nestedatt--namespaces--private_connectivities--aws_private_link_info"></a>
### Nested Schema for `namespaces.private_connectivities.aws_private_link_info`

Read-Only:

- `allowed_principal_arns` (List of String) The list of principal arns that are allowed to access the namespace on the private link.
- `vpc_endpoint_service_names` (List of String) The list of vpc endpoint service names that are associated with the namespace.



<a id="nestedatt--namespaces--endpoints"></a>
### Nested Schema for `namespaces.endpoints`

Read-Only:

- `grpc_address` (String) The gRPC hostport address that the temporal workers, clients and tctl connect to.
- `web_address` (String) The web UI address.


<a id="nestedatt--namespaces--limits"></a>
### Nested Schema for `namespaces.limits`

Read-Only:

- `actions_per_second_limit` (Number) The number of actions per second (APS) that is currently allowed for the namespace. The namespace may be throttled if its APS exceeds the limit.