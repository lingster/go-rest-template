# Run: ruby tool/fastlane/host_lock_test.rb
require "minitest/autorun"
require "tmpdir"
require_relative "host_lock"

class HostLockTest < Minitest::Test
  def setup
    @tmp = Dir.mktmpdir("host_lock_test")
    @dir = File.join(@tmp, "keychain.lock")
    @spawned = []
  end

  def teardown
    @spawned.each do |pid|
      Process.kill("KILL", pid)
      Process.wait(pid)
    rescue Errno::ESRCH, Errno::ECHILD
      nil
    end
    FileUtils.rm_rf(@tmp)
  end

  # A live process we can kill, standing in for another build holding the lock.
  def live_pid
    pid = Process.spawn("sleep", "300")
    @spawned << pid
    pid
  end

  def dead_pid
    pid = Process.spawn("true")
    Process.wait(pid)
    pid
  end

  def hold(pid)
    Dir.mkdir(@dir)
    File.write(File.join(@dir, "pid"), pid.to_s)
  end

  def sync(**opts, &block)
    HostLock.synchronize(@dir, poll: 0.02, wait_max: 5, **opts, &block)
  end

  def test_runs_the_block_and_removes_the_lock
    ran = false
    sync { ran = File.directory?(@dir) }
    assert ran, "lock is held inside the block"
    refute File.exist?(@dir), "lock is released after it"
  end

  def test_records_the_pid
    sync { assert_equal Process.pid, HostLock.holder_pid(@dir) }
  end

  def test_returns_the_blocks_value
    assert_equal 7, sync { 7 }
  end

  def test_releases_when_the_block_raises
    assert_raises(RuntimeError) { sync { raise "boom" } }
    refute File.exist?(@dir)
  end

  def test_creates_missing_parent_directories
    @dir = File.join(@tmp, "a", "b", "lock")
    assert_equal :ok, sync { :ok }
  end

  def test_separate_processes_never_overlap
    log = File.join(@tmp, "log")
    pids = Array.new(5) do |i|
      Process.fork do
        HostLock.synchronize(@dir, poll: 0.01, wait_max: 30) do
          File.write(log, "in #{i}\n", mode: "a")
          sleep 0.05
          File.write(log, "out #{i}\n", mode: "a")
        end
        exit!(0)
      end
    end
    pids.each { |pid| Process.wait(pid) }
    lines = File.readlines(log).map(&:split)
    assert_equal 10, lines.size
    lines.each_slice(2) { |enter, leave| assert_equal ["in", leave[1]], enter, "in/out must alternate: #{lines.inspect}" }
    assert_equal %w[in out], lines.first(2).map(&:first)
    refute File.exist?(@dir)
  end

  def test_many_waiters_on_a_stale_lock_never_overlap
    hold(dead_pid)
    log = File.join(@tmp, "log")
    pids = Array.new(6) do |i|
      Process.fork do
        HostLock.synchronize(@dir, poll: 0.01, wait_max: 30) do
          File.write(log, "in #{i}\n", mode: "a")
          sleep 0.03
          File.write(log, "out #{i}\n", mode: "a")
        end
        exit!(0)
      end
    end
    pids.each { |pid| Process.wait(pid) }
    lines = File.readlines(log).map(&:split)
    assert_equal 12, lines.size
    lines.each_slice(2) { |enter, leave| assert_equal ["in", leave[1]], enter, "in/out must alternate: #{lines.inspect}" }
  end

  def test_waits_while_a_live_process_holds_the_lock_then_takes_it
    holder = live_pid
    hold(holder)
    Thread.new do
      sleep 0.3
      Process.kill("KILL", holder) # the holder dies without releasing
      Process.wait(holder) # reaped, as a real parent would: a zombie still answers kill(0)
    end
    messages = []
    waited = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    sync(log: ->(m) { messages << m }) { :got_it }
    assert_operator Process.clock_gettime(Process::CLOCK_MONOTONIC) - waited, :>=, 0.25
    assert(messages.any? { |m| m.include?("waiting for") && m.include?("pid #{holder}") }, messages.inspect)
    assert(messages.any? { |m| m.include?("removing stale lock") }, messages.inspect)
  end

  def test_recovers_a_stale_lock_of_a_dead_process
    dead = dead_pid
    hold(dead)
    messages = []
    sync(log: ->(m) { messages << m }) { assert_equal Process.pid, HostLock.holder_pid(@dir) }
    assert(messages.any? { |m| m.include?("removing stale lock") && m.include?(dead.to_s) }, messages.inspect)
    refute File.exist?(@dir)
    refute File.exist?("#{@dir}.steal"), "steal guard is cleaned up"
  end

  def test_times_out_naming_the_holder
    holder = live_pid
    hold(holder)
    error = assert_raises(HostLock::TimeoutError) { HostLock.synchronize(@dir, poll: 0.02, wait_max: 0.2) { flunk } }
    assert_includes error.message, "pid #{holder}"
    assert_includes error.message, @dir
    assert File.directory?(@dir), "the holder's lock is untouched"
  end

  def test_does_not_release_a_lock_someone_else_holds_now
    other = live_pid
    sync do
      FileUtils.rm_rf(@dir) # e.g. cleared by hand, then taken by another build
      hold(other)
    end
    assert_equal other, HostLock.holder_pid(@dir)
  end

  def test_a_lock_without_a_pid_is_a_holder_still_writing_it_until_the_grace_passes
    Dir.mkdir(@dir)
    refute HostLock.stale?(@dir)
    assert_raises(HostLock::TimeoutError) { HostLock.synchronize(@dir, poll: 0.02, wait_max: 0.2) { flunk } }
    old = Time.now - HostLock::INIT_GRACE - 5
    File.utime(old, old, @dir)
    assert HostLock.stale?(@dir)
    assert_equal :ok, sync { :ok }
  end

  def test_a_garbled_pid_file_is_stale
    Dir.mkdir(@dir)
    File.write(File.join(@dir, "pid"), "garbage")
    assert HostLock.stale?(@dir)
  end

  def test_an_abandoned_steal_guard_is_cleared
    hold(dead_pid)
    guard = "#{@dir}.steal"
    Dir.mkdir(guard)
    old = Time.now - HostLock::INIT_GRACE - 5
    File.utime(old, old, guard)
    assert_equal :ok, sync { :ok }
    refute File.exist?(guard)
  end

  def test_settings_come_from_the_environment
    ENV["CI_HOST_LOCK_POLL"] = "0.5"
    ENV["CI_HOST_LOCK_WAIT_MAX"] = "12"
    assert_equal 0.5, HostLock.default(:CI_HOST_LOCK_POLL, 15)
    assert_equal 12.0, HostLock.default(:CI_HOST_LOCK_WAIT_MAX, 21_600)
  ensure
    ENV.delete("CI_HOST_LOCK_POLL")
    ENV.delete("CI_HOST_LOCK_WAIT_MAX")
    assert_equal 15, HostLock.default(:CI_HOST_LOCK_POLL, 15)
  end
end
