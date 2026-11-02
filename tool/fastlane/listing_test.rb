# Run: ruby tool/fastlane/listing_test.rb
require "minitest/autorun"
require "tmpdir"
require_relative "listing"

class ListingTest < Minitest::Test
  LISTING = File.expand_path("../../flutter_app/android/fastlane/metadata/android/en-US", __dir__)

  def test_checked_in_en_us_listing_is_valid
    assert_equal [], Listing.problems(LISTING)
  end

  def test_text_limits
    Dir.mktmpdir do |dir|
      File.write(File.join(dir, "title.txt"), "T" * 31)
      File.write(File.join(dir, "short_description.txt"), "s" * 80)
      File.write(File.join(dir, "full_description.txt"), "é" * 4000) # characters, not bytes
      problems = Listing.text_problems(dir)
      assert_equal ["title.txt has 31 characters, the limit is 30"], problems
    end
  end

  def test_missing_and_empty_text_files_are_reported
    Dir.mktmpdir do |dir|
      File.write(File.join(dir, "title.txt"), "\n")
      problems = Listing.text_problems(dir)
      assert_includes problems, "title.txt is empty"
      assert_includes problems, "short_description.txt is missing"
    end
  end

  def test_png_info_reads_size_and_colour_type
    path = File.join(LISTING, "images", "featureGraphic.png")
    assert_equal [1024, 500, Listing::RGB], Listing.png_info(path)
  end

  def test_png_problems_flag_wrong_size_and_alpha
    icon = File.join(LISTING, "images", "icon.png") # 512x512 RGBA
    problems = Listing.png_problems(icon, [1024, 500], alpha: false)
    assert_equal 2, problems.size
    assert_match(/expected 1024x500/, problems[0])
    assert_match(/alpha channel/, problems[1])
  end

  IOS = File.expand_path("../../flutter_app/ios/fastlane", __dir__)

  def test_checked_in_app_store_listing_is_valid
    assert_equal [], Listing.ios_problems(File.join(IOS, "metadata"), File.join(IOS, "screenshots"))
  end

  def test_checked_in_locales_use_different_keywords
    sets = %w[en-GB en-US].map do |l|
      File.read(File.join(IOS, "metadata", l, "keywords.txt")).split(",")
    end
    assert_equal [], sets[0] & sets[1]
  end

  def write_locale(dir, **overrides)
    files = { "name.txt" => "Foo Bar", "subtitle.txt" => "Baz Plans", "keywords.txt" => "exercise,diet" }
    files.merge(overrides.transform_keys { |k| "#{k}.txt" }).each { |f, t| File.write(File.join(dir, f), t) }
  end

  def test_keywords_need_no_spaces_and_no_name_or_subtitle_words
    Dir.mktmpdir do |dir|
      write_locale(dir, keywords: "exercise, diet,plans,foo,BAR")
      problems = Listing.keyword_problems(dir)
      assert_equal 2, problems.size
      assert_match(/space after a comma/, problems[0])
      assert_match(/repeats words.*plans, foo, BAR/, problems[1])
    end
  end

  def test_clean_keywords_pass
    Dir.mktmpdir do |dir|
      write_locale(dir)
      assert_equal [], Listing.keyword_problems(dir)
    end
  end

  def test_ios_text_limits_and_urls
    Dir.mktmpdir do |dir|
      write_locale(dir, keywords: "k" * 101, subtitle: "s" * 31, support_url: "http://x.test")
      problems = Listing.ios_locale_problems(dir)
      assert_includes problems, "keywords.txt has 101 characters, the limit is 100"
      assert_includes problems, "subtitle.txt has 31 characters, the limit is 30"
      assert_includes problems, "support_url.txt is not an https URL"
      assert_includes problems, "promotional_text.txt is missing"
    end
  end

  def test_ios_root_files
    Dir.mktmpdir do |dir|
      File.write(File.join(dir, "primary_category.txt"), "games")
      assert_equal ["copyright.txt is missing", "primary_category.txt must be an App Store category id such as PRODUCTIVITY"],
                   Listing.ios_root_problems(dir)
    end
  end

  def test_ios_screenshots_need_both_sizes
    Dir.mktmpdir do |dir|
      problems = Listing.ios_screenshot_problems(dir)
      assert_equal 2, problems.size
      assert_match(/iPhone/, problems[0])
      assert_match(/iPad/, problems[1])
    end
  end
end
