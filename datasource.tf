data "aws_caller_identity" "this" {
  
}

data "aws_iam_policy_document" "allow_cf_oai" {
  statement {
    sid     = "AllowCloudFrontOAIRead"
    actions = ["s3:GetObject"]
    resources = ["${module.openapi_bucket.s3_bucket_arn}/*"]

    principals {
      type        = "AWS"
      identifiers = [module.cdn.cloudfront_origin_access_identity_iam_arns[0]]
    }
  }
}