terraform {
  backend "s3" {
    bucket  = "fernando.rodrigues-tfstates"
    key     = "cloudbuilder/terraform.tfstate"
    region  = "us-east-1"
    profile = "pessoal"
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region  = "us-east-1"
  profile = "pessoal"
}