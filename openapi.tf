module "cdn" {
  source  = "terraform-aws-modules/cloudfront/aws"
  version = "~> 5.0"

  # REMOVA os aliases (ou deixe como lista vazia)
  # aliases = ["cdn.${var.project}.com"]

  comment             = "${var.project}-cloudfront"
  enabled             = true
  is_ipv6_enabled     = true
  price_class         = "PriceClass_All"
  retain_on_delete    = false
  wait_for_deployment = false

  default_root_object = "index.html"

  # OAI (segue igual ao seu)
  create_origin_access_identity = true
  origin_access_identities = {
    s3_bucket_one = "CloudFront may access this bucket"
  }

  origin = {
    s3_docs = {
      # Use o domain_name REST do bucket (NÃO o website endpoint)
      domain_name = module.openapi_bucket.s3_bucket_bucket_regional_domain_name
      s3_origin_config = {
        origin_access_identity = "s3_bucket_one"
      }
    }
  }

  default_cache_behavior = {
    target_origin_id       = "s3_docs"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true
    query_string           = false
  }

  # <<< Certificado padrão do CloudFront >>>
  viewer_certificate = {
    cloudfront_default_certificate = true
    minimum_protocol_version       = "TLSv1.2_2021"
  }
}

module "openapi_bucket" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 5.3"

  bucket = "${var.project}-openapi"
  acl    = "private"

  control_object_ownership = true
  object_ownership         = "ObjectWriter"

  website = {
    index_document = "index.html"
  }

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
  attach_policy = true

  policy = data.aws_iam_policy_document.allow_cf_oai.json

  versioning = {
    enabled = false
  }
}

module "openapi_config" {
  source  = "terraform-aws-modules/s3-bucket/aws//modules/object"
  version = "~> 4.0"
  
  bucket      = module.openapi_bucket.s3_bucket_id
  key         = "openapi.yaml"
  file_source = "dist/openapi.yaml"

  etag = filemd5("dist/openapi.yaml")

  acl  = "private"
  tags = local.tags
}