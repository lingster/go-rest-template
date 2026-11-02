# Version name and build number for store builds, shared by the Android and
# iOS Fastfiles. Tests:
#   ruby tool/fastlane/build_info_test.rb
#
# Releases are GitHub releases: the tag (v1.2.3, or v1.2.3-rc.1 for a
# pre-release) is the version, so pubspec.yaml never needs a bump.
module BuildInfo
  VERSION_LINE = /^version:\s*([^+\s]+)(?:\+(\d+))?/
  # vMAJOR.MINOR.PATCH with an optional semver pre-release/build suffix.
  TAG = /\Av(\d+\.\d+\.\d+)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?\z/

  module_function

  # Release tag v1.2.3 / v1.2.3-rc.1 -> "1.2.3". No tag (a local run) ->
  # pubspec's version. Anything else is an error, so a mistyped tag can never
  # reach a store.
  def name(tag:, pubspec:)
    return pubspec_version(pubspec) if tag.to_s.empty?

    match = tag.match(TAG) or
      raise ArgumentError, "release tag #{tag.inspect} is not semver; use vMAJOR.MINOR.PATCH (e.g. v1.2.3 or v1.2.3-rc.1)"
    match[1]
  end

  # Strictly above every number the store already has, the pubspec number and
  # the CI run number: unique across re-runs and across workflows, and
  # tracking GITHUB_RUN_NUMBER whenever that is ahead.
  def next_number(store_numbers:, pubspec:, run_number: nil)
    floor = (store_numbers + [pubspec_number(pubspec)]).max + 1
    [floor, run_number.to_i].max
  end

  # The +N part of pubspec's version, or 0.
  def pubspec_number(pubspec)
    pubspec.match(VERSION_LINE)&.[](2).to_i
  end

  def pubspec_version(pubspec)
    match = pubspec.match(VERSION_LINE) or raise ArgumentError, "no version: in pubspec.yaml"
    match[1]
  end
end
