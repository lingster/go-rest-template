# iOS code-signing checks for ios/fastlane/Fastfile. Pure functions, so they
# run anywhere: ruby tool/fastlane/signing_test.rb
require "digest"

module Signing
  # The fields of a provisioning profile the build depends on.
  Summary = Struct.new(:name, :uuid, :team_name, :team_id, :cert_sha1s, keyword_init: true)

  module_function

  # Uppercase SHA-1 of a DER certificate: what Xcode and codesign accept as an
  # exact identity, unlike "Apple Distribution" which matches every such cert.
  def sha1(der)
    Digest::SHA1.hexdigest(der).upcase
  end

  # Summary of a decoded profile plist (`security cms -D` + Plist.parse_xml;
  # <data> entries arrive as StringIO).
  def summary(plist)
    certs = Array(plist["DeveloperCertificates"]).map { |c| c.respond_to?(:read) ? c.read : c }
    Summary.new(
      name: plist["Name"],
      uuid: plist["UUID"],
      team_name: plist["TeamName"],
      team_id: Array(plist["TeamIdentifier"]).first,
      cert_sha1s: certs.map { |c| sha1(c) }
    )
  end

  # Why [profile] cannot sign with [cert_sha1] for the expected team; empty when it can.
  # An empty [team_name] (APPLE_TEAM_NAME unset) skips the team-name check.
  def problems(profile, team_id:, cert_sha1:, team_name: nil)
    msgs = []
    msgs << "profile is for team #{profile.team_id}, expected #{team_id}" unless profile.team_id == team_id
    unless team_name.to_s.empty? || profile.team_name.to_s.include?(team_name)
      msgs << "profile team name is #{profile.team_name.inspect}, expected #{team_name}"
    end
    unless profile.cert_sha1s.include?(cert_sha1)
      msgs << "profile does not include signing certificate #{cert_sha1}"
    end
    msgs
  end

  # Paths of installed profiles named like [keep] but older: Xcode resolves a
  # profile by name, so a stale copy can win and carry the wrong certificate.
  def stale(installed, keep:)
    installed.select { |_, p| p.name == keep.name && p.uuid != keep.uuid }.keys
  end

  # Export options pinned to one profile (by UUID) and one certificate (by SHA-1).
  def export_options(team_id:, app_id:, profile_uuid:, cert_sha1:)
    <<~PLIST
      <?xml version="1.0" encoding="UTF-8"?>
      <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
      <plist version="1.0"><dict>
        <key>method</key><string>app-store-connect</string>
        <key>teamID</key><string>#{team_id}</string>
        <key>signingStyle</key><string>manual</string>
        <key>signingCertificate</key><string>#{cert_sha1}</string>
        <key>provisioningProfiles</key><dict><key>#{app_id}</key><string>#{profile_uuid}</string></dict>
        <key>uploadSymbols</key><true/>
      </dict></plist>
    PLIST
  end
end
