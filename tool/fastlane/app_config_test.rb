# Run: ruby tool/fastlane/app_config_test.rb
require "minitest/autorun"
require_relative "app_config"

class AppConfigTest < Minitest::Test
  HOSTED = { "RUNNER_ENVIRONMENT" => "github-hosted", "RUNNER_TEMP" => "/tmp/runner" }.freeze

  def test_team_id_comes_from_the_environment
    assert_equal "ABCDE12345", AppConfig.apple_team_id({ "APPLE_TEAM_ID" => " ABCDE12345 " })
  end

  def test_team_id_is_required_when_there_is_no_default
    error = assert_raises(ArgumentError) { AppConfig.apple_team_id({}) }
    assert_match(/APPLE_TEAM_ID/, error.message)
  end

  def test_optional_values_are_nil_when_blank
    assert_nil AppConfig.apple_app_id({ "APPLE_APP_ID" => "" })
    assert_nil AppConfig.apple_team_name({})
    assert_equal "123", AppConfig.apple_app_id({ "APPLE_APP_ID" => "123" })
  end

  def test_keychain_on_a_self_hosted_mac_is_in_the_user_library
    assert_equal "/Users/ci/Library/Keychains/#{AppConfig::KEYCHAIN_NAME}.keychain-db",
                 AppConfig.keychain_path({}, home: "/Users/ci")
  end

  def test_keychain_on_a_hosted_runner_is_temporary
    assert_equal "/tmp/runner/#{AppConfig::KEYCHAIN_NAME}.keychain-db", AppConfig.keychain_path(HOSTED)
  end

  def test_keychain_path_can_be_overridden
    assert_equal "/k/other.keychain-db", AppConfig.keychain_path(HOSTED.merge("CI_KEYCHAIN_PATH" => "/k/other.keychain-db"))
  end

  def test_keychain_password_is_required_off_hosted_runners_and_generated_on_them
    assert_raises(ArgumentError) { AppConfig.keychain_password({}) }
    assert_equal "pw", AppConfig.keychain_password({ "CI_KEYCHAIN_PASSWORD" => "pw" })
    refute_empty AppConfig.keychain_password(HOSTED)
  end

  def test_dart_defines_skip_unset_values
    env = { "API_BASE_URL" => "https://api.example.com", "GOOGLE_SERVER_CLIENT_ID" => "", "GOOGLE_IOS_CLIENT_ID" => "x" }
    assert_equal %w[--dart-define=APP_VERSION=1.2.3 --dart-define=API_BASE_URL=https://api.example.com
                    --dart-define=GOOGLE_IOS_CLIENT_ID=x],
                 AppConfig.dart_defines("1.2.3", env)
    assert_equal ["--dart-define=APP_VERSION=1.2.3"], AppConfig.dart_defines("1.2.3", {})
  end
end
