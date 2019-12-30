provider "google" {
    scopes = [
    "https://www.googleapis.com/auth/cloud-platform",
    "https://www.googleapis.com/auth/userinfo.email",
  ]
}

data "google_service_account_access_token" "default" {
 provider = "google"
 target_service_account = "impersonated-account@fabled-ray-104117.iam.gserviceaccount.com"
 scopes = ["userinfo-email", "cloud-platform"]
 lifetime = "300s"
}

data "google_client_openid_userinfo" "me" { }

output "source-email" {
  value = "${data.google_client_openid_userinfo.me.email}"
}

provider "google" {
   alias  = "impersonated"
   access_token = "${data.google_service_account_access_token.default.access_token}"
}

data "google_client_openid_userinfo" "reallyme" {
  provider = "google.impersonated"
}

output "target-email" {
  value = "${data.google_client_openid_userinfo.reallyme.email}"
}
