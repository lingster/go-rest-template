# Checks of the store listings against the stores' limits, so a bad listing
# fails in CI instead of in the upload:
#   Google Play: flutter_app/android/fastlane/metadata/android/<language>/
#   App Store:   flutter_app/ios/fastlane/metadata/<locale>/ and
#                flutter_app/ios/fastlane/screenshots/<locale>/ (deliver layout)
# Tests:
#   ruby tool/fastlane/listing_test.rb
module Listing
  # Text file => max characters (Play counts characters, not bytes).
  TEXT_LIMITS = {
    "title.txt" => 30,
    "short_description.txt" => 80,
    "full_description.txt" => 4000
  }.freeze

  # App Store (fastlane deliver 2.240.1 file names; limits from App Store Connect).
  IOS_TEXT_LIMITS = {
    "name.txt" => 30,
    "subtitle.txt" => 30,
    "keywords.txt" => 100,
    "promotional_text.txt" => 170,
    "description.txt" => 4000,
    "release_notes.txt" => 4000,
    "support_url.txt" => 2048,
    "marketing_url.txt" => 2048,
    "privacy_url.txt" => 2048
  }.freeze
  IOS_URL_FILES = %w[support_url.txt marketing_url.txt privacy_url.txt].freeze
  IOS_ROOT_FILES = %w[copyright.txt primary_category.txt].freeze
  # App Store Connect category id, e.g. PRODUCTIVITY or HEALTH_AND_FITNESS
  # (https://docs.fastlane.tools/actions/deliver/#reference for the list).
  IOS_CATEGORY = /\A[A-Z][A-Z_]+\z/
  # Screenshot sizes (portrait) deliver maps to the 6.9" iPhone and 13" iPad slots.
  IPHONE_SIZE = [1320, 2868].freeze
  IPAD_SIZE = [2064, 2752].freeze
  IOS_SCREENSHOTS = (1..10).freeze

  PHONE_SCREENSHOTS = (2..8).freeze
  PHONE_SIZE = [1080, 1920].freeze
  FEATURE_GRAPHIC_SIZE = [1024, 500].freeze
  ICON_SIZE = [512, 512].freeze
  PNG_SIGNATURE = "\x89PNG\r\n\x1a\n".b.freeze
  RGB = 2 # PNG colour type: truecolour, no alpha

  module_function

  # Human-readable problems of one language directory; empty when it is fine.
  def problems(dir)
    text_problems(dir) + image_problems(dir)
  end

  def text_problems(dir, limits = TEXT_LIMITS)
    limits.flat_map do |file, max|
      path = File.join(dir, file)
      next ["#{file} is missing"] unless File.file?(path)

      length = File.read(path, encoding: "UTF-8").strip.length
      if length.zero? then ["#{file} is empty"]
      elsif length > max then ["#{file} has #{length} characters, the limit is #{max}"]
      else []
      end
    end
  end

  # Problems of the whole App Store listing: the metadata root, every locale
  # folder in it, and the screenshots of the same locales.
  def ios_problems(metadata, screenshots)
    locales = Dir.children(metadata).select { |d| File.directory?(File.join(metadata, d)) }.sort
    return ["no locale folders in #{metadata}"] if locales.empty?

    problems = ios_root_problems(metadata)
    locales.each do |locale|
      found = ios_locale_problems(File.join(metadata, locale)) +
              ios_screenshot_problems(File.join(screenshots, locale))
      problems.concat(found.map { |p| "#{locale}: #{p}" })
    end
    problems
  end

  def ios_root_problems(metadata)
    problems = IOS_ROOT_FILES.reject { |f| File.file?(File.join(metadata, f)) }.map { |f| "#{f} is missing" }
    category = File.join(metadata, "primary_category.txt")
    if File.file?(category) && !File.read(category).strip.match?(IOS_CATEGORY)
      problems << "primary_category.txt must be an App Store category id such as PRODUCTIVITY"
    end
    problems
  end

  def ios_locale_problems(dir)
    problems = text_problems(dir, IOS_TEXT_LIMITS)
    problems.concat(keyword_problems(dir))
    IOS_URL_FILES.each do |file|
      path = File.join(dir, file)
      next unless File.file?(path)

      problems << "#{file} is not an https URL" unless File.read(path).strip.match?(%r{\Ahttps://\S+\z})
    end
    problems
  end

  # Apple wants "a,b,c" with no space after a comma, and already indexes the
  # name and subtitle, so keywords must not repeat their words.
  def keyword_problems(dir)
    path = File.join(dir, "keywords.txt")
    return [] unless File.file?(path)

    text = File.read(path, encoding: "UTF-8").strip
    problems = []
    problems << "keywords.txt has a space after a comma or around a comma" if text.match?(/\s*,\s+|\s+,/)
    taken = %w[name.txt subtitle.txt].flat_map { |f| words(File.join(dir, f)) }.map { |w| stem(w) }
    repeated = text.split(",").map(&:strip).select { |k| words_in(k).any? { |w| taken.include?(stem(w)) } }
    problems << "keywords.txt repeats words of the name or subtitle: #{repeated.join(", ")}" unless repeated.empty?
    problems
  end

  def words(path)
    File.file?(path) ? words_in(File.read(path, encoding: "UTF-8")) : []
  end

  def words_in(text)
    text.downcase.scan(/[[:alnum:]]+/)
  end

  # Singular-ish form, as Apple matches plurals to singulars.
  def stem(word)
    word.length > 3 ? word.sub(/s\z/, "") : word
  end

  # Screenshots of one locale: 1-10 at the iPhone size and 1-10 at the iPad
  # size (the app supports iPad), nothing else, RGB without alpha.
  def ios_screenshot_problems(dir)
    shots = Dir[File.join(dir, "*.{png,jpg,jpeg}")].sort
    sizes = shots.map { |f| png_info(f)&.first(2) }
    problems = shots.flat_map do |f|
      size = png_info(f)&.first(2)
      if [IPHONE_SIZE, IPAD_SIZE].include?(size) then png_problems(f, size, alpha: false)
      else ["#{File.basename(f)} is #{size ? size.join("x") : "not a PNG"}, expected #{IPHONE_SIZE.join("x")} or #{IPAD_SIZE.join("x")}"]
      end
    end
    { "iPhone 6.9\"" => IPHONE_SIZE, "iPad 13\"" => IPAD_SIZE }.each do |label, size|
      count = sizes.count(size)
      problems << "need #{IOS_SCREENSHOTS} #{label} screenshots, found #{count}" unless IOS_SCREENSHOTS.cover?(count)
    end
    problems
  end

  def image_problems(dir)
    images = File.join(dir, "images")
    shots = Dir[File.join(images, "phoneScreenshots", "*.{png,jpg,jpeg}")].sort
    problems = []
    problems << "need #{PHONE_SCREENSHOTS} phone screenshots, found #{shots.size}" unless PHONE_SCREENSHOTS.cover?(shots.size)
    shots.each { |f| problems.concat(png_problems(f, PHONE_SIZE, alpha: false)) }
    problems.concat(png_problems(File.join(images, "featureGraphic.png"), FEATURE_GRAPHIC_SIZE, alpha: false))
    problems.concat(png_problems(File.join(images, "icon.png"), ICON_SIZE, alpha: true))
  end

  # [width, height, colour type] from a PNG's IHDR, or nil when it is not a PNG.
  def png_info(path)
    head = File.binread(path, 26)
    return nil unless head && head.start_with?(PNG_SIGNATURE)

    head[16, 8].unpack("NN") + [head.getbyte(25)]
  end

  def png_problems(path, size, alpha:)
    name = File.basename(path)
    return ["#{name} is missing"] unless File.file?(path)

    width, height, color = png_info(path)
    return ["#{name} is not a PNG"] unless width

    problems = []
    problems << "#{name} is #{width}x#{height}, expected #{size.join("x")}" unless [width, height] == size
    problems << "#{name} has an alpha channel; Play needs 24-bit PNG" if !alpha && color != RGB
    problems
  end
end
