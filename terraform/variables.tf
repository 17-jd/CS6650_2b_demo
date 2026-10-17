# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-east-1"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "hw5-product-api"
}

variable "service_name" {
  type    = string
  default = "hw5-product-api"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}

variable "allowed_cidrs" {
  type        = list(string)
  description = "Client IP ranges allowed to call the API, for example your public IP/32"
}

variable "docker_host" {
  type        = string
  default     = "unix:///var/run/docker.sock"
  description = "Docker daemon socket; use the Colima socket on macOS"
}
