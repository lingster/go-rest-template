# The ONE place for app-specific values the Fastfiles need. scripts/init_project.py
# renames the ids and the keychain name (and fills DEFAULT_APPLE_TEAM_ID when you
# pass --apple-team-id); everything not known at template time comes from the
# environment (GitHub repo variables, see docs/STORES.md), so nothing else in the
# release tooling mentions your app. Tests:
#   ruby tool/fastlane/app_config_test.rb
module AppConfig
  # Android applicationId and iOS bundle id (renamed from the init `bundle_id`).
  ANDROID_PACKAGE = "com.example.saas_app".freeze
  IOS_BUNDLE_ID = "com.example.saasApp".freeze
  # Apple developer team id, used when the APPLE_TEAM_ID variable is not set.
  DEFAULT_APPLE_TEAM_ID = "".freeze
  # Basename of the CI keychain on a self-hosted Mac (hosted runners use a temporary one).
  KEYCHAIN_NAME = "go-rest-template-ci".freeze

  module_function

  # Apple developer team id (10 characters, developer.apple.com, Membership). Required for iOS builds.
  def apple_team_id(env = ENV)
    value(env, "APPLE_TEAM_ID") || DEFAULT_APPLE_TEAM_ID.then { |id| id.empty? ? nil : id } ||
      raise(ArgumentError, "Apple team id unknown: set the APPLE_TEAM_ID repo variable (docs/STORES.md)")
  end

  # App Store Connect "Apple ID" of the app (numeric). Optional: without it fastlane looks the app up by bundle id.
  def apple_app_id(env = ENV)
    value(env, "APPLE_APP_ID")
  end

  # Legal team name as Apple shows it. Optional: when set, a provisioning profile of another team is rejected.
  def apple_team_name(env = ENV)
    value(env, "APPLE_TEAM_NAME")
  end

  # Hosted macOS runners are clean VMs: a temporary keychain under RUNNER_TEMP. A
  # self-hosted Mac keeps a long-lived one (CI_KEYCHAIN_PATH to reuse another).
  def keychain_path(env = ENV, home: Dir.home)
    explicit = value(env, "CI_KEYCHAIN_PATH")
    return File.expand_path(explicit) if explicit

    dir = if env["RUNNER_ENVIRONMENT"] == "github-hosted" && value(env, "RUNNER_TEMP")
            env["RUNNER_TEMP"]
          else
            File.join(home, "Library/Keychains")
          end
    File.join(dir, "#{KEYCHAIN_NAME}.keychain-db")
  end

  # CI_KEYCHAIN_PASSWORD; hosted runners may omit it (the keychain is thrown away with the VM).
  def keychain_password(env = ENV)
    value(env, "CI_KEYCHAIN_PASSWORD") ||
      (env["RUNNER_ENVIRONMENT"] == "github-hosted" ? "ci-#{Process.pid}-#{rand(36**12).to_s(36)}" : nil) ||
      raise(ArgumentError, "CI_KEYCHAIN_PASSWORD secret is missing (docs/STORES.md)")
  end

  # `--dart-define=K=V` arguments of a store build. Store builds never get dev/test config.
  # API_BASE_URL unset falls back to the app default; the Google client IDs are optional.
  def dart_defines(version, env = ENV)
    defines = { "APP_VERSION" => version }
    %w[API_BASE_URL GOOGLE_SERVER_CLIENT_ID GOOGLE_IOS_CLIENT_ID].each do |name|
      defines[name] = value(env, name) if value(env, name)
    end
    defines.map { |k, v| "--dart-define=#{k}=#{v}" }
  end

  # Stripped value of an environment variable; nil when unset or blank
  # (an unset GitHub variable reaches us as "").
  def value(env, name)
    text = env[name].to_s.strip
    text.empty? ? nil : text
  end
end
