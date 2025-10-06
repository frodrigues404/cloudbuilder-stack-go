module "openapi-bucket-policy" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-policy"

  name        = "${var.project}-openapi-bucket"
  path        = "/"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "s3:getObject",
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
 })
}

module "iam_beanstalk_ec2_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "~> 5.60"

  name = "${local.project}-${local.environment}-eb-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  managed_policy_arns = [
    "arn:aws:iam::aws:policy/AWSElasticBeanstalkWebTier",
    "arn:aws:iam::aws:policy/AWSElasticBeanstalkMulticontainerDocker",
    "arn:aws:iam::aws:policy/AWSElasticBeanstalkWorkerTier",
    "arn:aws:iam::aws:policy/AWSElasticBeanstalkEnhancedHealth"
  ]

  tags = local.tags
}

# Instance profile que o Beanstalk vai usar
resource "aws_iam_instance_profile" "beanstalk_ec2_profile" {
  name = "${local.project}-${local.environment}-eb-ec2-profile"
  role = module.iam_beanstalk_ec2_role.iam_role_name
}

# Role para o Elastic Beanstalk Service em si (permite criar recursos)
module "iam_beanstalk_service_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "~> 5.60"

  name = "${local.project}-${local.environment}-eb-service-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "elasticbeanstalk.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  managed_policy_arns = [
    "arn:aws:iam::aws:policy/service-role/AWSElasticBeanstalkService"
  ]

  tags = local.tags
}
