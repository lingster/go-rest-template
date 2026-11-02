# Run: ruby tool/fastlane/build_info_test.rb
require "minitest/autorun"
require_relative "build_info"

class BuildInfoTest < Minitest::Test
  PUBSPEC = "name: saas_app\nversion: 1.0.0+1\n"

  def test_name_from_release_tag
    assert_equal "1.2.3", BuildInfo.name(tag: "v1.2.3", pubspec: PUBSPEC)
  end

  def test_name_from_prerelease_tag
    assert_equal "0.2.0", BuildInfo.name(tag: "v0.2.0-rc.1", pubspec: PUBSPEC)
  end

  def test_name_without_tag_uses_pubspec
    assert_equal "1.0.0", BuildInfo.name(tag: "", pubspec: PUBSPEC)
    assert_equal "1.0.0", BuildInfo.name(tag: nil, pubspec: PUBSPEC)
  end

  def test_non_semver_tags_are_rejected
    %w[1.2.3 v1.2 v1.2.3.4 release-1 vX.Y.Z v01].each do |tag|
      error = assert_raises(ArgumentError) { BuildInfo.name(tag: tag, pubspec: PUBSPEC) }
      assert_match(/not semver/, error.message)
    end
  end

  def test_next_number_is_above_the_store
    assert_equal 8, BuildInfo.next_number(store_numbers: [3, 7], pubspec: PUBSPEC)
  end

  def test_next_number_uses_pubspec_when_store_is_empty
    assert_equal 2, BuildInfo.next_number(store_numbers: [], pubspec: PUBSPEC)
  end

  def test_next_number_follows_run_number_when_ahead
    assert_equal 42, BuildInfo.next_number(store_numbers: [7], pubspec: PUBSPEC, run_number: "42")
  end

  def test_next_number_never_repeats_on_rerun
    # A re-run keeps GITHUB_RUN_NUMBER; the store already holds that number.
    assert_equal 43, BuildInfo.next_number(store_numbers: [42], pubspec: PUBSPEC, run_number: "42")
  end
end
