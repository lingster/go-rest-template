# Run: ruby tool/fastlane/signing_test.rb
require "minitest/autorun"
require "digest"
require "stringio"
require_relative "signing"

class SigningTest < Minitest::Test
  CERT = "fake-der-cert".b
  OTHER = "another-der-cert".b
  CERT_SHA1 = Digest::SHA1.hexdigest(CERT).upcase

  def profile(team_name: "Acme Limited", team_id: "ABCDE12345", certs: [CERT])
    {
      "Name" => "com.acme.app AppStore",
      "UUID" => "1111-2222",
      "TeamName" => team_name,
      "TeamIdentifier" => [team_id],
      "DeveloperCertificates" => certs.map { |c| StringIO.new(c) }
    }
  end

  def test_sha1_is_uppercase_hex_of_der
    assert_equal CERT_SHA1, Signing.sha1(CERT)
  end

  def test_summary_reads_profile_fields
    s = Signing.summary(profile(certs: [CERT, OTHER]))
    assert_equal "com.acme.app AppStore", s.name
    assert_equal "1111-2222", s.uuid
    assert_equal "Acme Limited", s.team_name
    assert_equal "ABCDE12345", s.team_id
    assert_equal [CERT_SHA1, Signing.sha1(OTHER)], s.cert_sha1s
  end

  def test_summary_accepts_plain_string_certs
    s = Signing.summary(profile.merge("DeveloperCertificates" => [CERT]))
    assert_equal [CERT_SHA1], s.cert_sha1s
  end

  def test_no_problems_for_matching_profile_with_the_cert
    assert_empty problems(profile)
  end

  def test_flags_profile_from_another_team
    msgs = problems(profile(team_name: "Other Limited", team_id: "OTHER1234"))
    assert(msgs.any? { |m| m.include?("team OTHER1234") })
    assert(msgs.any? { |m| m.include?("Other Limited") })
  end

  def test_team_name_check_is_skipped_when_not_configured
    # a fresh plist per call: <data> entries are IO objects that can be read once
    summary = -> { Signing.summary(profile(team_name: "Whatever Ltd")) }
    assert_empty Signing.problems(summary.call, team_id: "ABCDE12345", cert_sha1: CERT_SHA1)
    assert_empty Signing.problems(summary.call, team_id: "ABCDE12345", team_name: "", cert_sha1: CERT_SHA1)
  end

  def test_flags_profile_without_the_signing_cert
    msgs = problems(profile(certs: [OTHER]))
    assert(msgs.any? { |m| m.include?(CERT_SHA1) })
  end

  def test_export_options_pin_profile_uuid_and_cert_sha1
    xml = Signing.export_options(team_id: "ABCDE12345", app_id: "com.acme.app",
                                 profile_uuid: "1111-2222", cert_sha1: CERT_SHA1)
    assert_includes xml, "<key>teamID</key><string>ABCDE12345</string>"
    assert_includes xml, "<key>signingCertificate</key><string>#{CERT_SHA1}</string>"
    assert_includes xml, "<key>com.acme.app</key><string>1111-2222</string>"
    assert_includes xml, "<key>method</key><string>app-store-connect</string>"
  end

  def test_stale_profiles_share_the_name_but_not_the_uuid
    keep = Signing::Summary.new(name: "P", uuid: "new")
    old = Signing::Summary.new(name: "P", uuid: "old")
    other = Signing::Summary.new(name: "Other app", uuid: "x")
    installed = { "/p/new.mobileprovision" => keep, "/p/old.mobileprovision" => old,
                  "/p/x.mobileprovision" => other }
    assert_equal ["/p/old.mobileprovision"], Signing.stale(installed, keep: keep)
  end

  private

  def problems(plist)
    Signing.problems(Signing.summary(plist), team_id: "ABCDE12345", team_name: "Acme",
                                             cert_sha1: CERT_SHA1)
  end
end
