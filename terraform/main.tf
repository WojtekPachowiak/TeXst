terraform {
  required_providers {
    authentik = {
      source  = "goauthentik/authentik"
      version = "2026.5"
    }
  }
}

variable "authentik_token" {
  type      = string
  sensitive = true
}

provider "authentik" {
  url      = "https://auth.texstwojtek.com"
  token    = var.authentik_token
  insecure = true
}

variable "authentik_background" {
  type    = string
  default = "branding/background.jpg"
}


//========================================== github login

data "authentik_flow" "source_auth" {
  slug = "default-source-authentication"
}

data "authentik_flow" "source_enroll" {
  slug = "default-source-enrollment"
}

variable "github_client_id" {
  type    = string
  default = "Ov23liS2X6cgy32RO6kZ"
}

variable "github_client_secret" {
  type      = string
  sensitive = true
}

resource "authentik_source_oauth" "github" {
  name                = "Github"
  slug                = "github"
  provider_type       = "github"
  consumer_key        = var.github_client_id
  consumer_secret     = var.github_client_secret
  authentication_flow = data.authentik_flow.source_auth.id
  enrollment_flow     = data.authentik_flow.source_enroll.id
}


//==================== GOOGLE LOGIN

variable "google_client_id" {
  type    = string
  default = "532785147557-nhcm4nen1l2gvd3hu6s81rlet93r6uiq.apps.googleusercontent.com"
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}

resource "authentik_source_oauth" "google" {
  name                = "Google"
  slug                = "google"
  provider_type       = "google"
  consumer_key        = var.google_client_id
  consumer_secret     = var.google_client_secret
  authentication_flow = data.authentik_flow.source_auth.id
  enrollment_flow     = data.authentik_flow.source_enroll.id
}

//======================== LOGOUT

resource "authentik_flow" "logout" {
  name        = "custom-logout"
  title       = "You've been signed out"
  slug        = "custom-logout"
  designation = "invalidation"
  background  = var.authentik_background
}

resource "authentik_stage_user_logout" "logout" {
  name = "end-authentik-session"
}

resource "authentik_flow_stage_binding" "logout_1" {
  target = authentik_flow.logout.uuid
  stage  = authentik_stage_user_logout.logout.id
  order  = 10
}

//======================RECOVERY

resource "authentik_flow" "recovery" {
  name        = "custom-recovery"
  title       = "Reset your password"
  slug        = "custom-recovery"
  designation = "recovery"
  background  = var.authentik_background
}

resource "authentik_stage_identification" "recovery_ident" {
  name        = "recovery-identification"
  user_fields = ["username", "email"]
}

resource "authentik_stage_email" "recovery_email" {
  name                = "recovery-email"
  use_global_settings = true # or set host/port/username/password/use_tls explicitly
  subject             = "Reset your password"
}

resource "authentik_stage_prompt_field" "new_password" {
  name      = "password"
  field_key = "password"
  label     = "New password"
  type      = "password"
  required  = true
  order     = 0
}
resource "authentik_stage_prompt_field" "new_password_repeat" {
  name      = "password_repeat"
  field_key = "password_repeat"
  label     = "Confirm password"
  type      = "password"
  required  = true
  order     = 1
}
resource "authentik_stage_prompt" "recovery_password_prompt" {
  name = "recovery-set-password"
  fields = [
    authentik_stage_prompt_field.new_password.id,
    authentik_stage_prompt_field.new_password_repeat.id,
  ]
}

resource "authentik_stage_user_write" "recovery_write" {
  name = "recovery-user-write"
}

data "authentik_stage" "default_login" {
  name = "default-authentication-login"
}

resource "authentik_flow_stage_binding" "recovery_1" {
  target = authentik_flow.recovery.uuid
  stage  = authentik_stage_identification.recovery_ident.id
  order  = 10
}
resource "authentik_flow_stage_binding" "recovery_2" {
  target = authentik_flow.recovery.uuid
  stage  = authentik_stage_email.recovery_email.id
  order  = 20
}
resource "authentik_flow_stage_binding" "recovery_3" {
  target = authentik_flow.recovery.uuid
  stage  = authentik_stage_prompt.recovery_password_prompt.id
  order  = 30
}
resource "authentik_flow_stage_binding" "recovery_4" {
  target = authentik_flow.recovery.uuid
  stage  = authentik_stage_user_write.recovery_write.id
  order  = 40
}
resource "authentik_flow_stage_binding" "recovery_5" {
  target = authentik_flow.recovery.uuid
  stage  = data.authentik_stage.default_login.id
  order  = 100
}

//==================================== SIGN UP LOGIN+PASSWORD

resource "authentik_flow" "signup" {
  name        = "custom-signup"
  title       = "Create your account"
  slug        = "custom-signup"
  designation = "enrollment"
  background  = var.authentik_background
}

resource "authentik_stage_prompt_field" "signup_username" {
  name      = "signup-username"
  field_key = "username" # matches the User model field the User Write stage will populate
  label     = "Username"
  type      = "username"
  required  = true
  order     = 0
}
# resource "authentik_stage_prompt_field" "signup_name" {
#   name      = "signup-name"
#   field_key = "name"
#   label     = "Full name"
#   type      = "text"
#   required  = true
#   order     = 1
# }
resource "authentik_stage_prompt_field" "signup_email" {
  name      = "signup-email"
  field_key = "email"
  label     = "Email"
  type      = "email"
  required  = true
  order     = 2
}
resource "authentik_stage_prompt_field" "signup_password" {
  name      = "signup-password"
  field_key = "password"
  label     = "Password"
  type      = "password"
  required  = true
  order     = 3
}
resource "authentik_stage_prompt_field" "signup_password_repeat" {
  name      = "signup-password-repeat"
  field_key = "password_repeat"
  label     = "Confirm password"
  type      = "password"
  required  = true
  order     = 4
}

resource "authentik_stage_prompt" "signup_prompt" {
  name = "signup-prompt"
  fields = [
    authentik_stage_prompt_field.signup_username.id,
    # authentik_stage_prompt_field.signup_name.id,
    authentik_stage_prompt_field.signup_email.id,
    authentik_stage_prompt_field.signup_password.id,
    authentik_stage_prompt_field.signup_password_repeat.id,
  ]
  validation_policies = [authentik_policy_password.signup_password.id] # see below
}

resource "authentik_stage_user_write" "signup_write" {
  name                     = "signup-user-write"
  user_creation_mode       = "always_create"
  user_type                = "external" # same reasoning as the GitHub fix — avoid the "external" restrictions
  create_users_as_inactive = true       # set true if you add email verification, below
}

resource "authentik_stage_email" "signup_verify" {
  name                     = "signup-email-verification"
  use_global_settings      = true
  activate_user_on_success = true
  subject                  = "Confirm your account"
  template                 = "email/account_confirmation.html"
}


resource "authentik_flow_stage_binding" "signup_1" {
  target = authentik_flow.signup.uuid
  stage  = authentik_stage_prompt.signup_prompt.id
  order  = 10
}

resource "authentik_flow_stage_binding" "signup_2" {
  target = authentik_flow.signup.uuid
  stage  = authentik_stage_user_write.signup_write.id
  order  = 20
}

resource "authentik_flow_stage_binding" "signup_2b" {
  target = authentik_flow.signup.uuid
  stage  = authentik_stage_email.signup_verify.id
  order  = 25 # between prompt (10) and user_write (20)
}

resource "authentik_flow_stage_binding" "signup_3" {
  target = authentik_flow.signup.uuid
  stage  = data.authentik_stage.default_login.id # the same data source used elsewhere
  order  = 100
}

//========================================== CUSTOM LOGIN

resource "authentik_flow" "login" {
  name        = "custom-login"
  title       = "Sign in"
  slug        = "custom-login"
  designation = "authentication"
  background  = var.authentik_background
}

resource "authentik_stage_password" "login_password" {
  name     = "login-password"
  backends = ["authentik.core.auth.InbuiltBackend"]
}


resource "authentik_stage_identification" "login_ident" {
  name           = "login-identification"
  user_fields    = ["username", "email"]
  password_stage = authentik_stage_password.login_password.id
  sources = [
    authentik_source_oauth.github.uuid,
    authentik_source_oauth.google.uuid
  ]
  recovery_flow             = authentik_flow.recovery.uuid
  case_insensitive_matching = true
  enrollment_flow           = authentik_flow.signup.uuid
}

resource "authentik_policy_password" "signup_password" {
  name                    = "signup-password-policy"
  length_min              = 8
  amount_uppercase        = 1
  amount_lowercase        = 1
  amount_digits           = 1
  check_have_i_been_pwned = true
  error_message           = "Password must be at least 8 characters, include upper/lowercase and a digit, and not appear in known breaches."
}

resource "authentik_flow_stage_binding" "login_1" {
  target = authentik_flow.login.uuid
  stage  = authentik_stage_identification.login_ident.id
  order  = 10
}
resource "authentik_flow_stage_binding" "login_2" {
  target = authentik_flow.login.uuid
  stage  = data.authentik_stage.default_login.id
  order  = 20
}




//==================== BRANDING

# data "authentik_flow" "invalidation" {
#   slug = "default-invalidation-flow"
# }

# import {
#   to = authentik_flow.invalidation
#   id  = data.authentik_flow.invalidation.id
# }

# resource "authentik_flow" "invalidation" {
#   background = ""
#   designation = "invalidation"
#   name=""
#   invalidation=""
# }

# data "authentik_brand" "authentik-default" {
#   domain = "authentik-default"
# }

# import {
#   to = authentik_brand.default_brand
#   id = data.authentik_brand.authentik-default.id
# }

resource "authentik_brand" "texstwojtek_brand" {
  domain  = "texstwojtek.com"
  default = false

  # flow_invalidation                = data.authentik_flow.invalidation.id
  flow_invalidation                = authentik_flow.logout.uuid
  flow_authentication              = authentik_flow.login.uuid
  flow_recovery                    = authentik_flow.recovery.uuid
  default_application              = authentik_application.tex-typst-rendering-cluster-2.uuid
  branding_title                   = "TeXst"
  branding_logo                    = "branding/logo.svg"
  branding_favicon                 = "branding/favicon.ico"
  branding_default_flow_background = "branding/background.jpg"
  # branding_custom_css              = <<-CSS
  #   :root {
  #     --ak-accent: #6366f1;
  #   }
  #   .pf-c-login__main {
  #     backdrop-filter: blur(6px);
  #   }
  # CSS
  # lifecycle { prevent_destroy = true }
}

// ================ PROVIDER

data "authentik_flow" "authorization_implicit_consent" {
  slug = "default-provider-authorization-implicit-consent"
}

resource "authentik_provider_proxy" "traefik_forward_auth" {
  name               = "traefik-forward-auth"
  mode               = "forward_single"
  external_host      = "https://app.texstwojtek.com" # Authentik's own public URL
  authorization_flow = data.authentik_flow.authorization_implicit_consent.id
  # invalidation_flow     = data.authentik_flow.invalidation.id
  invalidation_flow     = authentik_flow.logout.uuid
  access_token_validity = "hours=24"

  # authentication_flow = authentik_flow.login.uuid
}


//=================================== APPLICATION

resource "authentik_application" "tex-typst-rendering-cluster-2" {
  name              = "tex-typst-rendering-cluster-2"
  slug              = "tex-typst-rendering-cluster-2"
  protocol_provider = authentik_provider_proxy.traefik_forward_auth.id
  meta_launch_url   = "https://app.texstwojtek.com" # not a "real" app people click into

  lifecycle {
    ignore_changes = [meta_icon] # provider doesn't manage the auto-generated icon well
  }
}

// ===================== OUTPOST

# data "http" "embedded_outpost" {
#   url = "https://auth.texstwojtek.com/api/v3/outposts/instances/"
#   request_headers = {
#     Authorization = "Bearer ${var.authentik_token}"
#     Accept        = "application/json"
#   }
# }

# import {
#   to = authentik_outpost.embedded
#   id = jsondecode(data.http.embedded_outpost.response_body).results[0].pky
# }

# resource "authentik_outpost" "embedded" {
#   name = "authentik Embedded Outpost"
#   type = "proxy"
#   protocol_providers = [
#     authentik_provider_proxy.traefik_forward_auth.id,
#   ]
#   # service_connection = "403a40bd-821a-4a5b-9a89-ba3938e1fa8e"  # service_connection_obj.pk from the curl
#   lifecycle { prevent_destroy = true }
# }

data "authentik_outpost" "embedded" {
  name = "authentik Embedded Outpost"
}

resource "authentik_outpost_provider_attachment" "attachment" {
  outpost           = data.authentik_outpost.embedded.id
  protocol_provider = authentik_provider_proxy.traefik_forward_auth.id
}
