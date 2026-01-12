<!-- BEGIN_TF_DOCS -->
## Requirements

No requirements.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_cloudflare"></a> [cloudflare](#provider\_cloudflare) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [cloudflare_record.records](https://registry.terraform.io/providers/hashicorp/cloudflare/latest/docs/resources/record) | resource |
| [cloudflare_zone.zone](https://registry.terraform.io/providers/hashicorp/cloudflare/latest/docs/data-sources/zone) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_cloudflare_records"></a> [cloudflare\_records](#input\_cloudflare\_records) | A map of Cloudflare records to create | <pre>map(object({<br>    name    = string<br>    content   = string<br>    type    = string<br>    ttl     = number<br>    proxied = bool<br>  }))</pre> | n/a | yes |
| <a name="input_cloudflare_zone_name"></a> [cloudflare\_zone\_name](#input\_cloudflare\_zone\_name) | The name of the Cloudflare zone | `string` | n/a | yes |

## Outputs

No outputs.
<!-- END_TF_DOCS -->