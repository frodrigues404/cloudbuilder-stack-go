variable "name" {}
variable "handler" {}
variable "path" {}
variable "api_execution_arn" {}

variable "description" {
    default = ""
}
variable "variables" {
    default = null
}
variable "attach_policy_json" {
  default = false
}
variable "timeout" {
  default = 120
}
variable "policy_json" {
  default = null
}