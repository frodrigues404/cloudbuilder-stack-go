locals {
  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

variable "environment" {
  default = "development"
}

variable "project" {
  default = "cloudbuilder"
}

variable "region" {
  default = "us-east-1"
}